package fornitori

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

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/arak"
)

func TestAccessRoleRequired(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/fornitori/v1/provider", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without claims, got %d", rec.Code)
	}
}

func TestMissingArakReturnsServiceUnavailable(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, nil, nil, nil)

	req := authedRequest(http.MethodGet, "/fornitori/v1/provider", nil, "app_fornitori_access")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for missing arak, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), codeDependencyUnavailable) {
		t.Fatalf("expected dependency code in response, got %s", rec.Body.String())
	}
}

func TestReadonlyRoleBlocksAdminWrites(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, nil, nil, nil)

	req := authedRequest(
		http.MethodPost,
		"/fornitori/v1/category",
		strings.NewReader(`{"name":"DURC"}`),
		"app_fornitori_access",
		"app_fornitori_readonly",
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for readonly write, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), codeReadonlyDenied) {
		t.Fatalf("expected readonly code in response, got %s", rec.Body.String())
	}
}

func TestSkipQualificationRequiresRole(t *testing.T) {
	arakSrv := fakeArakServer(t)
	defer arakSrv.Close()

	client := arak.New(arak.Config{
		BaseURL:      arakSrv.URL,
		TokenURL:     arakSrv.URL + "/token",
		ClientID:     "client",
		ClientSecret: "secret",
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, client, nil, nil)

	req := authedRequest(
		http.MethodPut,
		"/fornitori/v1/provider/12",
		strings.NewReader(`{"skip_qualification_validation":true}`),
		"app_fornitori_access",
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing skip role, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), codeSkipRoleRequired) {
		t.Fatalf("expected skip role code in response, got %s", rec.Body.String())
	}
}

func TestDevAdminCanSetSkipQualification(t *testing.T) {
	var gotPath string
	var gotBody string
	arakSrv := fakeArakServer(t, func(r *http.Request, body []byte) {
		gotPath = r.URL.Path
		gotBody = string(body)
	})
	defer arakSrv.Close()

	client := arak.New(arak.Config{
		BaseURL:      arakSrv.URL,
		TokenURL:     arakSrv.URL + "/token",
		ClientID:     "client",
		ClientSecret: "secret",
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, client, nil, nil)

	req := authedRequest(
		http.MethodPut,
		"/fornitori/v1/provider/12",
		strings.NewReader(`{"skip_qualification_validation":true}`),
		"app_fornitori_access",
		"app_devadmin",
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for devadmin skip write, got %d body=%s", rec.Code, rec.Body.String())
	}
	if gotPath != "/arak/provider-qualification/v1/provider/12" {
		t.Fatalf("unexpected upstream path %q", gotPath)
	}
	if gotBody != `{"skip_qualification_validation":true}` {
		t.Fatalf("unexpected upstream body %q", gotBody)
	}
}

func TestHandleCountriesReturnsArakCountries(t *testing.T) {
	h := &Handler{db: openFornitoriTestDB(t, "countries")}

	req := httptest.NewRequest(http.MethodGet, "/fornitori/v1/country", nil)
	rec := httptest.NewRecorder()
	h.handleCountries(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var rows []country
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("failed to decode countries response: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 countries, got %d: %#v", len(rows), rows)
	}
	if rows[0].Code != "FR" || rows[0].Name != "Francia" || rows[1].Code != "IT" || rows[1].Name != "Italia" {
		t.Fatalf("unexpected countries payload: %#v", rows)
	}
}

func authedRequest(method, target string, body io.Reader, roles ...string) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Content-Type", "application/json")
	claims := auth.Claims{Subject: "u1", Roles: roles, RawToken: "token"}
	return req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, claims))
}

func fakeArakServer(t *testing.T, inspect ...func(*http.Request, []byte)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"arak-token","expires_in":3600}`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		if len(inspect) > 0 {
			inspect[0](r, body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(w, bytes.NewReader([]byte(`{"ok":true}`)))
	}))
}

func openFornitoriTestDB(t *testing.T, mode string) *sql.DB {
	t.Helper()

	registerFornitoriTestDriver()

	db, err := sql.Open(fornitoriTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const fornitoriTestDriverName = "fornitori_test_driver"

var registerFornitoriDriverOnce sync.Once

func registerFornitoriTestDriver() {
	registerFornitoriDriverOnce.Do(func() {
		sql.Register(fornitoriTestDriverName, fornitoriTestDriver{})
	})
}

type fornitoriTestDriver struct{}

func (fornitoriTestDriver) Open(name string) (driver.Conn, error) {
	return fornitoriTestConn{mode: name}, nil
}

type fornitoriTestConn struct {
	mode string
}

func (c fornitoriTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c fornitoriTestConn) Close() error { return nil }

func (c fornitoriTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c fornitoriTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.mode == "countries" && strings.Contains(query, "provider_qualifications.country") {
		return &fornitoriTestRows{
			columns: []string{"code", "name"},
			values: [][]driver.Value{
				{"FR", "Francia"},
				{"IT", "Italia"},
			},
		}, nil
	}
	return nil, errors.New("unexpected query")
}

func (c fornitoriTestConn) Ping(context.Context) error { return nil }

var _ driver.QueryerContext = fornitoriTestConn{}
var _ driver.Pinger = fornitoriTestConn{}

type fornitoriTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *fornitoriTestRows) Columns() []string {
	return r.columns
}

func (r *fornitoriTestRows) Close() error { return nil }

func (r *fornitoriTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
