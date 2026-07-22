package binocolo

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strings"
	"time"
)

// HTTP endpoint orchestration for the deposited-filing pipeline (issue #78, Fase 8). The
// handlers are thin: they parse + delegate here, where the service composes the F3–F7 store
// and service primitives. No new business logic lives in the handlers.
//
// Invariants carried from the plan:
//   - identity is resolved from the {companyKey} path segment (never trusted from the body);
//   - the fiscal identity (vat/tax → fiscal_key) is the deposited-filing key, NOT company_key;
//   - NO price/cost field is ever surfaced (operator-only data stays out of the UI);
//   - every mutating enqueue passes a non-empty ContextCompanyKey so the baseline deep-dive can
//     be triggered (an empty context skips the baseline ⇒ the filing degrades).

// maFilingUploadMaxBytes caps an uploaded balance-sheet PDF (a free synchronous guard). 30 MiB
// comfortably covers a full camerale deposit while rejecting obvious abuse before any work.
const maFilingUploadMaxBytes = 30 << 20

// Endpoint-level sentinels, mapped to snake_case HTTP codes in maHTTPError. Kept here (next to
// the endpoints that raise them) so the filing surface owns its own error vocabulary.
var (
	errMAFilingIdentityUnresolved            = errors.New("ma filing identity unresolved")
	errMAFilingInvalidPDF                    = errors.New("ma filing upload is not a pdf")
	errMAFilingEmptyUpload                   = errors.New("ma filing upload is empty")
	errMAFilingTooLarge                      = errors.New("ma filing upload too large")
	errMAFilingNotFound                      = errors.New("ma filing not found")
	errMAFilingBlobMissing                   = errors.New("ma filing blob missing")
	errMAFilingReasonRequired                = errors.New("ma filing override reason required")
	errMAFilingIdentityOverrideNotApplicable = errors.New("ma filing identity override not applicable")
	errMAFilingSearchIDRequired              = errors.New("ma filing search id required")
	errMAFilingBalanceSheetsRequired         = errors.New("ma filing balance sheet ids required")
	errMAFilingAcquisitionNotFound           = errors.New("ma filing acquisition not found")
	errMAFilingAcquisitionNotRetryable       = errors.New("ma filing acquisition not retryable")
	errMANIProposalNotFound                  = errors.New("ma ni proposal not found")
	errMANIActionInvalid                     = errors.New("ma ni action invalid")
	errMANIRatifiedTreatmentInvalid          = errors.New("ma ni ratified treatment invalid")
	errMANIRatifiedTreatmentRequired         = errors.New("ma ni ratified treatment required")
	errMANIRatifiedAmountRequired            = errors.New("ma ni ratified amount required")
	errMANINarrativeNotRegenerable           = errors.New("ma ni narrative not regenerable")
)

// ---------------------------------------------------------------------------
// Response shapes (camelCase, no prices). F9 replicates these in api/types.ts.
// ---------------------------------------------------------------------------

// MAFilingsResponse is the deposited-filing panel of the scheda: the fiscal identity, the
// fascicoli ordered by closing date DESC, the latest search offer, and every still-OPEN
// acquisition (a procurement not yet bound to a filing) — so an in-progress acquire, or a
// stalled/failed one that needs a manual resume, never vanishes from the UI. No vendor cost.
type MAFilingsResponse struct {
	Identity         MAFilingIdentity          `json:"identity"`
	Filings          []MAFilingRow             `json:"filings"`
	LatestSearch     *MAFilingSearchView       `json:"latestSearch"`
	AcquisitionsOpen []MAFilingAcquisitionView `json:"acquisitionsOpen"`
}

// MAFilingIdentity is the fiscal identity the filings are keyed by.
type MAFilingIdentity struct {
	VAT       string `json:"vat,omitempty"`
	Tax       string `json:"tax,omitempty"`
	FiscalKey string `json:"fiscalKey"`
}

