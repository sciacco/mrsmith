package binocolo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/brave"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/llm"
	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/openapiit"
)

type Deps struct {
	OpenAPIIT  *openapiit.Client
	Brave      *brave.Client
	LLM        *llm.Service
	AnisettaDB *sql.DB
}

type Handler struct {
	openapiit          *openapiit.Client
	brave              *brave.Client
	companySearchCache companySearchCacheStore
	provinceCache      provinceCacheStore
	ateco              atecoStore
	ma                 *maService
}

// RegisterRoutes wires the binocolo HTTP routes and returns the deep-dive worker
// run function (or nil) so the caller can run it under the app context + worker
// WaitGroup for graceful shutdown. Callers that don't need it may ignore it.
func RegisterRoutes(mux *http.ServeMux, deps Deps) func(context.Context) {
	var cache companySearchCacheStore
	var provinceCache provinceCacheStore
	var maStore maWorkspaceStore
	var ateco atecoStore
	var sqlStore *SQLStore
	if deps.AnisettaDB != nil {
		sqlStore = NewSQLStore(deps.AnisettaDB)
		cache = sqlStore
		provinceCache = sqlStore
		maStore = sqlStore
		ateco = sqlStore
	}
	llmProvider := newMALLMProvider(deps.LLM)
	h := &Handler{
		openapiit:          deps.OpenAPIIT,
		brave:              deps.Brave,
		companySearchCache: cache,
		provinceCache:      provinceCache,
		ateco:              ateco,
		ma:                 newMAService(maStore, cache, provinceCache, ateco, deps.OpenAPIIT, llmProvider),
	}
	// Background workers, returned so main.go runs them under appCtx + workerWG for
	// graceful shutdown. Both are DB-backed and resume pending rows on restart:
	//   - maJobWorker drains the async session-job queue (estimate today) so the
	//     estimate runs off the request path and can't be killed by a client timeout.
	//   - maDeepWorker drains queued IT-full deep-dive jobs.
	var runners []func(context.Context)
	if sqlStore != nil {
		runners = append(runners, newMAJobWorker(h.ma, sqlStore).run)
		if deps.OpenAPIIT != nil {
			runners = append(runners, newMADeepWorker(sqlStore, deps.OpenAPIIT, llmProvider, h.ma.loadPricing).run)
		}
	}
	var runWorkers func(context.Context)
	if len(runners) > 0 {
		runWorkers = func(ctx context.Context) {
			var wg sync.WaitGroup
			for _, run := range runners {
				wg.Add(1)
				go func(run func(context.Context)) {
					defer wg.Done()
					run(ctx)
				}(run)
			}
			wg.Wait()
		}
	}
	protect := acl.RequireRole(applaunch.BinocoloAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	handle("GET /binocolo/v1/provinces", h.handleListProvinces)
	handle("GET /binocolo/v1/companies/search", h.handleSearchCompanies)
	handle("GET /binocolo/v1/ma/llm-options", h.handleListMALLMOptions)
	handle("GET /binocolo/v1/ma/parameters", h.handleListMAParameters)
	handle("GET /binocolo/v1/ma/ateco/search", h.handleSearchAteco)
	handle("PUT /binocolo/v1/ma/parameters", h.handleUpdateMAParameter)
	handle("GET /binocolo/v1/ma/sessions", h.handleListMASessions)
	handle("POST /binocolo/v1/ma/sessions", h.handleCreateMASession)
	handle("GET /binocolo/v1/ma/sessions/{id}", h.handleGetMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/archive", h.handleArchiveMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/restore", h.handleRestoreMASession)
	handle("DELETE /binocolo/v1/ma/sessions/{id}", h.handleDeleteMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/purge", h.handlePurgeMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/rating", h.handleRateMATarget)
	handle("POST /binocolo/v1/ma/sessions/{id}/deep-dive", h.handleDeepDiveMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/estimate", h.handleEstimateMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/execute", h.handleExecuteMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/export", h.handleExportMASession)
	handle("POST /binocolo/v1/ma/deep/recompute", h.handleRecomputeMADeep)
	handle("POST /binocolo/v1/ma/deep/regenerate-briefs", h.handleRegenerateMADeepBriefs)
	handle("GET /binocolo/v1/companies/{vat}/dossier", h.handleGetCompanyDossier)
	handle("POST /binocolo/v1/companies/{vat}/dossier", h.handleCreateCompanyDossier)
	handle("POST /binocolo/v1/test/domain-resolution", h.handleTestDomainResolution)
	handle("POST /binocolo/v1/web-search", h.handleWebSearch)
	return runWorkers
}

func (h *Handler) requireOpenAPIIT(w http.ResponseWriter) bool {
	if h.openapiit == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "openapiit_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireBrave(w http.ResponseWriter) bool {
	if h.brave == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "brave_not_configured")
		return false
	}
	return true
}

