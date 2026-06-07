package aenad

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const component = "aenad"

type Deps struct {
	Mistra *sql.DB
	Logger *slog.Logger
}

type Handler struct {
	mistra *sql.DB
	logger *slog.Logger
}

func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	h := &Handler{
		mistra: deps.Mistra,
		logger: logger.With("component", component),
	}

	protect := acl.RequireRole(applaunch.AenadAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	handle("GET /aenad/v1/document-types", h.handleDocumentTypes)
	handle("GET /aenad/v1/documents", h.handleDocuments)
	handle("GET /aenad/v1/documents/{id}/rows", h.handleDocumentRows)
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
