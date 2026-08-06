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
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/sciacco/mrsmith/internal/auth"
)

func TestDuplicateQuoteProductInsertHasMatchingColumnsAndPlaceholders(t *testing.T) {
	if err := validateCloneProductInsertSQL(cloneProductInsertSQL, 13); err != nil {
		t.Fatal(err)
	}
}

func TestHandleDuplicateQuoteCopiesCommercialConfigurationForOperator(t *testing.T) {
	state := openCloneTestDB(t, "clone-success")
	h := &Handler{db: state.db}

	req := cloneRequest("/quotes/v1/quotes/42/duplicate", "operator@example.com")
	rec := httptest.NewRecorder()
	h.handleDuplicateQuote(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		ID          int    `json:"id"`
		QuoteNumber string `json:"quote_number"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID != 900 || response.QuoteNumber != "SP-9000/2026" || response.Status != "DRAFT" {
		t.Fatalf("unexpected response: %#v", response)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(state.procPayload), &payload); err != nil {
		t.Fatalf("decode procedure payload: %v", err)
	}
	if payload["owner"] != "77" || payload["document_date"] != "2026-02-03" || payload["status"] != "DRAFT" {
		t.Fatalf("operator/date/status not applied: %#v", payload)
	}
	if payload["customer_id"] != float64(123) || payload["hs_deal_id"] != float64(456) || payload["deal_number"] != "D-42" {
		t.Fatalf("commercial references not preserved: %#v", payload)
	}
	for _, field := range []string{"hs_quote_id", "notes", "replace_orders", "date_sent", "hs_esign_contacts", "hs_sign_status", "hs_esign_date"} {
		if value, ok := payload[field]; !ok || value != nil {
			t.Fatalf("%s = %#v, want explicit null", field, value)
		}
	}
	if strings.Contains(state.sourceQuery, "ARCHIVED") {
		t.Fatal("duplicate source load introduced an unsupported ARCHIVED visibility filter")
	}
	if state.beginOptions.Isolation != driver.IsolationLevel(sql.LevelRepeatableRead) {
		t.Fatalf("clone transaction isolation = %v, want repeatable read", state.beginOptions.Isolation)
	}
	if !state.committed {
		t.Fatal("expected clone transaction to commit")
	}

	wantRows := []cloneDestinationRow{
		{ID: 1001, KitID: 20, Position: int64(2), InternalName: "Source two", BundlePrefixRow: "B-2"},
		{ID: 1002, KitID: 10, Position: int64(4), InternalName: "Source one", BundlePrefixRow: "B-1"},
	}
	if !reflect.DeepEqual(state.committedRows, wantRows) {
		t.Fatalf("kit rows were not copied with exact order, positions, and labels: %#v", state.committedRows)
	}
	wantProducts := [][]driver.Value{
		{int64(1001), "P-TWO", int64(0), int64(-1), false, "0.00000", "8.12500", int64(1), "Other", false, "0", "Other description", false},
		{int64(1002), "P-ONE", int64(1), int64(4), true, "10.25000", "2.50000", int64(7), "Group", true, "3", "Exact description", true},
	}
	if !reflect.DeepEqual(state.committedProducts, wantProducts) {
		t.Fatalf("product configuration was not copied exactly for every source product: %#v", state.committedProducts)
	}
	if !reflect.DeepEqual(state.committedGeneratedProductDeletes, []int64{1001, 1002}) {
		t.Fatalf("trigger-generated products were not deleted for each destination row: %#v", state.committedGeneratedProductDeletes)
	}
	if state.committedHeader == nil || !state.committedHeader.cleared() {
		t.Fatalf("final notes and excluded header state were not cleared: %#v", state.committedHeader)
	}
}

func TestHandleDuplicateQuoteDoesNotCreateWithoutActiveOwnerMapping(t *testing.T) {
	state := openCloneTestDB(t, "clone-no-owner")
	h := &Handler{db: state.db}

	req := cloneRequest("/quotes/v1/quotes/42/duplicate", "missing@example.com")
	rec := httptest.NewRecorder()
	h.handleDuplicateQuote(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if state.numberAllocated || state.headInserted || state.committed || len(state.committedRows) != 0 {
		t.Fatalf("owner failure created clone state: %#v", state)
	}
}

func TestHandleDuplicateQuoteRollsBackOnDestinationWriteFailure(t *testing.T) {
	for _, mode := range []string{"clone-product-failure", "clone-row-insert-failure", "clone-row-restore-failure"} {
		t.Run(mode, func(t *testing.T) {
			state := openCloneTestDB(t, mode)
			h := &Handler{db: state.db}

			req := cloneRequest("/quotes/v1/quotes/42/duplicate", "operator@example.com")
			rec := httptest.NewRecorder()
			h.handleDuplicateQuote(rec, req)

			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
			}
			if !state.rolledBack || state.committed {
				t.Fatalf("expected transaction rollback, got committed=%t rolled_back=%t", state.committed, state.rolledBack)
			}
			if state.committedHeader != nil || len(state.committedRows) != 0 || len(state.committedProducts) != 0 {
				t.Fatalf("rollback left committed destination state: %#v", state)
			}
			if state.pendingHeader != nil || len(state.pendingRows) != 0 || len(state.pendingProducts) != 0 {
				t.Fatalf("rollback did not discard pending destination state: %#v", state)
			}
		})
	}
}

func cloneRequest(path, email string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.SetPathValue("id", "42")
	return req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{Email: email}))
}

type cloneDestinationRow struct {
	ID              int
	KitID           int64
	Position        driver.Value
	InternalName    driver.Value
	BundlePrefixRow driver.Value
	HsLineItemID    driver.Value
	HsLineItemNRC   driver.Value
}

type cloneDestinationHeader struct {
	notes, replaceOrders, dateSent, hsQuoteID, hsEsignEnabled, hsEsignContacts, hsSignStatus, hsEsignDate any
}

func (h *cloneDestinationHeader) cleared() bool {
	return h.notes == nil && h.replaceOrders == nil && h.dateSent == nil && h.hsQuoteID == nil &&
		h.hsEsignEnabled == nil && h.hsEsignContacts == nil && h.hsSignStatus == nil && h.hsEsignDate == nil
}

type cloneTestState struct {
	db                               *sql.DB
	procPayload                      string
	numberAllocated                  bool
	headInserted                     bool
	committed                        bool
	rolledBack                       bool
	beginOptions                     driver.TxOptions
	sourceQuery                      string
	pendingHeader                    *cloneDestinationHeader
	committedHeader                  *cloneDestinationHeader
	pendingRows                      []cloneDestinationRow
	committedRows                    []cloneDestinationRow
	pendingProducts                  [][]driver.Value
	committedProducts                [][]driver.Value
	pendingGeneratedProductDeletes   []int64
	committedGeneratedProductDeletes []int64
}

var (
	cloneTestRegister sync.Once
	cloneTestStates   sync.Map
)

func openCloneTestDB(t *testing.T, mode string) *cloneTestState {
	t.Helper()
	cloneTestRegister.Do(func() { sql.Register("quotes_clone_handler_test_driver", cloneTestDriver{}) })
	state := &cloneTestState{}
	cloneTestStates.Store(mode, state)
	db, err := sql.Open("quotes_clone_handler_test_driver", mode)
	if err != nil {
		t.Fatalf("open clone test db: %v", err)
	}
	state.db = db
	t.Cleanup(func() {
		_ = db.Close()
		cloneTestStates.Delete(mode)
	})
	return state
}

type cloneTestDriver struct{}

func (cloneTestDriver) Open(name string) (driver.Conn, error) {
	value, ok := cloneTestStates.Load(name)
	if !ok {
		return nil, errors.New("unknown clone test mode")
	}
	return &cloneTestConn{mode: name, state: value.(*cloneTestState)}, nil
}

type cloneTestConn struct {
	mode  string
	state *cloneTestState
}

func (c *cloneTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (c *cloneTestConn) Close() error { return nil }
func (c *cloneTestConn) Begin() (driver.Tx, error) {
	return c.begin(driver.TxOptions{})
}
func (c *cloneTestConn) BeginTx(_ context.Context, options driver.TxOptions) (driver.Tx, error) {
	return c.begin(options)
}
func (c *cloneTestConn) begin(options driver.TxOptions) (driver.Tx, error) {
	c.state.beginOptions = options
	return &cloneTestTx{state: c.state}, nil
}
func (c *cloneTestConn) Ping(context.Context) error { return nil }

func (c *cloneTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "CURRENT_DATE::text"):
		c.state.sourceQuery = query
		return cloneRows(
			[]string{"customer_id", "ragione_sociale", "deal_number", "document_type", "template", "services", "proposal_type", "initial_term_months", "next_term_months", "bill_months", "delivered_in_days", "nrc_charge_time", "description", "hs_deal_id", "payment_method", "trial", "rif_ordcli", "rif_tech_nom", "rif_tech_tel", "rif_tech_email", "rif_altro_tech_nom", "rif_altro_tech_tel", "rif_altro_tech_email", "rif_adm_nom", "rif_adm_tech_tel", "rif_adm_tech_email", "current_date"},
			[][]driver.Value{{int64(123), "Customer", "D-42", "TSC-ORDINE-RIC", "template-1", "[5]", "NUOVO", int64(12), int64(24), int64(2), int64(30), int64(2), "Description", int64(456), "402   ", "trial text", "ord", "tech", "tel", "mail", "alt", "alt-tel", "alt-mail", "adm", "adm-tel", "adm-mail", "2026-02-03"}},
		), nil
	case strings.Contains(query, "FROM loader.hubs_owner"):
		if c.mode == "clone-no-owner" {
			return cloneRows([]string{"id"}, nil), nil
		}
		return cloneRows([]string{"id"}, [][]driver.Value{{"77"}}), nil
	case strings.Contains(query, "common.new_document_number"):
		c.state.numberAllocated = true
		return cloneRows([]string{"new_document_number"}, [][]driver.Value{{"SP-9000/2026"}}), nil
	case strings.Contains(query, "quotes.ins_quote_head"):
		c.state.headInserted = true
		c.state.procPayload = cloneStringArg(args[0])
		c.state.pendingHeader = &cloneDestinationHeader{
			notes: "trigger note", replaceOrders: "trigger replacement", dateSent: "trigger date",
			hsQuoteID: "trigger hs quote", hsEsignEnabled: true, hsEsignContacts: "trigger contacts",
			hsSignStatus: "trigger status", hsEsignDate: "trigger esign date",
		}
		return cloneRows([]string{"ins_quote_head"}, [][]driver.Value{{[]byte(`{"id":900,"status":"OK"}`)}}), nil
	case strings.Contains(query, "INSERT INTO quotes.quote_rows") && strings.Contains(query, "RETURNING id"):
		if c.mode == "clone-row-insert-failure" {
			return nil, errors.New("kit row insert failed")
		}
		row := cloneDestinationRow{
			ID: 1000 + len(c.state.pendingRows) + 1, KitID: cloneIntArg(args[1]), Position: cloneValue(args[2]),
			HsLineItemID: "trigger hs mrc", HsLineItemNRC: "trigger hs nrc",
		}
		c.state.pendingRows = append(c.state.pendingRows, row)
		return cloneRows([]string{"id"}, [][]driver.Value{{int64(row.ID)}}), nil
	case strings.Contains(query, "FROM quotes.quote_rows qr"):
		return cloneRows([]string{"id", "kit_id", "internal_name", "bundle_prefix_row", "position"}, [][]driver.Value{{int64(20), int64(20), "Source two", "B-2", int64(2)}, {int64(10), int64(10), "Source one", "B-1", int64(4)}}), nil
	case strings.Contains(query, "FROM quotes.quote_rows_products"):
		rowID := cloneIntArg(args[0])
		if rowID == 10 {
			return cloneRows([]string{"product_code", "minimum", "maximum", "required", "nrc", "mrc", "position", "group_name", "included", "quantity", "extended_description", "main_product"}, [][]driver.Value{{"P-ONE", int64(1), int64(4), true, "10.25000", "2.50000", int64(7), "Group", true, "3", "Exact description", true}}), nil
		}
		return cloneRows([]string{"product_code", "minimum", "maximum", "required", "nrc", "mrc", "position", "group_name", "included", "quantity", "extended_description", "main_product"}, [][]driver.Value{{"P-TWO", int64(0), int64(-1), false, "0.00000", "8.12500", int64(1), "Other", false, "0", "Other description", false}}), nil
	default:
		return nil, errors.New("unexpected clone query: " + query)
	}
}

func (c *cloneTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	switch {
	case strings.Contains(query, "INSERT INTO quotes.quote_rows_products"):
		if c.mode == "clone-product-failure" {
			return nil, errors.New("product insert failed")
		}
		if err := validateCloneProductInsertSQL(query, len(args)); err != nil {
			return nil, err
		}
		rowID := cloneIntArg(args[0])
		foundDestination := false
		for _, row := range c.state.pendingRows {
			if int64(row.ID) == rowID {
				foundDestination = true
				break
			}
		}
		if !foundDestination {
			return nil, errors.New("clone product INSERT does not target the destination row")
		}
		values := make([]driver.Value, len(args))
		for i := range args {
			values[i] = args[i].Value
		}
		c.state.pendingProducts = append(c.state.pendingProducts, values)
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "DELETE FROM quotes.quote_rows_products"):
		c.state.pendingGeneratedProductDeletes = append(c.state.pendingGeneratedProductDeletes, cloneIntArg(args[0]))
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "SET replace_orders = NULL"):
		if c.state.pendingHeader != nil {
			c.state.pendingHeader.replaceOrders = nil
			c.state.pendingHeader.dateSent = nil
			c.state.pendingHeader.hsQuoteID = nil
			c.state.pendingHeader.hsEsignEnabled = nil
			c.state.pendingHeader.hsEsignContacts = nil
			c.state.pendingHeader.hsSignStatus = nil
			c.state.pendingHeader.hsEsignDate = nil
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "SET notes = NULL"):
		if c.state.pendingHeader != nil {
			c.state.pendingHeader.notes = nil
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "UPDATE quotes.quote_rows"):
		if !strings.Contains(query, "hs_line_item_id = NULL") || !strings.Contains(query, "hs_line_item_nrc = NULL") {
			return nil, errors.New("clone row restore does not clear HubSpot line item state")
		}
		if c.mode == "clone-row-restore-failure" {
			return nil, errors.New("kit row label restore failed")
		}
		rowID := int(cloneIntArg(args[2]))
		for i := range c.state.pendingRows {
			if c.state.pendingRows[i].ID == rowID {
				c.state.pendingRows[i].InternalName = cloneValue(args[0])
				c.state.pendingRows[i].BundlePrefixRow = cloneValue(args[1])
				c.state.pendingRows[i].HsLineItemID = nil
				c.state.pendingRows[i].HsLineItemNRC = nil
			}
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "SET rif_ordcli"):
		return driver.RowsAffected(1), nil
	default:
		return nil, errors.New("unexpected clone exec: " + query)
	}
}

func validateCloneProductInsertSQL(query string, argumentCount int) error {
	normalized := strings.Join(strings.Fields(query), " ")
	const expected = "INSERT INTO quotes.quote_rows_products (quote_row_id, product_code, minimum, maximum, required, nrc, mrc, position, group_name, included, quantity, extended_description, main_product) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)"
	if normalized != expected {
		return errors.New("clone product INSERT has the wrong SQL shape: " + normalized)
	}
	if argumentCount != 13 {
		return errors.New("clone product INSERT has a mismatched argument count")
	}
	return nil
}

var _ driver.QueryerContext = (*cloneTestConn)(nil)
var _ driver.ExecerContext = (*cloneTestConn)(nil)
var _ driver.ConnBeginTx = (*cloneTestConn)(nil)
var _ driver.Pinger = (*cloneTestConn)(nil)

type cloneTestTx struct{ state *cloneTestState }

func (tx *cloneTestTx) Commit() error {
	tx.state.committed = true
	tx.state.committedRows = append([]cloneDestinationRow(nil), tx.state.pendingRows...)
	tx.state.committedProducts = append([][]driver.Value(nil), tx.state.pendingProducts...)
	tx.state.committedGeneratedProductDeletes = append([]int64(nil), tx.state.pendingGeneratedProductDeletes...)
	if tx.state.pendingHeader != nil {
		header := *tx.state.pendingHeader
		tx.state.committedHeader = &header
	}
	tx.state.pendingRows = nil
	tx.state.pendingProducts = nil
	tx.state.pendingGeneratedProductDeletes = nil
	tx.state.pendingHeader = nil
	return nil
}
func (tx *cloneTestTx) Rollback() error {
	tx.state.rolledBack = true
	tx.state.pendingRows = nil
	tx.state.pendingProducts = nil
	tx.state.pendingGeneratedProductDeletes = nil
	tx.state.pendingHeader = nil
	return nil
}

type cloneTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func cloneRows(columns []string, values [][]driver.Value) *cloneTestRows {
	return &cloneTestRows{columns: columns, values: values}
}
func (r *cloneTestRows) Columns() []string { return r.columns }
func (r *cloneTestRows) Close() error      { return nil }
func (r *cloneTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func cloneStringArg(value driver.NamedValue) string {
	if v, ok := value.Value.(string); ok {
		return v
	}
	return ""
}
func cloneIntArg(value driver.NamedValue) int64 {
	switch v := value.Value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	}
	return 0
}
func cloneValue(value driver.NamedValue) driver.Value { return value.Value }
