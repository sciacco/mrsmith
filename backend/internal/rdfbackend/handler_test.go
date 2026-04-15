package rdfbackend

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
	"sync/atomic"
	"testing"

	"github.com/sciacco/mrsmith/internal/auth"
)

func TestHandleListSuppliersReturnsPaginatedResults(t *testing.T) {
	state := &rdfBackendTestState{
		queryResults: map[string]rdfBackendRowsResult{
			"SELECT COUNT(*) FROM public.rdf_fornitori WHERE nome ILIKE $1": {
				columns: []string{"count"},
				values:  [][]driver.Value{{int64(12)}},
			},
			"SELECT id, nome FROM public.rdf_fornitori WHERE nome ILIKE $1 ORDER BY nome DESC LIMIT $2 OFFSET $3": {
				columns: []string{"id", "nome"},
				values: [][]driver.Value{
					{int64(21), "Tim Cloud"},
					{int64(25), "Tim Fiber"},
				},
			},
		},
	}
	db := openRDFBackendTestDB(t, state)
	h := &Handler{db: db}

	req := httptest.NewRequest(http.MethodGet, "/rdf-backend/v1/fornitori?search=tim&sort=nome&order=desc&page=2&pageSize=5", nil)
	rec := httptest.NewRecorder()

	h.handleListSuppliers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body listSuppliersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Total != 12 {
		t.Fatalf("expected total 12, got %d", body.Total)
	}
	if len(body.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(body.Items))
	}
	if body.Items[0].ID != 21 || body.Items[0].Nome != "Tim Cloud" {
		t.Fatalf("unexpected first item: %#v", body.Items[0])
	}

	if len(state.queries) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(state.queries))
	}
	if got := state.queries[0].args[0].Value; got != "%tim%" {
		t.Fatalf("expected count search arg %%tim%%, got %#v", got)
	}
	if got := state.queries[1].query; !strings.Contains(got, "ORDER BY nome DESC") {
		t.Fatalf("expected list query sorted by nome desc, got %q", got)
	}
	if got := state.queries[1].args[1].Value; got != int64(5) {
		t.Fatalf("expected limit arg 5, got %#v", got)
	}
	if got := state.queries[1].args[2].Value; got != int64(5) {
		t.Fatalf("expected offset arg 5, got %#v", got)
	}
}

func TestHandleListSuppliersFallsBackToSafeSortDefaults(t *testing.T) {
	state := &rdfBackendTestState{
		queryResults: map[string]rdfBackendRowsResult{
			"SELECT COUNT(*) FROM public.rdf_fornitori": {
				columns: []string{"count"},
				values:  [][]driver.Value{{int64(0)}},
			},
			"SELECT id, nome FROM public.rdf_fornitori ORDER BY id ASC LIMIT $1 OFFSET $2": {
				columns: []string{"id", "nome"},
			},
		},
	}
	db := openRDFBackendTestDB(t, state)
	h := &Handler{db: db}

	req := httptest.NewRequest(http.MethodGet, "/rdf-backend/v1/fornitori?sort=drop%20table&order=nope", nil)
	rec := httptest.NewRecorder()

	h.handleListSuppliers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(state.queries) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(state.queries))
	}
	if got := state.queries[1].query; !strings.Contains(got, "ORDER BY id ASC") {
		t.Fatalf("expected list query to fall back to id asc, got %q", got)
	}
}

func TestHandleCreateSupplierValidatesName(t *testing.T) {
	state := &rdfBackendTestState{}
	db := openRDFBackendTestDB(t, state)
	h := &Handler{db: db}

	req := httptest.NewRequest(http.MethodPost, "/rdf-backend/v1/fornitori", strings.NewReader(`{"nome":"   "}`))
	rec := httptest.NewRecorder()

	h.handleCreateSupplier(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"nome_required"}` {
		t.Fatalf("unexpected body %q", rec.Body.String())
	}
}

func TestHandleUpdateSupplierReturnsNotFound(t *testing.T) {
	state := &rdfBackendTestState{
		queryResults: map[string]rdfBackendRowsResult{
			"UPDATE public.rdf_fornitori SET nome = $1 WHERE id = $2 RETURNING id, nome": {
				columns: []string{"id", "nome"},
			},
		},
	}
	db := openRDFBackendTestDB(t, state)
	h := &Handler{db: db}

	req := httptest.NewRequest(http.MethodPatch, "/rdf-backend/v1/fornitori/42", strings.NewReader(`{"nome":"Nuovo nome"}`))
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	h.handleUpdateSupplier(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"not_found"}` {
		t.Fatalf("unexpected body %q", rec.Body.String())
	}
}

