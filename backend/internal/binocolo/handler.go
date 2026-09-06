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
	"time"

	"github.com/google/uuid"
	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/brave"
	"github.com/sciacco/mrsmith/internal/platform/googledrive"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/llm"
	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/openapiit"
	"github.com/sciacco/mrsmith/internal/platform/scrape"
)

type Deps struct {
	OpenAPIIT   *openapiit.Client
	Brave       *brave.Client
	Scrape      *scrape.Client
	LLM         *llm.Service
	GoogleDrive *googledrive.Service
	AnisettaDB  *sql.DB
	// InstanceOwner stamps enqueued ma_job rows with this instance's stable identity
	// (config InstanceOwner) so they are pre-leased to it and foreign workers on the
	// shared Anisetta DB can't steal them. Empty tolerated (see EnqueueMAJob).
	InstanceOwner string
	// FilingDocAICompare abilita il confronto OCR<->DocAI nell'ingest bilanci
	// (issue #78). FilingDocumentID è il documentId DocuEngine del "Bilancio Ottico"
	// pinnato per environment. Non ancora usati (F2): consumati dalle fasi 3-5.
	FilingDocAICompare bool
	FilingDocumentID   string
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
	h.ma.brave = deps.Brave
	h.ma.owner = deps.InstanceOwner
	// Guard the assignment: a nil *scrape.Client stored in the interface field would
	// be a non-nil interface (typed-nil), defeating the s.scrape == nil fallback.
	if deps.Scrape != nil {
		h.ma.scrape = deps.Scrape
	}
	// Google Drive documents (issue #98). Soft dependency: nil degrades the
	// documents surface to a not_configured (503) state, the Scheda is unaffected.
	if deps.GoogleDrive != nil {
		h.ma.drive = deps.GoogleDrive
	}
	if sqlStore != nil {
		h.ma.kb = sqlStore
		// Deposited-filing pipeline (issue #78): the filing lifecycle store and the
		// pinned "Bilancio Ottico" documentId. filingDocumentID empty => the DocuEngine
		// channel is not configured and a filing_search fails definitively.
		h.ma.filing = sqlStore
		h.ma.filingDocumentID = deps.FilingDocumentID
		h.ma.filingDocAICompare = deps.FilingDocAICompare
	}
	// Background workers, returned so main.go runs them under appCtx + workerWG for
	// graceful shutdown. Both are DB-backed and resume pending rows on restart:
	//   - maJobWorker drains the async session-job queue (estimate today) so the
	//     estimate runs off the request path and can't be killed by a client timeout.
	//   - maDeepWorker drains queued IT-full deep-dive jobs.
	var runners []func(context.Context)
	if sqlStore != nil {
		runners = append(runners, newMAJobWorker(h.ma, sqlStore, deps.InstanceOwner).run)
		if deps.OpenAPIIT != nil {
			runners = append(runners, newMADeepWorker(sqlStore, deps.OpenAPIIT, llmProvider, h.ma.loadPricing, h.ma.buildMADeepBriefFilingContext).run)
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
	handle("GET /binocolo/v1/ma/catalog/provinces", h.handleListMAProvinceCatalog)
	handle("PUT /binocolo/v1/ma/parameters", h.handleUpdateMAParameter)
	handle("GET /binocolo/v1/ma/initiatives", h.handleListMAInitiatives)
	handle("GET /binocolo/v1/ma/pipeline", h.handleGetMAPipeline)
	handle("POST /binocolo/v1/ma/initiatives", h.handleCreateMAInitiative)
	handle("PATCH /binocolo/v1/ma/initiatives/{id}", h.handleUpdateMAInitiative)
	handle("POST /binocolo/v1/ma/initiatives/{id}/archive", h.handleArchiveMAInitiative)
	handle("POST /binocolo/v1/ma/initiatives/{id}/restore", h.handleRestoreMAInitiative)
	handle("DELETE /binocolo/v1/ma/initiatives/{id}", h.handleDeleteMAInitiative)
	handle("POST /binocolo/v1/ma/initiatives/{id}/purge", h.handlePurgeMAInitiative)
	handle("POST /binocolo/v1/ma/sessions/{id}/initiative", h.handleSetMASessionInitiative)
	handle("GET /binocolo/v1/ma/initiatives/{id}", h.handleGetMAInitiativeBoard)
	handle("POST /binocolo/v1/ma/initiatives/{id}/cards", h.handleCreateMAInitiativeCard)
	handle("GET /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/dossier", h.handleGetMACardDossier)
	handle("POST /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/state", h.handleSetMACardState)
	handle("POST /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/close", h.handleCloseMACard)
	handle("POST /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/remove", h.handleRemoveMACard)
	handle("POST /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/reopen", h.handleReopenMACard)
	handle("POST /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/deep-dive", h.handleDeepDiveMACard)
	handle("GET /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/thesis-reading", h.handleGetCardThesisReading)
	handle("POST /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/thesis-reading", h.handleGenerateCardThesisReading)
	handle("GET /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/irl", h.handleGetCardIRL)
	handle("POST /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/irl/seed", h.handleSeedCardIRL)
	handle("POST /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/irl/items", h.handleAddCardIRLItem)
	handle("PATCH /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/irl/items/{itemId}", h.handleUpdateCardIRLItem)
	handle("DELETE /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/irl/items/{itemId}", h.handleDeleteCardIRLItem)
	handle("POST /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/irl/reorder", h.handleReorderCardIRL)
	handle("POST /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/irl/export", h.handleExportCardIRL)
	handle("GET /binocolo/v1/ma/companies", h.handleSearchMACompanies)
	handle("GET /binocolo/v1/ma/companies/areas", h.handleListMACompanySearchAreas)
	handle("POST /binocolo/v1/ma/companies/{companyKey}/deep-dive", h.handleDeepDiveMACompany)
	handle("POST /binocolo/v1/ma/companies/{companyKey}/brief/regenerate", h.handleRegenerateMACompanyBrief)
	handle("GET /binocolo/v1/ma/companies/{companyKey}/overview", h.handleGetMACompanyOverview)
	handle("GET /binocolo/v1/ma/companies/{companyKey}/activity", h.handleGetMACompanyActivity)
	handle("POST /binocolo/v1/ma/companies/{companyKey}/annotations", h.handleCreateMAAnnotation)
	handle("PATCH /binocolo/v1/ma/annotations/{id}", h.handleUpdateMAAnnotation)
	handle("DELETE /binocolo/v1/ma/annotations/{id}", h.handleDeleteMAAnnotation)
	handle("GET /binocolo/v1/ma/companies/{companyKey}/contacts", h.handleListMACompanyContacts)
	handle("POST /binocolo/v1/ma/companies/{companyKey}/contacts", h.handleCreateMACompanyContact)
	handle("PUT /binocolo/v1/ma/companies/{companyKey}/contacts/{contactId}", h.handleUpdateMACompanyContact)
	handle("DELETE /binocolo/v1/ma/companies/{companyKey}/contacts/{contactId}", h.handleDeleteMACompanyContact)
	handle("GET /binocolo/v1/ma/companies/{companyKey}/agreements", h.handleListMACompanyAgreements)
	handle("POST /binocolo/v1/ma/companies/{companyKey}/agreements", h.handleCreateMACompanyAgreement)
	handle("PUT /binocolo/v1/ma/companies/{companyKey}/agreements/{agreementId}", h.handleUpdateMACompanyAgreement)
	handle("DELETE /binocolo/v1/ma/companies/{companyKey}/agreements/{agreementId}", h.handleDeleteMACompanyAgreement)
	// Google Drive documents (issue #98, executing PRD #85). Company-scoped
	// listing (lazy-ensures the folder) and the idempotent card subfolder ensure.
	handle("GET /binocolo/v1/ma/companies/{companyKey}/documents", h.handleListMACompanyDocuments)
	handle("POST /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/drive-folder", h.handleEnsureMACardDriveFolder)
	handle("GET /binocolo/v1/ma/companies/{companyKey}/registry", h.handleGetMACompanyRegistry)
	handle("POST /binocolo/v1/ma/companies/{companyKey}/registry/facts", h.handleCreateMACompanyFact)
	handle("POST /binocolo/v1/ma/companies/{companyKey}/registry/facts/{factId}/revoke", h.handleRevokeMACompanyFact)
	// Deposited-filing pipeline (issue #78, Fase 8). Company-scoped filing lifecycle + the
	// filing/proposal-scoped operations. Identity is resolved from {companyKey}; filings are
	// keyed by fiscal identity, never company_key.
	handle("GET /binocolo/v1/ma/companies/{companyKey}/filings", h.handleListMAFilings)
	handle("POST /binocolo/v1/ma/companies/{companyKey}/filings", h.handleUploadMAFiling)
	handle("POST /binocolo/v1/ma/companies/{companyKey}/filings/search", h.handleSearchMAFilings)
	handle("POST /binocolo/v1/ma/companies/{companyKey}/filings/acquire", h.handleAcquireMAFilings)
	handle("POST /binocolo/v1/ma/companies/{companyKey}/filings/acquisitions/{id}/retry", h.handleRetryMAFilingAcquisition)
	handle("POST /binocolo/v1/ma/filings/{id}/identity-override", h.handleOverrideMAFilingIdentity)
	handle("GET /binocolo/v1/ma/filings/{id}/pdf", h.handleGetMAFilingPDF)
	handle("GET /binocolo/v1/ma/filings/{id}/proposals", h.handleGetMAFilingProposals)
	handle("POST /binocolo/v1/ma/proposals/{id}/decision", h.handleDecideMANIProposal)
	handle("GET /binocolo/v1/ma/filings/{id}/narrative", h.handleGetMAFilingNarrative)
	handle("POST /binocolo/v1/ma/filings/{id}/narrative/regenerate", h.handleRegenerateMAFilingNarrative)
	handle("GET /binocolo/v1/ma/sessions", h.handleListMASessions)
	handle("POST /binocolo/v1/ma/sessions", h.handleCreateMASession)
	handle("GET /binocolo/v1/ma/sessions/{id}", h.handleGetMASession)
	handle("GET /binocolo/v1/ma/sessions/{id}/targets", h.handleListMATargetRows)
	handle("GET /binocolo/v1/ma/sessions/{id}/targets/{targetId}", h.handleGetMATarget)
	handle("GET /binocolo/v1/ma/sessions/{id}/targets/{targetId}/thesis-reading", h.handleGetTargetThesisReading)
	handle("POST /binocolo/v1/ma/sessions/{id}/targets/{targetId}/thesis-reading", h.handleGenerateTargetThesisReading)
	handle("POST /binocolo/v1/ma/sessions/{id}/archive", h.handleArchiveMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/restore", h.handleRestoreMASession)
	handle("DELETE /binocolo/v1/ma/sessions/{id}", h.handleDeleteMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/purge", h.handlePurgeMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/rating", h.handleRateMATarget)
	handle("POST /binocolo/v1/ma/sessions/{id}/outcome", h.handleAddMATargetOutcome)
	handle("POST /binocolo/v1/ma/sessions/{id}/rescore", h.handleRescoreMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/targets/manual", h.handleManualAddMATarget)
	handle("POST /binocolo/v1/ma/sessions/{id}/associate-domain", h.handleAssociateMATargetDomain)
	handle("POST /binocolo/v1/ma/sessions/{id}/confirm-group-site", h.handleConfirmMAGroupSite)
	handle("POST /binocolo/v1/ma/sessions/{id}/no-website", h.handleDeclareMANoWebsite)
	handle("GET /binocolo/v1/ma/sessions/{id}/gated-progress", h.handleGetMAGatedProgress)
	handle("GET /binocolo/v1/ma/sessions/{id}/verification-queue", h.handleGetMAVerificationQueue)
	handle("GET /binocolo/v1/ma/sessions/{id}/sector-eval", h.handleGetSectorEval)
	handle("PUT /binocolo/v1/ma/sessions/{id}/sector-eval/label", h.handleSetSectorEvalLabel)
	handle("PUT /binocolo/v1/ma/sessions/{id}/web-validation", h.handleUpsertMAWebValidation)
	handle("POST /binocolo/v1/ma/sessions/{id}/web-validation/enrich", h.handleEnrichMAWebValidation)
	handle("POST /binocolo/v1/ma/sessions/{id}/deep-dive", h.handleDeepDiveMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/estimate", h.handleEstimateMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/execute", h.handleExecuteMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/gated-search", h.handleGatedSearchMASession)
	handle("POST /binocolo/v1/ma/sessions/{id}/export", h.handleExportMASession)
	handle("POST /binocolo/v1/ma/deep/recompute", h.handleRecomputeMADeep)
	handle("POST /binocolo/v1/ma/deep/regenerate-briefs", h.handleRegenerateMADeepBriefs)
	handle("GET /binocolo/v1/ma/deep/inspect", h.handleInspectMADeep)
	handle("PUT /binocolo/v1/ma/companies/{companyKey}/bm-family", h.handleRatifyBMFamily)
	handle("GET /binocolo/v1/companies/{vat}/dossier", h.handleGetCompanyDossier)
	handle("POST /binocolo/v1/companies/{vat}/dossier", h.handleCreateCompanyDossier)
	handle("POST /binocolo/v1/test/gated-search", h.handleTestGatedSearch)
	handle("POST /binocolo/v1/test/domain-resolution", h.handleTestDomainResolution)
	handle("POST /binocolo/v1/test/sector-classification", h.handleTestSectorClassification)
	handle("POST /binocolo/v1/test/sector-eval-models", h.handleCompareSectorEvalModels)
	handle("POST /binocolo/v1/test/ateco-retrieval", h.handleTestAtecoRetrieval)
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
	detail, err := h.ma.getSessionShape(r.Context(), id, wantsMATargetsNone(r))
	if err != nil {
		h.maFailure(w, r, "ma_session_get", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

func (h *Handler) handleListMATargetRows(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	rows, err := h.ma.sessionTargetRows(r.Context(), id)
	if err != nil {
		h.maFailure(w, r, "ma_target_rows", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, MATargetListResponse{Items: rows})
}

func (h *Handler) handleGetMATarget(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	targetID, ok := maTargetID(w, r)
	if !ok {
		return
	}
	target, err := h.ma.sessionTargetDetail(r.Context(), id, targetID)
	if err != nil {
		h.maFailure(w, r, "ma_target_get", err, "session_id", id, "target_id", targetID)
		return
	}
	httputil.JSON(w, http.StatusOK, target)
}

func (h *Handler) handleGetTargetThesisReading(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	targetID, ok := maTargetID(w, r)
	if !ok {
		return
	}
	reading, err := h.ma.getTargetThesisReading(r.Context(), id, targetID)
	if err != nil {
		h.maFailure(w, r, "ma_target_thesis_reading_get", err, "session_id", id, "target_id", targetID)
		return
	}
	if reading == nil {
		httputil.Error(w, http.StatusNotFound, "thesis_reading_not_generated")
		return
	}
	httputil.JSON(w, http.StatusOK, reading)
}

func (h *Handler) handleGenerateTargetThesisReading(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	targetID, ok := maTargetID(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_target_thesis_reading_generate", id, map[string]string{"targetId": targetID}, subject, email)
	if !traceOK {
		return
	}
	reading, err := h.ma.generateTargetThesisReading(r.Context(), id, targetID, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_target_thesis_reading_generate", err, "session_id", id, "target_id", targetID)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, reading)
}

func (h *Handler) handleListMAInitiatives(w http.ResponseWriter, r *http.Request) {
	visibility := r.URL.Query().Get("visibility")
	if strings.TrimSpace(visibility) == "" && strings.TrimSpace(r.URL.Query().Get("archived")) == "1" {
		visibility = maSessionVisibilityArchived
	}
	items, err := h.ma.listInitiatives(r.Context(), visibility)
	if err != nil {
		h.maFailure(w, r, "ma_initiative_list", err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"items": items})
}

type maInitiativeCreateRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type maInitiativeUpdateRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
}

func (h *Handler) handleCreateMAInitiative(w http.ResponseWriter, r *http.Request) {
	var body maInitiativeCreateRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	initiative, err := h.ma.createInitiative(r.Context(), body.Title, body.Description, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_initiative_create", err)
		return
	}
	httputil.JSON(w, http.StatusOK, initiative)
}

func (h *Handler) handleUpdateMAInitiative(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	var body maInitiativeUpdateRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_initiative_update", "", map[string]any{"initiativeId": id, "title": body.Title, "description": body.Description}, subject, email)
	if !traceOK {
		return
	}
	initiative, err := h.ma.updateInitiativeInfo(r.Context(), id, body.Title, body.Description, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_initiative_update", err, "initiative_id", id)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, initiative)
}

func (h *Handler) handleArchiveMAInitiative(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_initiative_archive", "", map[string]string{"action": maSessionLifecycleArchive, "initiativeId": id}, subject, email)
	if !traceOK {
		return
	}
	if err := h.ma.archiveInitiative(r.Context(), id, subject, email); err != nil {
		h.maFailure(w, r, "ma_initiative_archive", err, "initiative_id", id)
		return
	}
	h.completeMATraceSuccess(r, http.StatusNoContent)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleRestoreMAInitiative(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_initiative_restore", "", map[string]string{"action": maSessionLifecycleRestore, "initiativeId": id}, subject, email)
	if !traceOK {
		return
	}
	if err := h.ma.restoreInitiative(r.Context(), id, subject, email); err != nil {
		h.maFailure(w, r, "ma_initiative_restore", err, "initiative_id", id)
		return
	}
	h.completeMATraceSuccess(r, http.StatusNoContent)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleDeleteMAInitiative(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_initiative_delete", "", map[string]string{"action": maSessionLifecycleDelete, "initiativeId": id}, subject, email)
	if !traceOK {
		return
	}
	if err := h.ma.softDeleteInitiative(r.Context(), id, subject, email); err != nil {
		h.maFailure(w, r, "ma_initiative_delete", err, "initiative_id", id)
		return
	}
	h.completeMATraceSuccess(r, http.StatusNoContent)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handlePurgeMAInitiative(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_initiative_purge", "", map[string]string{"action": maSessionLifecyclePurge, "initiativeId": id}, subject, email)
	if !traceOK {
		return
	}
	if err := h.ma.purgeInitiative(r.Context(), id, subject, email); err != nil {
		h.maFailure(w, r, "ma_initiative_purge", err, "initiative_id", id)
		return
	}
	h.completeMATraceSuccess(r, http.StatusNoContent)
	w.WriteHeader(http.StatusNoContent)
}

type maSessionInitiativeRequest struct {
	InitiativeID string `json:"initiativeId"`
}

func (h *Handler) handleSetMASessionInitiative(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	var body maSessionInitiativeRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	body.InitiativeID = strings.TrimSpace(body.InitiativeID)
	if body.InitiativeID == "" {
		httputil.Error(w, http.StatusBadRequest, "initiative_id_required")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	if err := h.ma.setSessionInitiative(r.Context(), id, body.InitiativeID, subject, email); err != nil {
		h.maFailure(w, r, "ma_session_set_initiative", err, "session_id", id)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleGetMAInitiativeBoard(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	board, err := h.ma.getInitiativeBoard(r.Context(), id)
	if err != nil {
		h.maFailure(w, r, "ma_initiative_board_get", err, "initiative_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, board)
}

// handleGetMAPipeline serve la dashboard aggregata (sola lettura) di tutte le
// iniziative attive (KANBAN-V2-PLAN §B2).
func (h *Handler) handleGetMAPipeline(w http.ResponseWriter, r *http.Request) {
	pipeline, err := h.ma.getPipeline(r.Context())
	if err != nil {
		h.maFailure(w, r, "ma_pipeline_get", err)
		return
	}
	httputil.JSON(w, http.StatusOK, pipeline)
}

func (h *Handler) handleCreateMAInitiativeCard(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	var body MACreateInitiativeCardRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_initiative_card_create", "", map[string]any{"initiativeId": id, "domainSupplied": strings.TrimSpace(body.Domain) != ""}, subject, email)
	if !traceOK {
		return
	}
	result, err := h.ma.createDirectInitiativeCard(r.Context(), id, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_initiative_card_create", err, "initiative_id", id)
		return
	}
	h.completeMATraceSuccess(r, http.StatusCreated)
	httputil.JSON(w, http.StatusCreated, result)
}

func (h *Handler) handleGetMACardDossier(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	dossier, err := h.ma.getCardDossier(r.Context(), id, companyKey)
	if err != nil {
		h.maFailure(w, r, "ma_card_dossier_get", err, "initiative_id", id, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, dossier)
}

func (h *Handler) handleSetMACardState(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	var body MACardStateRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	card, err := h.ma.setCardState(r.Context(), id, companyKey, body.State, body.RecontactOn, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_card_state_set", err, "initiative_id", id, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, card)
}

func (h *Handler) handleCloseMACard(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	var body MACardCloseRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	result, err := h.ma.closeCard(r.Context(), id, companyKey, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_card_close", err, "initiative_id", id, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, result)
}

func (h *Handler) handleRemoveMACard(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	var body MACardRemoveRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	result, err := h.ma.removeCard(r.Context(), id, companyKey, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_card_remove", err, "initiative_id", id, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, result)
}

func (h *Handler) handleReopenMACard(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	card, err := h.ma.reopenCard(r.Context(), id, companyKey, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_card_reopen", err, "initiative_id", id, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, card)
}

func (h *Handler) handleDeepDiveMACard(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
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
	r, traceOK = h.startMATrace(w, r, "ma_card_deep_dive", "", map[string]any{"initiativeId": id, "companyKey": companyKey, "acknowledgeCost": body.AcknowledgeCost}, subject, email)
	if !traceOK {
		return
	}
	result, err := h.ma.deepDiveCard(r.Context(), id, companyKey, body.AcknowledgeCost, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_card_deep_dive", err, "initiative_id", id, "company_key", companyKey)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, result)
}

// handleGetCardThesisReading ritorna la lettura di tesi persistita della card
// (404 se mai generata) con la staleness rispetto alla tesi corrente. Read-only.
func (h *Handler) handleGetCardThesisReading(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	reading, err := h.ma.getCardThesisReading(r.Context(), id, companyKey)
	if err != nil {
		h.maFailure(w, r, "ma_thesis_reading_get", err, "initiative_id", id, "company_key", companyKey)
		return
	}
	if reading == nil {
		httputil.Error(w, http.StatusNotFound, "thesis_reading_not_generated")
		return
	}
	httputil.JSON(w, http.StatusOK, reading)
}

// handleGenerateCardThesisReading genera (o rigenera, azione esplicita) la
// lettura di tesi della card: LLM sui soli artefatti già calcolati + tesi della
// sessione di provenienza. Costo in centesimi, nessun cancello di spesa.
func (h *Handler) handleGenerateCardThesisReading(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	// Adapter card legacy: la generazione risolve la sessione di provenienza e
	// registra l'evento con session_id/company_key nello storage session-scoped.
	r, traceOK = h.startMATrace(w, r, "ma_thesis_reading_generate", "", nil, subject, email)
	if !traceOK {
		return
	}
	reading, err := h.ma.generateCardThesisReading(r.Context(), id, companyKey, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_thesis_reading_generate", err, "initiative_id", id, "company_key", companyKey)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, reading)
}

// handleGetCardIRL lista le voci IRL della card. Read-only, nessun gate
// operativo: l'IRL sopravvive all'archiviazione.
func (h *Handler) handleGetCardIRL(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	items, err := h.ma.getCardIRL(r.Context(), id, companyKey)
	if err != nil {
		h.maFailure(w, r, "ma_irl_get", err, "initiative_id", id, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// handleSeedCardIRL assembla le proposte dalle 4 fonti e inserisce solo i
// source_ref nuovi (re-seed additivo, la curatela non si tocca).
func (h *Handler) handleSeedCardIRL(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_irl_seed", "", nil, subject, email)
	if !traceOK {
		return
	}
	report, err := h.ma.seedCardIRL(r.Context(), id, companyKey, email)
	if err != nil {
		h.maFailure(w, r, "ma_irl_seed", err, "initiative_id", id, "company_key", companyKey)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, report)
}

// handleAddCardIRLItem aggiunge una voce dell'analista.
func (h *Handler) handleAddCardIRLItem(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Category string `json:"category"`
		Question string `json:"question"`
	}
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_irl_item_add", "", body, subject, email)
	if !traceOK {
		return
	}
	item, err := h.ma.addCardIRLItem(r.Context(), id, companyKey, body.Category, body.Question, email)
	if err != nil {
		h.maFailure(w, r, "ma_irl_item_add", err, "initiative_id", id, "company_key", companyKey)
		return
	}
	h.completeMATraceSuccess(r, http.StatusCreated)
	httputil.JSON(w, http.StatusCreated, item)
}

// handleUpdateCardIRLItem applica la patch parziale (categoria/domanda/stato).
func (h *Handler) handleUpdateCardIRLItem(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	itemID := strings.TrimSpace(r.PathValue("itemId"))
	var body MAIRLItemPatch
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_irl_item_update", "", body, subject, email)
	if !traceOK {
		return
	}
	item, err := h.ma.updateCardIRLItem(r.Context(), id, companyKey, itemID, body)
	if err != nil {
		h.maFailure(w, r, "ma_irl_item_update", err, "initiative_id", id, "company_key", companyKey, "item_id", itemID)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleDeleteCardIRLItem(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	itemID := strings.TrimSpace(r.PathValue("itemId"))
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_irl_item_delete", "", nil, subject, email)
	if !traceOK {
		return
	}
	if err := h.ma.deleteCardIRLItem(r.Context(), id, companyKey, itemID); err != nil {
		h.maFailure(w, r, "ma_irl_item_delete", err, "initiative_id", id, "company_key", companyKey, "item_id", itemID)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// handleReorderCardIRL riassegna le posizioni secondo l'ordine dell'array.
func (h *Handler) handleReorderCardIRL(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	var body struct {
		ItemIDs []string `json:"itemIds"`
	}
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_irl_reorder", "", body, subject, email)
	if !traceOK {
		return
	}
	if err := h.ma.reorderCardIRL(r.Context(), id, companyKey, body.ItemIDs); err != nil {
		h.maFailure(w, r, "ma_irl_reorder", err, "initiative_id", id, "company_key", companyKey)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, map[string]any{"reordered": len(body.ItemIDs)})
}

// handleExportCardIRL scarica l'XLSX del kick-off DD (pattern export sessione).
func (h *Handler) handleExportCardIRL(w http.ResponseWriter, r *http.Request) {
	id, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_irl_export", "", nil, subject, email)
	if !traceOK {
		return
	}
	content, filename, contentType, err := h.ma.exportCardIRL(r.Context(), id, companyKey, email)
	if err != nil {
		h.maFailure(w, r, "ma_irl_export", err, "initiative_id", id, "company_key", companyKey)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// maCompanyKeyPath extracts and normalizes the {companyKey} path segment
// shared by the registry endpoints (B3, PRD §6). Company keys are opaque
// slugs, not UUIDs, so no format validation beyond normalization + non-empty.
func maCompanyKeyPath(w http.ResponseWriter, r *http.Request) (string, bool) {
	companyKey := normalizeMACompanyKey(r.PathValue("companyKey"))
	if companyKey == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_ma_company_key")
		return "", false
	}
	return companyKey, true
}

func maFactID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := strings.TrimSpace(r.PathValue("factId"))
	if _, err := uuid.Parse(id); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_ma_fact_id")
		return "", false
	}
	return id, true
}

func (h *Handler) handleDeepDiveMACompany(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_company_deep_dive", "", map[string]string{"companyKey": companyKey}, subject, email)
	if !traceOK {
		return
	}
	result, err := h.ma.deepDiveCompany(r.Context(), companyKey, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_company_deep_dive", err, "company_key", companyKey)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, result)
}

func (h *Handler) handleListMACompanySearchAreas(w http.ResponseWriter, r *http.Request) {
	if h.ma.store == nil {
		h.maFailure(w, r, "ma_company_search_areas", errMAStoreUnavailable)
		return
	}
	result, err := h.ma.store.ListMACompanySearchAreas(r.Context())
	if err != nil {
		h.maFailure(w, r, "ma_company_search_areas", err)
		return
	}
	httputil.JSON(w, http.StatusOK, result)
}

func (h *Handler) handleSearchMACompanies(w http.ResponseWriter, r *http.Request) {
	options, err := parseMACompanySearch(r.URL.Query())
	if err != nil {
		h.maFailure(w, r, "ma_companies_search", err)
		return
	}
	result, err := h.ma.searchCompanies(r.Context(), options)
	if err != nil {
		h.maFailure(w, r, "ma_companies_search", err)
		return
	}
	httputil.JSON(w, http.StatusOK, result)
}

func (h *Handler) handleGetMACompanyOverview(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	overview, err := h.ma.getCompanyOverview(r.Context(), companyKey)
	if err != nil {
		h.maFailure(w, r, "ma_company_overview_get", err, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, overview)
}

func (h *Handler) handleGetMACompanyRegistry(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	registry, err := h.ma.getCompanyRegistry(r.Context(), companyKey)
	if err != nil {
		h.maFailure(w, r, "ma_company_registry_get", err, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, registry)
}

type maCompanyFactCreateRequest struct {
	Kind        string `json:"kind"`
	Note        string `json:"note"`
	VATCode     string `json:"vatCode"`
	TaxCode     string `json:"taxCode"`
	CompanyName string `json:"companyName"`
}

func (h *Handler) handleCreateMACompanyFact(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	var body maCompanyFactCreateRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	fact, err := h.ma.addCompanyFact(r.Context(), companyKey, body.Kind, body.Note, body.VATCode, body.TaxCode, body.CompanyName, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_company_fact_create", err, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, fact)
}

type maCompanyFactRevokeRequest struct {
	Note string `json:"note"`
}

func (h *Handler) handleRevokeMACompanyFact(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	factID, ok := maFactID(w, r)
	if !ok {
		return
	}
	var body maCompanyFactRevokeRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	if err := h.ma.revokeCompanyFact(r.Context(), factID, body.Note, subject, email); err != nil {
		h.maFailure(w, r, "ma_company_fact_revoke", err, "company_key", companyKey, "fact_id", factID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleGetMACompanyActivity(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	activity, err := h.ma.listCompanyActivity(r.Context(), companyKey, r.URL.Query().Get("includeDeleted") == "true")
	if err != nil {
		h.maFailure(w, r, "ma_company_activity_list", err, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, activity)
}

func (h *Handler) handleCreateMAAnnotation(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	var body MAAnnotationCreateRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	annotation, err := h.ma.createAnnotation(r.Context(), companyKey, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_annotation_create", err, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusCreated, annotation)
}

func (h *Handler) handleUpdateMAAnnotation(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	var body MAAnnotationUpdateRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	if err := h.ma.updateAnnotation(r.Context(), id, body.Body, subject, email); err != nil {
		h.maFailure(w, r, "ma_annotation_update", err, "annotation_id", id)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleDeleteMAAnnotation(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	subject, email := companySearchRefreshActor(r.Context())
	if err := h.ma.deleteAnnotation(r.Context(), id, subject, email); err != nil {
		h.maFailure(w, r, "ma_annotation_delete", err, "annotation_id", id)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleListMACompanyContacts(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	contacts, err := h.ma.listCompanyContacts(r.Context(), companyKey)
	if err != nil {
		h.maFailure(w, r, "ma_company_contacts_list", err, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, contacts)
}

func (h *Handler) handleCreateMACompanyContact(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	var body MACompanyContactWrite
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	contact, err := h.ma.createCompanyContact(r.Context(), companyKey, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_company_contact_create", err, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusCreated, contact)
}

func (h *Handler) handleUpdateMACompanyContact(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	contactID := strings.TrimSpace(r.PathValue("contactId"))
	var body MACompanyContactReplaceRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	contact, err := h.ma.updateCompanyContact(r.Context(), companyKey, contactID, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_company_contact_update", err, "company_key", companyKey, "contact_id", contactID)
		return
	}
	httputil.JSON(w, http.StatusOK, contact)
}

func (h *Handler) handleDeleteMACompanyContact(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	contactID := strings.TrimSpace(r.PathValue("contactId"))
	subject, email := companySearchRefreshActor(r.Context())
	if err := h.ma.deleteCompanyContact(r.Context(), companyKey, contactID, subject, email); err != nil {
		h.maFailure(w, r, "ma_company_contact_delete", err, "company_key", companyKey, "contact_id", contactID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleListMACompanyAgreements(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	agreements, err := h.ma.listCompanyAgreements(r.Context(), companyKey)
	if err != nil {
		h.maFailure(w, r, "ma_company_agreements_list", err, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, agreements)
}

func (h *Handler) handleCreateMACompanyAgreement(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	var body MACompanyAgreementWrite
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	agreement, err := h.ma.createCompanyAgreement(r.Context(), companyKey, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_company_agreement_create", err, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusCreated, agreement)
}

func (h *Handler) handleUpdateMACompanyAgreement(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	agreementID := strings.TrimSpace(r.PathValue("agreementId"))
	var body MACompanyAgreementReplaceRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	agreement, err := h.ma.updateCompanyAgreement(r.Context(), companyKey, agreementID, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_company_agreement_update", err, "company_key", companyKey, "agreement_id", agreementID)
		return
	}
	httputil.JSON(w, http.StatusOK, agreement)
}

func (h *Handler) handleDeleteMACompanyAgreement(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	agreementID := strings.TrimSpace(r.PathValue("agreementId"))
	subject, email := companySearchRefreshActor(r.Context())
	if err := h.ma.deleteCompanyAgreement(r.Context(), companyKey, agreementID, subject, email); err != nil {
		h.maFailure(w, r, "ma_company_agreement_delete", err, "company_key", companyKey, "agreement_id", agreementID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListMACompanyDocuments lists the company's Drive folder children (lazy-
// ensuring the folder on first access). Drive is the source of truth; Binocolo
// owns only the binding. Folders sort first, then by name. The optional
// initiativeId query param is the Scheda's lens: it resolves the card's bound
// subfolder id so the frontend highlights by id, never by name.
func (h *Handler) handleListMACompanyDocuments(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	initiativeID := strings.TrimSpace(r.URL.Query().Get("initiativeId"))
	if initiativeID != "" {
		if _, err := uuid.Parse(initiativeID); err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_ma_initiative_id")
			return
		}
	}
	subject, email := companySearchRefreshActor(r.Context())
	docs, err := h.ma.listCompanyDocuments(r.Context(), companyKey, initiativeID, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_company_documents_list", err, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, docs)
}

// handleEnsureMACardDriveFolder idempotently ensures the card's Drive subfolder
// (under the company folder) and returns its id + browse link. 200 OK on every
// call — idempotent ensure semantics.
func (h *Handler) handleEnsureMACardDriveFolder(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := maInitiativeID(w, r)
	if !ok {
		return
	}
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	if _, err := h.ma.requireOperationalInitiativeCard(r.Context(), initiativeID, companyKey); err != nil {
		h.maFailure(w, r, "ma_card_drive_folder_ensure", err, "initiative_id", initiativeID, "company_key", companyKey)
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	folderID, webViewLink, err := h.ma.ensureCardFolder(r.Context(), initiativeID, companyKey, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_card_drive_folder_ensure", err, "initiative_id", initiativeID, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"folderId": folderID, "webViewLink": webViewLink})
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
	if err := h.ma.setTargetRating(r.Context(), id, body, subject, email); err != nil {
		h.maFailure(w, r, "ma_target_rate", err, "session_id", id)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleAddMATargetOutcome(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	var body MATargetOutcomeRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	if err := h.ma.addTargetOutcome(r.Context(), id, body, subject, email); err != nil {
		h.maFailure(w, r, "ma_target_outcome", err, "session_id", id)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleRescoreMASession is the analyst's thesis override: re-scores the
// session's advanced targets from the persisted payloads under the requested
// thesis (free — no vendor calls, synchronous) and returns the updated detail.
func (h *Handler) handleRescoreMASession(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	var body MARescoreRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	detail, err := h.ma.rescoreSession(r.Context(), id, body.Thesis, subject, email, wantsMATargetsNone(r))
	if err != nil {
		h.maFailure(w, r, "ma_session_rescore", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

// handleManualAddMATarget enqueues an analyst-driven insertion of one company by
// VAT/tax code into an existing MA session. The worker fetches Advanced, validates
// the web/domain path, and re-scores the session.
func (h *Handler) handleManualAddMATarget(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	if !h.requireOpenAPIIT(w) {
		return
	}
	var body MAManualAddTargetRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	detail, err := h.ma.enqueueManualAdd(r.Context(), id, body.VATCode, body.Domain, subject, email, wantsMATargetsNone(r))
	if err != nil {
		if errors.Is(err, errMATargetAlreadyPresent) {
			httputil.Error(w, http.StatusConflict, errMATargetAlreadyPresent.Error())
			return
		}
		if errors.Is(err, errMAManualAddInFlight) {
			httputil.Error(w, http.StatusConflict, errMAManualAddInFlight.Error())
			return
		}
		h.maFailure(w, r, "ma_target_manual_add", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusAccepted, detail)
}

// handleAssociateMATargetDomain is the manual-review remedy: the operator supplies an
// official domain for a company the gate held as domain-unresolved. It enqueues a durable
// associate_domain job that re-gates that one company with the forced domain and, if it now
// survives, enriches + re-scores it. Returns the session in 'running'; the UI polls.
func (h *Handler) handleAssociateMATargetDomain(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	if !h.requireBrave(w) || !h.requireOpenAPIIT(w) {
		return
	}
	var body MAAssociateDomainRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	detail, err := h.ma.enqueueAssociateDomain(r.Context(), id, body.CompanyKey, body.Domain, "associate", subject, email, wantsMATargetsNone(r))
	if err != nil {
		h.maFailure(w, r, "ma_target_associate_domain", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusAccepted, detail)
}

func (h *Handler) handleConfirmMAGroupSite(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	if !h.requireBrave(w) || !h.requireOpenAPIIT(w) {
		return
	}
	var body MACompanyKeyRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	detail, err := h.ma.enqueueAssociateDomain(r.Context(), id, body.CompanyKey, "", "group_site", subject, email, wantsMATargetsNone(r))
	if err != nil {
		h.maFailure(w, r, "ma_target_confirm_group_site", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusAccepted, detail)
}

func (h *Handler) handleDeclareMANoWebsite(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	if !h.requireOpenAPIIT(w) {
		return
	}
	var body MACompanyKeyRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	detail, err := h.ma.enqueueAssociateDomain(r.Context(), id, body.CompanyKey, "", "no_website", subject, email, wantsMATargetsNone(r))
	if err != nil {
		h.maFailure(w, r, "ma_target_no_website", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusAccepted, detail)
}

func (h *Handler) handleGetMAGatedProgress(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	progress, err := h.ma.gatedProgress(r.Context(), id)
	if err != nil {
		h.maFailure(w, r, "ma_gated_progress", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, progress)
}

func (h *Handler) handleGetMAVerificationQueue(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	queue, err := h.ma.verificationQueue(r.Context(), id)
	if err != nil {
		h.maFailure(w, r, "ma_verification_queue", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, queue)
}

// handleGetSectorEval returns the UC2 sector-eval report for a session: each company
// with its current system prediction + human label, plus the aggregate metrics. Read-only.
func (h *Handler) handleGetSectorEval(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	report, err := h.ma.sectorEvalReport(r.Context(), id)
	if err != nil {
		h.maFailure(w, r, "ma_sector_eval", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, report)
}

// handleSetSectorEvalLabel upserts (or clears, with an empty label) the human ground-truth
// sector label for a company in a session.
func (h *Handler) handleSetSectorEvalLabel(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	var body SectorEvalLabelRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	if err := h.ma.setSectorEvalLabel(r.Context(), id, body, subject, email); err != nil {
		h.maFailure(w, r, "ma_sector_eval_label", err, "session_id", id)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleCompareSectorEvalModels replays the UC2 analyst over already-labeled sessions with
// multiple models in parallel (same prompt, model varies) and scores each against the
// human ground truth — to pick a backup analyst on a different provider without re-running
// the web pipeline. Read-only (no persistence beyond the per-call LLM audit).
func (h *Handler) handleCompareSectorEvalModels(w http.ResponseWriter, r *http.Request) {
	var body SectorModelCompareRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if len(body.SessionIDs) == 0 {
		httputil.Error(w, http.StatusBadRequest, "missing_session_ids")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	report, err := h.ma.compareSectorEvalModels(r.Context(), body, subject, email)
	if err != nil {
		h.maFailure(w, r, "sector_eval_models", err)
		return
	}
	httputil.JSON(w, http.StatusOK, report)
}

func (h *Handler) handleUpsertMAWebValidation(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	var body MAWebValidationUpsertRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_target_web_validation_upsert", id, body, subject, email)
	if !traceOK {
		return
	}
	validation, err := h.ma.upsertTargetWebValidation(r.Context(), id, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_target_web_validation_upsert", err, "session_id", id)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, validation)
}

func (h *Handler) handleEnrichMAWebValidation(w http.ResponseWriter, r *http.Request) {
	id, ok := maSessionID(w, r)
	if !ok {
		return
	}
	var body MAWebValidationEnrichRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeMABody(r, &body); err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_json")
			return
		}
	}
	subject, email := companySearchRefreshActor(r.Context())
	detail, err := h.ma.enqueueWebValidation(r.Context(), id, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_session_web_validation", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusAccepted, detail)
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
	detail, err := h.ma.deepDive(r.Context(), id, body.AcknowledgeCost, email, wantsMATargetsNone(r))
	if err != nil {
		h.maFailure(w, r, "ma_session_deep_dive", err, "session_id", id)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, detail)
}

// handleRecomputeMADeep rebuilds the deterministic scorecard (+ quality flags) for
// every cached deep analysis from its stored IT-full payload (no vendor call, no
// charge). Body opzionale {"valuation": true} ricostruisce anche la valuation
// (bridge + banda asimmetrica). Gated by the standard binocolo access role.
func (h *Handler) handleRecomputeMADeep(w http.ResponseWriter, r *http.Request) {
	subject, email := companySearchRefreshActor(r.Context())
	var body struct {
		Valuation bool `json:"valuation"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body) // body assente = solo scorecard
	var ok bool
	r, ok = h.startMATrace(w, r, "ma_deep_recompute", "", body, subject, email)
	if !ok {
		return
	}
	count, err := h.ma.recomputeMADeepScorecards(r.Context(), body.Valuation)
	if err != nil {
		h.maFailure(w, r, "ma_deep_recompute", err)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, map[string]any{"recomputed": count, "valuation": body.Valuation})
}

// handleRegenerateMADeepBriefs re-runs the LLM brief for cached analyses from their
// stored payload (no IT-full call). Used to roll out a new brief prompt after a prompt
// change; body {"companyKey": "..."} scopes the run to a single row (retry mirato).
// Gated by the standard binocolo access role.
func (h *Handler) handleRegenerateMADeepBriefs(w http.ResponseWriter, r *http.Request) {
	subject, email := companySearchRefreshActor(r.Context())
	var body struct {
		CompanyKey string `json:"companyKey"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	var ok bool
	r, ok = h.startMATrace(w, r, "ma_deep_regenerate_briefs", "", nil, subject, email)
	if !ok {
		return
	}
	// Una chiamata LLM per azienda cached: il batch supera il WriteTimeout del
	// server, che chiuderebbe la connessione (empty reply) troncando la
	// rigenerazione. Endpoint ops invocato a mano: si azzerano le deadline di
	// connessione per questa sola richiesta e si risponde a batch concluso.
	rc := http.NewResponseController(w)
	if err := rc.SetReadDeadline(time.Time{}); err != nil {
		logging.FromContext(r.Context()).Warn("binocolo regenerate briefs: clear read deadline", "component", "binocolo", "error", err)
	}
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		logging.FromContext(r.Context()).Warn("binocolo regenerate briefs: clear write deadline", "component", "binocolo", "error", err)
	}
	report, err := h.ma.regenerateMADeepBriefs(r.Context(), body.CompanyKey)
	if err != nil {
		h.maFailure(w, r, "ma_deep_regenerate_briefs", err)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, report)
}

// handleRegenerateMACompanyBrief re-runs the LLM brief for ONE company (issue #78, Fase 9): the
// analyst action behind the scheda's "brief stale" row. It reuses the global regen's per-row
// assembly (single LLM call, no IT-full call) and re-stamps brief_generated_at. 404 when the
// company has no deep analysis, 409 when the analysis is not ready. Gated by the access role.
func (h *Handler) handleRegenerateMACompanyBrief(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_company_brief_regenerate", "", map[string]any{"companyKey": companyKey}, subject, email)
	if !traceOK {
		return
	}
	// A single LLM call still risks the server WriteTimeout on a slow model; clear the per-request
	// deadlines like the batch endpoint so the connection isn't cut mid-generation.
	rc := http.NewResponseController(w)
	if err := rc.SetReadDeadline(time.Time{}); err != nil {
		logging.FromContext(r.Context()).Warn("binocolo regenerate company brief: clear read deadline", "component", "binocolo", "error", err)
	}
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		logging.FromContext(r.Context()).Warn("binocolo regenerate company brief: clear write deadline", "component", "binocolo", "error", err)
	}
	briefGeneratedAt, err := h.ma.regenerateMADeepBriefForCompany(r.Context(), companyKey)
	if err != nil {
		h.maFailure(w, r, "ma_company_brief_regenerate", err, "company_key", companyKey)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, map[string]any{"status": "regenerated", "briefGeneratedAt": briefGeneratedAt})
}

// handleInspectMADeep computes the Fase 0 read-only diagnostics over the cached deep
// payloads (division mix, granularity, delta coverage, vendor-vs-CEE reconciliation).
// No vendor call, nothing persisted. Gated by the standard binocolo access role.
func (h *Handler) handleInspectMADeep(w http.ResponseWriter, r *http.Request) {
	subject, email := companySearchRefreshActor(r.Context())
	var ok bool
	r, ok = h.startMATrace(w, r, "ma_deep_inspect", "", nil, subject, email)
	if !ok {
		return
	}
	report, err := h.ma.inspectMADeep(r.Context())
	if err != nil {
		h.maFailure(w, r, "ma_deep_inspect", err)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, report)
}

// handleRatifyBMFamily registra la ratifica/override dell'analista sulla famiglia
// di business model (Fase 3): body {"family": "..."}; vuota = revoca. La famiglia
// alimenta soglie RAG e riga Damodaran al prossimo ricalcolo dell'azienda.
func (h *Handler) handleRatifyBMFamily(w http.ResponseWriter, r *http.Request) {
	companyKey := strings.TrimSpace(r.PathValue("companyKey"))
	subject, email := companySearchRefreshActor(r.Context())
	var body MABMFamilyRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "body non valido")
		return
	}
	var ok bool
	// Trace senza session id (FK a ma_session): il companyKey non è una sessione.
	r, ok = h.startMATrace(w, r, "ma_bm_family_ratify", "", body, subject, email)
	if !ok {
		return
	}
	family, err := h.ma.ratifyBMFamily(r.Context(), companyKey, strings.TrimSpace(body.Family), subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_bm_family_ratify", err, "company_key", companyKey)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, family)
}

// handleGetCompanyDossier returns the cached dossier state for a P.IVA (poll target);
// it never enqueues analysis.
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
// miss it queues a fresh IT-full deep analysis directly.
func (h *Handler) handleCreateCompanyDossier(w http.ResponseWriter, r *http.Request) {
	vat, ok := maVATParam(w, r)
	if !ok {
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_company_dossier", "", nil, subject, email)
	if !traceOK {
		return
	}
	dossier, err := h.ma.companyDossier(r.Context(), vat, email)
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
	vat := normalizeMAVATOrTax(r.PathValue("vat"))
	if vat == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_vat")
		return "", false
	}
	return vat, true
}

func normalizeMAVATOrTax(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	if !isValidVATOrTax(v) {
		return ""
	}
	return v
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
	detail, err := h.ma.enqueueEstimate(r.Context(), id, body, subject, email, wantsMATargetsNone(r))
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
	detail, err := h.ma.enqueueExecute(r.Context(), id, body, subject, email, wantsMATargetsNone(r))
	if err != nil {
		h.maFailure(w, r, "ma_session_execute", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusAccepted, detail)
}

// handleGatedSearchMASession enqueues the gated-search pipeline (address → UC2 gate →
// advanced+score on survivors), the coexisting alternative to execute. Same request
// shape as execute; the synchronous half validates the estimate and enforces the
// surface cap, then returns 202 while the worker runs the multi-stage job. The UI
// polls GET .../sessions/{id} until status leaves 'running'.
func (h *Handler) handleGatedSearchMASession(w http.ResponseWriter, r *http.Request) {
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
	detail, err := h.ma.enqueueGatedSearch(r.Context(), id, body, subject, email, false, wantsMATargetsNone(r))
	if err != nil {
		h.maFailure(w, r, "ma_session_gated_search", err, "session_id", id)
		return
	}
	httputil.JSON(w, http.StatusAccepted, detail)
}

// handleTestGatedSearch runs the gated-search funnel INLINE (off the shared queue) on an
// existing session that already has a fresh estimate — the developer test surface for the
// end-to-end pipeline. Same validation as the real endpoint (estimate freshness + surface
// cap); returns 202 while a detached goroutine runs address→gate→advanced+score. The test
// page polls GET .../sessions/{id} and reads the keep/forse/salta buckets off the targets.
func (h *Handler) handleTestGatedSearch(w http.ResponseWriter, r *http.Request) {
	var body MAGatedSearchTestRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	sessionID := strings.TrimSpace(body.SessionID)
	if sessionID == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_session_id")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	req := MAExecuteSessionRequest{StrategyType: body.StrategyType, Limit: body.Limit}
	detail, err := h.ma.enqueueGatedSearch(r.Context(), sessionID, req, subject, email, true, false)
	if err != nil {
		h.maFailure(w, r, "ma_session_gated_search", err, "session_id", sessionID)
		return
	}
	httputil.JSON(w, http.StatusAccepted, detail)
}

// handleTestAtecoRetrieval previews the ATECO concept retrieval for a raw sector
// text: it returns the cosine of every KB concept (targets + distractors), which
// clear the relative threshold, and the resulting candidates/divisions. Optional
// relThreshold/coreRatio/floor/cap overrides tune the config for this call only
// (never persisted), so the net width can be A/B'd on real cosines.
func (h *Handler) handleTestAtecoRetrieval(w http.ResponseWriter, r *http.Request) {
	var body MAAtecoRetrievalPreviewRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_text")
		return
	}
	preview, err := h.ma.previewMAAtecoRetrieval(r.Context(), body)
	if err != nil {
		h.maFailure(w, r, "ateco_retrieval_preview", err)
		return
	}
	httputil.JSON(w, http.StatusOK, preview)
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
	var duplicate *maCardAlreadyPresentError
	if errors.As(err, &duplicate) {
		httputil.JSON(w, status, map[string]any{"error": code, "companyKey": duplicate.CompanyKey})
		return
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
	if errors.Is(err, errMABraveUnavailable) {
		return http.StatusServiceUnavailable, "brave_not_configured", "warn"
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
	if errors.Is(err, errMAInitiativeArchived) {
		return http.StatusConflict, "ma_initiative_archived", "warn"
	}
	if errors.Is(err, errMAInitiativeDeleted) {
		return http.StatusConflict, "ma_initiative_deleted", "warn"
	}
	if errors.Is(err, errMAInitiativePurged) {
		return http.StatusConflict, "ma_initiative_purged", "warn"
	}
	if errors.Is(err, errMAInitiativeNotFound) {
		return http.StatusNotFound, "ma_initiative_not_found", "warn"
	}
	if errors.Is(err, errMACardAlreadyPresent) {
		return http.StatusConflict, "card_already_present", "warn"
	}
	if errors.Is(err, errMAVATNotFound) {
		return http.StatusNotFound, "vat_not_found", "warn"
	}
	if errors.Is(err, errMAInvalidVAT) {
		return http.StatusUnprocessableEntity, "invalid_vat", "warn"
	}
	if errors.Is(err, errMACompanyFactActive) {
		return http.StatusBadRequest, "ma_company_fact_already_active", "warn"
	}
	if errors.Is(err, errAtecoCodeNotFound) {
		return http.StatusBadRequest, "invalid_ateco_code", "warn"
	}
	if errors.Is(err, errMACardDossierNotFound) {
		return http.StatusNotFound, "ma_card_dossier_not_found", "warn"
	}
	if errors.Is(err, errMAAnnotationNotFound) {
		return http.StatusNotFound, "ma_annotation_not_found", "warn"
	}
	if errors.Is(err, errMACompanyContactNotFound) {
		return http.StatusNotFound, "ma_company_contact_not_found", "warn"
	}
	if errors.Is(err, errMACompanyAgreementNotFound) {
		return http.StatusNotFound, "ma_company_agreement_not_found", "warn"
	}
	// Deposited-filing endpoints (issue #78, Fase 8).
	if errors.Is(err, errMAFilingIdentityUnresolved) {
		return http.StatusUnprocessableEntity, "filing_identity_unresolved", "warn"
	}
	if errors.Is(err, errMAFilingInvalidPDF) {
		return http.StatusUnprocessableEntity, "invalid_pdf", "warn"
	}
	if errors.Is(err, errMAFilingEmptyUpload) {
		return http.StatusUnprocessableEntity, "empty_file", "warn"
	}
	if errors.Is(err, errMAFilingTooLarge) {
		return http.StatusRequestEntityTooLarge, "file_too_large", "warn"
	}
	if errors.Is(err, errMAFilingNotFound) {
		return http.StatusNotFound, "filing_not_found", "warn"
	}
	if errors.Is(err, errMAFilingBlobMissing) {
		return http.StatusNotFound, "filing_blob_missing", "warn"
	}
	if errors.Is(err, errMAFilingReasonRequired) {
		return http.StatusUnprocessableEntity, "reason_required", "warn"
	}
	if errors.Is(err, errMAFilingIdentityOverrideNotApplicable) {
		return http.StatusConflict, "identity_override_not_applicable", "warn"
	}
	if errors.Is(err, errMAFilingSearchIDRequired) {
		return http.StatusUnprocessableEntity, "search_id_required", "warn"
	}
	if errors.Is(err, errMAFilingBalanceSheetsRequired) {
		return http.StatusUnprocessableEntity, "balance_sheet_ids_required", "warn"
	}
	if errors.Is(err, errMAFilingAcquisitionNotFound) {
		return http.StatusNotFound, "acquisition_not_found", "warn"
	}
	if errors.Is(err, errMAFilingAcquisitionNotRetryable) {
		return http.StatusConflict, "acquisition_not_retryable", "warn"
	}
	if errors.Is(err, errMANIProposalNotFound) {
		return http.StatusNotFound, "proposal_not_found", "warn"
	}
	if errors.Is(err, errMANIActionInvalid) {
		return http.StatusUnprocessableEntity, "invalid_action", "warn"
	}
	if errors.Is(err, errMANIRatifiedTreatmentInvalid) {
		return http.StatusUnprocessableEntity, "invalid_ratified_treatment", "warn"
	}
	if errors.Is(err, errMANIRatifiedTreatmentRequired) {
		return http.StatusUnprocessableEntity, "ratified_treatment_required", "warn"
	}
	if errors.Is(err, errMANIRatifiedAmountRequired) {
		return http.StatusUnprocessableEntity, "ratified_amount_required", "warn"
	}
	if errors.Is(err, errMANINarrativeNotRegenerable) {
		return http.StatusConflict, "narrative_not_regenerable", "warn"
	}
	// Per-company brief regeneration (issue #78, Fase 9).
	if errors.Is(err, errMADeepBriefDeepAbsent) {
		return http.StatusNotFound, "deep_absent", "warn"
	}
	if errors.Is(err, errMADeepBriefDeepNotReady) {
		return http.StatusConflict, "deep_not_ready", "warn"
	}
	if errors.Is(err, errMADeepBriefScorecardUnavailable) {
		return http.StatusConflict, "brief_not_regenerable", "warn"
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
	// Google Drive documents (issue #98). The code strings are part of the
	// contract — the frontend branches on them; not_configured → 503 so the
	// Scheda renders a graceful "Documenti non disponibili" state.
	var driveErr *googledrive.Error
	if errors.As(err, &driveErr) {
		switch driveErr.Code {
		case googledrive.CodeNotConfigured:
			return http.StatusServiceUnavailable, "googledrive_not_configured", "warn"
		case googledrive.CodeNotFound:
			return http.StatusNotFound, "googledrive_not_found", "warn"
		case googledrive.CodeTrashed:
			return http.StatusGone, "googledrive_trashed", "warn"
		case googledrive.CodeOutsideContextRoot:
			return http.StatusGone, "googledrive_outside_context_root", "warn"
		case googledrive.CodePermissionDenied:
			return http.StatusBadGateway, "googledrive_permission_denied", "warn"
		case googledrive.CodeInvalidCredentials:
			return http.StatusBadGateway, "googledrive_invalid_credentials", "warn"
		case googledrive.CodeRateLimited:
			return http.StatusTooManyRequests, "googledrive_rate_limited", "warn"
		case googledrive.CodeUnavailable:
			return http.StatusBadGateway, "googledrive_unavailable", "warn"
		default:
			return http.StatusBadGateway, "googledrive_invalid_response", "warn"
		}
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

func maInitiativeID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := strings.TrimSpace(r.PathValue("id"))
	if _, err := uuid.Parse(id); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_ma_initiative_id")
		return "", false
	}
	return id, true
}

func maTargetID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := strings.TrimSpace(r.PathValue("targetId"))
	if _, err := uuid.Parse(id); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_ma_target_id")
		return "", false
	}
	return id, true
}

func wantsMATargetsNone(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("targets")), "none")
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
