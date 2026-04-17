package coperture

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sciacco/mrsmith/internal/auth"
)

func TestRequireDBReturns503WhenNil(t *testing.T) {
	h := &Handler{}
	rec := httptest.NewRecorder()
	ok := h.requireDB(rec)
	if ok {
		t.Fatal("expected requireDB to return false")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "coperture_database_not_configured") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleListStatesUnwrapsFunctionJSON(t *testing.T) {
	h := &Handler{db: openCopertureTestDB(t, "states")}

	req := httptest.NewRequest(http.MethodGet, "/coperture/v1/states", nil)
	rec := httptest.NewRecorder()

	h.handleListStates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var states []locationOption
	if err := json.Unmarshal(rec.Body.Bytes(), &states); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d", len(states))
	}
	if states[0].Name != "Milano" {
		t.Fatalf("expected first state Milano, got %q", states[0].Name)
	}
}

func TestHandleListCitiesUsesExpectedQueryAndReturnsRows(t *testing.T) {
	h := &Handler{db: openCopertureTestDB(t, "cities")}

	req := httptest.NewRequest(http.MethodGet, "/coperture/v1/states/12/cities", nil)
	req.SetPathValue("stateId", "12")
	rec := httptest.NewRecorder()

	h.handleListCities(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var cities []locationOption
	if err := json.Unmarshal(rec.Body.Bytes(), &cities); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(cities) != 2 {
		t.Fatalf("expected 2 cities, got %d", len(cities))
	}
	if cities[1].Name != "Sesto San Giovanni" {
		t.Fatalf("expected second city Sesto San Giovanni, got %q", cities[1].Name)
	}
}

func TestRegisterRoutesEnforcesACLAndNilDBFallback(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, nil)

	t.Run("missing claims", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/coperture/v1/states", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("missing role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/coperture/v1/states", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
			Name:  "Viewer",
			Email: "viewer@example.com",
			Roles: []string{"viewer"},
		}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("valid role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/coperture/v1/states", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
			Name:  "Coperture User",
			Email: "coperture@example.com",
			Roles: []string{"app_coperture_access"},
		}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "coperture_database_not_configured") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})
}

func openCopertureTestDB(t *testing.T, mode string) *sql.DB {
	t.Helper()
	registerCopertureTestDriver()

	db, err := sql.Open(copertureTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const copertureTestDriverName = "coperture_test_driver"

var registerCopertureDriverOnce sync.Once

func registerCopertureTestDriver() {
	registerCopertureDriverOnce.Do(func() {
		sql.Register(copertureTestDriverName, copertureTestDriver{})
	})
}

type copertureTestDriver struct{}

func (copertureTestDriver) Open(name string) (driver.Conn, error) {
	return &copertureTestConn{mode: name}, nil
}

type copertureTestConn struct {
	mode string
}

func (c *copertureTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *copertureTestConn) Close() error { return nil }

func (c *copertureTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *copertureTestConn) Ping(context.Context) error { return nil }

func (c *copertureTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch c.mode {
	case "states":
		if strings.TrimSpace(query) != "SELECT coperture.get_states()" {
			return nil, errors.New("unexpected states query: " + query)
		}
		payload, err := os.ReadFile(filepath.Join("testdata", "states_function.json"))
		if err != nil {
			return nil, err
		}
		return &copertureTestRows{
			columns: []string{"get_states"},
			values:  [][]driver.Value{{string(payload)}},
		}, nil
	case "cities":
		if !strings.Contains(query, "FROM coperture.network_coverage_cities") || !strings.Contains(query, "ORDER BY name") {
			return nil, errors.New("unexpected cities query: " + query)
		}
		if len(args) != 1 || args[0].Value != int64(12) {
			return nil, errors.New("unexpected cities query args")
		}
		return &copertureTestRows{
			columns: []string{"id", "name"},
			values: [][]driver.Value{
				{int64(44), "Milano"},
				{int64(45), "Sesto San Giovanni"},
			},
		}, nil
	case "coverage":
		if strings.Contains(query, "SELECT coperture.get_coverage_details_types()") {
			payload, err := os.ReadFile(filepath.Join("testdata", "detail_types_function.json"))
			if err != nil {
				return nil, err
			}
			return &copertureTestRows{
				columns: []string{"get_coverage_details_types"},
				values:  [][]driver.Value{{string(payload)}},
			}, nil
		}
		if !strings.Contains(query, "FROM coperture.v_get_coverage AS v") || !strings.Contains(query, "ORDER BY v.operator, v.tech") {
			return nil, errors.New("unexpected coverage query: " + query)
		}
		if len(args) != 1 || args[0].Value != int64(44) {
			return nil, errors.New("unexpected coverage query args")
		}
		profiles, err := os.ReadFile(filepath.Join("testdata", "coverage_profiles.json"))
		if err != nil {
			return nil, err
		}
		details, err := os.ReadFile(filepath.Join("testdata", "coverage_details.json"))
		if err != nil {
			return nil, err
		}
		return &copertureTestRows{
			columns: []string{"coverage_id", "operator_id", "operator_name", "logo_url", "tech", "profiles", "details"},
			values: [][]driver.Value{
				{
					"71",
					int64(1),
					"TIM",
					"https://static.cdlan.business/x/logo_tim.png",
					"FTTH",
					string(profiles),
					string(details),
				},
			},
		}, nil
	default:
		return nil, errors.New("unexpected mode: " + c.mode)
	}
}

type copertureTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *copertureTestRows) Columns() []string {
	return r.columns
}

func (r *copertureTestRows) Close() error {
	return nil
}

func (r *copertureTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
