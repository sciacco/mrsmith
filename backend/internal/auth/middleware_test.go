package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/pkg/middleware"
)

func TestMiddlewareMarksMissingBearerAsRoutineAuthFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewWithWriter(&buf, "debug")
	m := &Middleware{verifier: &oidc.IDTokenVerifier{}}

	handler := middleware.RequestID(middleware.AccessLog(logger)(m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called without bearer auth")
	}))))

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.Header.Set("X-Request-ID", "req-missing-bearer")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	entries := decodeAuthMiddlewareLogs(t, buf.String())
	var accessLog map[string]any
	for _, entry := range entries {
		if entry["level"] == "WARN" || entry["level"] == "ERROR" {
			t.Fatalf("routine missing-bearer auth failure should not warn/error: %#v", entry)
		}
		if entry["msg"] == "request completed with auth failure" {
			accessLog = entry
		}
	}
	if accessLog == nil {
		t.Fatalf("expected final auth-failure access log, got %#v", entries)
	}
	if accessLog["level"] != "INFO" {
		t.Fatalf("expected INFO access log, got %#v", accessLog["level"])
	}
	if accessLog["auth_failure_reason"] != "missing_bearer" {
		t.Fatalf("expected missing_bearer reason, got %#v", accessLog["auth_failure_reason"])
	}
}

func decodeAuthMiddlewareLogs(t *testing.T, raw string) []map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 0 {
		t.Fatal("expected log lines")
	}

	entries := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("failed to decode log line %q: %v", line, err)
		}
		entries = append(entries, entry)
	}
	return entries
}
