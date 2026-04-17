package staticspa

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlerServesConcreteFiles(t *testing.T) {
	root := buildStaticFixture(t)
	handler := New(root)

	req := httptest.NewRequest(http.MethodGet, "/apps/budget/assets/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "budget-asset" {
		t.Fatalf("expected asset body, got %q", body)
	}
}

func TestHandlerFallsBackToBudgetIndexForDeepLinks(t *testing.T) {
	root := buildStaticFixture(t)
	handler := New(root)

	req := httptest.NewRequest(http.MethodGet, "/apps/budget/budgets/12", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "budget-shell") {
		t.Fatalf("expected budget index, got %q", body)
	}
}

func TestHandlerFallsBackToPortalIndexForRootRoutes(t *testing.T) {
	root := buildStaticFixture(t)
	handler := New(root)

	req := httptest.NewRequest(http.MethodGet, "/launcher", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "portal-shell") {
		t.Fatalf("expected portal index, got %q", body)
	}
}

func TestHandlerDoesNotFallbackMissingAssetRequests(t *testing.T) {
	root := buildStaticFixture(t)
	handler := New(root)

	req := httptest.NewRequest(http.MethodGet, "/apps/budget/assets/missing.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandlerFallsBackToComplianceIndexForDeepLinks(t *testing.T) {
	root := buildStaticFixture(t)
	handler := New(root)

	req := httptest.NewRequest(http.MethodGet, "/apps/compliance/domains/123", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "compliance-shell") {
		t.Fatalf("expected compliance index, got %q", body)
	}
}

func TestHandlerFallsBackToReportsIndexForDeepLinks(t *testing.T) {
	root := buildStaticFixture(t)
	handler := New(root)

	req := httptest.NewRequest(http.MethodGet, "/apps/reports/orders/preview", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "reports-shell") {
		t.Fatalf("expected reports index, got %q", body)
	}
}

func TestHandlerFallsBackToCopertureIndexForDeepLinks(t *testing.T) {
	root := buildStaticFixture(t)
	handler := New(root)

	req := httptest.NewRequest(http.MethodGet, "/apps/coperture/coperture", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "coperture-shell") {
		t.Fatalf("expected coperture index, got %q", body)
	}
}

func buildStaticFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "index.html"), "<html>portal-shell</html>")
	writeFixtureFile(t, filepath.Join(root, "apps", "budget", "index.html"), "<html>budget-shell</html>")
	writeFixtureFile(t, filepath.Join(root, "apps", "budget", "assets", "app.js"), "budget-asset")
	writeFixtureFile(t, filepath.Join(root, "apps", "compliance", "index.html"), "<html>compliance-shell</html>")
	writeFixtureFile(t, filepath.Join(root, "apps", "coperture", "index.html"), "<html>coperture-shell</html>")
	writeFixtureFile(t, filepath.Join(root, "apps", "reports", "index.html"), "<html>reports-shell</html>")
	return root
}

func writeFixtureFile(t *testing.T, filename string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatalf("failed to create fixture dir: %v", err)
	}
	if err := os.WriteFile(filename, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}
}
