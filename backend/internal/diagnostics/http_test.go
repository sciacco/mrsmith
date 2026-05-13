package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/authz"
)

type fakeStore struct {
	events       []Event
	inserted     chan struct{}
	lastFilter   ListFilter
	insertErr    error
	listErr      error
	getErr       error
	deleteBefore time.Time
}

func (s *fakeStore) InsertEvents(_ context.Context, events []Event) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	s.events = append(s.events, events...)
	if s.inserted != nil {
		select {
		case s.inserted <- struct{}{}:
		default:
		}
	}
	return nil
}

func (s *fakeStore) ListEvents(_ context.Context, filter ListFilter) ([]Event, error) {
	s.lastFilter = filter
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.events, nil
}

func (s *fakeStore) GetEvent(_ context.Context, id int64) (Event, bool, error) {
	if s.getErr != nil {
		return Event{}, false, s.getErr
	}
	for _, event := range s.events {
		if event.ID == id {
			return event, true, nil
		}
	}
	return Event{}, false, nil
}

func (s *fakeStore) DeleteBefore(_ context.Context, before time.Time) (int64, error) {
	s.deleteBefore = before
	return 0, nil
}

func TestDiagnosticsRoutesRequireDevAdmin(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Store: &fakeStore{}})

	req := httptest.NewRequest(http.MethodGet, "/diagnostics/v1/status", nil)
	req = withClaims(req, auth.Claims{Roles: []string{"app_reports_access"}})
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestHandleListEventsParsesFilters(t *testing.T) {
	store := &fakeStore{events: []Event{{ID: 10, Level: "WARN", Message: "warning"}}}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Store: store})

	req := httptest.NewRequest(http.MethodGet, "/diagnostics/v1/events?level=warn&component=http&operation=list&request_id=req-1&path=/api/test&limit=999", nil)
	req = withClaims(req, auth.Claims{Roles: []string{authz.DevAdminRole}})
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.lastFilter.Level != "WARN" || store.lastFilter.Component != "http" || store.lastFilter.Operation != "list" {
		t.Fatalf("unexpected filter: %#v", store.lastFilter)
	}
	if store.lastFilter.RequestID != "req-1" || store.lastFilter.Path != "/api/test" {
		t.Fatalf("unexpected request/path filter: %#v", store.lastFilter)
	}
	if store.lastFilter.Limit != maxListLimit {
		t.Fatalf("expected capped limit %d, got %d", maxListLimit, store.lastFilter.Limit)
	}

	var body struct {
		Events []Event `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Events) != 1 || body.Events[0].ID != 10 {
		t.Fatalf("unexpected events response: %#v", body.Events)
	}
}

func TestHandleGetEvent(t *testing.T) {
	store := &fakeStore{events: []Event{{ID: 42, Level: "ERROR", Message: "boom"}}}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Store: store})

	req := httptest.NewRequest(http.MethodGet, "/diagnostics/v1/events/42", nil)
	req = withClaims(req, auth.Claims{Roles: []string{authz.DevAdminRole}})
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var event Event
	if err := json.Unmarshal(rec.Body.Bytes(), &event); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if event.ID != 42 {
		t.Fatalf("expected event 42, got %#v", event)
	}
}

func TestHandleListEventsStoreFailureIsSanitized(t *testing.T) {
	store := &fakeStore{listErr: errors.New("database password leaked")}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Store: store})

	req := httptest.NewRequest(http.MethodGet, "/diagnostics/v1/events", nil)
	req = withClaims(req, auth.Claims{Roles: []string{authz.DevAdminRole}})
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if rec.Body.String() != "{\"error\":\"internal_server_error\"}\n" {
		t.Fatalf("expected sanitized response, got %q", rec.Body.String())
	}
}

func withClaims(req *http.Request, claims auth.Claims) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, claims))
}
