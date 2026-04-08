package listini

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// Handler holds references to both databases and optional services.
type Handler struct {
	mistraDB *sql.DB         // Mistra PostgreSQL
	grappaDB *sql.DB         // Grappa MySQL
	hubspot  *HubSpotService // nil if HUBSPOT_API_KEY not set
	carbone  *CarboneService // nil if CARBONE_API_KEY not set
}

// RegisterRoutes mounts all listini endpoints on the given mux.
func RegisterRoutes(mux *http.ServeMux, mistraDB, grappaDB *sql.DB, hubspot *HubSpotService, carbone *CarboneService) {
	h := &Handler{mistraDB: mistraDB, grappaDB: grappaDB, hubspot: hubspot, carbone: carbone}
	protect := acl.RequireRole(applaunch.ListiniAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	// ── Mistra: Customers ──
	handle("GET /listini/v1/customers", h.handleListCustomers)
	handle("GET /listini/v1/customers/erp-linked", h.handleListERPLinkedCustomers)

	// ── Mistra: Kits ──
	handle("GET /listini/v1/kits", h.handleListKits)
	handle("GET /listini/v1/kits/{id}/products", h.handleGetKitProducts)
	handle("GET /listini/v1/kits/{id}/help-url", h.handleGetKitHelpURL)
	handle("POST /listini/v1/kits/{id}/pdf", h.handleGenerateKitPDF)

	// ── Mistra: Customer Groups ──
	handle("GET /listini/v1/customer-groups", h.handleListCustomerGroups)
	handle("GET /listini/v1/customer-groups/{id}/kit-discounts", h.handleListKitDiscountsByGroup)
	handle("GET /listini/v1/customers/{id}/groups", h.handleGetCustomerGroups)
	handle("PATCH /listini/v1/customers/{id}/groups", h.handleSyncCustomerGroups)

	// ── Mistra: Credits ──
	handle("GET /listini/v1/customers/{id}/credit", h.handleGetCreditBalance)
	handle("GET /listini/v1/customers/{id}/transactions", h.handleListTransactions)
	handle("POST /listini/v1/customers/{id}/transactions", h.handleCreateTransaction)

	// ── Mistra: Timoo ──
	handle("GET /listini/v1/customers/{id}/pricing/timoo", h.handleGetTimooPricing)
	handle("PUT /listini/v1/customers/{id}/pricing/timoo", h.handleUpsertTimooPricing)

	// ── Grappa: Customers ──
	handle("GET /listini/v1/grappa/customers", h.handleListGrappaCustomers)

	// ── Grappa: IaaS Pricing ──
	handle("GET /listini/v1/grappa/customers/{id}/iaas-pricing", h.handleGetIaaSPricing)
	handle("POST /listini/v1/grappa/customers/{id}/iaas-pricing", h.handleUpsertIaaSPricing)

	// ── Grappa: IaaS Accounts ──
	handle("GET /listini/v1/grappa/iaas-accounts", h.handleListIaaSAccounts)
	handle("PATCH /listini/v1/grappa/iaas-accounts/credits", h.handleBatchUpdateIaaSCredits)

	// ── Grappa: Racks ──
	handle("GET /listini/v1/grappa/rack-customers", h.handleListRackCustomers)
	handle("GET /listini/v1/grappa/customers/{id}/racks", h.handleListCustomerRacks)
	handle("PATCH /listini/v1/grappa/racks/discounts", h.handleBatchUpdateRackDiscounts)
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

func pathID(r *http.Request, name string) (int, error) {
	return strconv.Atoi(r.PathValue(name))
}

func decodeBody(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	args := []any{"component", "listini", "operation", operation}
	args = append(args, attrs...)
	httputil.InternalError(w, r, err, "database operation failed", args...)
}

func (h *Handler) rowError(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return true
	}
	h.dbFailure(w, r, operation, err, attrs...)
	return true
}

func (h *Handler) rowsDone(w http.ResponseWriter, r *http.Request, rows *sql.Rows, operation string, attrs ...any) bool {
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, operation, err, attrs...)
		return false
	}
	return true
}

func (h *Handler) rollbackTx(r *http.Request, tx *sql.Tx, operation string, attrs ...any) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		args := []any{"component", "listini", "operation", operation, "error", err}
		args = append(args, attrs...)
		logging.FromContext(r.Context()).Warn("transaction rollback failed", args...)
	}
}
