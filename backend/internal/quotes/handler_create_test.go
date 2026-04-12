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

func TestHandleCreateQuoteRejectsIaaSTemplateMissingKit(t *testing.T) {
	resetCreateHandlerTracker("create-iaas-missing-kit")

	h := &Handler{db: openCreateHandlerTestDB(t, "create-iaas-missing-kit")}

	req := httptest.NewRequest(http.MethodPost, "/quotes/v1/quotes", strings.NewReader(`{
		"template":"tmpl-iaas-missing-kit",
		"owner":"7",
		"kit_ids":[99]
	}`))
	rec := httptest.NewRecorder()

	h.handleCreateQuote(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Error != "iaas_template_missing_kit" {
		t.Fatalf("unexpected error payload: %#v", payload)
	}
}

func TestHandleCreateQuoteRejectsIaaSTemplateKitNotFound(t *testing.T) {
	resetCreateHandlerTracker("create-iaas-kit-unavailable")

	h := &Handler{db: openCreateHandlerTestDB(t, "create-iaas-kit-unavailable")}

	req := httptest.NewRequest(http.MethodPost, "/quotes/v1/quotes", strings.NewReader(`{
		"template":"tmpl-iaas-kit-unavailable",
		"owner":"7",
		"kit_ids":[99]
	}`))
	rec := httptest.NewRecorder()

	h.handleCreateQuote(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Error != "iaas_template_kit_not_found" {
		t.Fatalf("unexpected error payload: %#v", payload)
	}
}

func TestHandleCreateQuoteDerivesIaaSKitAndServicesFromTemplate(t *testing.T) {
	resetCreateHandlerTracker("create-iaas-success")

	h := &Handler{db: openCreateHandlerTestDB(t, "create-iaas-success")}

	req := httptest.NewRequest(http.MethodPost, "/quotes/v1/quotes", strings.NewReader(`{
		"template":"tmpl-iaas-success",
		"owner":"7",
		"document_type":"TSC-ORDINE",
		"services":"999",
		"initial_term_months":12,
		"next_term_months":12,
		"bill_months":6,
		"kit_ids":[777,888]
	}`))
	rec := httptest.NewRecorder()

	h.handleCreateQuote(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	tracker := createHandlerTrackerForMode("create-iaas-success")
	if tracker.procPayload == "" {
		t.Fatal("expected procedure payload to be captured")
	}
	if len(tracker.insertedKitIDs) != 1 || tracker.insertedKitIDs[0] != 62 {
		t.Fatalf("inserted kit ids = %#v, want [62]", tracker.insertedKitIDs)
	}

	var procBody map[string]any
	if err := json.Unmarshal([]byte(tracker.procPayload), &procBody); err != nil {
		t.Fatalf("failed to decode procedure payload: %v", err)
	}
	if got := stringFromAny(procBody["services"]); got != "[12]" {
		t.Fatalf("services = %q, want [12]", got)
	}
	if got := stringFromAny(procBody["document_type"]); got != "TSC-ORDINE-RIC" {
		t.Fatalf("document_type = %q, want TSC-ORDINE-RIC", got)
	}
	if got := intFromAny(procBody["initial_term_months"]); got != 1 {
		t.Fatalf("initial_term_months = %d, want 1", got)
	}
	if got := intFromAny(procBody["next_term_months"]); got != 1 {
		t.Fatalf("next_term_months = %d, want 1", got)
	}
	if got := intFromAny(procBody["bill_months"]); got != 1 {
		t.Fatalf("bill_months = %d, want 1", got)
	}
}

func TestHandleCreateQuoteRejectsMissingServices(t *testing.T) {
	resetCreateHandlerTracker("create-standard-missing-services")

	h := &Handler{db: openCreateHandlerTestDB(t, "create-standard-missing-services")}

	req := httptest.NewRequest(http.MethodPost, "/quotes/v1/quotes", strings.NewReader(`{
		"template":"tmpl-standard",
		"owner":"7",
		"services":""
	}`))
	rec := httptest.NewRecorder()

	h.handleCreateQuote(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Error != "quote_services_required" {
		t.Fatalf("unexpected error payload: %#v", payload)
	}
}

func TestHandleCreateQuoteRejectsIaaSTemplateMissingService(t *testing.T) {
	resetCreateHandlerTracker("create-iaas-missing-service")

	h := &Handler{db: openCreateHandlerTestDB(t, "create-iaas-missing-service")}

	req := httptest.NewRequest(http.MethodPost, "/quotes/v1/quotes", strings.NewReader(`{
		"template":"tmpl-iaas-missing-service",
		"owner":"7",
		"services":"999",
		"kit_ids":[99]
	}`))
	rec := httptest.NewRecorder()

	h.handleCreateQuote(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Error != "quote_services_required" {
		t.Fatalf("unexpected error payload: %#v", payload)
	}
}

func openCreateHandlerTestDB(t *testing.T, mode string) *sql.DB {
	t.Helper()

	registerCreateHandlerTestDriver()

	db, err := sql.Open(createHandlerTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open create test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const createHandlerTestDriverName = "quotes_create_handler_test_driver"

var (
	registerCreateHandlerDriverOnce sync.Once
	createHandlerTrackers           sync.Map
)

type createHandlerTracker struct {
	procPayload    string
	insertedKitIDs []int
}

func resetCreateHandlerTracker(mode string) {
	createHandlerTrackers.Store(mode, &createHandlerTracker{})
}

func createHandlerTrackerForMode(mode string) *createHandlerTracker {
	if tracker, ok := createHandlerTrackers.Load(mode); ok {
		return tracker.(*createHandlerTracker)
	}
	tracker := &createHandlerTracker{}
	createHandlerTrackers.Store(mode, tracker)
	return tracker
}

func registerCreateHandlerTestDriver() {
	registerCreateHandlerDriverOnce.Do(func() {
		sql.Register(createHandlerTestDriverName, createHandlerTestDriver{})
	})
}

type createHandlerTestDriver struct{}

func (createHandlerTestDriver) Open(name string) (driver.Conn, error) {
	return &createHandlerTestConn{mode: name}, nil
}

type createHandlerTestConn struct {
	mode string
}

func (c *createHandlerTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *createHandlerTestConn) Close() error { return nil }

func (c *createHandlerTestConn) Begin() (driver.Tx, error) {
	return &createHandlerTestTx{}, nil
}

func (c *createHandlerTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return &createHandlerTestTx{}, nil
}

func (c *createHandlerTestConn) Ping(context.Context) error { return nil }

func (c *createHandlerTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	tracker := createHandlerTrackerForMode(c.mode)

	switch {
	case strings.Contains(query, `SELECT common.new_document_number('SP-')`):
		return stringRows("new_document_number", "SP-0001/2026"), nil
	case strings.Contains(query, "FROM quotes.template WHERE template_id = $1"):
		switch c.mode {
		case "create-iaas-missing-kit":
			return &createHandlerTestRows{
				columns: []string{"template_type", "kit_id", "service_category_id", "is_colo"},
				values:  [][]driver.Value{{"iaas", nil, int64(12), false}},
			}, nil
		case "create-iaas-kit-unavailable":
			return &createHandlerTestRows{
				columns: []string{"template_type", "kit_id", "service_category_id", "is_colo"},
				values:  [][]driver.Value{{"iaas", int64(999), int64(12), false}},
			}, nil
		case "create-iaas-success":
			return &createHandlerTestRows{
				columns: []string{"template_type", "kit_id", "service_category_id", "is_colo"},
				values:  [][]driver.Value{{"iaas", int64(62), int64(12), false}},
			}, nil
		case "create-iaas-missing-service":
			return &createHandlerTestRows{
				columns: []string{"template_type", "kit_id", "service_category_id", "is_colo"},
				values:  [][]driver.Value{{"iaas", int64(62), nil, false}},
			}, nil
		default:
			return &createHandlerTestRows{columns: []string{"template_type", "kit_id", "service_category_id", "is_colo"}}, nil
		}
	case strings.Contains(query, "FROM products.kit k"):
		switch c.mode {
		case "create-iaas-kit-unavailable":
			return &createHandlerTestRows{
				columns: []string{"exists"},
				values:  [][]driver.Value{{false}},
			}, nil
		case "create-iaas-success":
			return &createHandlerTestRows{
				columns: []string{"exists"},
				values:  [][]driver.Value{{true}},
			}, nil
		case "create-iaas-missing-service":
			return &createHandlerTestRows{
				columns: []string{"exists"},
				values:  [][]driver.Value{{true}},
			}, nil
		default:
			return &createHandlerTestRows{
				columns: []string{"exists"},
				values:  [][]driver.Value{{true}},
			}, nil
		}
	case strings.Contains(query, `SELECT quotes.ins_quote_head($1::json)`):
		tracker.procPayload = createNamedString(args[0])
		return bytesRows("ins_quote_head", []byte(`{"id":101,"status":"OK","message":""}`)), nil
	case strings.Contains(query, `SELECT id, quote_number, status FROM quotes.quote WHERE id = $1`):
		return &createHandlerTestRows{
			columns: []string{"id", "quote_number", "status"},
			values:  [][]driver.Value{{int64(101), "SP-0001/2026", "DRAFT"}},
		}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

func (c *createHandlerTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	tracker := createHandlerTrackerForMode(c.mode)
	switch {
	case strings.Contains(query, `INSERT INTO quotes.quote_rows (quote_id, kit_id, position) VALUES ($1, $2, $3)`):
		tracker.insertedKitIDs = append(tracker.insertedKitIDs, createNamedInt(args[1]))
		return driver.RowsAffected(1), nil
	default:
		return nil, errors.New("unexpected exec: " + query)
	}
}

var _ driver.QueryerContext = (*createHandlerTestConn)(nil)
var _ driver.ExecerContext = (*createHandlerTestConn)(nil)
var _ driver.ConnBeginTx = (*createHandlerTestConn)(nil)
var _ driver.Pinger = (*createHandlerTestConn)(nil)

type createHandlerTestTx struct{}

func (createHandlerTestTx) Commit() error   { return nil }
func (createHandlerTestTx) Rollback() error { return nil }

type createHandlerTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *createHandlerTestRows) Columns() []string {
	return r.columns
}

func (r *createHandlerTestRows) Close() error { return nil }

func (r *createHandlerTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func createNamedString(value driver.NamedValue) string {
	if v, ok := value.Value.(string); ok {
		return v
	}
	return ""
}

func createNamedInt(value driver.NamedValue) int {
	switch v := value.Value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func stringFromAny(value any) string {
	if v, ok := value.(string); ok {
		return v
	}
	return ""
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return 0
	}
}
