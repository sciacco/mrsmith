package cpbackoffice

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// Deps bundles every external dependency the cp-backoffice handlers need.
// Individual fields may be nil when the backing dependency is not configured;
// the per-endpoint handlers return 503 in that case via requireArak or
// requireMistra. There is no package-global state.
type Deps struct {
	Arak   *arak.Client
	Mistra *sql.DB
	Logger *slog.Logger
}

// RegisterRoutes mounts every cp-backoffice endpoint on the given mux.
// Customer state routes are also reused by AFC Tools and allow app_afctools_access.
// User/admin/variables routes require the full app_cpbackoffice_access role.
// Biometric routes require the separate app_cpbackoffice_biometric_access role.
//
// The mux is expected to be the shared /api mux from backend/cmd/server/main.go,
// so Recover, RequestID, CORS, AccessLog, and auth middleware apply automatically.
func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	fullProtect := acl.RequireRole(applaunch.CPBackofficeAccessRoles()...)
	customerStateRoles := append(applaunch.CPBackofficeAccessRoles(), applaunch.AFCToolsAccessRoles()...)
	customerStateProtect := acl.RequireRole(customerStateRoles...)
	biometricProtect := acl.RequireRole(applaunch.CPBackofficeBiometricAccessRoles()...)
	handleCustomerState := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, customerStateProtect(handler))
	}
	handleFull := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, fullProtect(handler))
	}
	handleBiometric := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, biometricProtect(handler))
	}

	handleCustomerState("GET /cp-backoffice/v1/customers", handleListCustomers(deps))
	handleCustomerState("POST /cp-backoffice/v1/customers/export", handleExportCustomers(deps))
	handleCustomerState("GET /cp-backoffice/v1/customer-states", handleListCustomerStates(deps))
	handleCustomerState("PUT /cp-backoffice/v1/customers/{id}/state", handleUpdateCustomerState(deps))
	handleFull("PUT /cp-backoffice/v1/customers/{id}/variables", handleUpdateCustomerVariables(deps))
	handleFull("GET /cp-backoffice/v1/users", handleListUsers(deps))
	handleFull("POST /cp-backoffice/v1/admins", handleCreateAdmin(deps))
	handleBiometric("GET /cp-backoffice/v1/biometric-requests", handleListBiometricRequests(deps))
	handleBiometric("GET /cp-backoffice/v1/biometric-requests/active-users/pdf", handleDownloadActiveBiometricUsersPDF(deps))
	handleBiometric("POST /cp-backoffice/v1/biometric-requests/{id}/completion", handleSetBiometricCompleted(deps))
}

// --- Shared helpers (names locked by S2 contract) ---

// requireArak returns true when the upstream gateway client is configured and
// the handler may proceed. When it returns false it has already written a 503
// response; the caller must return immediately.
func requireArak(d Deps) bool {
	return d.Arak != nil
}

// requireMistra returns true when the biometric database handle is configured
// and the handler may proceed. When it returns false it has already written
// a 503 response; the caller must return immediately.
func requireMistra(d Deps) bool {
	return d.Mistra != nil
}

// writeUpstreamUnavailable emits the 503 body used when the upstream gateway
// dependency is missing. Kept separate from requireArak so the helper keeps
// the exact signature required by the contract.
func writeUpstreamUnavailable(w http.ResponseWriter) {
	httputil.Error(w, http.StatusServiceUnavailable, "upstream_gateway_not_configured")
}

// writeDatabaseUnavailable emits the 503 body used when the database
// dependency is missing.
func writeDatabaseUnavailable(w http.ResponseWriter) {
	httputil.Error(w, http.StatusServiceUnavailable, "database_not_configured")
}

// dbFailure is the single sink for internal database errors. It writes a
// sanitized 500 response and keeps the real cause in the server logs under
// component="cpbackoffice" together with the route-level operation tag.
func dbFailure(w http.ResponseWriter, r *http.Request, err error, op string) {
	httputil.InternalError(w, r, err, "database operation failed",
		"component", "cpbackoffice", "operation", op)
}

// upstreamFailure is the analog of dbFailure for gateway-backed routes.
func upstreamFailure(w http.ResponseWriter, r *http.Request, err error, op string) {
	httputil.InternalError(w, r, err, "upstream gateway call failed",
		"component", "cpbackoffice", "operation", op)
}

// --- Handler bodies live in sibling files:
//     - arak.go:      gateway-backed routes (S3 — landed)
//     - biometric.go: database-backed routes (S4 — landed)
// ---
