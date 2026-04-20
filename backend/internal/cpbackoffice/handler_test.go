package cpbackoffice

import (
	"context"
	"database/sql"
	"database/sql/driver"
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

// Every registered route and the HTTP method used to exercise it, kept in one
// table so auth-gating and dep-guard tests stay in lockstep with handler.go.
type routeCase struct {
	name        string
	method      string
	path        string
	body        string
	needsArak   bool
	needsMistra bool
}

var registeredRoutes = []routeCase{
	{name: "list_customers", method: http.MethodGet, path: "/cp-backoffice/v1/customers", needsArak: true},
	{name: "list_customer_states", method: http.MethodGet, path: "/cp-backoffice/v1/customer-states", needsArak: true},
	{name: "update_customer_state", method: http.MethodPut, path: "/cp-backoffice/v1/customers/42/state", body: `{"state_id":7}`, needsArak: true},
	{name: "list_users", method: http.MethodGet, path: "/cp-backoffice/v1/users?customer_id=7", needsArak: true},
	{name: "create_admin", method: http.MethodPost, path: "/cp-backoffice/v1/admins", body: `{"customer_id":1,"nome":"Jane","cognome":"Doe","email":"jane@example.com","telefono":"0000","maintenance_on_primary_email":false,"marketing_on_primary_email":false}`, needsArak: true},
	{name: "list_biometric_requests", method: http.MethodGet, path: "/cp-backoffice/v1/biometric-requests", needsMistra: true},
	{name: "set_biometric_completed", method: http.MethodPost, path: "/cp-backoffice/v1/biometric-requests/42/completion", body: `{"completed":true}`, needsMistra: true},
}

func newRouteRequest(rc routeCase) *http.Request {
	var body io.Reader
	if rc.body != "" {
		body = strings.NewReader(rc.body)
	}
	return httptest.NewRequest(rc.method, rc.path, body)
}

func newTestDeps(t *testing.T, withArak, withMistra bool) Deps {
	t.Helper()
	d := Deps{}
	if withArak {
		d.Arak = newStubArakClient(t)
	}
	if withMistra {
		d.Mistra = openCPBackofficeTestDB(t)
	}
	return d
}

func newMux(deps Deps) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterRoutes(mux, deps)
	return mux
}

func withRoleClaims(req *http.Request, roles ...string) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
		Subject: "test-subject",
		Name:    "Test Operator",
		Email:   "test@example.com",
		Roles:   roles,
	}))
}

func TestRoutesReturn401WithoutClaims(t *testing.T) {
	mux := newMux(newTestDeps(t, true, true))

	for _, rc := range registeredRoutes {
		t.Run(rc.name, func(t *testing.T) {
			req := newRouteRequest(rc)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRoutesReturn403WithoutRole(t *testing.T) {
	mux := newMux(newTestDeps(t, true, true))

	for _, rc := range registeredRoutes {
		t.Run(rc.name, func(t *testing.T) {
			req := newRouteRequest(rc)
			req = withRoleClaims(req, "some_other_role")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRoutesReachHandlerWithRole(t *testing.T) {
	// With all deps configured, each route is reached past auth gating. The
	// stub gateway returns 501 for every non-token path; S3 forwards that as
	// a business error (status 501, error "upstream_error"). The biometric
	// routes (S4) are wired to the database: with the bare stub driver they
	// reach dbFailure and surface as 500. Either way, the auth-gating
	// contract is identical.
	mux := newMux(newTestDeps(t, true, true))

	for _, rc := range registeredRoutes {
		t.Run(rc.name, func(t *testing.T) {
			req := newRouteRequest(rc)
			req = withRoleClaims(req, "app_cpbackoffice_access")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			want := http.StatusNotImplemented
			if rc.needsMistra {
				want = http.StatusInternalServerError
			}
			if rec.Code != want {
				t.Fatalf("expected %d, got %d: %s", want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMissingArakReturns503ForUpstreamRoutes(t *testing.T) {
	// Database present, upstream gateway missing.
	mux := newMux(newTestDeps(t, false, true))

	for _, rc := range registeredRoutes {
		if !rc.needsArak {
			continue
		}
		t.Run(rc.name, func(t *testing.T) {
			req := newRouteRequest(rc)
			req = withRoleClaims(req, "app_cpbackoffice_access")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "upstream_gateway_not_configured") {
				t.Fatalf("unexpected body: %q", rec.Body.String())
			}
		})
	}
}

func TestMissingMistraReturns503ForDatabaseRoutes(t *testing.T) {
	// Upstream gateway present, database missing.
	mux := newMux(newTestDeps(t, true, false))

	for _, rc := range registeredRoutes {
		if !rc.needsMistra {
			continue
		}
		t.Run(rc.name, func(t *testing.T) {
			req := newRouteRequest(rc)
			req = withRoleClaims(req, "app_cpbackoffice_access")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "database_not_configured") {
				t.Fatalf("unexpected body: %q", rec.Body.String())
			}
		})
	}
}

func TestRequireArakAndRequireMistra(t *testing.T) {
	if requireArak(Deps{}) {
		t.Fatalf("expected requireArak to be false on zero Deps")
	}
	if requireMistra(Deps{}) {
		t.Fatalf("expected requireMistra to be false on zero Deps")
	}

	d := Deps{
		Arak:   newStubArakClient(t),
		Mistra: openCPBackofficeTestDB(t),
	}
	if !requireArak(d) {
		t.Fatalf("expected requireArak to be true when gateway client is set")
	}
	if !requireMistra(d) {
		t.Fatalf("expected requireMistra to be true when database is set")
	}
}

// --- Test doubles ---

// newStubArakClient returns a gateway client whose underlying HTTP server
// responds to everything with 501. S2 handlers never call .Do(); the client
// is only used so requireArak returns true.
func newStubArakClient(t *testing.T) *arak.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"stub","expires_in":300}`))
			return
		}
		http.Error(w, "stub", http.StatusNotImplemented)
	}))
	t.Cleanup(srv.Close)

	return arak.New(arak.Config{
		BaseURL:      srv.URL,
		TokenURL:     srv.URL + "/token",
		ClientID:     "cp-backoffice-test",
		ClientSecret: "cp-backoffice-secret",
	})
}

func openCPBackofficeTestDB(t *testing.T) *sql.DB {
	t.Helper()
	registerCPBackofficeTestDriver()

	db, err := sql.Open(cpBackofficeTestDriverName, "stub")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const cpBackofficeTestDriverName = "cpbackoffice_test_driver"

var registerCPBackofficeDriverOnce sync.Once

func registerCPBackofficeTestDriver() {
	registerCPBackofficeDriverOnce.Do(func() {
		sql.Register(cpBackofficeTestDriverName, cpBackofficeTestDriver{})
	})
}

type cpBackofficeTestDriver struct{}

func (cpBackofficeTestDriver) Open(string) (driver.Conn, error) {
	return &cpBackofficeTestConn{}, nil
}

type cpBackofficeTestConn struct{}

func (c *cpBackofficeTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *cpBackofficeTestConn) Close() error { return nil }

func (c *cpBackofficeTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *cpBackofficeTestConn) Ping(context.Context) error { return nil }
