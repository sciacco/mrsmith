package notifications

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const (
	defaultListLimit = 30
	maxListLimit     = 100
)

type Handler struct {
	store  Store
	logger *slog.Logger
}

func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	store := deps.Store
	if store == nil && deps.DB != nil {
		store = NewSQLStore(deps.DB)
	}
	h := &Handler{store: store, logger: logger.With("component", component)}

	mux.HandleFunc("GET /notifications/v1/summary", h.handleSummary)
	mux.HandleFunc("GET /notifications/v1/items", h.handleList)
	mux.HandleFunc("POST /notifications/v1/items/{id}/read", h.handleRead)
	mux.HandleFunc("POST /notifications/v1/items/read-all", h.handleReadAll)
	mux.HandleFunc("POST /notifications/v1/items/{id}/archive", h.handleArchive)
}

func (h *Handler) handleSummary(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	email, ok := currentUserEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "missing_user_email")
		return
	}
	summary, err := h.store.Summary(r.Context(), email)
	if err != nil {
		h.handleStoreError(w, r, err, "summary")
		return
	}
	httputil.JSON(w, http.StatusOK, summary)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	email, ok := currentUserEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "missing_user_email")
		return
	}
	query := r.URL.Query()
	status, ok := parseListStatus(query.Get("status"))
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_status")
		return
	}
	limit := parseLimit(query.Get("limit"))
	cursorCreatedAt, cursorID, err := decodeCursor(query.Get("cursor"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_cursor")
		return
	}
	result, err := h.store.List(r.Context(), ListInput{
		Email:           email,
		Status:          status,
		AppID:           strings.TrimSpace(query.Get("app_id")),
		Limit:           limit,
		CursorCreatedAt: cursorCreatedAt,
		CursorID:        cursorID,
	})
	if err != nil {
		h.handleStoreError(w, r, err, "list")
		return
	}
	var nextCursor string
	if result.HasNext {
		nextCursor = encodeCursor(result.NextCreatedAt, result.NextID)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"items":      result.Items,
		"nextCursor": nextCursor,
	})
}

func (h *Handler) handleRead(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	email, ok := currentUserEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "missing_user_email")
		return
	}
	id, ok := parseRecipientID(w, r)
	if !ok {
		return
	}
	updated, err := h.store.MarkRead(r.Context(), email, id)
	if err != nil {
		h.handleStoreError(w, r, err, "read")
		return
	}
	if !updated {
		httputil.Error(w, http.StatusNotFound, "notification_not_found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleReadAll(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	email, ok := currentUserEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "missing_user_email")
		return
	}
	count, err := h.store.MarkAllRead(r.Context(), email)
	if err != nil {
		h.handleStoreError(w, r, err, "read_all")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]int64{"updated": count})
}

func (h *Handler) handleArchive(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	email, ok := currentUserEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "missing_user_email")
		return
	}
	id, ok := parseRecipientID(w, r)
	if !ok {
		return
	}
	updated, err := h.store.Archive(r.Context(), email, id)
	if err != nil {
		h.handleStoreError(w, r, err, "archive")
		return
	}
	if !updated {
		httputil.Error(w, http.StatusNotFound, "notification_not_found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) requireStore(w http.ResponseWriter) bool {
	if h.store != nil {
		return true
	}
	httputil.Error(w, http.StatusServiceUnavailable, "notifications_database_not_configured")
	return false
}

func (h *Handler) handleStoreError(w http.ResponseWriter, r *http.Request, err error, operation string) {
	if isStoreNotReady(err) {
		h.requestLogger(r, operation).Warn("notifications database not ready", "error", err)
		httputil.Error(w, http.StatusServiceUnavailable, "notifications_database_not_ready")
		return
	}
	httputil.InternalError(w, r, err, "notifications request failed", "component", component, "operation", operation)
}

func (h *Handler) requestLogger(r *http.Request, operation string) *slog.Logger {
	if h.logger != nil {
		return h.logger.With("request_id", logging.RequestID(r.Context()), "operation", operation)
	}
	return logging.FromContext(r.Context()).With("component", component, "operation", operation)
}

func currentUserEmail(r *http.Request) (string, bool) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		return "", false
	}
	email := normalizeEmail(claims.Email)
	if email == "" {
		return "", false
	}
	return email, true
}

func parseListStatus(value string) (ListStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(ListStatusAll):
		return ListStatusAll, true
	case string(ListStatusUnread):
		return ListStatusUnread, true
	default:
		return "", false
	}
}

func parseLimit(value string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || limit <= 0 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}

func parseRecipientID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_notification_id")
		return 0, false
	}
	return id, true
}

type listCursor struct {
	CreatedAt string `json:"createdAt"`
	ID        int64  `json:"id"`
}

func encodeCursor(createdAt time.Time, id int64) string {
	raw, _ := json.Marshal(listCursor{
		CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
		ID:        id,
	})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(value string) (time.Time, int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return time.Time{}, 0, err
	}
	var cursor listCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return time.Time{}, 0, err
	}
	if cursor.ID <= 0 {
		return time.Time{}, 0, errors.New("cursor id is required")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, cursor.CreatedAt)
	if err != nil || createdAt.IsZero() {
		return time.Time{}, 0, errors.New("cursor timestamp is required")
	}
	return createdAt, cursor.ID, nil
}

func isStoreNotReady(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "3F000", // invalid_schema_name
		"42501", // insufficient_privilege
		"42P01", // undefined_table
		"42703": // undefined_column
		return true
	default:
		return false
	}
}
