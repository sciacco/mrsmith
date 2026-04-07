package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sciacco/mrsmith/internal/platform/logging"
)

func TestAccessLogCapturesRequestMetadata(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewWithWriter(&buf, "debug")

	handler := RequestID(AccessLog(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := logging.RequestID(r.Context()); got != "req-123" {
			t.Fatalf("expected request id in context, got %q", got)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})))

	req := httptest.NewRequest(http.MethodPost, "/api/demo", nil)
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("User-Agent", "middleware-test")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") != "req-123" {
		t.Fatalf("expected response request id header, got %q", rec.Header().Get("X-Request-ID"))
	}

	entry := decodeMiddlewareLog(t, buf.String())
	if entry["msg"] != "request completed" {
		t.Fatalf("expected request completed log, got %#v", entry["msg"])
	}
	if entry["request_id"] != "req-123" {
		t.Fatalf("expected request_id=req-123, got %#v", entry["request_id"])
	}
	if entry["method"] != http.MethodPost {
		t.Fatalf("expected POST method, got %#v", entry["method"])
	}
	if entry["path"] != "/api/demo" {
		t.Fatalf("expected /api/demo path, got %#v", entry["path"])
	}
	if entry["status"] != float64(http.StatusCreated) {
		t.Fatalf("expected 201 status, got %#v", entry["status"])
	}
}

func TestRecoverReturnsSanitizedJSONAndLogsPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewWithWriter(&buf, "debug")

	handler := Recover(logger)(RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["error"] != "internal_server_error" {
		t.Fatalf("expected sanitized error payload, got %#v", body["error"])
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID header on panic response")
	}

	entry := decodeMiddlewareLog(t, buf.String())
	if entry["msg"] != "panic recovered" {
		t.Fatalf("expected panic recovered log, got %#v", entry["msg"])
	}
	if entry["request_id"] != rec.Header().Get("X-Request-ID") {
		t.Fatalf("expected panic log request id to match response header, got %#v", entry["request_id"])
	}
	if entry["path"] != "/panic" {
		t.Fatalf("expected panic path, got %#v", entry["path"])
	}
}

func decodeMiddlewareLog(t *testing.T, raw string) map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(lines))
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("failed to decode log line: %v", err)
	}
	return entry
}
