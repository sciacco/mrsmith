package quotes

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/hubspot"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// Handler holds dependencies for all quotes endpoints.
type Handler struct {
	db        *sql.DB         // Mistra PostgreSQL (quotes, products, loader schemas)
	alyanteDB *sql.DB         // Alyante ERP MSSQL (read-only, optional)
	hs        *hubspot.Client // HubSpot API client (optional)
}

// RegisterRoutes mounts all quotes endpoints on the given mux.
func RegisterRoutes(mux *http.ServeMux, db, alyanteDB *sql.DB, hs *hubspot.Client) {
	h := &Handler{db: db, alyanteDB: alyanteDB, hs: hs}
	protect := acl.RequireRole(applaunch.QuotesAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	// ── Reference data (read-only) ──
	handle("GET /quotes/v1/templates", h.handleListTemplates)
	handle("GET /quotes/v1/categories", h.handleListCategories)
	handle("GET /quotes/v1/kits", h.handleListKits)
	handle("GET /quotes/v1/customers", h.handleListCustomers)
	handle("GET /quotes/v1/deals", h.handleListDeals)
	handle("GET /quotes/v1/deals/{id}", h.handleGetDeal)
	handle("GET /quotes/v1/owners", h.handleListOwners)
	handle("GET /quotes/v1/payment-methods", h.handleListPaymentMethods)
	handle("GET /quotes/v1/customer-payment/{customerId}", h.handleCustomerPayment)
	handle("GET /quotes/v1/customer-orders/{customerId}", h.handleCustomerOrders)

	// ── Quote CRUD ──
	handle("GET /quotes/v1/quotes", h.handleListQuotes)
	handle("POST /quotes/v1/quotes", h.handleCreateQuote)
	handle("GET /quotes/v1/quotes/{id}", h.handleGetQuote)
	handle("PUT /quotes/v1/quotes/{id}", h.handleUpdateQuote)
	handle("GET /quotes/v1/quotes/{id}/hs-status", h.handleGetHSStatus)

	// ── Kit rows and products ──
	handle("GET /quotes/v1/quotes/{id}/rows", h.handleListRows)
	handle("POST /quotes/v1/quotes/{id}/rows", h.handleAddRow)
	handle("DELETE /quotes/v1/quotes/{id}/rows/{rowId}", h.handleDeleteRow)
	handle("PUT /quotes/v1/quotes/{id}/rows/{rowId}/position", h.handleUpdateRowPosition)
	handle("GET /quotes/v1/quotes/{id}/rows/{rowId}/products", h.handleListProducts)
	handle("PUT /quotes/v1/quotes/{id}/rows/{rowId}/products/{productId}", h.handleUpdateProduct)

	// ── Publish ──
	handle("POST /quotes/v1/quotes/{id}/publish", h.handlePublish)

	// ── Delete ──
	handle("DELETE /quotes/v1/quotes/{id}", h.handleDeleteQuote)
}

// ── Shared helpers ──

func (h *Handler) requireDB(w http.ResponseWriter) bool {
	if h.db == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "mistra_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireAlyante(w http.ResponseWriter) bool {
	if h.alyanteDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "alyante_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireHS(w http.ResponseWriter) bool {
	if h.hs == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "hubspot_not_configured")
		return false
	}
	return true
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	args := []any{"component", "quotes", "operation", operation}
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
