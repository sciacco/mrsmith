package binocolo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/openapiit"
	"github.com/sciacco/mrsmith/internal/platform/openrouter"
)

type Deps struct {
	OpenAPIIT  *openapiit.Client
	OpenRouter *openrouter.Client
	AnisettaDB *sql.DB
}

type Handler struct {
	openapiit          *openapiit.Client
	companySearchCache companySearchCacheStore
	ma                 *maService
}

func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	var cache companySearchCacheStore
	var maStore maWorkspaceStore
	if deps.AnisettaDB != nil {
		store := NewSQLStore(deps.AnisettaDB)
		cache = store
		maStore = store
	}
	h := &Handler{
		openapiit:          deps.OpenAPIIT,
		companySearchCache: cache,
		ma:                 newMAService(maStore, cache, deps.OpenAPIIT, deps.OpenRouter),
	}
	protect := acl.RequireRole(applaunch.BinocoloAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	handle("GET /binocolo/v1/provinces", h.handleListProvinces)
	handle("GET /binocolo/v1/companies/search", h.handleSearchCompanies)
	handle("GET /binocolo/v1/ma/llm-options", h.handleListMALLMOptions)
	handle("GET /binocolo/v1/ma/sessions", h.handleListMASessions)
	handle("POST /binocolo/v1/ma/sessions", h.handleCreateMASession)
	handle("GET /binocolo/v1/ma/sessions/{id}", h.handleGetMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/estimate", h.handleEstimateMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/execute", h.handleExecuteMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/export", h.handleExportMASession)
}

func (h *Handler) requireOpenAPIIT(w http.ResponseWriter) bool {
	if h.openapiit == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "openapiit_not_configured")
		return false
	}
	return true
}

func (h *Handler) openAPIITFailure(w http.ResponseWriter, r *http.Request, operation string, err error) {
	logging.FromContext(r.Context()).Warn(
		"openapi.it request failed",
		"component", "binocolo",
		"operation", operation,
		"error", err,
	)
	httputil.Error(w, http.StatusBadGateway, "openapiit_upstream_error")
}

func (h *Handler) binocoloCacheFailure(w http.ResponseWriter, r *http.Request, err error) {
	logging.FromContext(r.Context()).Error(
		"binocolo cache request failed",
		"component", "binocolo",
		"error", err,
	)
	httputil.Error(w, http.StatusInternalServerError, "binocolo_cache_error")
}

func (h *Handler) handleListMALLMOptions(w http.ResponseWriter, r *http.Request) {
	options, err := h.ma.listLLMOptions(r.Context())
	if err != nil {
		h.maFailure(w, r, "ma_llm_options", err)
		return
	}
	httputil.JSON(w, http.StatusOK, options)
}

func (h *Handler) handleListMASessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.ma.listSessions(r.Context())
	if err != nil {
		h.maFailure(w, r, "ma_sessions_list", err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"items": sessions})
}

func (h *Handler) handleCreateMASession(w http.ResponseWriter, r *http.Request) {
	var body MACreateSessionRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	detail, err := h.ma.createSession(r.Context(), body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_session_create", err)
		return
	}
	httputil.JSON(w, http.StatusCreated, detail)
}

func (h *Handler) handleGetMASession(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	detail, err := h.ma.getSession(r.Context(), id)
	if err != nil {
		h.maFailure(w, r, "ma_session_get", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

func (h *Handler) handleEstimateMASession(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	var body MAEstimateSessionRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	detail, err := h.ma.estimateSession(r.Context(), id, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_session_estimate", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

func (h *Handler) handleExecuteMASession(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	var body MAExecuteSessionRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	detail, err := h.ma.executeSession(r.Context(), id, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_session_execute", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

func (h *Handler) handleExportMASession(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	var body MAExportRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeMABody(r, &body); err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_json")
			return
		}
	}
	_, email := companySearchRefreshActor(r.Context())
	content, filename, contentType, err := h.ma.exportSession(r.Context(), id, body.Format, email)
	if err != nil {
		h.maFailure(w, r, "ma_session_export", err, "session_id", id)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (h *Handler) maFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	status, code, logLevel := maHTTPError(err)
	logAttrs := []any{"component", "binocolo", "operation", operation, "error", err}
	logAttrs = append(logAttrs, attrs...)
	logger := logging.FromContext(r.Context())
	if logLevel == "warn" {
		logger.Warn("binocolo ma request failed", logAttrs...)
	} else {
		logger.Error("binocolo ma request failed", logAttrs...)
	}
	httputil.Error(w, status, code)
}

func maHTTPError(err error) (int, string, string) {
	if errors.Is(err, errMAStoreUnavailable) {
		return http.StatusServiceUnavailable, "binocolo_workspace_not_configured", "warn"
	}
	if errors.Is(err, errMAOpenAPIITUnavailable) {
		return http.StatusServiceUnavailable, "openapiit_not_configured", "warn"
	}
	if errors.Is(err, errMAOpenRouterUnavailable) {
		return http.StatusServiceUnavailable, "openrouter_not_configured", "warn"
	}
	if errors.Is(err, errMALLMConfigUnavailable) {
		return http.StatusServiceUnavailable, "binocolo_llm_config_not_configured", "warn"
	}
	if errors.Is(err, errMAEstimateTooLarge) {
		return http.StatusBadRequest, "estimate_too_large", "warn"
	}
	if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows in result set") {
		return http.StatusNotFound, "ma_session_not_found", "warn"
	}
	if errors.Is(err, errMAStrategyInvalid) {
		return http.StatusBadRequest, "invalid_ma_request", "warn"
	}
	var openAPIITErr *openapiit.APIError
	if errors.As(err, &openAPIITErr) {
		if openAPIITErr.StatusCode == http.StatusPaymentRequired {
			return http.StatusPaymentRequired, "openapiit_credit_required", "warn"
		}
		return http.StatusBadGateway, "openapiit_upstream_error", "warn"
	}
	var openRouterErr *openrouter.APIError
	if errors.As(err, &openRouterErr) {
		return http.StatusBadGateway, "openrouter_upstream_error", "warn"
	}
	return http.StatusInternalServerError, "binocolo_ma_error", "error"
}

func maSessionID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := strings.TrimSpace(r.PathValue("id"))
	if _, err := uuid.Parse(id); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_ma_session_id")
		return "", false
	}
	return id, true
}

func decodeMABody(r *http.Request, dst any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}
