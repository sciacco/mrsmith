package portal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
)

func TestHandleListAppsFiltersAppsByRole(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, applaunch.Catalog(nil))

	req := httptest.NewRequest(http.MethodGet, "/portal/apps", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
		Name:  "John Doe",
		Email: "john@example.com",
		Roles: []string{"app_budget_access"},
	}))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body struct {
		Categories []applaunch.Category `json:"categories"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(body.Categories) != 1 || len(body.Categories[0].Apps) != 1 {
		t.Fatalf("expected 1 visible app, got %#v", body.Categories)
	}
	if body.Categories[0].Apps[0].ID != applaunch.BudgetAppID {
		t.Fatalf("expected budget app, got %q", body.Categories[0].Apps[0].ID)
	}
	if body.Categories[0].Apps[0].Href != applaunch.BudgetAppHref {
		t.Fatalf("expected budget href %q, got %q", applaunch.BudgetAppHref, body.Categories[0].Apps[0].Href)
	}
}

func TestHandleListAppsComplianceRoleSeesComplianceOnly(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, applaunch.Catalog(nil))

	req := httptest.NewRequest(http.MethodGet, "/portal/apps", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
		Name:  "Jane Doe",
		Email: "jane@example.com",
		Roles: []string{"app_compliance_access"},
	}))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body struct {
		Categories []applaunch.Category `json:"categories"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(body.Categories) != 1 || len(body.Categories[0].Apps) != 1 {
		t.Fatalf("expected 1 visible app, got %#v", body.Categories)
	}
	if body.Categories[0].Apps[0].ID != applaunch.ComplianceAppID {
		t.Fatalf("expected compliance app, got %q", body.Categories[0].Apps[0].ID)
	}
}

func TestHandleListAppsBothBudgetAndComplianceRoles(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, applaunch.Catalog(nil))

	req := httptest.NewRequest(http.MethodGet, "/portal/apps", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
		Name:  "Admin User",
		Email: "admin@example.com",
		Roles: []string{"app_budget_access", "app_compliance_access"},
	}))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body struct {
		Categories []applaunch.Category `json:"categories"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	total := 0
	for _, cat := range body.Categories {
		total += len(cat.Apps)
	}
	if total != 2 {
		t.Fatalf("expected 2 apps (budget + compliance), got %d", total)
	}
}

func TestHandleListAppsDevAdminSeesEverything(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, applaunch.Catalog(nil))

	req := httptest.NewRequest(http.MethodGet, "/portal/apps", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
		Name:  "Dev Admin",
		Email: "app_devadmin@example.com",
		Roles: []string{"app_devadmin"},
	}))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body struct {
		Categories []applaunch.Category `json:"categories"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	total := 0
	for _, cat := range body.Categories {
		total += len(cat.Apps)
	}
	if total != 20 {
		t.Fatalf("expected 20 apps for app_devadmin, got %d", total)
	}
}

func TestHandleListAppsRequiresClaims(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, applaunch.Catalog(nil))

	req := httptest.NewRequest(http.MethodGet, "/portal/apps", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
