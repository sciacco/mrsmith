package reports

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// Handler holds references to all three databases and the Carbone XLSX service.
type Handler struct {
	mistraDB   *sql.DB // Mistra PostgreSQL (loader schema)
	grappaDB   *sql.DB // Grappa MySQL
	anisettaDB *sql.DB // Anisetta PostgreSQL
	carbone    *CarboneService
}

// RegisterRoutes mounts all reports endpoints on the given mux.
func RegisterRoutes(mux *http.ServeMux, mistraDB, grappaDB, anisettaDB *sql.DB, carbone *CarboneService) {
	h := &Handler{mistraDB: mistraDB, grappaDB: grappaDB, anisettaDB: anisettaDB, carbone: carbone}
	protect := acl.RequireRole(applaunch.ReportsAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	// -- Lookups --
	handle("GET /reports/v1/order-statuses", h.handleOrderStatuses)
	handle("GET /reports/v1/connection-types", h.handleConnectionTypes)

	// -- Orders --
	handle("POST /reports/v1/orders/preview", h.handleOrdersPreview)
	handle("POST /reports/v1/orders/export", h.handleOrdersExport)

	// -- Active Lines --
	handle("POST /reports/v1/active-lines/preview", h.handleActiveLinesPreview)
	handle("POST /reports/v1/active-lines/export", h.handleActiveLinesExport)

	// -- Pending Activations --
	handle("GET /reports/v1/pending-activations", h.handlePendingActivations)
	handle("GET /reports/v1/pending-activations/{orderNumber}/rows", h.handlePendingActivationRows)

	// -- Upcoming Renewals --
	handle("GET /reports/v1/upcoming-renewals", h.handleUpcomingRenewals)
	handle("GET /reports/v1/upcoming-renewals/{customerId}/rows", h.handleUpcomingRenewalRows)

	// -- MOR Anomalies --
	handle("GET /reports/v1/mor-anomalies", h.handleMorAnomalies)

	// -- Timoo --
	handle("GET /reports/v1/timoo/daily-stats", h.handleTimooDailyStats)

	// -- AOV --
	handle("POST /reports/v1/aov/preview", h.handleAovPreview)
}

// -- Shared helpers --

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
	args := []any{"component", "reports", "operation", operation}
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
