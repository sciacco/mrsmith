package support

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const (
	maxRequestBodyBytes = 96 * 1024
	maxMessageLength    = 4000
)

type Handler struct {
	store  Store
	mailer Mailer
	logger *slog.Logger
}

func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	var store Store
	if deps.DB != nil {
		store = NewSQLStore(deps.DB)
	}
	h := &Handler{
		store:  store,
		mailer: deps.Mailer,
		logger: logger.With("component", component),
	}
	mux.HandleFunc("POST /support/v1/requests", h.handleCreateRequest)
}

func (h *Handler) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "support_database_not_configured")
		return
	}

	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "missing_auth_claims")
		return
	}

	payload, err := decodeCreatePayload(w, r)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request")
		return
	}

	input, err := buildCreateInput(payload, claims)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	id, err := h.store.CreateRequest(r.Context(), input)
	if err != nil {
		if isSupportStoreNotReady(err) {
			h.requestLogger(r).Warn("support database not ready", "error", err)
			httputil.Error(w, http.StatusServiceUnavailable, "support_database_not_ready")
			return
		}
		h.internalError(w, r, err, "support_request_insert_failed")
		return
	}

	emailStatus := emailNotificationSkipped
	recipients, err := h.store.GetStringListConfig(r.Context(), configNamespaceSupport, configKeyEmailTo)
	if err != nil {
		emailStatus = emailNotificationFailed
		h.requestLogger(r).Warn("support notification config failed", "request_id", id, "error", err)
	} else {
		emailStatus, err = sendSupportNotification(r.Context(), h.mailer, input, id, recipients)
		if err != nil {
			h.requestLogger(r).Warn("support notification email failed", "request_id", id, "error", err)
		}
	}

	if err := h.store.UpdateEmailStatus(r.Context(), id, emailStatus, claims); err != nil {
		h.requestLogger(r).Warn("support email status update failed", "request_id", id, "email_status", emailStatus, "error", err)
	}

	httputil.JSON(w, http.StatusCreated, createRequestResponse{
		ID:                id,
		Status:            "open",
		EmailNotification: emailStatus,
	})
}

func decodeCreatePayload(w http.ResponseWriter, r *http.Request) (createRequestPayload, error) {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	var payload createRequestPayload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		return payload, err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return payload, errors.New("request body must contain a single JSON object")
	}
	return payload, nil
}

func buildCreateInput(payload createRequestPayload, claims auth.Claims) (CreateRequestInput, error) {
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		return CreateRequestInput{}, errors.New("message_required")
	}
	if len(message) > maxMessageLength {
		return CreateRequestInput{}, errors.New("message_too_long")
	}

	priority := normalizePriority(payload.Priority)
	if priority == "" {
		return CreateRequestInput{}, errors.New("invalid_priority")
	}

	technicalContextIncluded := true
	if payload.TechnicalContextIncluded != nil {
		technicalContextIncluded = *payload.TechnicalContextIncluded
	}

	contextValue, err := decodeAndSanitizeContext(payload.Context)
	if err != nil {
		return CreateRequestInput{}, errors.New("invalid_context")
	}

	contextMap, _ := contextValue.(map[string]any)
	appID := stringFromContext(contextMap, "app", "id")
	appName := stringFromContext(contextMap, "app", "name")
	pageURL := stringFromContext(contextMap, "page", "url")
	pagePath := stringFromContext(contextMap, "page", "path")
	if appID == "" {
		appID = "unknown"
	}
	if !technicalContextIncluded {
		contextValue = minimalContext(contextMap)
	}

	return CreateRequestInput{
		Priority:                 priority,
		AppID:                    appID,
		AppName:                  appName,
		PageURL:                  pageURL,
		PagePath:                 pagePath,
		Message:                  message,
		Requester:                claims,
		TechnicalContextIncluded: technicalContextIncluded,
		Context:                  contextValue,
	}, nil
}

func minimalContext(context map[string]any) map[string]any {
	result := map[string]any{}
	for _, key := range []string{"app", "page"} {
		if value, ok := context[key]; ok {
			result[key] = value
		}
	}
	return result
}

func normalizePriority(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "normal":
		return "normal"
	case "low":
		return "low"
	case "high":
		return "high"
	case "urgent":
		return "urgent"
	default:
		return ""
	}
}

func stringFromContext(context map[string]any, group string, key string) string {
	if context == nil {
		return ""
	}
	rawGroup, ok := context[group].(map[string]any)
	if !ok {
		return ""
	}
	rawValue, ok := rawGroup[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(rawValue)
}

func (h *Handler) internalError(w http.ResponseWriter, r *http.Request, err error, operation string) {
	httputil.InternalError(w, r, err, "support request failed", "component", component, "operation", operation)
}

func (h *Handler) requestLogger(r *http.Request) *slog.Logger {
	if h.logger != nil {
		return h.logger.With("request_id", logging.RequestID(r.Context()))
	}
	return logging.FromContext(r.Context()).With("component", component)
}

func isSupportStoreNotReady(err error) bool {
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
