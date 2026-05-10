package support

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
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
	if len(store.created.Attachments) != 0 {
		t.Fatalf("expected no attachments, got %d", len(store.created.Attachments))
	}
	contextBytes, _ := json.Marshal(store.created.Context)
	if strings.Contains(string(contextBytes), "do-not-store") {
		t.Fatalf("sensitive token leaked into stored context: %s", string(contextBytes))
	}
}

func TestHandleCreateRequestPersistsMultipartAttachmentsAndSendsNotification(t *testing.T) {
	store := &fakeStore{recipients: []string{"support@example.com"}}
	mailer := &fakeMailer{enabled: true}
	h := &Handler{store: store, mailer: mailer}

	payload := `{
		"message": "La pagina mostra un errore",
		"priority": "high",
		"technicalContextIncluded": true,
		"context": {
			"app": {"id": "quotes", "name": "Proposte"},
			"page": {"path": "/quotes"}
		}
	}`
	req := authenticatedMultipartRequest(t, payload, []testUpload{
		{name: "screen.png", contentType: "image/png", content: "\x89PNG\r\n\x1a\nimage"},
		{name: "notes.txt", contentType: "text/plain", content: "steps to reproduce"},
	})
	rec := httptest.NewRecorder()

	h.handleCreateRequest(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.created == nil {
		t.Fatal("expected support request to be persisted")
	}
	if len(store.created.Attachments) != 2 {
		t.Fatalf("expected two attachments, got %d", len(store.created.Attachments))
	}
	if got := store.created.Attachments[0]; got.Filename != "screen.png" || got.ContentType != "image/png" || got.SizeBytes == 0 || got.ContentSHA256 == "" {
		t.Fatalf("unexpected first attachment: %#v", got)
	}
	if string(store.created.Attachments[1].Content) != "steps to reproduce" {
		t.Fatalf("unexpected second attachment content: %q", string(store.created.Attachments[1].Content))
	}
	if len(mailer.messages) != 1 {
		t.Fatalf("expected one email, got %d", len(mailer.messages))
	}
	if len(mailer.messages[0].Attachments) != 3 {
		t.Fatalf("expected context plus two email attachments, got %d", len(mailer.messages[0].Attachments))
	}
}

func TestHandleCreateRequestRejectsUnsupportedAttachmentType(t *testing.T) {
	store := &fakeStore{recipients: []string{"support@example.com"}}
	mailer := &fakeMailer{enabled: true}
	h := &Handler{store: store, mailer: mailer}

	req := authenticatedMultipartRequest(t, `{"message":"Serve aiuto","priority":"normal","context":{"app":{"id":"rda"}}}`, []testUpload{
		{name: "script.exe", contentType: "application/x-msdownload", content: "\x00\x01\x02binary"},
	})
	rec := httptest.NewRecorder()

	h.handleCreateRequest(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unsupported_attachment_type") {
		t.Fatalf("expected unsupported_attachment_type, got %s", rec.Body.String())
	}
	if store.created != nil {
		t.Fatal("request should not be persisted when attachment validation fails")
	}
	if len(mailer.messages) != 0 {
		t.Fatalf("expected no email, got %d", len(mailer.messages))
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

type testUpload struct {
	name        string
	contentType string
	content     string
}

func authenticatedMultipartRequest(t *testing.T, payload string, uploads []testUpload) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("payload", payload); err != nil {
		t.Fatalf("write payload field: %v", err)
	}
	for _, upload := range uploads {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="attachments"; filename="`+upload.name+`"`)
		header.Set("Content-Type", upload.contentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatalf("create attachment part: %v", err)
		}
		if _, err := part.Write([]byte(upload.content)); err != nil {
			t.Fatalf("write attachment content: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := authenticatedRequest(body.String())
	req.Body = io.NopCloser(bytes.NewReader(body.Bytes()))
	req.ContentLength = int64(body.Len())
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
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
