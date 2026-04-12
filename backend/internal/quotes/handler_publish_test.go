package quotes

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	if len(requests) != 5 {
		t.Fatalf("expected 5 HubSpot requests, got %d: %#v", len(requests), requests)
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
	if requests[3].Method != http.MethodGet || requests[3].Path != "/crm/v3/objects/quotes/555" || !strings.Contains(requests[3].RawQuery, "associations=line_items") {
		t.Fatalf("unexpected line-item association request: %#v", requests[3])
	}
	if got := nestedString(requests[4].Body, "properties", "hs_status"); got != "APPROVED" {
		t.Fatalf("final PATCH hs_status = %q, want APPROVED", got)
	}
}

func TestHandlePublishSyncsLineItemsDescriptionsAndComments(t *testing.T) {
	resetPublishHandlerTracker("publish-line-items")

	serverState := newHubSpotQuoteServer(t, false)
	serverState.lineItemIDs = []int64{9999}
	defer serverState.server.Close()

	h := &Handler{
		db: openPublishHandlerTestDB(t, "publish-line-items"),
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
		t.Fatalf("expected publish success, got %#v", response)
	}

	tracker := publishHandlerTrackerForMode("publish-line-items")
	if tracker.dbStatus != "APPROVED" {
		t.Fatalf("db status = %q, want APPROVED", tracker.dbStatus)
	}
	if tracker.storedMRC[901] == 0 {
		t.Fatalf("expected hs_line_item_id to be stored for row 901")
	}
	if tracker.storedNRC[901] == 0 {
		t.Fatalf("expected hs_line_item_nrc to be stored for row 901")
	}

	requests := serverState.requests()
	lineItemPosts := make([]hubSpotRequest, 0, 2)
	var finalPatch *hubSpotRequest
	deletedOrphan := false
	for i := range requests {
		req := requests[i]
		if req.Method == http.MethodPost && req.Path == "/crm/v3/objects/line_item" {
			lineItemPosts = append(lineItemPosts, req)
		}
		if req.Method == http.MethodPatch && req.Path == "/crm/v3/objects/quotes/555" &&
			nestedString(req.Body, "properties", "hs_status") == "APPROVED" {
			finalPatch = &requests[i]
		}
		if req.Method == http.MethodDelete && req.Path == "/crm/v3/objects/line_item/9999" {
			deletedOrphan = true
		}
	}

	if len(lineItemPosts) != 2 {
		t.Fatalf("expected 2 line-item POSTs, got %d: %#v", len(lineItemPosts), requests)
	}
	if got := nestedString(lineItemPosts[0].Body, "properties", "name"); got != "A) Kit Alpha IT" {
		t.Fatalf("MRC name = %q, want %q", got, "A) Kit Alpha IT")
	}
	if got := nestedString(lineItemPosts[0].Body, "properties", "description"); !strings.Contains(got, "Dettaglio della soluzione:") {
		t.Fatalf("MRC description missing heading: %q", got)
	}
	if got := nestedFloat(lineItemPosts[0].Body, "properties", "hs_position_on_quote"); got != 0 {
		t.Fatalf("MRC position = %v, want 0", got)
	}
	if got := nestedString(lineItemPosts[1].Body, "properties", "description"); got != "Corrispettivi una tantum" {
		t.Fatalf("NRC description = %q, want %q", got, "Corrispettivi una tantum")
	}
	if got := nestedFloat(lineItemPosts[1].Body, "properties", "hs_position_on_quote"); got != 1 {
		t.Fatalf("NRC position = %v, want 1", got)
	}
	if finalPatch == nil {
		t.Fatalf("expected final quote PATCH with APPROVED status, got %#v", requests)
	}
	if got := nestedString(finalPatch.Body, "properties", "hs_comments"); got != "Trial 30 giorni. <p>Dettaglio proposta</p>" {
		t.Fatalf("hs_comments = %q, want %q", got, "Trial 30 giorni. <p>Dettaglio proposta</p>")
	}
	if !deletedOrphan {
		t.Fatalf("expected orphan line item delete call, got %#v", requests)
	}
}

