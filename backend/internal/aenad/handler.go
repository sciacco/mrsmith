package aenad

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const component = "aenad"

// pdfRenderer is satisfied by *CarboneService; kept as an interface for tests.
type pdfRenderer interface {
	GeneratePDF(ctx context.Context, templateID string, payload any) ([]byte, error)
}

type Deps struct {
	Mistra *sql.DB
	Logger *slog.Logger
	// ConfigDB is the Anisetta connection hosting mrsmith.runtime_config.
	ConfigDB *sql.DB
	Carbone  *CarboneService
}

type Handler struct {
	mistra   *sql.DB
	logger   *slog.Logger
	configDB *sql.DB
	carbone  pdfRenderer
}

func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	h := &Handler{
		mistra:   deps.Mistra,
		logger:   logger.With("component", component),
		configDB: deps.ConfigDB,
	}
	if deps.Carbone != nil {
		h.carbone = deps.Carbone
	}

	protect := acl.RequireRole(applaunch.AenadAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	handle("GET /aenad/v1/document-types", h.handleDocumentTypes)
	handle("GET /aenad/v1/documents", h.handleDocuments)
	handle("GET /aenad/v1/documents/{id}", h.handleGetDocument)
	handle("GET /aenad/v1/documents/{id}/rows", h.handleDocumentRows)
	handle("GET /aenad/v1/documents/{id}/pdf", h.handleDocumentPDF)
	handle("PUT /aenad/v1/documents/{id}", h.handleUpdateDocument)
	handle("GET /aenad/v1/customers", h.handleCustomers)
	handle("GET /aenad/v1/payment-methods", h.handlePaymentMethods)
}

func (h *Handler) requireMistra(w http.ResponseWriter) bool {
	if h.mistra == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "aenad_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	args := []any{"component", component, "operation", operation}
	args = append(args, attrs...)
	httputil.InternalError(w, r, err, "database operation failed", args...)
}
