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

	"github.com/sciacco/mrsmith/internal/platform/hubspot"
)

func TestPaymentMethodLabelQueryUsesLoaderSchemaColumns(t *testing.T) {
	if !strings.Contains(paymentMethodLabelQuery, "desc_pagamento") {
		t.Fatalf("paymentMethodLabelQuery missing desc_pagamento: %s", paymentMethodLabelQuery)
	}
	if !strings.Contains(paymentMethodLabelQuery, "cod_pagamento") {
		t.Fatalf("paymentMethodLabelQuery missing cod_pagamento: %s", paymentMethodLabelQuery)
	}
	if strings.Contains(paymentMethodLabelQuery, "descrizione") || strings.Contains(paymentMethodLabelQuery, "codice") {
		t.Fatalf("paymentMethodLabelQuery regressed to stale column names: %s", paymentMethodLabelQuery)
	}
}

func TestHandlePublishRepublishUnlocksLockedHubSpotQuote(t *testing.T) {
	resetPublishHandlerTracker("publish-republish")

	serverState := newHubSpotQuoteServer(t, false)
	defer serverState.server.Close()

	h := &Handler{
		db: openPublishHandlerTestDB(t, "publish-republish"),
		hs: hubspot.NewWithBaseURL("test-token", serverState.server.URL, serverState.server.Client()),
	}

	req := httptest.NewRequest(http.MethodPost, "/quotes/v1/quotes/42/publish", nil)
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	h.handlePublish(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Success bool `json:"success"`
		Steps   []struct {
			Step   int    `json:"step"`
			Name   string `json:"name"`
			Status string `json:"status"`
			Error  string `json:"error,omitempty"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode publish response: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected success response, got %#v", response)
	}

	tracker := publishHandlerTrackerForMode("publish-republish")
	if tracker.dbStatus != "APPROVED" {
		t.Fatalf("db status = %q, want APPROVED", tracker.dbStatus)
	}

	requests := serverState.requests()
	if len(requests) != 4 {
		t.Fatalf("expected 4 HubSpot requests, got %d: %#v", len(requests), requests)
	}
	if requests[0].Method != http.MethodGet || requests[0].Path != "/crm/v3/objects/quotes/555" {
		t.Fatalf("unexpected first request: %#v", requests[0])
	}
	if !strings.Contains(requests[0].RawQuery, "hs_locked") {
		t.Fatalf("quote status request missing hs_locked property: %s", requests[0].RawQuery)
	}
	if got := nestedString(requests[1].Body, "properties", "hs_status"); got != "DRAFT" {
		t.Fatalf("unlock PATCH hs_status = %q, want DRAFT", got)
	}
	if requests[2].Method != http.MethodPut || requests[2].Path != "/crm/v4/objects/quotes/555/associations/default/quote_template/123" {
		t.Fatalf("unexpected template association request: %#v", requests[2])
	}
	if got := nestedString(requests[3].Body, "properties", "hs_status"); got != "APPROVED" {
		t.Fatalf("final PATCH hs_status = %q, want APPROVED", got)
	}
}

func TestHandlePublishReturnsStepErrorWhenUnlockFails(t *testing.T) {
	resetPublishHandlerTracker("publish-republish")

	serverState := newHubSpotQuoteServer(t, true)
	defer serverState.server.Close()

	h := &Handler{
		db: openPublishHandlerTestDB(t, "publish-republish"),
		hs: hubspot.NewWithBaseURL("test-token", serverState.server.URL, serverState.server.Client()),
	}

	req := httptest.NewRequest(http.MethodPost, "/quotes/v1/quotes/42/publish", nil)
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	h.handlePublish(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Success bool `json:"success"`
		Steps   []struct {
			Step   int    `json:"step"`
			Name   string `json:"name"`
			Status string `json:"status"`
			Error  string `json:"error,omitempty"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode publish response: %v", err)
	}
	if response.Success {
		t.Fatalf("expected publish failure, got %#v", response)
	}
	if len(response.Steps) == 0 {
		t.Fatalf("expected failure step, got %#v", response)
	}
	lastStep := response.Steps[len(response.Steps)-1]
	if lastStep.Step != 3 || lastStep.Status != "error" {
		t.Fatalf("expected step 3 error, got %#v", lastStep)
	}
	if !strings.Contains(lastStep.Error, "unlock HubSpot quote") {
		t.Fatalf("expected unlock error message, got %q", lastStep.Error)
	}

	tracker := publishHandlerTrackerForMode("publish-republish")
	if tracker.dbStatus != "" {
		t.Fatalf("unexpected DB status update on unlock failure: %q", tracker.dbStatus)
	}

	requests := serverState.requests()
	if len(requests) != 2 {
		t.Fatalf("expected only GET + unlock PATCH before failure, got %d: %#v", len(requests), requests)
	}
	if got := nestedString(requests[1].Body, "properties", "hs_status"); got != "DRAFT" {
		t.Fatalf("unlock PATCH hs_status = %q, want DRAFT", got)
	}
}

func TestHandleGetHSStatusReturnsLockedFlag(t *testing.T) {
	serverState := newHubSpotQuoteServer(t, false)
	defer serverState.server.Close()

	h := &Handler{
		db: openPublishHandlerTestDB(t, "publish-hs-status"),
		hs: hubspot.NewWithBaseURL("test-token", serverState.server.URL, serverState.server.Client()),
	}

	req := httptest.NewRequest(http.MethodGet, "/quotes/v1/quotes/42/hs-status", nil)
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	h.handleGetHSStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		HSStatus string `json:"hs_status"`
		HSLocked *bool  `json:"hs_locked"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode hs-status response: %v", err)
	}
	if response.HSStatus != "APPROVED" {
		t.Fatalf("hs_status = %q, want APPROVED", response.HSStatus)
	}
	if response.HSLocked == nil || !*response.HSLocked {
		t.Fatalf("hs_locked = %#v, want true", response.HSLocked)
	}
}

func openPublishHandlerTestDB(t *testing.T, mode string) *sql.DB {
	t.Helper()

	registerPublishHandlerTestDriver()

	db, err := sql.Open(publishHandlerTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open publish test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const publishHandlerTestDriverName = "quotes_publish_handler_test_driver"

var (
	registerPublishHandlerDriverOnce sync.Once
	publishHandlerTrackers           sync.Map
)

type publishHandlerTracker struct {
	rowToJSONCalls int
	savedPayload   string
	dbStatus       string
}

func resetPublishHandlerTracker(mode string) {
	publishHandlerTrackers.Store(mode, &publishHandlerTracker{})
}

func publishHandlerTrackerForMode(mode string) *publishHandlerTracker {
	if tracker, ok := publishHandlerTrackers.Load(mode); ok {
		return tracker.(*publishHandlerTracker)
	}
	tracker := &publishHandlerTracker{}
	publishHandlerTrackers.Store(mode, tracker)
	return tracker
}

func registerPublishHandlerTestDriver() {
	registerPublishHandlerDriverOnce.Do(func() {
		sql.Register(publishHandlerTestDriverName, publishHandlerTestDriver{})
	})
}

type publishHandlerTestDriver struct{}

func (publishHandlerTestDriver) Open(name string) (driver.Conn, error) {
	return &publishHandlerTestConn{mode: name}, nil
}

type publishHandlerTestConn struct {
	mode string
}

func (c *publishHandlerTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *publishHandlerTestConn) Close() error { return nil }

func (c *publishHandlerTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *publishHandlerTestConn) Ping(context.Context) error { return nil }

func (c *publishHandlerTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	tracker := publishHandlerTrackerForMode(c.mode)

	switch {
	case strings.Contains(query, `SELECT row_to_json(q) FROM quotes.quote q WHERE q.id = $1`):
		tracker.rowToJSONCalls++
		payload := `{"id":42,"quote_number":"SP-42/2026"}`
		if tracker.rowToJSONCalls > 1 {
			payload = `{"id":42,"status":"APPROVED"}`
		}
		return bytesRows("row_to_json", []byte(payload)), nil
	case strings.Contains(query, `SELECT quotes.upd_quote_head($1::json)`):
		tracker.savedPayload = publishNamedString(args[0])
		return bytesRows("upd_quote_head", []byte(`{"status":"OK","message":""}`)), nil
	case strings.Contains(query, `FROM quotes.v_quote_rows_products vqrp`):
		return intRows("count", 0), nil
	case strings.Contains(query, `FROM quotes.quote WHERE id = $1`) && strings.Contains(query, "quote_number, customer_id, hs_deal_id, hs_quote_id"):
		return &publishHandlerTestRows{
			columns: []string{
				"quote_number", "customer_id", "hs_deal_id", "hs_quote_id", "template", "owner",
				"document_date", "notes", "status", "description",
				"bill_months", "initial_term_months", "next_term_months", "delivered_in_days",
				"nrc_charge_time", "payment_method",
			},
			values: [][]driver.Value{{
				"SP-42/2026", int64(77), int64(88), int64(555), "123", "7",
				"2026-04-11", "", "DRAFT", "",
				int64(3), int64(12), int64(12), int64(30),
				int64(2), "402",
			}},
		}, nil
	case strings.Contains(query, `FROM quotes.template WHERE template_id = $1`):
		return &publishHandlerTestRows{
			columns: []string{"template_type", "is_colo", "lang"},
			values:  [][]driver.Value{{"standard", false, "it"}},
		}, nil
	case strings.Contains(query, paymentMethodLabelQuery):
		return stringRows("desc_pagamento", "Bonifico"), nil
	case strings.Contains(query, `SELECT email FROM loader.hubs_owner WHERE id = $1`):
		return stringRows("email", "owner@example.com"), nil
	case strings.Contains(query, `FROM quotes.quote_rows qr`):
		return &publishHandlerTestRows{
			columns: []string{"id", "kit_id", "internal_name", "nrc_row", "mrc_row", "hs_line_item_id", "hs_line_item_nrc"},
			values:  nil,
		}, nil
	case c.mode == "publish-hs-status" && strings.Contains(query, `SELECT hs_quote_id, status FROM quotes.quote WHERE id = $1`):
		return &publishHandlerTestRows{
			columns: []string{"hs_quote_id", "status"},
			values:  [][]driver.Value{{int64(555), "APPROVED"}},
		}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

func (c *publishHandlerTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	tracker := publishHandlerTrackerForMode(c.mode)
	switch {
	case strings.Contains(query, `UPDATE quotes.quote SET status = $1, date_sent = NOW() WHERE id = $2`):
		tracker.dbStatus = publishNamedString(args[0])
		return driver.RowsAffected(1), nil
	default:
		return nil, errors.New("unexpected exec: " + query)
	}
}

var _ driver.QueryerContext = (*publishHandlerTestConn)(nil)
var _ driver.ExecerContext = (*publishHandlerTestConn)(nil)
var _ driver.Pinger = (*publishHandlerTestConn)(nil)

type publishHandlerTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *publishHandlerTestRows) Columns() []string {
	return r.columns
}

func (r *publishHandlerTestRows) Close() error { return nil }

func (r *publishHandlerTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

type hubSpotRequest struct {
	Method   string
	Path     string
	RawQuery string
	Body     map[string]any
}

type hubSpotQuoteServerState struct {
	server     *httptest.Server
	mu         sync.Mutex
	reqs       []hubSpotRequest
	failUnlock bool
}

func newHubSpotQuoteServer(t *testing.T, failUnlock bool) *hubSpotQuoteServerState {
	t.Helper()

	state := &hubSpotQuoteServerState{failUnlock: failUnlock}
	state.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := hubSpotRequest{
			Method:   r.Method,
			Path:     r.URL.Path,
			RawQuery: r.URL.RawQuery,
		}

		if r.Body != nil {
			defer r.Body.Close()
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			if len(bodyBytes) > 0 {
				if err := json.Unmarshal(bodyBytes, &req.Body); err != nil {
					t.Fatalf("failed to decode request body %q: %v", string(bodyBytes), err)
				}
			}
		}

		state.mu.Lock()
		state.reqs = append(state.reqs, req)
		state.mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/crm/v3/objects/quotes/555":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"properties": map[string]string{
					"hs_status": "APPROVED",
					"hs_locked": "true",
				},
			})
		case r.Method == http.MethodPatch && r.URL.Path == "/crm/v3/objects/quotes/555":
			if nestedString(req.Body, "properties", "hs_status") == "DRAFT" && state.failUnlock {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"status":"error","message":"Published Quote cannot be edited."}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{})
		case r.Method == http.MethodPut && r.URL.Path == "/crm/v4/objects/quotes/555/associations/default/quote_template/123":
			_ = json.NewEncoder(w).Encode(map[string]any{})
		default:
			t.Fatalf("unexpected HubSpot request: %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
	}))

	return state
}

func (s *hubSpotQuoteServerState) requests() []hubSpotRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]hubSpotRequest, len(s.reqs))
	copy(out, s.reqs)
	return out
}

func stringRows(column, value string) *publishHandlerTestRows {
	return &publishHandlerTestRows{
		columns: []string{column},
		values:  [][]driver.Value{{value}},
	}
}

func intRows(column string, value int64) *publishHandlerTestRows {
	return &publishHandlerTestRows{
		columns: []string{column},
		values:  [][]driver.Value{{value}},
	}
}

func bytesRows(column string, value []byte) *publishHandlerTestRows {
	return &publishHandlerTestRows{
		columns: []string{column},
		values:  [][]driver.Value{{value}},
	}
}

func nestedString(body map[string]any, keys ...string) string {
	var current any = body
	for _, key := range keys {
		next, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = next[key]
		if !ok {
			return ""
		}
	}
	if value, ok := current.(string); ok {
		return value
	}
	return ""
}

func publishNamedString(value driver.NamedValue) string {
	switch v := value.Value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}
