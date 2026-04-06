package budget

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sciacco/mrsmith/internal/auth"
)

func TestBudgetRoutesRequireRole(t *testing.T) {
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
