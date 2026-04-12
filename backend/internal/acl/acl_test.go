package acl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sciacco/mrsmith/internal/auth"
)

func TestRequireRole(t *testing.T) {
	protected := RequireRole("app_budget_access")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		claims     *auth.Claims
		wantStatus int
	}{
		{
			name:       "missing claims",
			claims:     nil,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong role",
			claims:     &auth.Claims{Roles: []string{"viewer"}},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "required role",
			claims:     &auth.Claims{Roles: []string{"app_budget_access"}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "devadmin bypass",
			claims:     &auth.Claims{Roles: []string{"devadmin"}},
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.claims != nil {
				req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, *tc.claims))
			}
			rec := httptest.NewRecorder()

			protected.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d", tc.wantStatus, rec.Code)
			}
		})
	}
}
