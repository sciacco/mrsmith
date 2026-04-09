package panoramica

import (
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

func TestOrdersSummaryNilDB(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest("GET", "/panoramica/v1/orders/summary", nil)
	rec := httptest.NewRecorder()
	h.handleListOrdersSummary(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestDailyChargesNilDB(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest("GET", "/panoramica/v1/iaas/daily-charges", nil)
	rec := httptest.NewRecorder()
	h.handleListDailyCharges(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestChargeBreakdownNilDB(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest("GET", "/panoramica/v1/iaas/charge-breakdown", nil)
	rec := httptest.NewRecorder()
	h.handleChargeBreakdown(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
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
