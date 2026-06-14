package raenad

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const component = "raenad"

// PDFRenderer is satisfied by the Aenad Carbone service. Raenad keeps the
// dependency behind an interface so later slices can test PDF behavior without
// calling Carbone.
type PDFRenderer interface {
	GeneratePDF(ctx context.Context, templateID string, payload any) ([]byte, error)
}

// Deps bundles Raenad's external systems. Handlers must check only the
// dependencies needed by their route so unrelated missing services do not hide
// the skeleton endpoint.
type Deps struct {
	Mistra       *sql.DB
	ConfigDB     *sql.DB
	Alyante      *sql.DB
	HubSpot      HubSpotProspectClient
	HubSpotStage HubSpotStageClient
	Carbone      PDFRenderer
	Logger       *slog.Logger
}

type Handler struct {
	deps   Deps
	logger *slog.Logger
}

func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	h := &Handler{
		deps:   deps,
		logger: logger.With("component", component),
	}

	protect := acl.RequireRole(applaunch.AenadAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	handle("GET /aenad/v1/quotes/customers", h.handleQuoteCustomers)
	handle("POST /aenad/v1/quotes/prospects", h.handleQuoteProspects)
	handle("GET /aenad/v1/quotes/payment-methods", h.handleQuotePaymentMethods)
	handle("GET /aenad/v1/quotes/stages", h.handleQuoteStages)
	handle("GET /aenad/v1/quotes/defaults", h.handleQuoteDefaults)
	handle("GET /aenad/v1/quotes/articles", h.handleQuoteArticles)

	handle("GET /aenad/v1/quotes", h.handleQuoteList)
	handle("POST /aenad/v1/quotes", h.handleQuoteCreate)
	handle("GET /aenad/v1/quotes/{id}", h.handleQuoteGet)
	handle("PUT /aenad/v1/quotes/{id}", h.handleQuoteUpdate)
	handle("POST /aenad/v1/quotes/{id}/ready", h.handleQuoteReady)
	handle("POST /aenad/v1/quotes/{id}/hubspot/retry", h.handleHubSpotRetry)
	handle("POST /aenad/v1/quotes/{id}/hubspot/stage", h.handleStageTransition)

	handle("GET /aenad/v1/quotes/{id}/pdf-exports", h.handlePDFExportList)
	handle("POST /aenad/v1/quotes/{id}/pdf-exports", h.handlePDFExportCreate)
	handle("GET /aenad/v1/quotes/{id}/pdf-exports/{exportID}/download", h.handlePDFExportDownload)
	handle("POST /aenad/v1/quotes/{id}/pdf-exports/{exportID}/attach", h.handlePDFExportAttach)
}

func (h *Handler) handleMistraStub(w http.ResponseWriter, _ *http.Request) {
	if !h.requireMistra(w) {
		return
	}
	h.notImplemented(w)
}

func (h *Handler) handleMistraConfigStub(w http.ResponseWriter, _ *http.Request) {
	if !h.requireMistra(w) || !h.requireConfigDB(w) {
		return
	}
	h.notImplemented(w)
}

func (h *Handler) handleAlyanteConfigStub(w http.ResponseWriter, _ *http.Request) {
	if !h.requireAlyante(w) || !h.requireConfigDB(w) {
		return
	}
	h.notImplemented(w)
}

func (h *Handler) handleHubSpotStub(w http.ResponseWriter, _ *http.Request) {
	if !h.requireHubSpot(w) {
		return
	}
	h.notImplemented(w)
}

func (h *Handler) handleStageStub(w http.ResponseWriter, _ *http.Request) {
	if !h.requireMistra(w) || !h.requireConfigDB(w) || !h.requireHubSpotStage(w) {
		return
	}
	h.notImplemented(w)
}

func (h *Handler) handlePDFStub(w http.ResponseWriter, _ *http.Request) {
	if !h.requireMistra(w) || !h.requireCarbone(w) {
		return
	}
	h.notImplemented(w)
}

func (h *Handler) requireMistra(w http.ResponseWriter) bool {
	if h.deps.Mistra == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "raenad_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireConfigDB(w http.ResponseWriter) bool {
	if h.deps.ConfigDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "raenad_config_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireAlyante(w http.ResponseWriter) bool {
	if h.deps.Alyante == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "alyante_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireHubSpot(w http.ResponseWriter) bool {
	if h.deps.HubSpot == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "hubspot_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireHubSpotStage(w http.ResponseWriter) bool {
	if h.deps.HubSpotStage == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "hubspot_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireCarbone(w http.ResponseWriter) bool {
	if h.deps.Carbone == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "raenad_pdf_not_configured")
		return false
	}
	return true
}

func (h *Handler) notImplemented(w http.ResponseWriter) {
	httputil.Error(w, http.StatusNotImplemented, "raenad_endpoint_not_implemented")
}
