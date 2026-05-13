package diagnostics

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/authz"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const (
	defaultListLimit = 100
	maxListLimit     = 500
)

type Deps struct {
	DB     *sql.DB
	Store  Store
	Sink   *Sink
	Logger *slog.Logger
}

type Handler struct {
	store  Store
	sink   *Sink
	logger *slog.Logger
}

func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	store := deps.Store
	if store == nil && deps.DB != nil {
		store = NewSQLStore(deps.DB)
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	h := &Handler{store: store, sink: deps.Sink, logger: logger.With("component", component)}
	protect := acl.RequireRole(authz.DevAdminRole)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}
	handle("GET /diagnostics/v1/events", h.handleListEvents)
	handle("GET /diagnostics/v1/events/{id}", h.handleGetEvent)
	handle("GET /diagnostics/v1/status", h.handleStatus)
}

func (h *Handler) handleListEvents(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	filter, ok := parseListFilter(w, r)
	if !ok {
		return
	}
	events, err := h.store.ListEvents(r.Context(), filter)
	if err != nil {
		h.handleStoreError(w, r, err, "list_events")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"events": events})
}

func (h *Handler) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_event_id")
		return
	}
	event, found, err := h.store.GetEvent(r.Context(), id)
	if err != nil {
		h.handleStoreError(w, r, err, "get_event")
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, event)
}

func (h *Handler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	status := SinkStatus{}
	if h.sink != nil {
		status = h.sink.Status()
	}
	httputil.JSON(w, http.StatusOK, status)
}

func parseListFilter(w http.ResponseWriter, r *http.Request) (ListFilter, bool) {
	query := r.URL.Query()
	since := time.Now().Add(-24 * time.Hour)
	if raw := strings.TrimSpace(query.Get("since")); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_since")
			return ListFilter{}, false
		}
		since = parsed
	}
	var before time.Time
	if raw := strings.TrimSpace(query.Get("before")); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_before")
			return ListFilter{}, false
		}
		before = parsed
	}
	limit := defaultListLimit
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			httputil.Error(w, http.StatusBadRequest, "invalid_limit")
			return ListFilter{}, false
		}
		limit = parsed
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	level := strings.ToUpper(strings.TrimSpace(query.Get("level")))
	if level != "" && level != "WARN" && level != "ERROR" {
		httputil.Error(w, http.StatusBadRequest, "invalid_level")
		return ListFilter{}, false
	}
	return ListFilter{
		Level:     level,
		Component: strings.TrimSpace(query.Get("component")),
		Operation: strings.TrimSpace(query.Get("operation")),
		RequestID: strings.TrimSpace(query.Get("request_id")),
		Path:      strings.TrimSpace(query.Get("path")),
		Since:     since,
		Before:    before,
		Limit:     limit,
	}, true
}

func (h *Handler) requireStore(w http.ResponseWriter) bool {
	if h.store == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "diagnostics_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) handleStoreError(w http.ResponseWriter, r *http.Request, err error, operation string) {
	if isDiagnosticsStoreNotReady(err) {
		h.requestLogger(r, operation).Warn("diagnostics database not ready", "error", err)
		httputil.Error(w, http.StatusServiceUnavailable, "diagnostics_database_not_ready")
		return
	}
	httputil.InternalError(w, r, err, "diagnostics request failed", "component", component, "operation", operation)
}

func (h *Handler) requestLogger(r *http.Request, operation string) *slog.Logger {
	if h.logger != nil {
		return h.logger.With("request_id", logging.RequestID(r.Context()), "operation", operation)
	}
	return logging.FromContext(r.Context()).With("component", component, "operation", operation)
}

func isDiagnosticsStoreNotReady(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "3F000", // invalid_schema_name
		"42501", // insufficient_privilege
		"42P01": // undefined_table
		return true
	default:
		return false
	}
}
