// Package afctools implements the AFC Tools mini-app backend.
// It is a 1:1 port of the legacy Appsmith app; see
// apps/afc-tools/afc-tools-migspec.md for the approved migration spec.
package afctools

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// Deps bundles every external dependency the afctools handlers need.
// Individual fields may be nil when the backing datasource is not configured;
// the per-endpoint handlers return 503 in that case.
type Deps struct {
	Vodka   *sql.DB
	Whmcs   *sql.DB
	Mistra  *sql.DB
	Grappa  *sql.DB
	Alyante *sql.DB
	Carbone *CarboneService
	Arak    *arak.Client
}

type Handler struct {
	deps Deps
}

// RegisterRoutes mounts every AFC Tools endpoint on the given mux.
// All routes are gated by the Keycloak role `app_afctools_access`.
func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	h := &Handler{deps: deps}
	protect := acl.RequireRole(applaunch.AFCToolsAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	// WHMCS (Prometeus)
	handle("GET /afc-tools/v1/whmcs/transactions", h.handleTransactions)
	handle("POST /afc-tools/v1/whmcs/transactions/export", h.handleTransactionsExport)
	handle("GET /afc-tools/v1/whmcs/invoice-lines", h.handleInvoiceLines)

	// Mistra
	handle("GET /afc-tools/v1/mistra/missing-articles", h.handleMissingArticles)
	handle("GET /afc-tools/v1/mistra/xconnect/orders", h.handleXConnectOrders)

	// Gateway PDF proxies
	handle("GET /afc-tools/v1/tickets/{ticketId}/pdf", h.handleTicketPDF)
	handle("GET /afc-tools/v1/orders/{orderId}/pdf", h.handleOrderPDF)

	// Grappa — Energia Colo
	handle("GET /afc-tools/v1/energia-colo/pivot", h.handleEnergiaColoPivot)
	handle("GET /afc-tools/v1/energia-colo/detail", h.handleEnergiaColoDetail)

	// Vodka — Ordini Sales / Dettaglio
	handle("GET /afc-tools/v1/orders", h.handleOrders)
	handle("GET /afc-tools/v1/orders/{id}", h.handleOrderHeader)
	handle("GET /afc-tools/v1/orders/{id}/rows", h.handleOrderRows)

	// Alyante — Report DDT cespiti
	handle("GET /afc-tools/v1/ddt-cespiti", h.handleDdtCespiti)
}

// -- Shared helpers --

func (h *Handler) requireDB(w http.ResponseWriter, db *sql.DB, name string) bool {
	if db == nil {
		httputil.Error(w, http.StatusServiceUnavailable, name+"_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error) {
	httputil.InternalError(w, r, err, "database operation failed",
		"component", "afctools", "operation", operation)
}
