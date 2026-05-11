package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sciacco/mrsmith/internal/auth"
)

func TestHandlerUsesClaimsEmailForRecipientScopedReads(t *testing.T) {
	store := &fakeHandlerStore{}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Store: store})

	req := authedNotificationsRequest(http.MethodGet, "/notifications/v1/items?status=unread&app_id=rda&limit=7", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	if store.listInput.Email != "user@example.com" || store.listInput.Status != ListStatusUnread || store.listInput.AppID != "rda" || store.listInput.Limit != 7 {
		t.Fatalf("handler did not use claims-scoped list input: %#v", store.listInput)
	}
	var body struct {
		Items      []NotificationItem `json:"items"`
		NextCursor string             `json:"nextCursor"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].ID != 55 {
		t.Fatalf("unexpected list response: %#v", body)
	}
}

func TestHandlerMutatesOnlyClaimsRecipientID(t *testing.T) {
	store := &fakeHandlerStore{markReadOK: true}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Store: store})

	req := authedNotificationsRequest(http.MethodPost, "/notifications/v1/items/55/read", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", res.Code, res.Body.String())
	}
	if store.markReadEmail != "user@example.com" || store.markReadID != 55 {
		t.Fatalf("unexpected mark-read target: email=%q id=%d", store.markReadEmail, store.markReadID)
	}
}

func TestHandlerRequiresStoreAndClaimsEmail(t *testing.T) {
	t.Run("store missing returns 503", func(t *testing.T) {
		mux := http.NewServeMux()
		RegisterRoutes(mux, Deps{})
		req := authedNotificationsRequest(http.MethodGet, "/notifications/v1/summary", nil)
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		if res.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", res.Code)
		}
	})
	t.Run("email missing returns 401", func(t *testing.T) {
		mux := http.NewServeMux()
		RegisterRoutes(mux, Deps{Store: &fakeHandlerStore{}})
		req := httptest.NewRequest(http.MethodGet, "/notifications/v1/summary", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{Subject: "u1"}))
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", res.Code)
		}
	})
}

type fakeHandlerStore struct {
	Store
	listInput     ListInput
	markReadEmail string
	markReadID    int64
	markReadOK    bool
}

func (s *fakeHandlerStore) Summary(context.Context, string) (Summary, error) {
	return Summary{TotalUnread: 1, UnreadByApp: map[string]int64{"rda": 1}}, nil
}

func (s *fakeHandlerStore) List(_ context.Context, input ListInput) (ListResult, error) {
	s.listInput = input
	return ListResult{
		Items: []NotificationItem{{
			ID:        55,
			AppID:     "rda",
			Title:     "Approval",
			CreatedAt: time.Date(2026, 5, 10, 8, 0, 0, 0, time.UTC),
			Metadata:  json.RawMessage(`{}`),
		}},
	}, nil
}

func (s *fakeHandlerStore) MarkRead(_ context.Context, email string, id int64) (bool, error) {
	s.markReadEmail = email
	s.markReadID = id
	return s.markReadOK, nil
}

func (s *fakeHandlerStore) MarkAllRead(context.Context, string) (int64, error) {
	return 1, nil
}

func (s *fakeHandlerStore) Archive(context.Context, string, int64) (bool, error) {
	return true, nil
}

func authedNotificationsRequest(method, target string, body any) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	claims := auth.Claims{Subject: "u1", Email: "USER@example.com", Name: "User", RawToken: "token"}
	return req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, claims))
}
