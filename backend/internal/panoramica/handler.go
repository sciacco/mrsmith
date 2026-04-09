package panoramica

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// Handler holds references to all three databases.
type Handler struct {
	mistraDB   *sql.DB // Mistra PostgreSQL (loader schema)
	grappaDB   *sql.DB // Grappa MySQL
	anisettaDB *sql.DB // Anisetta PostgreSQL
}

// RegisterRoutes mounts all panoramica endpoints on the given mux.
func RegisterRoutes(mux *http.ServeMux, mistraDB, grappaDB, anisettaDB *sql.DB) {
	h := &Handler{mistraDB: mistraDB, grappaDB: grappaDB, anisettaDB: anisettaDB}
	protect := acl.RequireRole(applaunch.PanoramicaAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	// ── Mistra: Customers ──
	handle("GET /panoramica/v1/customers/with-invoices", h.handleListCustomersWithInvoices)
	handle("GET /panoramica/v1/customers/with-orders", h.handleListCustomersWithOrders)
	handle("GET /panoramica/v1/customers/with-access-lines", h.handleListCustomersWithAccessLines)

	// ── Mistra: Orders ──
	handle("GET /panoramica/v1/order-statuses", h.handleListOrderStatuses)
	handle("GET /panoramica/v1/orders/summary", h.handleListOrdersSummary)
	handle("GET /panoramica/v1/orders/detail", h.handleListOrdersDetail)

	// ── Mistra: Invoices ──
	handle("GET /panoramica/v1/invoices", h.handleListInvoices)

	// ── Mistra: Access Lines ──
	handle("GET /panoramica/v1/connection-types", h.handleListConnectionTypes)
	handle("GET /panoramica/v1/access-lines", h.handleListAccessLines)

	// ── Grappa: IaaS ──
	handle("GET /panoramica/v1/iaas/accounts", h.handleListIaaSAccounts)
	handle("GET /panoramica/v1/iaas/daily-charges", h.handleListDailyCharges)
	handle("GET /panoramica/v1/iaas/monthly-charges", h.handleListMonthlyCharges)
	handle("GET /panoramica/v1/iaas/charge-breakdown", h.handleChargeBreakdown)
	handle("GET /panoramica/v1/iaas/windows-licenses", h.handleListWindowsLicenses)

	// ── Anisetta: Timoo ──
	handle("GET /panoramica/v1/timoo/tenants", h.handleListTimooTenants)
	handle("GET /panoramica/v1/timoo/pbx-stats", h.handleGetPbxStats)
}

// ── Shared helpers ──

func (h *Handler) requireMistra(w http.ResponseWriter) bool {
	if h.mistraDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "mistra_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireGrappa(w http.ResponseWriter) bool {
	if h.grappaDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "grappa_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireAnisetta(w http.ResponseWriter) bool {
	if h.anisettaDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "anisetta_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	args := []any{"component", "panoramica", "operation", operation}
	args = append(args, attrs...)
	httputil.InternalError(w, r, err, "database operation failed", args...)
}

func (h *Handler) rowsDone(w http.ResponseWriter, r *http.Request, rows *sql.Rows, operation string) bool {
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, operation+"_rows", err)
		return false
	}
	return true
}