func (h *Handler) braveFailure(w http.ResponseWriter, r *http.Request, operation string, err error) {
	logging.FromContext(r.Context()).Warn(
		"brave request failed",
		"component", "binocolo",
		"operation", operation,
		"error", err,
	)
	httputil.Error(w, http.StatusBadGateway, "brave_upstream_error")
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

func (h *Handler) handleListMAParameters(w http.ResponseWriter, r *http.Request) {
	items, err := h.ma.listParameters(r.Context())
	if err != nil {
		h.maFailure(w, r, "ma_parameters_list", err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) handleUpdateMAParameter(w http.ResponseWriter, r *http.Request) {
	var body MAParameterUpdateRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var ok bool
	r, ok = h.startMATrace(w, r, "ma_parameter_update", "", body, subject, email)
	if !ok {
		return
	}
	if err := h.ma.updateParameter(r.Context(), body.Key, body.Value, subject, email); err != nil {
		h.maFailure(w, r, "ma_parameter_update", err)
		return
	}
	h.completeMATraceSuccess(r, http.StatusNoContent)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleListMASessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.ma.listSessions(r.Context(), r.URL.Query().Get("visibility"))
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
	var ok bool
	r, ok = h.startMATrace(w, r, "ma_session_create", "", body, subject, email)
	if !ok {
		return
	}
	detail, err := h.ma.createSession(r.Context(), body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_session_create", err)
		return
	}
	h.completeMATraceSuccess(r, http.StatusCreated)
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

func (h *Handler) handleArchiveMASession(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	if err := h.ma.archiveSession(r.Context(), id, subject, email); err != nil {
		h.maFailure(w, r, "ma_session_archive", err, "session_id", id)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleRestoreMASession(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	if err := h.ma.restoreSession(r.Context(), id, subject, email); err != nil {
		h.maFailure(w, r, "ma_session_restore", err, "session_id", id)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleDeleteMASession(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	if err := h.ma.softDeleteSession(r.Context(), id, subject, email); err != nil {
		h.maFailure(w, r, "ma_session_delete", err, "session_id", id)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handlePurgeMASession(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	if err := h.ma.purgeSession(r.Context(), id, subject, email); err != nil {
		h.maFailure(w, r, "ma_session_purge", err, "session_id", id)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleRateMATarget(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	var body MATargetRatingRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	if err := h.ma.setTargetRating(r.Context(), id, body.CompanyKey, body.Rating, subject, email); err != nil {
		h.maFailure(w, r, "ma_target_rate", err, "session_id", id)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleDeepDiveMASession(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	var body MADeepDiveRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_session_deep_dive", id, body, subject, email)
	if !traceOK {
		return
	}
	detail, err := h.ma.deepDive(r.Context(), id, body.AcknowledgeCost, email)
	if err != nil {
		h.maFailure(w, r, "ma_session_deep_dive", err, "session_id", id)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, detail)
}

// handleRecomputeMADeep rebuilds the deterministic scorecard for every cached deep
// analysis from its stored IT-full payload (no vendor call, no charge). Used to roll
// out engine calibration fixes. Gated by the standard binocolo access role.
func (h *Handler) handleRecomputeMADeep(w http.ResponseWriter, r *http.Request) {
	subject, email := companySearchRefreshActor(r.Context())
	var ok bool
	r, ok = h.startMATrace(w, r, "ma_deep_recompute", "", nil, subject, email)
	if !ok {
		return
	}
	count, err := h.ma.recomputeMADeepScorecards(r.Context())
	if err != nil {
		h.maFailure(w, r, "ma_deep_recompute", err)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, map[string]any{"recomputed": count})
}

// handleRegenerateMADeepBriefs re-runs the LLM brief for every cached analysis from its
// stored payload (no IT-full call). Used to roll out a new brief prompt after a prompt
// change. Gated by the standard binocolo access role.
func (h *Handler) handleRegenerateMADeepBriefs(w http.ResponseWriter, r *http.Request) {
	subject, email := companySearchRefreshActor(r.Context())
	var ok bool
	r, ok = h.startMATrace(w, r, "ma_deep_regenerate_briefs", "", nil, subject, email)
	if !ok {
		return
	}
	count, err := h.ma.regenerateMADeepBriefs(r.Context())
	if err != nil {
		h.maFailure(w, r, "ma_deep_regenerate_briefs", err)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, map[string]any{"regenerated": count})
}

// handleGetCompanyDossier returns the cached dossier state for a P.IVA (poll target);
// it never triggers a paid lookup.
func (h *Handler) handleGetCompanyDossier(w http.ResponseWriter, r *http.Request) {
	vat, ok := maVATParam(w, r)
	if !ok {
		return
	}
	dossier, err := h.ma.getCompanyDossier(r.Context(), vat)
	if err != nil {
		h.maFailure(w, r, "ma_company_dossier_get", err)
		return
	}
	httputil.JSON(w, http.StatusOK, dossier)
}

// handleCreateCompanyDossier is the standalone P.IVA lookup: cache-first, and on a
// miss it queues a fresh IT-full deep-dive once the caller acknowledges the cost.
func (h *Handler) handleCreateCompanyDossier(w http.ResponseWriter, r *http.Request) {
	vat, ok := maVATParam(w, r)
	if !ok {
		return
	}
	var body MADeepDiveRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeMABody(r, &body); err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_json")
			return
		}
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_company_dossier", "", body, subject, email)
	if !traceOK {
		return
	}
	dossier, err := h.ma.companyDossier(r.Context(), vat, body.AcknowledgeCost, email)
	if err != nil {
		h.maFailure(w, r, "ma_company_dossier", err)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, dossier)
}

// maVATParam reads and validates the {vat} path segment: an 11-digit partita IVA /
// codice fiscale, or a 16-char individual codice fiscale.
func maVATParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	vat := strings.ToUpper(strings.TrimSpace(r.PathValue("vat")))
	if !isValidVATOrTax(vat) {
		httputil.Error(w, http.StatusBadRequest, "invalid_vat")
		return "", false
	}
	return vat, true
}

func isValidVATOrTax(v string) bool {
	switch len(v) {
	case 11:
		for _, c := range v {
			if c < '0' || c > '9' {
				return false
			}
		}
		return true
	case 16:
		for _, c := range v {
			if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')) {
				return false
			}
		}
		return true
	default:
		return false
	}
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
	// Estimate runs asynchronously (maJobWorker): the request only validates and
	// enqueues, so it returns immediately and cannot time out while the surface
	// probing happens — nor can a client disconnect cancel the work. The UI polls
	// GET .../sessions/{id} until status leaves 'estimating'. The operation trace is
	// owned by the worker, not this request.
	detail, err := h.ma.enqueueEstimate(r.Context(), id, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_session_estimate", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusAccepted, detail)
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
	// Execute runs asynchronously (maJobWorker): the request validates (estimate
	// freshness, surface ceiling, budget gate) and enqueues, so it returns at once
	// and cannot time out during the paid company fetch. The UI polls
	// GET .../sessions/{id} until status leaves 'running'. The operation trace is
	// owned by the worker, not this request.
	detail, err := h.ma.enqueueExecute(r.Context(), id, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_session_execute", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusAccepted, detail)
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
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_session_export", id, body, subject, email)
	if !traceOK {
		return
	}
	content, filename, contentType, err := h.ma.exportSession(r.Context(), id, body.Format, email)
	if err != nil {
		h.maFailure(w, r, "ma_session_export", err, "session_id", id)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (h *Handler) startMATrace(w http.ResponseWriter, r *http.Request, operation string, sessionID string, request any, subject, email string) (*http.Request, bool) {
	trace, err := h.ma.startTrace(r.Context(), maTraceStart{
		RequestID:        logging.RequestID(r.Context()),
		Operation:        operation,
		Method:           r.Method,
		Path:             r.URL.Path,
		SessionID:        sessionID,
		CreatedBySubject: subject,
		CreatedByEmail:   email,
		Request:          maTraceJSON(request),
	})
	if err != nil {
		h.maFailure(w, r, operation, err)
		return r, false
	}
	logging.AddAccessLogAttrs(r.Context(), "ma_trace_id", trace.id)
	return r.WithContext(withMATrace(r.Context(), trace)), true
}

func (h *Handler) completeMATraceSuccess(r *http.Request, status int) {
	if err := h.ma.completeTrace(r.Context(), maTraceComplete{Status: maTraceStatusSucceeded, HTTPStatus: status}); err != nil {
		logging.FromContext(r.Context()).Error("binocolo ma trace completion failed", "component", "binocolo", "operation", "ma_trace_complete", "trace_id", maTraceID(r.Context()), "error", err)
	}
}

func (h *Handler) maFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	status, code, logLevel := maHTTPError(err)
	if completeErr := h.ma.completeTrace(r.Context(), maTraceComplete{
		Status:       maTraceStatusFailed,
		HTTPStatus:   status,
		ErrorCode:    code,
		ErrorMessage: err.Error(),
	}); completeErr != nil {
		logging.FromContext(r.Context()).Error("binocolo ma trace completion failed", "component", "binocolo", "operation", "ma_trace_complete", "trace_id", maTraceID(r.Context()), "error", completeErr)
	}
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
	if errors.Is(err, errAtecoStoreUnavailable) {
		return http.StatusServiceUnavailable, "binocolo_ateco_not_configured", "warn"
	}
	if errors.Is(err, errMAEstimateTooLarge) {
		return http.StatusBadRequest, "estimate_too_large", "warn"
	}
	if errors.Is(err, errMAEstimateOverBudget) {
		return http.StatusConflict, "estimate_over_budget", "warn"
	}
	if errors.Is(err, errMAVisibilityInvalid) {
		return http.StatusBadRequest, "invalid_ma_visibility", "warn"
	}
	if errors.Is(err, errMASessionArchived) {
		return http.StatusConflict, "ma_session_archived", "warn"
	}
	if errors.Is(err, errMASessionDeleted) {
		return http.StatusConflict, "ma_session_deleted", "warn"
	}
	if errors.Is(err, errAtecoCodeNotFound) {
		return http.StatusBadRequest, "invalid_ateco_code", "warn"
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
	var openRouterErr *llm.APIError
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
