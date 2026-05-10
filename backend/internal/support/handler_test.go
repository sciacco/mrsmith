package support

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/email"
)

func TestDecodeAndSanitizeContextRemovesSensitiveKeys(t *testing.T) {
	raw := json.RawMessage(`{
		"app": {"id": "quotes", "name": "Proposte"},
		"authorization": "Bearer token",
		"headers": {"cookie": "sid=1", "x-request-id": "req-1"},
		"nested": {"refreshToken": "secret", "safe": "kept"},
		"body": {"message": "must not persist"}
	}`)

	value, err := decodeAndSanitizeContext(raw)
	if err != nil {
		t.Fatalf("decodeAndSanitizeContext: %v", err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal sanitized context: %v", err)
	}
	text := string(data)
	for _, forbidden := range []string{"authorization", "Bearer token", "cookie", "refreshToken", "secret", "body", "payload"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("sanitized context still contains %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, "kept") || !strings.Contains(text, "req-1") {
		t.Fatalf("sanitized context dropped safe values: %s", text)
	}
}

func TestHandleCreateRequestPersistsAndSendsNotification(t *testing.T) {
	store := &fakeStore{recipients: []string{"support@example.com"}}
	mailer := &fakeMailer{enabled: true}
	h := &Handler{store: store, mailer: mailer}

	req := authenticatedRequest(`{
		"message": "La pagina non carica i dati",
		"priority": "high",
		"technicalContextIncluded": true,
		"context": {
			"app": {"id": "quotes", "name": "Proposte"},
			"page": {"url": "https://mrsmith.test/apps/quotes", "path": "/quotes"},
			"api": {"recentRequests": [{"path": "/quotes/v1/quotes", "status": 500, "requestId": "req-123"}]},
			"token": "do-not-store"
		}
	}`)
	rec := httptest.NewRecorder()

	h.handleCreateRequest(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.created == nil {
		t.Fatal("expected support request to be persisted")
	}
	if store.created.AppID != "quotes" || store.created.AppName != "Proposte" {
		t.Fatalf("unexpected app context: %#v", store.created)
	}
	if store.emailStatus != emailNotificationSent {
		t.Fatalf("expected email status sent, got %q", store.emailStatus)
	}
	if len(mailer.messages) != 1 {
		t.Fatalf("expected one email, got %d", len(mailer.messages))
	}
	contextBytes, _ := json.Marshal(store.created.Context)
	if strings.Contains(string(contextBytes), "do-not-store") {
		t.Fatalf("sensitive token leaked into stored context: %s", string(contextBytes))
	}
}

func TestHandleCreateRequestSkipsEmailWithoutRecipients(t *testing.T) {
	store := &fakeStore{}
	mailer := &fakeMailer{enabled: true}
	h := &Handler{store: store, mailer: mailer}

	req := authenticatedRequest(`{"message":"Serve aiuto","priority":"normal","context":{"app":{"id":"rda"}}}`)
	rec := httptest.NewRecorder()

	h.handleCreateRequest(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.emailStatus != emailNotificationSkipped {
		t.Fatalf("expected email status skipped, got %q", store.emailStatus)
	}
	if len(mailer.messages) != 0 {
		t.Fatalf("expected no email, got %d", len(mailer.messages))
	}
}

func TestHandleCreateRequestReturns503WithoutStore(t *testing.T) {
	h := &Handler{}
	req := authenticatedRequest(`{"message":"Serve aiuto","priority":"normal"}`)
	rec := httptest.NewRecorder()

	h.handleCreateRequest(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleCreateRequestReturns503WhenSupportSchemaIsNotReady(t *testing.T) {
	h := &Handler{store: &fakeStore{createErr: &pgconn.PgError{Code: "42501", Message: "permission denied for schema mrsmith"}}}
	req := authenticatedRequest(`{"message":"Serve aiuto","priority":"normal","context":{"app":{"id":"rda"}}}`)
	rec := httptest.NewRecorder()

	h.handleCreateRequest(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "support_database_not_ready") {
		t.Fatalf("expected support_database_not_ready, got %s", rec.Body.String())
	}
}

func TestHandleCreateRequestValidatesMessage(t *testing.T) {
	h := &Handler{store: &fakeStore{}}
	req := authenticatedRequest(`{"message":"   ","priority":"normal"}`)
	rec := httptest.NewRecorder()

	h.handleCreateRequest(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func authenticatedRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/support/v1/requests", strings.NewReader(body))
	claims := auth.Claims{
		Subject: "sub-1",
		Email:   "operator@example.com",
		Name:    "operator",
		Roles:   []string{"app_quotes_access"},
	}
	return req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, claims))
}

type fakeStore struct {
	created     *CreateRequestInput
	recipients  []string
	emailStatus string
	createErr   error
}

func (s *fakeStore) CreateRequest(_ context.Context, input CreateRequestInput) (int64, error) {
	if s.createErr != nil {
		return 0, s.createErr
	}
	s.created = &input
	return 42, nil
}

func (s *fakeStore) UpdateEmailStatus(_ context.Context, _ int64, status string, _ auth.Claims) error {
	s.emailStatus = status
	return nil
}

func (s *fakeStore) GetStringListConfig(_ context.Context, _ string, _ string) ([]string, error) {
	return s.recipients, nil
}

type fakeMailer struct {
	enabled  bool
	messages []email.Message
}

func (m *fakeMailer) Enabled() bool {
	return m.enabled
}

func (m *fakeMailer) Send(_ context.Context, msg email.Message) error {
	m.messages = append(m.messages, msg)
	return nil
}