func TestHandleDeleteSupplierReturnsNotFoundWhenNothingDeleted(t *testing.T) {
	state := &rdfBackendTestState{
		execResults: map[string]rdfBackendExecResult{
			"DELETE FROM public.rdf_fornitori WHERE id = $1": {
				rowsAffected: 0,
			},
		},
	}
	db := openRDFBackendTestDB(t, state)
	h := &Handler{db: db}

	req := httptest.NewRequest(http.MethodDelete, "/rdf-backend/v1/fornitori/7", nil)
	req.SetPathValue("id", "7")
	rec := httptest.NewRecorder()

	h.handleDeleteSupplier(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"not_found"}` {
		t.Fatalf("unexpected body %q", rec.Body.String())
	}
}

func TestRegisterRoutesRequiresRole(t *testing.T) {
	state := &rdfBackendTestState{
		queryResults: map[string]rdfBackendRowsResult{
			"SELECT COUNT(*) FROM public.rdf_fornitori": {
				columns: []string{"count"},
				values:  [][]driver.Value{{int64(1)}},
			},
			"SELECT id, nome FROM public.rdf_fornitori ORDER BY id ASC LIMIT $1 OFFSET $2": {
				columns: []string{"id", "nome"},
				values:  [][]driver.Value{{int64(1), "Supplier One"}},
			},
		},
	}
	db := openRDFBackendTestDB(t, state)

	mux := http.NewServeMux()
	RegisterRoutes(mux, db)

	tests := []struct {
		name       string
		claims     *auth.Claims
		wantStatus int
	}{
		{name: "missing claims", claims: nil, wantStatus: http.StatusUnauthorized},
		{name: "wrong role", claims: &auth.Claims{Roles: []string{"viewer"}}, wantStatus: http.StatusForbidden},
		{name: "access role", claims: &auth.Claims{Roles: []string{"app_rdf_backend_access"}}, wantStatus: http.StatusOK},
		{name: "devadmin", claims: &auth.Claims{Roles: []string{"app_devadmin"}}, wantStatus: http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/rdf-backend/v1/fornitori", nil)
			if tc.claims != nil {
				req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, *tc.claims))
			}
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d", tc.wantStatus, rec.Code)
			}
		})
	}
}

type rdfBackendTestState struct {
	mu           sync.Mutex
	queries      []rdfBackendCapturedCall
	execs        []rdfBackendCapturedCall
	queryResults map[string]rdfBackendRowsResult
	execResults  map[string]rdfBackendExecResult
}

type rdfBackendCapturedCall struct {
	query string
	args  []driver.NamedValue
}

type rdfBackendRowsResult struct {
	columns []string
	values  [][]driver.Value
	err     error
}

type rdfBackendExecResult struct {
	rowsAffected int64
	err          error
}

const rdfBackendTestDriverName = "rdfbackend_test_driver"

var (
	registerRDFBackendDriverOnce sync.Once
	rdfBackendStateCounter       atomic.Uint64
	rdfBackendStatesMu           sync.Mutex
	rdfBackendStates             = map[string]*rdfBackendTestState{}
)

func openRDFBackendTestDB(t *testing.T, state *rdfBackendTestState) *sql.DB {
	t.Helper()

	registerRDFBackendDriverOnce.Do(func() {
		sql.Register(rdfBackendTestDriverName, rdfBackendTestDriver{})
	})

	dsn := t.Name() + "-" + strconv.FormatUint(rdfBackendStateCounter.Add(1), 10)
	rdfBackendStatesMu.Lock()
	rdfBackendStates[dsn] = state
	rdfBackendStatesMu.Unlock()

	db, err := sql.Open(rdfBackendTestDriverName, dsn)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		rdfBackendStatesMu.Lock()
		delete(rdfBackendStates, dsn)
		rdfBackendStatesMu.Unlock()
	})
	return db
}

type rdfBackendTestDriver struct{}

func (rdfBackendTestDriver) Open(name string) (driver.Conn, error) {
	rdfBackendStatesMu.Lock()
	state := rdfBackendStates[name]
	rdfBackendStatesMu.Unlock()
	if state == nil {
		return nil, errors.New("missing rdfbackend test state")
	}
	return rdfBackendTestConn{state: state}, nil
}

type rdfBackendTestConn struct {
	state *rdfBackendTestState
}

func (c rdfBackendTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c rdfBackendTestConn) Close() error { return nil }

func (c rdfBackendTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c rdfBackendTestConn) Ping(context.Context) error { return nil }

func (c rdfBackendTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.mu.Lock()
	c.state.queries = append(c.state.queries, rdfBackendCapturedCall{
		query: query,
		args:  append([]driver.NamedValue(nil), args...),
	})
	result, ok := c.state.queryResults[query]
	c.state.mu.Unlock()
	if !ok {
		return nil, errors.New("unexpected query: " + query)
	}
	if result.err != nil {
		return nil, result.err
	}
	return &rdfBackendTestRows{columns: result.columns, values: result.values}, nil
}

func (c rdfBackendTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.state.mu.Lock()
	c.state.execs = append(c.state.execs, rdfBackendCapturedCall{
		query: query,
		args:  append([]driver.NamedValue(nil), args...),
	})
	result, ok := c.state.execResults[query]
	c.state.mu.Unlock()
	if !ok {
		return nil, errors.New("unexpected exec: " + query)
	}
	if result.err != nil {
		return nil, result.err
	}
	return rdfBackendTestResult(result.rowsAffected), nil
}

var _ driver.QueryerContext = rdfBackendTestConn{}
var _ driver.ExecerContext = rdfBackendTestConn{}
var _ driver.Pinger = rdfBackendTestConn{}

type rdfBackendTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *rdfBackendTestRows) Columns() []string { return r.columns }

func (r *rdfBackendTestRows) Close() error { return nil }

func (r *rdfBackendTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

type rdfBackendTestResult int64

func (r rdfBackendTestResult) LastInsertId() (int64, error) {
	return 0, errors.New("not implemented")
}

func (r rdfBackendTestResult) RowsAffected() (int64, error) {
	return int64(r), nil
}
