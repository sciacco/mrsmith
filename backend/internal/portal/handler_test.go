package portal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
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

func TestHandleListAppsReportsRoleSeesReportsOnly(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, applaunch.Catalog(nil))

	req := httptest.NewRequest(http.MethodGet, "/portal/apps", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
		Name:  "Reports User",
		Email: "reports@example.com",
		Roles: []string{"app_reports_access"},
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
	if body.Categories[0].Apps[0].ID != applaunch.ReportsAppID {
		t.Fatalf("expected reports app, got %q", body.Categories[0].Apps[0].ID)
	}
	if body.Categories[0].Apps[0].Href != applaunch.ReportsAppHref {
		t.Fatalf("expected reports href %q, got %q", applaunch.ReportsAppHref, body.Categories[0].Apps[0].Href)
	}
}

func TestHandleListAppsCopertureRoleSeesCopertureOnly(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, applaunch.Catalog(nil))

	req := httptest.NewRequest(http.MethodGet, "/portal/apps", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
		Name:  "Coperture User",
		Email: "coperture@example.com",
		Roles: []string{"app_coperture_access"},
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
	if body.Categories[0].Apps[0].ID != applaunch.CopertureAppID {
		t.Fatalf("expected coperture app, got %q", body.Categories[0].Apps[0].ID)
	}
}

func TestHandleListAppsEnergiaDCRoleSeesEnergiaDCOnly(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, applaunch.Catalog(nil))

	req := httptest.NewRequest(http.MethodGet, "/portal/apps", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
		Name:  "Energia User",
		Email: "energia@example.com",
		Roles: []string{"app_energiadc_access"},
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
	if body.Categories[0].Apps[0].ID != applaunch.EnergiaDCAppID {
		t.Fatalf("expected energia-dc app, got %q", body.Categories[0].Apps[0].ID)
	}
}

func TestHandleListAppsSimulatoriVenditaRoleSeesSimulatoriVenditaOnly(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, applaunch.Catalog(nil))

	req := httptest.NewRequest(http.MethodGet, "/portal/apps", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
		Name:  "Simulatori User",
		Email: "simulatori@example.com",
		Roles: []string{"app_simulatorivendita_access"},
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
	if body.Categories[0].Apps[0].ID != applaunch.SimulatoriVenditaAppID {
		t.Fatalf("expected simulatori-vendita app, got %q", body.Categories[0].Apps[0].ID)
	}
}

func TestHandleListAppsDevAdminSeesEverything(t *testing.T) {
	mux := http.NewServeMux()
	catalog := applaunch.Catalog(nil)
	RegisterRoutes(mux, catalog)

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

	if got, want := visibleAppIDs(body.Categories), catalogAppIDs(catalog); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected all active app IDs %v for app_devadmin, got %v", want, got)
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

func visibleAppIDs(categories []applaunch.Category) []string {
	ids := make([]string, 0)
	for _, category := range categories {
		for _, app := range category.Apps {
			ids = append(ids, app.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func catalogAppIDs(definitions []applaunch.Definition) []string {
	ids := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		ids = append(ids, definition.ID)
	}
	sort.Strings(ids)
	return ids
}
