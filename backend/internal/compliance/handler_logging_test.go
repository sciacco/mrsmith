package compliance

import (
	"bytes"
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

	"github.com/sciacco/mrsmith/internal/platform/logging"
)

func TestHandleListBlocksSanitizesDatabaseErrors(t *testing.T) {
	db := openComplianceTestDB(t, "list-error")

	var buf bytes.Buffer
	logger := logging.NewWithWriter(&buf, "debug")
	h := &Handler{db: db}

	req := httptest.NewRequest(http.MethodGet, "/compliance/blocks", nil)
	req = req.WithContext(logging.IntoContext(req.Context(), logger))
	rec := httptest.NewRecorder()

	h.handleListBlocks(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "list query failed") {
		t.Fatalf("expected sanitized response body, got %q", rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "internal_server_error" {
		t.Fatalf("expected internal_server_error, got %#v", body["error"])
	}

	entry := decodeComplianceLog(t, buf.String())
	if entry["component"] != "compliance" {
		t.Fatalf("expected compliance component log, got %#v", entry["component"])
	}
	if entry["operation"] != "list_blocks" {
		t.Fatalf("expected list_blocks operation, got %#v", entry["operation"])
	}
}

func TestHandleGetBlockReturnsNotFoundOnNoRows(t *testing.T) {
	db := openComplianceTestDB(t, "get-no-rows")
	h := &Handler{db: db}

	req := httptest.NewRequest(http.MethodGet, "/compliance/blocks/42", nil)
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	h.handleGetBlock(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"not_found"}` {
		t.Fatalf("expected not_found payload, got %q", rec.Body.String())
	}
}

func TestHandleGetBlockSanitizesQueryRowFailures(t *testing.T) {
	db := openComplianceTestDB(t, "get-error")

	var buf bytes.Buffer
	logger := logging.NewWithWriter(&buf, "debug")
	h := &Handler{db: db}

	req := httptest.NewRequest(http.MethodGet, "/compliance/blocks/42", nil)
	req.SetPathValue("id", "42")
	req = req.WithContext(logging.IntoContext(req.Context(), logger))
	rec := httptest.NewRecorder()

	h.handleGetBlock(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "lookup failed") {
		t.Fatalf("expected sanitized response body, got %q", rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "internal_server_error" {
		t.Fatalf("expected internal_server_error, got %#v", body["error"])
	}

	entry := decodeComplianceLog(t, buf.String())
	if entry["operation"] != "get_block" {
		t.Fatalf("expected get_block log, got %#v", entry["operation"])
	}
}

func decodeComplianceLog(t *testing.T, raw string) map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(lines))
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("failed to decode log line: %v", err)
	}
	return entry
}

func openComplianceTestDB(t *testing.T, mode string) *sql.DB {
	t.Helper()

	registerComplianceTestDriver()

	db, err := sql.Open(complianceTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const complianceTestDriverName = "compliance_test_driver"

var registerComplianceDriverOnce sync.Once

func registerComplianceTestDriver() {
	registerComplianceDriverOnce.Do(func() {
		sql.Register(complianceTestDriverName, complianceTestDriver{})
	})
}

type complianceTestDriver struct{}

func (complianceTestDriver) Open(name string) (driver.Conn, error) {
	return complianceTestConn{mode: name}, nil
}

type complianceTestConn struct {
	mode string
}

func (c complianceTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (c complianceTestConn) Close() error              { return nil }
func (c complianceTestConn) Begin() (driver.Tx, error) { return nil, errors.New("not implemented") }

func (c complianceTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch c.mode {
	case "list-error":
		return nil, errors.New("list query failed")
	case "get-error":
		if strings.Contains(query, "WHERE b.id = $1") {
			return nil, errors.New("lookup failed")
		}
	case "get-no-rows":
		if strings.Contains(query, "WHERE b.id = $1") {
			return &complianceTestRows{columns: []string{"id", "request_date", "reference", "method_id", "description"}}, nil
		}
	}
	return &complianceTestRows{columns: []string{"id", "request_date", "reference", "method_id", "description"}}, nil
}

func (c complianceTestConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return nil, errors.New("not implemented")
}

func (c complianceTestConn) Ping(context.Context) error { return nil }

var _ driver.QueryerContext = complianceTestConn{}
var _ driver.ExecerContext = complianceTestConn{}
var _ driver.Pinger = complianceTestConn{}

type complianceTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *complianceTestRows) Columns() []string {
	return r.columns
}

func (r *complianceTestRows) Close() error { return nil }

func (r *complianceTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
