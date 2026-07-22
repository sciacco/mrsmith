package binocolo

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// HTTP handlers for the deposited-filing pipeline (issue #78, Fase 8). Thin: parse the path +
// body, pull the actor from the auth context (never the body), delegate to the maService, and
// render. Routes are wired in RegisterRoutes (handler.go); errors flow through h.maFailure,
// which maps the filing sentinels (maHTTPError) to snake_case codes.

// maFilingID validates the {id} path segment as a uuid (the ma_filing / proposal id).
func maFilingID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := strings.TrimSpace(r.PathValue("id"))
	if _, err := uuid.Parse(id); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_ma_filing_id")
		return "", false
	}
	return id, true
}

// maProposalID validates the {id} path segment as a uuid (the ma_ni_proposal id).
func maProposalID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := strings.TrimSpace(r.PathValue("id"))
	if _, err := uuid.Parse(id); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_ma_proposal_id")
		return "", false
	}
	return id, true
}

// handleListMAFilings — GET /ma/companies/{companyKey}/filings.
func (h *Handler) handleListMAFilings(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	resp, err := h.ma.listCompanyFilings(r.Context(), companyKey)
	if err != nil {
		h.maFailure(w, r, "ma_filings_list", err, "company_key", companyKey)
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

// handleUploadMAFiling — POST /ma/companies/{companyKey}/filings (multipart "file").
func (h *Handler) handleUploadMAFiling(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	// Cap the body slightly above the content limit so an oversize upload is rejected with a
	// clean error rather than an EOF mid-parse.
	r.Body = http.MaxBytesReader(w, r.Body, maFilingUploadMaxBytes+(1<<20))
	if err := r.ParseMultipartForm(maFilingUploadMaxBytes); err != nil {
		// A body past the MaxBytesReader ceiling is an oversize upload, not malformed JSON.
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httputil.Error(w, http.StatusRequestEntityTooLarge, "file_too_large")
			return
		}
		httputil.Error(w, http.StatusBadRequest, "invalid_upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "file_required")
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maFilingUploadMaxBytes+1))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_upload")
		return
	}
	if int64(len(content)) > maFilingUploadMaxBytes {
		httputil.Error(w, http.StatusRequestEntityTooLarge, "file_too_large")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_filing_upload", "", map[string]any{"companyKey": companyKey, "filename": header.Filename, "size": len(content)}, subject, email)
	if !traceOK {
		return
	}
	result, err := h.ma.uploadCompanyFiling(r.Context(), companyKey, header.Header.Get("Content-Type"), content, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_filing_upload", err, "company_key", companyKey)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, result)
}

// handleSearchMAFilings — POST /ma/companies/{companyKey}/filings/search.
func (h *Handler) handleSearchMAFilings(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	// Pre-check: the DocuEngine channel must be configured (the pinned "Bilancio Ottico"
	// documentId). Empty ⇒ a search would fail definitively, so refuse up front.
	if strings.TrimSpace(h.ma.filingDocumentID) == "" {
		httputil.Error(w, http.StatusServiceUnavailable, "docuengine_not_configured")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_filing_search", "", map[string]any{"companyKey": companyKey}, subject, email)
	if !traceOK {
		return
	}
	result, err := h.ma.startCompanyFilingSearch(r.Context(), companyKey, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_filing_search", err, "company_key", companyKey)
		return
	}
	h.completeMATraceSuccess(r, http.StatusAccepted)
	httputil.JSON(w, http.StatusAccepted, result)
}

type maFilingAcquireRequest struct {
	SearchID        string   `json:"searchId"`
	BalanceSheetIDs []string `json:"balanceSheetIds"`
}

// handleAcquireMAFilings — POST /ma/companies/{companyKey}/filings/acquire.
func (h *Handler) handleAcquireMAFilings(w http.ResponseWriter, r *http.Request) {
	companyKey, ok := maCompanyKeyPath(w, r)
	if !ok {
		return
	}
	var body maFilingAcquireRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_filing_acquire", "", map[string]any{"companyKey": companyKey, "searchId": body.SearchID, "count": len(body.BalanceSheetIDs)}, subject, email)
	if !traceOK {
		return
	}
	result, err := h.ma.acquireCompanyFilings(r.Context(), companyKey, body.SearchID, body.BalanceSheetIDs, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_filing_acquire", err, "company_key", companyKey)
		return
	}
	h.completeMATraceSuccess(r, http.StatusAccepted)
	httputil.JSON(w, http.StatusAccepted, result)
}

type maFilingIdentityOverrideRequest struct {
	Reason string `json:"reason"`
}

// handleOverrideMAFilingIdentity — POST /ma/filings/{id}/identity-override.
func (h *Handler) handleOverrideMAFilingIdentity(w http.ResponseWriter, r *http.Request) {
	id, ok := maFilingID(w, r)
	if !ok {
		return
	}
	var body maFilingIdentityOverrideRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_filing_identity_override", "", map[string]any{"filingId": id, "reason": body.Reason}, subject, email)
	if !traceOK {
		return
	}
	result, err := h.ma.applyFilingIdentityOverride(r.Context(), id, body.Reason, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_filing_identity_override", err, "filing_id", id)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, result)
}

// handleGetMAFilingPDF — GET /ma/filings/{id}/pdf (authenticated blob download).
func (h *Handler) handleGetMAFilingPDF(w http.ResponseWriter, r *http.Request) {
	id, ok := maFilingID(w, r)
	if !ok {
		return
	}
	pdf, filename, err := h.ma.filingPDF(r.Context(), id)
	if err != nil {
		h.maFailure(w, r, "ma_filing_pdf", err, "filing_id", id)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(pdf)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}

// handleGetMAFilingProposals — GET /ma/filings/{id}/proposals.
func (h *Handler) handleGetMAFilingProposals(w http.ResponseWriter, r *http.Request) {
	id, ok := maFilingID(w, r)
	if !ok {
		return
	}
	resp, err := h.ma.listFilingProposals(r.Context(), id)
	if err != nil {
		h.maFailure(w, r, "ma_filing_proposals", err, "filing_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

type maNIDecisionRequest struct {
	Action            string   `json:"action"`
	RatifiedAmount    *float64 `json:"ratifiedAmount"`
	RatifiedTreatment string   `json:"ratifiedTreatment"`
	Reason            string   `json:"reason"`
}

// handleDecideMANIProposal — POST /ma/proposals/{id}/decision.
func (h *Handler) handleDecideMANIProposal(w http.ResponseWriter, r *http.Request) {
	id, ok := maProposalID(w, r)
	if !ok {
		return
	}
	var body maNIDecisionRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	var traceOK bool
	r, traceOK = h.startMATrace(w, r, "ma_ni_decision", "", map[string]any{"proposalId": id, "action": body.Action}, subject, email)
	if !traceOK {
		return
	}
	result, err := h.ma.decideNIProposal(r.Context(), id, maNIDecisionInput{
		Action:            body.Action,
		RatifiedAmount:    body.RatifiedAmount,
		RatifiedTreatment: body.RatifiedTreatment,
		Reason:            body.Reason,
	}, subject, email)
	if err != nil {
		h.maFailure(w, r, "ma_ni_decision", err, "proposal_id", id)
		return
	}
	h.completeMATraceSuccess(r, http.StatusOK)
	httputil.JSON(w, http.StatusOK, result)
}
