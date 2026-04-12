package budget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/arak"
)

func TestBudgetRoutesRequireRole(t *testing.T) {
	t.Cleanup(func() {
		arakClient = nil
	})

	mux := http.NewServeMux()
	RegisterRoutes(mux)

	tests := []struct {
		name       string
		roles      []string
		wantStatus int
	}{
		{name: "missing claims", roles: nil, wantStatus: http.StatusUnauthorized},
		{name: "wrong role", roles: []string{"viewer"}, wantStatus: http.StatusForbidden},
		{name: "budget role", roles: []string{"app_budget_access"}, wantStatus: http.StatusOK},
		{name: "devadmin role", roles: []string{"devadmin"}, wantStatus: http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/budget/v1/budget?page_number=1", nil)
			if tc.roles != nil {
				req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
					Name:  "John Doe",
					Email: "john@example.com",
					Roles: tc.roles,
				}))
			}

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d", tc.wantStatus, rec.Code)
			}
		})
	}
}

func TestBudgetProxyTranslatesUpstreamAuthFailures(t *testing.T) {
	t.Cleanup(func() {
		arakClient = nil
	})

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "service-token",
				"expires_in":   300,
			})
		case "/arak/budget/v1/report/unassigned-users":
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	mux := http.NewServeMux()
	RegisterRoutes(mux, arak.New(arak.Config{
		BaseURL:      upstream.URL,
		TokenURL:     upstream.URL + "/token",
		ClientID:     "budget-client",
		ClientSecret: "budget-secret",
	}))

	req := httptest.NewRequest(
		http.MethodGet,
		"/budget/v1/report/unassigned-users?enabled=true&page_number=1&disable_pagination=true",
		nil,
	).WithContext(context.WithValue(context.Background(), auth.ClaimsKey, auth.Claims{
		Name:  "John Doe",
		Email: "john@example.com",
		Roles: []string{"app_budget_access"},
	}))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for upstream auth failure, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON error response, got decode error: %v", err)
	}
	if body["code"] != upstreamAuthFailedCode {
		t.Fatalf("expected code %q, got %q", upstreamAuthFailedCode, body["code"])
	}
	if body["error"] != "upstream authorization failed" {
		t.Fatalf("expected translated error message, got %q", body["error"])
	}
}
