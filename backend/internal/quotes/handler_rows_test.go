package quotes

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestHandleCustomerOrdersUsesResolvedERPCustomerID(t *testing.T) {
	resetQuotesHandlerTracker("customer-orders-mistra")
	resetQuotesHandlerTracker("customer-orders-alyante")

	h := &Handler{
		db:        openQuotesHandlerTestDB(t, "customer-orders-mistra"),
		alyanteDB: openQuotesHandlerTestDB(t, "customer-orders-alyante"),
	}

	req := httptest.NewRequest(http.MethodGet, "/quotes/v1/customer-orders/12345", nil)
	req.SetPathValue("customerId", "12345")
	rec := httptest.NewRecorder()

	h.handleCustomerOrders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 2 || got[0].Name != "ORD-200" || got[1].Name != "ORD-100" {
		t.Fatalf("unexpected orders payload: %#v", got)
	}

	if tracker := quotesHandlerTrackerForMode("customer-orders-mistra"); tracker.bridgeCustomerID != "12345" {
		t.Fatalf("bridge lookup used customer id %q, want %q", tracker.bridgeCustomerID, "12345")
	}
	alyanteTracker := quotesHandlerTrackerForMode("customer-orders-alyante")
	if alyanteTracker.alyanteCustomerID != "10642803691" {
		t.Fatalf("alyante lookup used ERP id %q, want %q", alyanteTracker.alyanteCustomerID, "10642803691")
	}
	if !strings.Contains(alyanteTracker.lastQuery, "ID_CLIENTE") {
		t.Fatalf("customer orders query did not use ID_CLIENTE: %s", alyanteTracker.lastQuery)
	}
}

func TestHandleUpdateProductAcceptsBooleanProcResult(t *testing.T) {
	resetQuotesHandlerTracker("update-product-success")

	h := &Handler{db: openQuotesHandlerTestDB(t, "update-product-success")}

	req := httptest.NewRequest(http.MethodPut, "/quotes/v1/quotes/1372/rows/3678/products/40240", strings.NewReader(`{"included":true,"quantity":0,"mrc":99,"extended_description":"notes"}`))
	req.SetPathValue("id", "1372")
	req.SetPathValue("rowId", "3678")
	req.SetPathValue("productId", "40240")
	rec := httptest.NewRecorder()

	h.handleUpdateProduct(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.OK {
		t.Fatalf("expected ok response, got %#v", response)
	}

	tracker := quotesHandlerTrackerForMode("update-product-success")
	if tracker.procPayload == "" {
		t.Fatal("expected procedure payload to be captured")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(tracker.procPayload), &payload); err != nil {
		t.Fatalf("failed to decode procedure payload: %v", err)
	}
	if payload["id"] != float64(40240) {
		t.Fatalf("payload id = %#v, want %d", payload["id"], 40240)
	}
	if payload["mrc"] != float64(0) {
		t.Fatalf("payload mrc = %#v, want 0 for spot quote", payload["mrc"])
	}
	if payload["quantity"] != float64(1) {
		t.Fatalf("payload quantity = %#v, want 1 for included zero-quantity product", payload["quantity"])
	}
	if payload["included"] != true {
		t.Fatalf("payload included = %#v, want true", payload["included"])
	}
}

func openQuotesHandlerTestDB(t *testing.T, mode string) *sql.DB {
	t.Helper()

	registerQuotesHandlerTestDriver()

	db, err := sql.Open(quotesHandlerTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const quotesHandlerTestDriverName = "quotes_handler_test_driver"

var (
	registerQuotesHandlerDriverOnce sync.Once
	quotesHandlerTrackers           sync.Map
)

type quotesHandlerTracker struct {
	bridgeCustomerID  string
	alyanteCustomerID string
	procPayload       string
	lastQuery         string
}

func resetQuotesHandlerTracker(mode string) {
	quotesHandlerTrackers.Store(mode, &quotesHandlerTracker{})
}

func quotesHandlerTrackerForMode(mode string) *quotesHandlerTracker {
	if tracker, ok := quotesHandlerTrackers.Load(mode); ok {
		return tracker.(*quotesHandlerTracker)
	}
	tracker := &quotesHandlerTracker{}
	quotesHandlerTrackers.Store(mode, tracker)
	return tracker
}

func registerQuotesHandlerTestDriver() {
	registerQuotesHandlerDriverOnce.Do(func() {
		sql.Register(quotesHandlerTestDriverName, quotesHandlerTestDriver{})
	})
}

type quotesHandlerTestDriver struct{}

func (quotesHandlerTestDriver) Open(name string) (driver.Conn, error) {
	return &quotesHandlerTestConn{mode: name}, nil
}

type quotesHandlerTestConn struct {
	mode string
}

func (c *quotesHandlerTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *quotesHandlerTestConn) Close() error { return nil }

func (c *quotesHandlerTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *quotesHandlerTestConn) Ping(context.Context) error { return nil }

func (c *quotesHandlerTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	tracker := quotesHandlerTrackerForMode(c.mode)
	tracker.lastQuery = query

	switch {
	case c.mode == "customer-orders-mistra" && strings.Contains(query, "SELECT numero_azienda FROM loader.hubs_company"):
		tracker.bridgeCustomerID = namedString(args[0])
		return &quotesHandlerTestRows{
			columns: []string{"numero_azienda"},
			values:  [][]driver.Value{{"10642803691"}},
		}, nil
	case c.mode == "customer-orders-alyante" && strings.Contains(query, "FROM Tsmi_Ordini"):
		tracker.alyanteCustomerID = namedString(args[0])
		return &quotesHandlerTestRows{
			columns: []string{"order_name"},
			values:  [][]driver.Value{{"ORD-200"}, {"ORD-100"}},
		}, nil
	case c.mode == "update-product-success" && strings.Contains(query, "SELECT qr.quote_id FROM quotes.quote_rows_products qrp"):
		return &quotesHandlerTestRows{
			columns: []string{"quote_id"},
			values:  [][]driver.Value{{int64(1372)}},
		}, nil
	case c.mode == "update-product-success" && strings.Contains(query, "SELECT COALESCE(q.document_type, '') FROM quotes.quote q"):
		return &quotesHandlerTestRows{
			columns: []string{"document_type"},
			values:  [][]driver.Value{{"TSC-ORDINE"}},
		}, nil
	case c.mode == "update-product-success" && strings.Contains(query, "SELECT quotes.upd_quote_row_product($1::json)"):
		tracker.procPayload = namedString(args[0])
		return &quotesHandlerTestRows{
			columns: []string{"upd_quote_row_product"},
			values:  [][]driver.Value{{true}},
		}, nil
	default:
		return &quotesHandlerTestRows{columns: []string{"stub"}}, nil
	}
}

var _ driver.QueryerContext = (*quotesHandlerTestConn)(nil)
var _ driver.Pinger = (*quotesHandlerTestConn)(nil)

type quotesHandlerTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *quotesHandlerTestRows) Columns() []string {
	return r.columns
}

func (r *quotesHandlerTestRows) Close() error { return nil }

func (r *quotesHandlerTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func namedString(value driver.NamedValue) string {
	if v, ok := value.Value.(string); ok {
		return v
	}
	return ""
}
