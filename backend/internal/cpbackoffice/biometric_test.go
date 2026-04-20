package cpbackoffice

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Expected SQL anchors locked by Slice S4. Asserted verbatim so a drift in
// handler construction is caught at test time, not in production.
var biometricListQueryAnchors = []string{
	"customers.biometric_request",
	"customers.user_struct",
	"customers.customer",
	"customers.user_entrance_detail",
	"ORDER BY data_richiesta DESC",
}

const biometricCompletionAnchor = "customers.biometric_request_set_completed($1::bigint, $2::boolean)"

// ───────────────────────────────── List handler ─────────────────────────────────

func TestHandleListBiometricRequests_QueryAnchorsAndOrdering(t *testing.T) {
	state := newBiometricTestState()
	state.listColumns = []string{
		"id", "nome", "cognome", "email", "azienda",
		"tipo_richiesta", "stato_richiesta", "data_richiesta",
		"data_approvazione", "is_biometric_lenel",
	}
	state.listRows = [][]driver.Value{
		{int64(1), "Foo", "Bar", "foo@bar.com", "Acme Srl",
			"accesso", true, mustParseTime(t, "2025-03-02T10:00:00Z"),
			mustParseTime(t, "2025-03-03T09:00:00Z"), true},
		{int64(2), "Null", "Date", "null@bar.com", "Beta Srl",
			"iscrizione", false, mustParseTime(t, "2025-01-02T03:04:05Z"),
			nil, false},
	}

	deps := Deps{Mistra: openBiometricTestDB(t, state)}

	req := httptest.NewRequest(http.MethodGet, "/cp-backoffice/v1/biometric-requests", nil)
	rec := httptest.NewRecorder()
	handleListBiometricRequests(deps)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Assert the SQL the handler issued contains every locked anchor.
	if state.lastListQuery == "" {
		t.Fatalf("expected handler to issue a SELECT; got empty query trace")
	}
	for _, anchor := range biometricListQueryAnchors {
		if !strings.Contains(state.lastListQuery, anchor) {
			t.Fatalf("expected query to contain %q, got:\n%s", anchor, state.lastListQuery)
		}
	}

	// Response JSON must be a 10-key array with exact types.
	var out []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(out))
	}

	requiredKeys := []string{
		"id", "nome", "cognome", "email", "azienda",
		"tipo_richiesta", "stato_richiesta", "data_richiesta",
		"data_approvazione", "is_biometric_lenel",
	}
	for _, row := range out {
		for _, key := range requiredKeys {
			if _, ok := row[key]; !ok {
				t.Fatalf("response row missing required key %q: %#v", key, row)
			}
		}
	}

	// Row 1: stato_richiesta bool (NOT "ok" / "pending"), approval populated,
	// is_biometric_lenel returned as bool.
	first := out[0]
	if got, ok := first["stato_richiesta"].(bool); !ok || got != true {
		t.Fatalf("expected row[0].stato_richiesta bool=true, got %#v", first["stato_richiesta"])
	}
	if got, ok := first["is_biometric_lenel"].(bool); !ok || got != true {
		t.Fatalf("expected row[0].is_biometric_lenel bool=true, got %#v", first["is_biometric_lenel"])
	}
	if first["data_approvazione"] == nil {
		t.Fatalf("expected row[0].data_approvazione populated, got nil")
	}

	// Row 2: nullable approval must serialize to null, not a zero timestamp.
	second := out[1]
	if second["data_approvazione"] != nil {
		t.Fatalf("expected row[1].data_approvazione to be null, got %#v", second["data_approvazione"])
	}
	if got, ok := second["stato_richiesta"].(bool); !ok || got != false {
		t.Fatalf("expected row[1].stato_richiesta bool=false, got %#v", second["stato_richiesta"])
	}
	if got, ok := second["is_biometric_lenel"].(bool); !ok || got != false {
		t.Fatalf("expected row[1].is_biometric_lenel bool=false, got %#v", second["is_biometric_lenel"])
	}
}

