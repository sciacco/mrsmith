package panoramica

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── Nil-DB guard tests (call handler methods directly to bypass ACL) ──

func TestRequireMistraReturns503WhenNil(t *testing.T) {
	h := &Handler{}
	rec := httptest.NewRecorder()
	ok := h.requireMistra(rec)
	if ok {
		t.Fatal("expected requireMistra to return false")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "mistra_database_not_configured") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestRequireGrappaReturns503WhenNil(t *testing.T) {
	h := &Handler{}
	rec := httptest.NewRecorder()
	ok := h.requireGrappa(rec)
	if ok {
		t.Fatal("expected requireGrappa to return false")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "grappa_database_not_configured") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestRequireAnisettaReturns503WhenNil(t *testing.T) {
	h := &Handler{}
	rec := httptest.NewRecorder()
	ok := h.requireAnisetta(rec)
	if ok {
		t.Fatal("expected requireAnisetta to return false")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "anisetta_database_not_configured") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

// ── Parameter validation tests (call handlers directly, nil DB returns 503 first) ──

func TestCustomersWithInvoicesNilDB(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest("GET", "/panoramica/v1/customers/with-invoices", nil)
	rec := httptest.NewRecorder()
	h.handleListCustomersWithInvoices(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestOrdersDetailNilDB(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest("GET", "/panoramica/v1/orders/detail", nil)
	rec := httptest.NewRecorder()
	h.handleListOrdersDetail(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestChargesNilDB(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest("GET", "/panoramica/v1/iaas/charges?domain=x&from=2026-01-01&to=2026-01-31&group=monthly", nil)
	rec := httptest.NewRecorder()
	h.handleListCharges(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestChargesByCategoryNilDB(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest("GET", "/panoramica/v1/iaas/charges-by-category?domain=x&from=2026-01-01&to=2026-01-31", nil)
	rec := httptest.NewRecorder()
	h.handleChargesByCategory(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

// ── Charge series parameter validation (handler returns 400 before touching the DB) ──

func TestChargesMissingDomain(t *testing.T) {
	h := &Handler{} // nil Grappa: requireGrappa short-circuits to 503 first
	req := httptest.NewRequest("GET", "/panoramica/v1/iaas/charges?from=2026-01-01&to=2026-01-31", nil)
	rec := httptest.NewRecorder()
	h.handleListCharges(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 (nil DB), got %d", rec.Code)
	}
}

func TestBucketExpressionValidation(t *testing.T) {
	for _, g := range []string{"daily", "weekly", "monthly", "quarterly", "yearly"} {
		expr, ok := bucketExpression(g)
		if !ok {
			t.Errorf("bucketExpression(%q): expected valid", g)
		}
		// Buckets must be plain YYYY-MM-DD strings, never RFC3339 time.Time.
		// The MySQL driver (parseTime) would otherwise serialize DATE columns as
		// e.g. 2025-12-22T00:00:00Z and break date validation/formatting.
		if !strings.Contains(expr, "DATE_FORMAT") {
			t.Errorf("bucketExpression(%q) = %q: expected a DATE_FORMAT-wrapped expression", g, expr)
		}
	}
	if _, ok := bucketExpression("hourly"); ok {
		t.Errorf("bucketExpression(%q): expected invalid", "hourly")
	}
}

func TestCategoryFromUsageType(t *testing.T) {
	cases := map[int64]string{
		2:    "VM",
		6:    "Storage",
		7:    "Storage",
		8:    "Storage",
		9:    "Storage",
		9998: "Licenze Windows",
		1:    "Altro",
		3:    "Altro",
		26:   "Altro",
		27:   "Altro",
	}
	for typ, want := range cases {
		got := categoryFromUsageType(sql.NullInt64{Int64: typ, Valid: true})
		if got != want {
			t.Errorf("categoryFromUsageType(%d): expected %q, got %q", typ, want, got)
		}
	}
	// NULL usage_type lands in Altro
	if got := categoryFromUsageType(sql.NullInt64{Valid: false}); got != "Altro" {
		t.Errorf("categoryFromUsageType(NULL): expected Altro, got %q", got)
	}
}

func TestTimooTenantsNilDB(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest("GET", "/panoramica/v1/timoo/tenants", nil)
	rec := httptest.NewRecorder()
	h.handleListTimooTenants(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestPbxStatsNilDB(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest("GET", "/panoramica/v1/timoo/pbx-stats", nil)
	rec := httptest.NewRecorder()
	h.handleGetPbxStats(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

// ── parseStringList tests ──

func TestParseStringList(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"a,b,c", 3},
		{" a , b , c ", 3},
		{"single", 1},
	}
	for _, tc := range tests {
		result := parseStringList(tc.input)
		if len(result) != tc.expected {
			t.Errorf("parseStringList(%q): expected %d items, got %d", tc.input, tc.expected, len(result))
		}
	}
}