func TestHandlePublishFailsWhenLineItemSyncFails(t *testing.T) {
	resetPublishHandlerTracker("publish-line-items-fail")

	serverState := newHubSpotQuoteServer(t, false)
	serverState.failLineItemCreate = true
	defer serverState.server.Close()

	h := &Handler{
		db: openPublishHandlerTestDB(t, "publish-line-items-fail"),
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
		t.Fatalf("expected failure steps, got %#v", response)
	}
	lastStep := response.Steps[len(response.Steps)-1]
	if lastStep.Step != 4 || lastStep.Status != "error" {
		t.Fatalf("expected step 4 error, got %#v", lastStep)
	}
	if !strings.Contains(lastStep.Error, "create MRC line item") {
		t.Fatalf("expected line-item create error, got %q", lastStep.Error)
	}

	tracker := publishHandlerTrackerForMode("publish-line-items-fail")
	if tracker.dbStatus != "" {
		t.Fatalf("expected no DB status update on step-4 failure, got %q", tracker.dbStatus)
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
	storedMRC      map[int]int64
	storedNRC      map[int]int64
	clearedNRC     map[int]bool
}

func resetPublishHandlerTracker(mode string) {
	publishHandlerTrackers.Store(mode, &publishHandlerTracker{
		storedMRC:  map[int]int64{},
		storedNRC:  map[int]int64{},
		clearedNRC: map[int]bool{},
	})
}

func publishHandlerTrackerForMode(mode string) *publishHandlerTracker {
	if tracker, ok := publishHandlerTrackers.Load(mode); ok {
		return tracker.(*publishHandlerTracker)
	}
	tracker := &publishHandlerTracker{
		storedMRC:  map[int]int64{},
		storedNRC:  map[int]int64{},
		clearedNRC: map[int]bool{},
	}
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
				"nrc_charge_time", "payment_method", "trial",
			},
			values: quoteHeadRowsForMode(c.mode),
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
	case strings.Contains(query, `FROM quotes.quote_rows qr`) && strings.Contains(query, `v_quote_rows_for_hs`):
		return &publishHandlerTestRows{
			columns: []string{
				"id", "kit_id", "translation_it", "translation_en", "internal_name", "nrc_row", "mrc_row",
				"hs_line_item_id", "hs_line_item_nrc", "descrizione_estesa", "descrizione_estesa_en",
			},
			values: quoteRowsForMode(c.mode),
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
	case strings.Contains(query, `UPDATE quotes.quote_rows SET hs_line_item_id = $1 WHERE id = $2`):
		tracker.storedMRC[publishNamedInt(args[1])] = publishNamedInt64(args[0])
		return driver.RowsAffected(1), nil
	case strings.Contains(query, `UPDATE quotes.quote_rows SET hs_line_item_nrc = $1 WHERE id = $2`):
		tracker.storedNRC[publishNamedInt(args[1])] = publishNamedInt64(args[0])
		return driver.RowsAffected(1), nil
	case strings.Contains(query, `UPDATE quotes.quote_rows SET hs_line_item_nrc = NULL WHERE id = $1`):
		tracker.clearedNRC[publishNamedInt(args[0])] = true
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
	server             *httptest.Server
	mu                 sync.Mutex
	reqs               []hubSpotRequest
	failUnlock         bool
	lineItemIDs        []int64
	nextLineID         int64
	failLineItemCreate bool
}

func newHubSpotQuoteServer(t *testing.T, failUnlock bool) *hubSpotQuoteServerState {
	t.Helper()

	state := &hubSpotQuoteServerState{
		failUnlock:  failUnlock,
		lineItemIDs: []int64{},
		nextLineID:  8000,
	}
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
			if strings.Contains(r.URL.RawQuery, "associations=line_items") {
				state.mu.Lock()
				results := make([]map[string]string, 0, len(state.lineItemIDs))
				for _, id := range state.lineItemIDs {
					results = append(results, map[string]string{"id": fmt.Sprintf("%d", id)})
				}
				state.mu.Unlock()
				_ = json.NewEncoder(w).Encode(map[string]any{
					"associations": map[string]any{
						"line items": map[string]any{
							"results": results,
						},
					},
				})
				return
			}
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
		case r.Method == http.MethodPost && r.URL.Path == "/crm/v3/objects/line_item":
			if state.failLineItemCreate {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"status":"error","message":"line item rejected"}`))
				return
			}
			state.mu.Lock()
			state.nextLineID++
			id := state.nextLineID
			state.lineItemIDs = append(state.lineItemIDs, id)
			state.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"id": fmt.Sprintf("%d", id)})
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/crm/v3/objects/line_item/"):
			_ = json.NewEncoder(w).Encode(map[string]any{})
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/crm/v3/objects/line_item/"):
			id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/crm/v3/objects/line_item/"), 10, 64)
			if err != nil {
				t.Fatalf("invalid line item delete path: %s", r.URL.Path)
			}
			state.mu.Lock()
			filtered := state.lineItemIDs[:0]
			for _, existingID := range state.lineItemIDs {
				if existingID != id {
					filtered = append(filtered, existingID)
				}
			}
			state.lineItemIDs = filtered
			state.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
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

func quoteHeadRowsForMode(mode string) [][]driver.Value {
	description := ""
	trial := ""
	if mode == "publish-line-items" || mode == "publish-line-items-fail" {
		description = "<p>Dettaglio proposta</p>"
		trial = "Trial 30 giorni. "
	}
	return [][]driver.Value{{
		"SP-42/2026", int64(77), int64(88), int64(555), "123", "7",
		"2026-04-11", "", "DRAFT", description,
		int64(3), int64(12), int64(12), int64(30),
		int64(2), "402", trial,
	}}
}

func quoteRowsForMode(mode string) [][]driver.Value {
	if mode == "publish-line-items" || mode == "publish-line-items-fail" {
		return [][]driver.Value{{
			int64(901), int64(321),
			"Kit Alpha IT", "Kit Alpha EN", "KIT_ALPHA",
			float64(49.9), float64(129.5),
			nil, nil,
			"[n. 1] <strong>Prodotto IT</strong> dettaglio",
			"[n. 1] <strong>Product EN</strong> details",
		}}
	}
	return nil
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

func publishNamedInt64(value driver.NamedValue) int64 {
	switch v := value.Value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func publishNamedInt(value driver.NamedValue) int {
	return int(publishNamedInt64(value))
}

func nestedFloat(body map[string]any, keys ...string) float64 {
	var current any = body
	for _, key := range keys {
		next, ok := current.(map[string]any)
		if !ok {
			return -1
		}
		current, ok = next[key]
		if !ok {
			return -1
		}
	}
	switch v := current.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return -1
	}
}