func TestHandleListBiometricRequests_EmptyResultReturnsEmptyArray(t *testing.T) {
	state := newBiometricTestState()
	state.listColumns = []string{
		"id", "nome", "cognome", "email", "azienda",
		"tipo_richiesta", "stato_richiesta", "data_richiesta",
		"data_approvazione", "is_biometric_lenel",
	}
	state.listRows = nil

	deps := Deps{Mistra: openBiometricTestDB(t, state)}

	req := httptest.NewRequest(http.MethodGet, "/cp-backoffice/v1/biometric-requests", nil)
	rec := httptest.NewRecorder()
	handleListBiometricRequests(deps)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("expected empty array, got: %s", rec.Body.String())
	}
}

// ───────────────────────────────── Completion handler ─────────────────────────────────

func TestHandleSetBiometricCompleted_CallsExactStoredFunction(t *testing.T) {
	state := newBiometricTestState()
	deps := Deps{Mistra: openBiometricTestDB(t, state)}

	req := httptest.NewRequest(http.MethodPost,
		"/cp-backoffice/v1/biometric-requests/42/completion",
		strings.NewReader(`{"completed":true}`))
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()
	handleSetBiometricCompleted(deps)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Exact response body.
	if strings.TrimSpace(rec.Body.String()) != `{"ok":true}` {
		t.Fatalf("expected {\"ok\":true} body, got %q", rec.Body.String())
	}

	// Exact stored-function signature.
	if state.lastCompletionQuery == "" {
		t.Fatalf("expected handler to exec a SELECT on the stored function; got empty trace")
	}
	if !strings.Contains(state.lastCompletionQuery, biometricCompletionAnchor) {
		t.Fatalf("expected query to contain %q, got:\n%s",
			biometricCompletionAnchor, state.lastCompletionQuery)
	}

	// Path id must be parsed as int64 and passed as the first argument.
	if len(state.lastCompletionArgs) != 2 {
		t.Fatalf("expected 2 args, got %d: %#v", len(state.lastCompletionArgs), state.lastCompletionArgs)
	}
	idArg, ok := state.lastCompletionArgs[0].Value.(int64)
	if !ok || idArg != 42 {
		t.Fatalf("expected first arg to be int64(42), got %#v", state.lastCompletionArgs[0].Value)
	}
	completedArg, ok := state.lastCompletionArgs[1].Value.(bool)
	if !ok || completedArg != true {
		t.Fatalf("expected second arg to be bool(true), got %#v", state.lastCompletionArgs[1].Value)
	}
}

func TestHandleSetBiometricCompleted_PassesFalse(t *testing.T) {
	state := newBiometricTestState()
	deps := Deps{Mistra: openBiometricTestDB(t, state)}

	req := httptest.NewRequest(http.MethodPost,
		"/cp-backoffice/v1/biometric-requests/7/completion",
		strings.NewReader(`{"completed":false}`))
	req.SetPathValue("id", "7")
	rec := httptest.NewRecorder()
	handleSetBiometricCompleted(deps)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(state.lastCompletionArgs) != 2 {
		t.Fatalf("expected 2 args, got %d", len(state.lastCompletionArgs))
	}
	completedArg, ok := state.lastCompletionArgs[1].Value.(bool)
	if !ok || completedArg != false {
		t.Fatalf("expected second arg to be bool(false), got %#v", state.lastCompletionArgs[1].Value)
	}
}

func TestHandleSetBiometricCompleted_InvalidIDReturns400(t *testing.T) {
	state := newBiometricTestState()
	deps := Deps{Mistra: openBiometricTestDB(t, state)}

	req := httptest.NewRequest(http.MethodPost,
		"/cp-backoffice/v1/biometric-requests/abc/completion",
		strings.NewReader(`{"completed":true}`))
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()
	handleSetBiometricCompleted(deps)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if state.lastCompletionQuery != "" {
		t.Fatalf("expected handler to reject before hitting DB, got query: %s",
			state.lastCompletionQuery)
	}
}