// MAFilingRow is one fascicolo: lifecycle + identity status, page count, error, and the DISTINCT
// acquisition origins that produced it. No price/cost fields (operator-only data).
type MAFilingRow struct {
	ID               string    `json:"id"`
	ClosingDate      string    `json:"closingDate,omitempty"`
	BalanceSheetID   string    `json:"balanceSheetId,omitempty"`
	BalanceSheetType string    `json:"balanceSheetType,omitempty"`
	TaxonomyVersion  string    `json:"taxonomyVersion,omitempty"`
	Status           string    `json:"status"`
	IdentityStatus   string    `json:"identityStatus"`
	PageCount        *int      `json:"pageCount,omitempty"`
	Error            string    `json:"error,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	Origins          []string  `json:"origins"`
}

// MAFilingSearchView is the latest DocuEngine search for the identity: its status and the
// deposited balance sheets it found — balanceSheetId / closingDate / type only, NEVER prices.
type MAFilingSearchView struct {
	ID        string                     `json:"id"`
	Status    string                     `json:"status"`
	Results   []MAFilingSearchResultView `json:"results"`
	UpdatedAt time.Time                  `json:"updatedAt"`
}

// MAFilingSearchResultView is one deposited balance sheet offered by a search (no prices).
type MAFilingSearchResultView struct {
	BalanceSheetID   string `json:"balanceSheetId"`
	ClosingDate      string `json:"closingDate,omitempty"`
	BalanceSheetType string `json:"balanceSheetType,omitempty"`
}

// MAFilingAcquisitionView is one still-open acquisition (procurement of a balance-sheet PDF not
// yet bound to a filing). closingDate/balanceSheetType are resolved at read from the linked
// search (a fact, never a price); Inflight is true only while a live filing_acquire job works
// it — a false Inflight on a non-terminal row is a stalled procurement the analyst can resume.
type MAFilingAcquisitionView struct {
	ID               string    `json:"id"`
	Status           string    `json:"status"`
	BalanceSheetID   string    `json:"balanceSheetId,omitempty"`
	ClosingDate      string    `json:"closingDate,omitempty"`
	BalanceSheetType string    `json:"balanceSheetType,omitempty"`
	Error            string    `json:"error,omitempty"`
	Inflight         bool      `json:"inflight"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// MAFilingUploadResponse is the result of an upload: the (possibly merged) filing id.
type MAFilingUploadResponse struct {
	FilingID string `json:"filingId"`
	Merged   bool   `json:"merged"`
}

// MAFilingSearchStartResponse reports the LIVE search the analyst should poll (on an enqueue
// dedup this is the pre-existing in-flight search, not the just-created orphan intent).
type MAFilingSearchStartResponse struct {
	SearchID string `json:"searchId"`
	Status   string `json:"status"`
}

// MAFilingAcquireResponse lists the acquisition ids actually enqueued (deduped ones are skipped).
type MAFilingAcquireResponse struct {
	AcquisitionIDs []string `json:"acquisitionIds"`
}

// MAFilingIdentityOverrideResponse acknowledges the one-shot manual identity acceptance.
type MAFilingIdentityOverrideResponse struct {
	Status string `json:"status"`
}

// MAFilingProposalsResponse is the nota-integrativa proposals of a filing's ACTIVE reading run,
// each with its decisions and derived state, plus the run identifiers.
type MAFilingProposalsResponse struct {
	RunID           string             `json:"runId,omitempty"`
	ProcessingRunID string             `json:"processingRunId,omitempty"`
	Proposals       []MANIProposalView `json:"proposals"`
}

// MANIProposalView is one immutable proposal with its full fields, its append-only decisions,
// and the state derived from the LATEST decision (coherent with the reducer).
type MANIProposalView struct {
	ID                   string             `json:"id"`
	ExerciseDate         string             `json:"exerciseDate,omitempty"`
	FattoOsservato       string             `json:"fattoOsservato"`
	ImportoLordo         *float64           `json:"importoLordo,omitempty"`
	TrattamentoCandidato string             `json:"trattamentoCandidato"`
	Direction            string             `json:"direction"`
	ImportoRettifica     *float64           `json:"importoRettifica,omitempty"`
	Incertezza           string             `json:"incertezza,omitempty"`
	Quote                string             `json:"quote,omitempty"`
	PageNo               *int               `json:"pageNo,omitempty"`
	Section              string             `json:"section,omitempty"`
	Label                string             `json:"label,omitempty"`
	Rationale            string             `json:"rationale,omitempty"`
	State                string             `json:"state"`
	Decisions            []MANIDecisionView `json:"decisions"`
}

// MANIDecisionView is one analyst decision. Actor is the operator email (subject fallback).
type MANIDecisionView struct {
	ID                  string    `json:"id"`
	Action              string    `json:"action"`
	RatifiedAmount      *float64  `json:"ratifiedAmount,omitempty"`
	RatifiedTreatment   string    `json:"ratifiedTreatment,omitempty"`
	Reason              string    `json:"reason,omitempty"`
	Actor               string    `json:"actor,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
	AutoReconfirmedFrom string    `json:"autoReconfirmedFrom,omitempty"`
}

// MANIDecisionResponse is the outcome of appending a decision: the new decision id and the
// proposal's resulting state (effective | rejected | revoked | pending).
type MANIDecisionResponse struct {
	DecisionID string `json:"decisionId"`
	State      string `json:"state"`
}

// Derived proposal states (coherent with reduceMANIEffectiveAdjustments).
const (
	maNIStateEffective = "effective"
	maNIStateRejected  = "rejected"
	maNIStateRevoked   = "revoked"
	maNIStatePending   = "pending"
)

// maNIDecisionInput is the validated decision the endpoint hands to the store (never carries
// AutoReconfirmedFrom — that field is reserved for the ingest's cross-run auto-reconfirmation).
type maNIDecisionInput struct {
	Action            string
	RatifiedAmount    *float64
	RatifiedTreatment string
	Reason            string
}

// ---------------------------------------------------------------------------
// Identity resolution (shared).
// ---------------------------------------------------------------------------

// resolveFilingIdentity maps a {companyKey} to its fiscal identity + canonical fiscal_key,
// collapsing "no valid vat/tax" to errMAFilingIdentityUnresolved (422). The deposited-filing
// pipeline is keyed by fiscal identity, never by company_key.
func (s *maService) resolveFilingIdentity(ctx context.Context, companyKey string) (maDeepDiveIdentity, string, error) {
	identity, _, err := s.resolveCompanyDeepDiveIdentity(ctx, companyKey)
	if err != nil {
		return maDeepDiveIdentity{}, "", errMAFilingIdentityUnresolved
	}
	fiscalKey := buildMAFiscalKey(identity.VATCode, identity.TaxCode)
	if fiscalKey == "" {
		return maDeepDiveIdentity{}, "", errMAFilingIdentityUnresolved
	}
	return identity, fiscalKey, nil
}

// ---------------------------------------------------------------------------
// 1. GET filings.
// ---------------------------------------------------------------------------

func (s *maService) listCompanyFilings(ctx context.Context, companyKey string) (MAFilingsResponse, error) {
	if s.store == nil || s.filing == nil {
		return MAFilingsResponse{}, errMAStoreUnavailable
	}
	identity, fiscalKey, err := s.resolveFilingIdentity(ctx, companyKey)
	if err != nil {
		return MAFilingsResponse{}, err
	}
	filings, err := s.filing.ListMAFilingsByFiscalKey(ctx, fiscalKey)
	if err != nil {
		return MAFilingsResponse{}, err
	}
	ids := make([]string, 0, len(filings))
	for i := range filings {
		ids = append(ids, filings[i].ID)
	}
	origins, err := s.filing.ListMAFilingOrigins(ctx, ids)
	if err != nil {
		return MAFilingsResponse{}, err
	}
	resp := MAFilingsResponse{
		Identity:         MAFilingIdentity{VAT: identity.VATCode, Tax: identity.TaxCode, FiscalKey: fiscalKey},
		Filings:          make([]MAFilingRow, 0, len(filings)),
		AcquisitionsOpen: []MAFilingAcquisitionView{},
	}
	for i := range filings {
		f := filings[i]
		row := MAFilingRow{
			ID:               f.ID,
			BalanceSheetID:   f.BalanceSheetID,
			BalanceSheetType: f.BalanceSheetType,
			TaxonomyVersion:  f.TaxonomyVersion,
			Status:           f.Status,
			IdentityStatus:   f.IdentityStatus,
			PageCount:        f.PageCount,
			Error:            f.Error,
			CreatedAt:        f.CreatedAt,
			Origins:          origins[f.ID],
		}
		if row.Origins == nil {
			row.Origins = []string{}
		}
		if f.ClosingDate != nil {
			row.ClosingDate = maDateValue(*f.ClosingDate)
		}
		resp.Filings = append(resp.Filings, row)
	}

	// Latest search: prefer one at 'results' (an acquirable offer), else the most recent search
	// of any status (so the UI can still show an intent/requested/unknown in progress).
	search, err := s.filing.GetLatestMAFilingSearchWithResults(ctx, fiscalKey)
	if err != nil {
		return MAFilingsResponse{}, err
	}
	if search == nil {
		search, err = s.filing.GetLatestMAFilingSearch(ctx, fiscalKey)
		if err != nil {
			return MAFilingsResponse{}, err
		}
	}
	if search != nil {
		resp.LatestSearch = buildMAFilingSearchView(search)
	}

	open, err := s.filing.ListMAFilingAcquisitionsOpen(ctx, fiscalKey)
	if err != nil {
		return MAFilingsResponse{}, err
	}
	for _, a := range open {
		resp.AcquisitionsOpen = append(resp.AcquisitionsOpen, MAFilingAcquisitionView{
			ID:               a.ID,
			Status:           a.Status,
			BalanceSheetID:   a.BalanceSheetID,
			ClosingDate:      a.ClosingDate,
			BalanceSheetType: a.BalanceSheetType,
			Error:            a.Error,
			Inflight:         a.HasInflightJob,
			CreatedAt:        a.CreatedAt,
			UpdatedAt:        a.UpdatedAt,
		})
	}
	return resp, nil
}

// buildMAFilingSearchView projects a search row to the UI view: status + the deposited balance
// sheets (balanceSheetId / closingDate / type). Prices in the raw jsonb are never surfaced.
func buildMAFilingSearchView(search *maFilingSearch) *MAFilingSearchView {
	view := &MAFilingSearchView{ID: search.ID, Status: search.Status, Results: []MAFilingSearchResultView{}, UpdatedAt: search.UpdatedAt}
	if len(search.Results) > 0 {
		var rows []maFilingSearchResultRow
		if err := json.Unmarshal(search.Results, &rows); err == nil {
			for _, r := range rows {
				// Normalize the vendor closing date (ISO8601Z or bare) to YYYY-MM-DD so the
				// search result matches the closingDate format of MAFilingRow; keep the raw
				// value only if it is unparseable.
				closing := r.BalanceSheetDate
				if t, ok := parseMAFilingDate(r.BalanceSheetDate); ok {
					closing = maDateValue(t)
				}
				view.Results = append(view.Results, MAFilingSearchResultView{
					BalanceSheetID:   r.BalanceSheetID,
					ClosingDate:      closing,
					BalanceSheetType: r.BalanceSheetType,
				})
			}
		}
	}
	return view
}

// ---------------------------------------------------------------------------
// 2. POST filings (upload).
// ---------------------------------------------------------------------------

// maFilingIngestRecoverable reports whether a re-uploaded identical filing should restart the
// ingest: a `failed` pipeline, or an `identity_blocked` one still awaiting validation
// (pending_validation — the soft/unreadable block). A hard `mismatch` (a readable but
// different fiscal code) is NOT recoverable by re-uploading the same bytes — it stays blocked
// until an explicit override.
func maFilingIngestRecoverable(f *maFiling) bool {
	switch f.Status {
	case maFilingStatusFailed:
		return true
	case maFilingStatusIdentityBlocked:
		return f.IdentityStatus == maFilingIdentityPendingValidation
	default:
		return false
	}
}

// uploadCompanyFiling ingests an analyst-supplied balance-sheet PDF. Free synchronous guards
// (mime sniff + declared header, size cap, digests) run first. Dedup: a bit-identical filing for
// the same fiscal identity is returned as merged=true WITHOUT a new ingest, unless it is failed
// (then the ingest is re-enqueued — idempotent on payload->>'filingId'). The new-filing path
// writes the acquisition intent FIRST, so any mid-flow error leaves a reconcilable trace.
func (s *maService) uploadCompanyFiling(ctx context.Context, companyKey, declaredMime string, pdf []byte, subject, email string) (MAFilingUploadResponse, error) {
	if s.store == nil || s.filing == nil {
		return MAFilingUploadResponse{}, errMAStoreUnavailable
	}
	identity, fiscalKey, err := s.resolveFilingIdentity(ctx, companyKey)
	if err != nil {
		return MAFilingUploadResponse{}, err
	}
	if len(pdf) == 0 {
		return MAFilingUploadResponse{}, errMAFilingEmptyUpload
	}
	if len(pdf) > maFilingUploadMaxBytes {
		return MAFilingUploadResponse{}, errMAFilingTooLarge
	}
	if http.DetectContentType(pdf) != "application/pdf" {
		return MAFilingUploadResponse{}, errMAFilingInvalidPDF
	}
	if m := strings.TrimSpace(declaredMime); m != "" {
		if parsed, _, perr := mime.ParseMediaType(m); perr == nil && parsed != "application/pdf" {
			return MAFilingUploadResponse{}, errMAFilingInvalidPDF
		}
	}
	md5Sum := md5.Sum(pdf)
	sha256Sum := sha256.Sum256(pdf)
	md5Hex := hex.EncodeToString(md5Sum[:])
	sha256Hex := hex.EncodeToString(sha256Sum[:])

	// Dedup pre-check: identical bytes for the same fiscal identity. A filing stuck in a
	// RECOVERABLE broken state — `failed`, or `identity_blocked` still `pending_validation`
	// (the soft, unreadable block — never a hard `mismatch`) — is reset to `queued` and the
	// ingest is re-enqueued: re-uploading the same PDF is the analyst's clean restart. It
	// re-runs OCR from scratch (a few cents), guaranteeing fresh pages WITH the vendor's
	// table `extras`, so the fixed identity/parse pipeline can now read the fiscal code out
	// of the frontespizio table and clear the gate that previously blocked it. Any other
	// state ⇒ merged, no re-ingest.
	if existing, derr := s.filing.GetMAFilingByBlob(ctx, fiscalKey, md5Hex); derr != nil {
		return MAFilingUploadResponse{}, derr
	} else if existing != nil {
		if maFilingIngestRecoverable(existing) {
			if uerr := s.filing.UpdateMAFilingStatus(ctx, existing.ID, maFilingStatusQueued, ""); uerr != nil {
				return MAFilingUploadResponse{}, uerr
			}
			if eerr := s.enqueueFilingIngest(ctx, existing.ID, fiscalKey, companyKey, subject, email); eerr != nil {
				return MAFilingUploadResponse{}, eerr
			}
		}
		return MAFilingUploadResponse{FilingID: existing.ID, Merged: true}, nil
	}

	// New filing. Intent FIRST (auditable), then blob, then filing, then bind + complete, then
	// enqueue the ingest with a non-empty context company key (so the baseline deep-dive fires).
	acqID, err := s.filing.CreateMAFilingAcquisitionIntent(ctx, maFilingAcquisitionCreate{
		Origin:            maFilingOriginUpload,
		ContextCompanyKey: companyKey,
		ActorSubject:      subject,
		ActorEmail:        email,
	})
	if err != nil {
		return MAFilingUploadResponse{}, err
	}
	if err := s.filing.InsertMAFilingBlob(ctx, md5Hex, sha256Hex, pdf); err != nil {
		return MAFilingUploadResponse{}, err
	}
	filingID, merged, err := s.filing.CreateOrMergeMAFiling(ctx, maFilingCreate{
		VAT:              identity.VATCode,
		Tax:              identity.TaxCode,
		BlobMD5:          md5Hex,
		CreatedBySubject: subject,
		CreatedByEmail:   email,
	})
	if err != nil {
		return MAFilingUploadResponse{}, err
	}
	if err := s.filing.LinkMAFilingAcquisitionFiling(ctx, acqID, filingID); err != nil {
		return MAFilingUploadResponse{}, err
	}
	if err := s.filing.CompleteMAFilingAcquisition(ctx, acqID, filingID); err != nil {
		return MAFilingUploadResponse{}, err
	}
	if err := s.enqueueFilingIngest(ctx, filingID, fiscalKey, companyKey, subject, email); err != nil {
		return MAFilingUploadResponse{}, err
	}
	return MAFilingUploadResponse{FilingID: filingID, Merged: merged}, nil
}

// ---------------------------------------------------------------------------
// 3. POST filings/search.
// ---------------------------------------------------------------------------

// startCompanyFilingSearch opens a durable DocuEngine search for the identity and reports the
// LIVE search to poll. On an enqueue dedup, startFilingSearch returns a fresh orphan intent id;
// the in-flight job points at the real search, so we resolve that (F8 pendenza: read the live
// search, don't trust the returned id blindly).
func (s *maService) startCompanyFilingSearch(ctx context.Context, companyKey, subject, email string) (MAFilingSearchStartResponse, error) {
	if s.store == nil || s.filing == nil {
		return MAFilingSearchStartResponse{}, errMAStoreUnavailable
	}
	identity, fiscalKey, err := s.resolveFilingIdentity(ctx, companyKey)
	if err != nil {
		return MAFilingSearchStartResponse{}, err
	}
	taxCode := maVendorTaxCode(identity.VATCode, identity.TaxCode)
	searchID, err := s.startFilingSearch(ctx, fiscalKey, taxCode, subject, email)
	if err != nil {
		return MAFilingSearchStartResponse{}, err
	}
	liveID := searchID
	if inflight, ierr := s.filing.GetInflightMAFilingSearchID(ctx, fiscalKey); ierr == nil && inflight != "" {
		liveID = inflight
	}
	status := maFilingSearchIntent
	if rec, gerr := s.filing.GetMAFilingSearch(ctx, liveID); gerr == nil && rec != nil {
		status = rec.Status
	}
	return MAFilingSearchStartResponse{SearchID: liveID, Status: status}, nil
}

// ---------------------------------------------------------------------------
// 4. POST filings/acquire.
// ---------------------------------------------------------------------------

func (s *maService) acquireCompanyFilings(ctx context.Context, companyKey, searchID string, balanceSheetIDs []string, subject, email string) (MAFilingAcquireResponse, error) {
	if s.store == nil || s.filing == nil {
		return MAFilingAcquireResponse{}, errMAStoreUnavailable
	}
	identity, _, err := s.resolveFilingIdentity(ctx, companyKey)
	if err != nil {
		return MAFilingAcquireResponse{}, err
	}
	searchID = strings.TrimSpace(searchID)
	if searchID == "" {
		return MAFilingAcquireResponse{}, errMAFilingSearchIDRequired
	}
	cleaned := make([]string, 0, len(balanceSheetIDs))
	for _, b := range balanceSheetIDs {
		if t := strings.TrimSpace(b); t != "" {
			cleaned = append(cleaned, t)
		}
	}
	if len(cleaned) == 0 {
		return MAFilingAcquireResponse{}, errMAFilingBalanceSheetsRequired
	}
	ids, err := s.startFilingAcquisitions(ctx, searchID, cleaned, identity.VATCode, identity.TaxCode, companyKey, subject, email)
	if err != nil {
		return MAFilingAcquireResponse{}, err
	}
	if ids == nil {
		ids = []string{}
	}
	return MAFilingAcquireResponse{AcquisitionIDs: ids}, nil
}

// ---------------------------------------------------------------------------
// 4b. POST filings/acquisitions/{id}/retry.
// ---------------------------------------------------------------------------

// retryFilingAcquisition re-drives a still-open acquisition that lost its worker (a job killed by
// poll_timeout / exhausted attempts, or an intent orphan left by an enqueue dedup) — the paid
// acquisition never had a filing to show, so without this it was invisible AND unrecoverable.
//
// It re-enqueues a filing_acquire job with the SAME acquisitionId, so the resume starts from the
// PERSISTED acquisition status, never a fresh spend by default:
//   - requested/unknown with a docuengine_request_id ⇒ the resume GETs/reconciles that request
//     (never a new POST — the vendor call was already paid);
//   - failed BEFORE any request id ⇒ a clean 'intent' restart, where the explicit analyst gesture
//     authorises a possible new spend (the deep-dive-retry contract);
//   - failed WITH a request id ⇒ reopened to 'unknown' so the reconcile adopts the paid request.
//
// The mig-115 inflight index dedups a racing enqueue. 409 when the row is done / already bound to
// a filing / already worked by a live job; 404 when it does not exist.
func (s *maService) retryFilingAcquisition(ctx context.Context, companyKey, acquisitionID, subject, email string) (MAFilingAcquireResponse, error) {
	if s.store == nil || s.filing == nil {
		return MAFilingAcquireResponse{}, errMAStoreUnavailable
	}
	identity, fiscalKey, err := s.resolveFilingIdentity(ctx, companyKey)
	if err != nil {
		return MAFilingAcquireResponse{}, err
	}
	acq, err := s.filing.GetMAFilingAcquisition(ctx, acquisitionID)
	if err != nil {
		return MAFilingAcquireResponse{}, err
	}
	if acq == nil {
		return MAFilingAcquireResponse{}, errMAFilingAcquisitionNotFound
	}
	// Not retryable: already procured (done / bound to a filing) or actively being worked.
	if acq.Status == maFilingAcquisitionDone || strings.TrimSpace(acq.FilingID) != "" {
		return MAFilingAcquireResponse{}, errMAFilingAcquisitionNotRetryable
	}
	inflight, err := s.filing.HasInflightMAFilingAcquireJob(ctx, acquisitionID)
	if err != nil {
		return MAFilingAcquireResponse{}, err
	}
	if inflight {
		return MAFilingAcquireResponse{}, errMAFilingAcquisitionNotRetryable
	}
	switch acq.Status {
	case maFilingAcquisitionFailed:
		// The ONLY sanctioned exit from a terminal 'failed' row, on the explicit retry gesture:
		// resume via the persisted request when there is one, else restart clean.
		if strings.TrimSpace(acq.DocuEngineRequestID) != "" {
			if rerr := s.filing.ReopenMAFilingAcquisitionUnknown(ctx, acquisitionID); rerr != nil {
				return MAFilingAcquireResponse{}, rerr
			}
		} else if rerr := s.filing.ReopenMAFilingAcquisitionIntent(ctx, acquisitionID); rerr != nil {
			return MAFilingAcquireResponse{}, rerr
		}
	case maFilingAcquisitionIntent, maFilingAcquisitionRequested, maFilingAcquisitionDownloaded, maFilingAcquisitionUnknown:
		// A non-terminal open row with no live job: re-enqueue and let the job resume from the
		// persisted status — no state mutation needed.
	default:
		return MAFilingAcquireResponse{}, errMAFilingAcquisitionNotRetryable
	}
	// Rebuild the acquire payload as startFilingAcquisitions would: the searchId is the acquisition's
	// linked (consumed) search when it has one, so a resume re-uses that request; "" otherwise.
	searchID, _ := s.filing.GetMAFilingSearchIDConsumedByAcquisition(ctx, acquisitionID)
	payload, err := json.Marshal(maFilingAcquireJobPayload{
		FiscalKey:         fiscalKey,
		AcquisitionID:     acquisitionID,
		SearchID:          searchID,
		BalanceSheetID:    acq.BalanceSheetID,
		ContextCompanyKey: companyKey,
		TaxCode:           maVendorTaxCode(identity.VATCode, identity.TaxCode),
		VAT:               normalizeMAFiscalValue(identity.VATCode),
		Tax:               normalizeMAFiscalValue(identity.TaxCode),
	})
	if err != nil {
		return MAFilingAcquireResponse{}, err
	}
	if _, _, err := s.store.EnqueueMAJob(ctx, maJobEnqueue{
		JobType: maJobTypeFilingAcquire,
		Status:  maJobStatusPending,
		Subject: subject,
		Email:   email,
		Payload: payload,
		Owner:   s.owner,
	}); err != nil {
		return MAFilingAcquireResponse{}, err
	}
	return MAFilingAcquireResponse{AcquisitionIDs: []string{acquisitionID}}, nil
}

// ---------------------------------------------------------------------------
// 5. POST filings/{id}/identity-override.
// ---------------------------------------------------------------------------

// applyFilingIdentityOverride records the one-shot manual acceptance of an unreadable-identity
// filing and RE-ENQUEUES the ingest (nothing else resumes the pipeline after an override). The
// resume context company key is taken from the filing's latest acquisition ("" tolerated: the
// enqueue guard then skips the baseline, leaving a degraded filing rather than a wrong baseline).
func (s *maService) applyFilingIdentityOverride(ctx context.Context, filingID, reason, subject, email string) (MAFilingIdentityOverrideResponse, error) {
	if s.store == nil || s.filing == nil {
		return MAFilingIdentityOverrideResponse{}, errMAStoreUnavailable
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return MAFilingIdentityOverrideResponse{}, errMAFilingReasonRequired
	}
	filing, err := s.filing.GetMAFiling(ctx, filingID)
	if err != nil {
		return MAFilingIdentityOverrideResponse{}, err
	}
	if filing == nil {
		return MAFilingIdentityOverrideResponse{}, errMAFilingNotFound
	}
	applied, err := s.filing.ApplyMAFilingIdentityOverride(ctx, filingID, subject, email, reason)
	if err != nil {
		return MAFilingIdentityOverrideResponse{}, err
	}
	if !applied {
		return MAFilingIdentityOverrideResponse{}, errMAFilingIdentityOverrideNotApplicable
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_filing_identity_override",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"filing_id": filingID, "reason": reason, "by": email, "subject": subject}),
	})
	// Re-enqueue the ingest so the override actually resumes the (identity_blocked) pipeline.
	ctxKey, _ := s.filing.GetMAFilingLatestAcquisitionContext(ctx, filingID)
	if err := s.enqueueFilingIngest(ctx, filingID, filing.FiscalKey, ctxKey, subject, email); err != nil {
		return MAFilingIdentityOverrideResponse{}, err
	}
	return MAFilingIdentityOverrideResponse{Status: maFilingIdentityOverride}, nil
}

// ---------------------------------------------------------------------------
// 6. GET filings/{id}/pdf.
// ---------------------------------------------------------------------------

// filingPDF returns the stored PDF bytes + a sensible download filename (fiscal_key + closing).
// 404 (errMAFilingNotFound / errMAFilingBlobMissing) when the filing or its blob is absent.
func (s *maService) filingPDF(ctx context.Context, filingID string) ([]byte, string, error) {
	if s.store == nil || s.filing == nil {
		return nil, "", errMAStoreUnavailable
	}
	filing, err := s.filing.GetMAFiling(ctx, filingID)
	if err != nil {
		return nil, "", err
	}
	if filing == nil {
		return nil, "", errMAFilingNotFound
	}
	md5Hex := strings.TrimSpace(filing.BlobMD5)
	if md5Hex == "" {
		return nil, "", errMAFilingBlobMissing
	}
	pdf, _, _, err := s.filing.GetMAFilingBlob(ctx, md5Hex)
	if err != nil {
		return nil, "", err
	}
	if len(pdf) == 0 {
		return nil, "", errMAFilingBlobMissing
	}
	return pdf, maFilingPDFFilename(filing), nil
}

// maFilingPDFFilename builds a stable, safe download name from the fiscal identity + closing
// date. fiscal_key is normally alnum and closing_date YYYY-MM-DD, but the components are
// sanitized regardless (defense in depth) so nothing can break the Content-Disposition header.
func maFilingPDFFilename(f *maFiling) string {
	base := strings.TrimSpace(f.FiscalKey)
	if base == "" {
		base = "bilancio"
	}
	if f.ClosingDate != nil {
		base += "_" + maDateValue(*f.ClosingDate)
	}
	return "bilancio_" + sanitizeMAFilenameComponent(base) + ".pdf"
}

// sanitizeMAFilenameComponent collapses every character outside [A-Za-z0-9_-] to '_', so a
// stray separator, whitespace, or quote in the fiscal/closing component cannot escape the
// quoted Content-Disposition filename.
func sanitizeMAFilenameComponent(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '_'
		}
	}, s)
}

// ---------------------------------------------------------------------------
// 7. GET filings/{id}/proposals.
// ---------------------------------------------------------------------------

func (s *maService) listFilingProposals(ctx context.Context, filingID string) (MAFilingProposalsResponse, error) {
	if s.store == nil || s.filing == nil {
		return MAFilingProposalsResponse{}, errMAStoreUnavailable
	}
	filing, err := s.filing.GetMAFiling(ctx, filingID)
	if err != nil {
		return MAFilingProposalsResponse{}, err
	}
	if filing == nil {
		return MAFilingProposalsResponse{}, errMAFilingNotFound
	}
	run, err := s.filing.GetActiveMANIReadingRun(ctx, filingID)
	if err != nil {
		return MAFilingProposalsResponse{}, err
	}
	props, err := s.filing.ListMANIProposalsWithDecisions(ctx, filingID)
	if err != nil {
		return MAFilingProposalsResponse{}, err
	}
	resp := MAFilingProposalsResponse{Proposals: make([]MANIProposalView, 0, len(props))}
	if run != nil {
		resp.RunID = run.ID
		resp.ProcessingRunID = run.ProcessingRunID
	}
	for _, p := range props {
		resp.Proposals = append(resp.Proposals, buildMANIProposalView(p.Proposal, p.Decisions))
	}
	return resp, nil
}

// buildMANIProposalView projects one proposal + its decisions to the UI view with the derived
// state. Decisions are already ordered oldest-first by the store.
func buildMANIProposalView(p maNIProposal, decisions []maNIDecision) MANIProposalView {
	view := MANIProposalView{
		ID:                   p.ID,
		FattoOsservato:       p.FattoOsservato,
		ImportoLordo:         p.ImportoLordo,
		TrattamentoCandidato: p.TrattamentoCandidato,
		Direction:            p.Direction,
		ImportoRettifica:     p.ImportoRettifica,
		Incertezza:           p.Incertezza,
		Quote:                p.Quote,
		PageNo:               p.PageNo,
		Section:              p.Section,
		Label:                p.Label,
		Rationale:            p.Rationale,
		State:                classifyMANIProposalState(p, decisions),
		Decisions:            make([]MANIDecisionView, 0, len(decisions)),
	}
	if p.ExerciseDate != nil {
		view.ExerciseDate = maDateValue(*p.ExerciseDate)
	}
	for _, d := range decisions {
		actor := d.ActorEmail
		if actor == "" {
			actor = d.ActorSubject
		}
		view.Decisions = append(view.Decisions, MANIDecisionView{
			ID:                  d.ID,
			Action:              d.Action,
			RatifiedAmount:      d.RatifiedAmount,
			RatifiedTreatment:   d.RatifiedTreatment,
			Reason:              d.Reason,
			Actor:               actor,
			CreatedAt:           d.CreatedAt,
			AutoReconfirmedFrom: d.AutoReconfirmedFrom,
		})
	}
	return view
}

// latestMANIDecision returns the current decision for a proposal (newest by created_at, id
// tie-break — the SAME ordering as the reducer), or nil when there are none.
func latestMANIDecision(decisions []maNIDecision) *maNIDecision {
	var latest *maNIDecision
	for i := range decisions {
		d := &decisions[i]
		if latest == nil || d.CreatedAt.After(latest.CreatedAt) || (d.CreatedAt.Equal(latest.CreatedAt) && d.ID > latest.ID) {
			latest = d
		}
	}
	return latest
}

// classifyMANIProposalState derives the proposal state from its latest decision, reusing the
// reducer for the effectiveness test (never duplicating the effect rules): a ratify is
// "effective" only when reduceMANIEffectiveAdjustments would surface it, else "pending"; a
// reject/revoke maps to its literal state; no decision ⇒ "pending".
func classifyMANIProposalState(p maNIProposal, decisions []maNIDecision) string {
	latest := latestMANIDecision(decisions)
	if latest == nil {
		return maNIStatePending
	}
	switch latest.Action {
	case maNIActionReject:
		return maNIStateRejected
	case maNIActionRevoke:
		return maNIStateRevoked
	case maNIActionRatify:
		if len(reduceMANIEffectiveAdjustments([]maNIProposal{p}, decisions)) > 0 {
			return maNIStateEffective
		}
		return maNIStatePending
	default:
		return maNIStatePending
	}
}

// ---------------------------------------------------------------------------
// 8. POST proposals/{id}/decision.
// ---------------------------------------------------------------------------

// decideNIProposal appends an analyst decision to a proposal (append-only). The actor comes from
// the auth context; AutoReconfirmedFrom is NEVER set from a human endpoint. Validations mirror
// the reducer so a ratify that could never take effect is rejected legibly (422) up front.
func (s *maService) decideNIProposal(ctx context.Context, proposalID string, in maNIDecisionInput, subject, email string) (MANIDecisionResponse, error) {
	if s.store == nil || s.filing == nil {
		return MANIDecisionResponse{}, errMAStoreUnavailable
	}
	action := strings.TrimSpace(in.Action)
	if action != maNIActionRatify && action != maNIActionReject && action != maNIActionRevoke {
		return MANIDecisionResponse{}, errMANIActionInvalid
	}
	proposal, err := s.filing.GetMANIProposal(ctx, proposalID)
	if err != nil {
		return MANIDecisionResponse{}, err
	}
	if proposal == nil {
		return MANIDecisionResponse{}, errMANIProposalNotFound
	}
	treatment := strings.TrimSpace(in.RatifiedTreatment)
	amount := in.RatifiedAmount
	if action == maNIActionRatify {
		if err := validateMANIRatify(*proposal, amount, treatment); err != nil {
			return MANIDecisionResponse{}, err
		}
	} else {
		// reject / revoke carry no promotion fields (keep the decision row unambiguous).
		amount = nil
		treatment = ""
	}
	decisionID, err := s.filing.InsertMANIDecision(ctx, maNIDecisionCreate{
		ProposalID:        proposalID,
		Action:            action,
		RatifiedAmount:    amount,
		RatifiedTreatment: treatment,
		Reason:            strings.TrimSpace(in.Reason),
		ActorSubject:      subject,
		ActorEmail:        email,
	})
	if err != nil {
		return MANIDecisionResponse{}, err
	}
	decisions, err := s.filing.ListMANIDecisionsByProposals(ctx, []string{proposalID})
	if err != nil {
		return MANIDecisionResponse{}, err
	}
	return MANIDecisionResponse{DecisionID: decisionID, State: classifyMANIProposalState(*proposal, decisions)}, nil
}

// validateMANIRatify enforces that a ratify resolves to a real effect, mirroring the reducer:
//   - the effective treatment (ratified_treatment when set, else the candidate) must be
//     ebitda|pfn — a dd_only candidate needs an explicit promotion treatment;
//   - a signed amount must be resolvable — required when the direction is uncertain, when the
//     candidate is dd_only (promotion), or when the proposal has no importo_rettifica to fall
//     back on (an uncertain verso, or a certain verso without amount, has no effect).
//
// INVARIANT: this endpoint guard is deliberately STRICTER than reduceMANIEffectiveAdjustments —
// the reducer silently drops an ineffective ratify, whereas here we reject it up front with a
// legible 422 so the analyst never records a ratify that could never take effect. The dd_only
// promotion branch always demands ratifiedAmount because, by F6 design, a dd_only candidate is
// the demotion bucket and carries a nil importo_rettifica (there is nothing to fall back on).
func validateMANIRatify(p maNIProposal, amount *float64, ratifiedTreatment string) error {
	effTreatment := p.TrattamentoCandidato
	if ratifiedTreatment != "" {
		if ratifiedTreatment != maNITrattamentoEbitda && ratifiedTreatment != maNITrattamentoPFN {
			return errMANIRatifiedTreatmentInvalid
		}
		effTreatment = ratifiedTreatment
	}
	if effTreatment != maNITrattamentoEbitda && effTreatment != maNITrattamentoPFN {
		return errMANIRatifiedTreatmentRequired
	}
	if amount == nil {
		promotion := p.TrattamentoCandidato == maNITrattamentoDDOnly
		if promotion || p.Direction == maNIDirectionUncertain || p.ImportoRettifica == nil {
			return errMANIRatifiedAmountRequired
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 9. GET filings/{id}/narrative  +  10. POST filings/{id}/narrative/regenerate (issue #80).
// ---------------------------------------------------------------------------

// MAFilingNarrativeResponse is the nota-integrativa NARRATIVE reading of a filing's current run:
// its status + budget-truncation flag + the immutable context observations (attribuzioni | rischi
// | piani | profilo). Additive to the filing surface; carries no numbers and no prices.
type MAFilingNarrativeResponse struct {
	RunID        string                `json:"runId,omitempty"`
	Status       string                `json:"status"` // absent | running | ready | failed
	GeneratedAt  *time.Time            `json:"generatedAt,omitempty"`
	Truncated    bool                  `json:"truncated"`
	Observations []MANIObservationView `json:"observations"`
}

// MANIObservationView is one narrative observation for the UI (no amount — qualitative by design).
type MANIObservationView struct {
	ID     string `json:"id"`
	Tipo   string `json:"tipo"`
	Claim  string `json:"claim,omitempty"`
	Quote  string `json:"quote,omitempty"`
	PageNo *int   `json:"pageNo,omitempty"`
}

// MAFilingNarrativeRegenerateResponse acknowledges an enqueued regeneration ("in corso").
type MAFilingNarrativeRegenerateResponse struct {
	Status string `json:"status"`
}

// maNINarrativeStatusAbsent is the GET status when a filing has no narrative run yet (the mig-118
// binding is not applied, or the ingest predates the narrative feature): the UI offers "Genera".
const maNINarrativeStatusAbsent = "absent"

// getFilingNarrative returns the filing's current narrative run + its observations (SELECT-only).
// 'absent' when no run exists yet. 404 when the filing itself is unknown.
func (s *maService) getFilingNarrative(ctx context.Context, filingID string) (MAFilingNarrativeResponse, error) {
	if s.store == nil || s.filing == nil {
		return MAFilingNarrativeResponse{}, errMAStoreUnavailable
	}
	filing, err := s.filing.GetMAFiling(ctx, filingID)
	if err != nil {
		return MAFilingNarrativeResponse{}, err
	}
	if filing == nil {
		return MAFilingNarrativeResponse{}, errMAFilingNotFound
	}
	resp := MAFilingNarrativeResponse{Status: maNINarrativeStatusAbsent, Observations: []MANIObservationView{}}
	run, err := s.filing.GetActiveMANINarrativeRun(ctx, filingID)
	if err != nil {
		// mig 118 not applied ⇒ narrative tables absent (42P01): report "absent", never 500 —
		// the whole scheda would otherwise error on every open before the migration lands.
		if maNINarrativeSchemaMissing(err) {
			return resp, nil
		}
		return MAFilingNarrativeResponse{}, err
	}
	if run == nil {
		return resp, nil
	}
	resp.RunID = run.ID
	resp.Status = run.Status
	resp.Truncated = run.Truncated
	generatedAt := run.CreatedAt
	resp.GeneratedAt = &generatedAt
	observations, err := s.filing.ListMANIObservationsByRun(ctx, run.ID)
	if err != nil {
		// Defensive symmetry: both tables ship in mig 118 together, so if the run table resolved
		// the observation table is present too — but tolerate 42P01 here as well (run kept, no
		// observations) rather than 500.
		if maNINarrativeSchemaMissing(err) {
			return resp, nil
		}
		return MAFilingNarrativeResponse{}, err
	}
	for _, o := range observations {
		resp.Observations = append(resp.Observations, MANIObservationView{
			ID:     o.ID,
			Tipo:   o.Tipo,
			Claim:  o.Claim,
			Quote:  o.Quote,
			PageNo: o.PageNo,
		})
	}
	return resp, nil
}

// regenerateFilingNarrative enqueues an async narrative regeneration for a ready|degraded filing.
// It works ONLY on the persisted OCR pages (never re-OCR, never a vendor call — LLM only). The
// registry binding (scope ma_ni_narrative) is resolved SYNCHRONOUSLY here so a missing binding
// (mig 118 not applied) fails the POST with a clear config error instead of silently enqueuing a
// job that would never produce a narrative. The durable filing_narrative job is deduped on the
// filing id (mig-118 inflight index) so a double-click enqueues at most one; the «in corso» state
// becomes observable via GET once the worker opens the run.
func (s *maService) regenerateFilingNarrative(ctx context.Context, filingID, subject, email string) (MAFilingNarrativeRegenerateResponse, error) {
	if s.store == nil || s.filing == nil {
		return MAFilingNarrativeRegenerateResponse{}, errMAStoreUnavailable
	}
	if s.llmp == nil {
		return MAFilingNarrativeRegenerateResponse{}, errMALLMConfigUnavailable
	}
	filing, err := s.filing.GetMAFiling(ctx, filingID)
	if err != nil {
		return MAFilingNarrativeRegenerateResponse{}, err
	}
	if filing == nil {
		return MAFilingNarrativeRegenerateResponse{}, errMAFilingNotFound
	}
	if filing.Status != maFilingStatusReady && filing.Status != maFilingStatusDegraded {
		return MAFilingNarrativeRegenerateResponse{}, errMANINarrativeNotRegenerable
	}
	// Config guard: a missing model/prompt binding surfaces a clear error at the POST (not silent).
	if _, err := s.llmp.ResolveModel(ctx, maModelScopeNINarrative, ""); err != nil {
		return MAFilingNarrativeRegenerateResponse{}, llmConfigError(err)
	}
	if _, err := s.llmp.ResolvePrompt(ctx, maModelScopeNINarrative, ""); err != nil {
		return MAFilingNarrativeRegenerateResponse{}, llmConfigError(err)
	}
	payload, err := json.Marshal(maFilingNarrativeJobPayload{FilingID: filingID})
	if err != nil {
		return MAFilingNarrativeRegenerateResponse{}, err
	}
	if _, _, err := s.store.EnqueueMAJob(ctx, maJobEnqueue{
		JobType: maJobTypeFilingNarrative,
		Status:  maJobStatusPending,
		Subject: subject,
		Email:   email,
		Payload: payload,
		Owner:   s.owner,
	}); err != nil {
		return MAFilingNarrativeRegenerateResponse{}, err
	}
	return MAFilingNarrativeRegenerateResponse{Status: maNINarrativeStatusRunning}, nil
}