// ───────────────────────────────── Test doubles ─────────────────────────────────

// biometricTestState carries the mock-driver behavior for one test. The
// driver itself is registered once at process scope so sql.Open succeeds;
// per-test state is threaded through via sql.Open's name argument and looked
// up in biometricStates.
type biometricTestState struct {
	mu                  sync.Mutex
	listColumns         []string
	listRows            [][]driver.Value
	lastListQuery       string
	lastCompletionQuery string
	lastCompletionArgs  []driver.NamedValue
}

func newBiometricTestState() *biometricTestState {
	return &biometricTestState{}
}

var (
	biometricStates    = struct {
		sync.Mutex
		m map[string]*biometricTestState
	}{m: map[string]*biometricTestState{}}
	biometricStatesSeq int64
)

func openBiometricTestDB(t *testing.T, state *biometricTestState) *sql.DB {
	t.Helper()
	registerBiometricTestDriver()

	biometricStates.Lock()
	biometricStatesSeq++
	name := "bio-" + strconv.FormatInt(biometricStatesSeq, 10)
	biometricStates.m[name] = state
	biometricStates.Unlock()

	db, err := sql.Open(biometricTestDriverName, name)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		biometricStates.Lock()
		delete(biometricStates.m, name)
		biometricStates.Unlock()
		_ = db.Close()
	})
	return db
}

const biometricTestDriverName = "cpbackoffice_biometric_test_driver"

var registerBiometricDriverOnce sync.Once

func registerBiometricTestDriver() {
	registerBiometricDriverOnce.Do(func() {
		sql.Register(biometricTestDriverName, biometricTestDriver{})
	})
}

type biometricTestDriver struct{}

func (biometricTestDriver) Open(name string) (driver.Conn, error) {
	biometricStates.Lock()
	state := biometricStates.m[name]
	biometricStates.Unlock()
	if state == nil {
		return nil, errors.New("biometric test state not registered: " + name)
	}
	return &biometricTestConn{state: state}, nil
}

type biometricTestConn struct {
	state *biometricTestState
}

func (c *biometricTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not implemented; use QueryContext/ExecContext")
}

func (c *biometricTestConn) Close() error { return nil }

func (c *biometricTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("begin not implemented")
}

func (c *biometricTestConn) Ping(context.Context) error { return nil }

func (c *biometricTestConn) QueryContext(_ context.Context, query string,
	_ []driver.NamedValue) (driver.Rows, error) {
	c.state.mu.Lock()
	c.state.lastListQuery = query
	columns := append([]string(nil), c.state.listColumns...)
	rows := make([][]driver.Value, len(c.state.listRows))
	for i, r := range c.state.listRows {
		rows[i] = append([]driver.Value(nil), r...)
	}
	c.state.mu.Unlock()

	return &biometricTestRows{columns: columns, values: rows}, nil
}

func (c *biometricTestConn) ExecContext(_ context.Context, query string,
	args []driver.NamedValue) (driver.Result, error) {
	c.state.mu.Lock()
	c.state.lastCompletionQuery = query
	c.state.lastCompletionArgs = append([]driver.NamedValue(nil), args...)
	c.state.mu.Unlock()
	return biometricTestResult{}, nil
}

var _ driver.QueryerContext = (*biometricTestConn)(nil)
var _ driver.ExecerContext = (*biometricTestConn)(nil)
var _ driver.Pinger = (*biometricTestConn)(nil)

type biometricTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *biometricTestRows) Columns() []string { return r.columns }

func (r *biometricTestRows) Close() error { return nil }

func (r *biometricTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

type biometricTestResult struct{}

func (biometricTestResult) LastInsertId() (int64, error) { return 0, nil }
func (biometricTestResult) RowsAffected() (int64, error) { return 1, nil }

// ───────────────────────────────── Utilities ─────────────────────────────────

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("failed to parse time %q: %v", s, err)
	}
	return v
}

