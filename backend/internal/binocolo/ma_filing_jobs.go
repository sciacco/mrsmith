package binocolo

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/openapiit"
)

// Deposited-filing DocuEngine jobs (issue #78, Fase 4). Two durable state machines run
// off the request path on the shared ma_job queue: filing_search (find deposited balance
// sheets for a fiscal identity) and filing_acquire (buy one balance-sheet PDF from a
// search's results). The overriding invariant is ABSOLUTE idempotency of the paid vendor
// calls: no POST/PATCH is ever re-issued for the same intent without an intervening GET
// that observes the request already advanced.
//
// Error discipline (both jobs):
//   - DEFINITIVE vendor error (a typed *DocuEngineError business error, or a 4xx
//     *APIError other than 429) ⇒ the business row transitions to `failed`; the request
//     was not created / cannot succeed, so there is nothing to reconcile.
//   - AMBIGUOUS vendor error (network/timeout, 5xx, 429) ⇒ the business row transitions to
//     `unknown`, persisting whatever we hold (the docuengine_request_id when a prior step
//     obtained it). It is NEVER resolved heuristically and NEVER blind-retried: the next
//     tick reconciles from the persisted state (GetRequest when we have an id, else
//     ListRequests + timestamp window + readableSearch.taxCode match).
//   - Waiting on the vendor (results not ready, document generating, unknown being
//     reconciled) is signalled with errMAFilingPollPending — an expected state, not a
//     failure — so the worker re-runs the row next tick within the WALL-CLOCK poll window
//     (created_at + maFilingPollWindow), never bumping the attempt counter and never taking
//     the maJobMaxAttempts terminal give-up path (that budget is reserved for real infra
//     errors, including the post-payment finalize). Past the window the business row goes to
//     'unknown' (not 'failed'), so a later reconciliation can still adopt the paid request.
//
// The retry ALWAYS resumes from the durable business row (search/acquisition status),
// never from an in-flight call. The worker dispatch (ma_job_worker.go) turns the sentinel
// into a re-run and, on terminal give-up, fails the business row unless it is `unknown`.

// errMAFilingPollPending tells the worker a filing job is still waiting on DocuEngine.
// It is wrapped by helpers, so callers must test it with errors.Is.
var errMAFilingPollPending = errors.New("ma filing poll pending")

// errMAFilingMD5Mismatch is an internal sentinel: the downloaded PDF digest did not match
// the vendor md5 after a re-download. The download is unpaid, so a mismatch fails the
// acquisition (a corrupt fetch) without re-charging.
var errMAFilingMD5Mismatch = errors.New("ma filing download md5 mismatch")

// maFilingStore is the deposited-filing persistence the filing jobs need (the F3
// lifecycle methods only). Kept narrow so the jobs stay testable with an in-memory fake;
// *SQLStore satisfies it.
type maFilingStore interface {
	// search lifecycle
	CreateMAFilingSearchIntent(ctx context.Context, fiscalKey, subject, email string) (string, error)
	GetMAFilingSearch(ctx context.Context, id string) (*maFilingSearch, error)
	SetMAFilingSearchRequested(ctx context.Context, id, docuRequestID string) error
	SetMAFilingSearchResults(ctx context.Context, id string, results []byte) error
	SetMAFilingSearchUnknown(ctx context.Context, id, errMsg string) error
	FailMAFilingSearch(ctx context.Context, id, errMsg string) error
	AdoptMAFilingSearchRequest(ctx context.Context, id, docuRequestID string) error
	ClaimMAFilingSearchConsumed(ctx context.Context, searchID, acquisitionID string) (bool, error)
	// acquisition lifecycle
	CreateMAFilingAcquisitionIntent(ctx context.Context, in maFilingAcquisitionCreate) (string, error)
	GetMAFilingAcquisition(ctx context.Context, id string) (*maFilingAcquisition, error)
	SetMAFilingAcquisitionRequested(ctx context.Context, id, docuRequestID string) error
	SetMAFilingAcquisitionDownloaded(ctx context.Context, id string) error
	CompleteMAFilingAcquisition(ctx context.Context, id, filingID string) error
	FailMAFilingAcquisition(ctx context.Context, id, errMsg string) error
	SetMAFilingAcquisitionUnknown(ctx context.Context, id, errMsg string) error
	AdoptMAFilingAcquisitionRequest(ctx context.Context, id, docuRequestID string) error
	// blob + filing
	InsertMAFilingBlob(ctx context.Context, md5Hex, sha256Hex string, pdf []byte) error
	CreateOrMergeMAFiling(ctx context.Context, in maFilingCreate) (string, bool, error)
	// reconciliation exclusion set (pre-POST idempotency)
	ListMAFilingReferencedRequestIDs(ctx context.Context, excludeSearchID, excludeAcquisitionID string) (map[string]bool, error)
	// ingest (F5): OCR/parse pipeline on a downloaded filing.
	GetMAFiling(ctx context.Context, id string) (*maFiling, error)
	GetMAFilingBlob(ctx context.Context, md5Hex string) ([]byte, string, int64, error)
	CreateMAFilingProcessingRun(ctx context.Context, filingID, ocrModel, ocrParamsVersion, parseVersion, requestID string) (string, error)
	InsertMAFilingPages(ctx context.Context, runID string, pages []maFilingPage) error
	GetMAFilingPages(ctx context.Context, runID string) ([]maFilingPage, error)
	GetMAFilingPageMarkdown(ctx context.Context, runID string, pageNo int) (string, error)
	SetMAFilingPageCount(ctx context.Context, id string, pageCount int) error
	UpdateMAFilingStatus(ctx context.Context, id, status, errMsg string) error
	SetMAFilingParsed(ctx context.Context, id string, closingDate *time.Time, balanceSheetType, taxonomyVersion string, pageCount int) error
	SetMAFilingIdentityStatus(ctx context.Context, id, identityStatus string) error
	UpsertMAFilingExtract(ctx context.Context, runID string, exerciseDate time.Time, sp, ce, checks, docai, docaiDiff []byte) error
	HasMAFilingDocuEngineAcquisition(ctx context.Context, filingID string) (bool, error)
	GetMAFilingExtractsByFiscalKey(ctx context.Context, fiscalKey, excludeFilingID string, exerciseDates []time.Time) ([]maFilingPeerExtract, error)
	SweepMAFilingIngestOrphans(ctx context.Context, olderThan time.Duration) ([]maFilingIngestOrphan, error)
}

// maDocuEngine is the DocuEngine sub-client seam the filing jobs drive. *openapiit.
// DocuEngineClient satisfies it; a fake is injected in tests so no real, paid vendor call
// is ever made. Kept to exactly the methods the jobs use.
type maDocuEngine interface {
	CreateRequest(ctx context.Context, body openapiit.DocuRequestCreate) (openapiit.DocuRequest, error)
	GetRequest(ctx context.Context, id string) (openapiit.DocuRequest, error)
	SubmitSearch(ctx context.Context, id, taxCode string) (openapiit.DocuRequest, error)
	SelectResult(ctx context.Context, id, resultID string) (openapiit.DocuRequest, error)
	ListRequests(ctx context.Context) ([]openapiit.DocuRequestSummary, error)
	ListRequestDocuments(ctx context.Context, id string) ([]openapiit.DocuDownload, error)
	DownloadFile(ctx context.Context, signedURL string) ([]byte, error)
}

// docuEngine returns the DocuEngine seam: the injected fake in tests, else the live
// sub-client. Nil when neither is configured (the caller surfaces errMAOpenAPIITUnavailable).
func (s *maService) docuEngine() maDocuEngine {
	if s.docu != nil {
		return s.docu
	}
	if s.openapiit == nil {
		return nil
	}
	return s.openapiit.DocuEngine()
}

// maFilingSearchJobPayload is the filing_search job's self-contained args. FiscalKey keys
// the intent row and the inflight dedup (mig 115); TaxCode is the value POSTed to
// DocuEngine (the codice fiscale — see maVendorTaxCode).
type maFilingSearchJobPayload struct {
	FiscalKey string `json:"fiscalKey"`
	SearchID  string `json:"searchId"`
	TaxCode   string `json:"taxCode"`
}

// maFilingAcquireJobPayload is the filing_acquire job's args. Beyond the contract keys it
// carries TaxCode/VAT/Tax: TaxCode is needed by the lost-claim NEW-request path (it must
// SubmitSearch(taxCode) itself), and VAT/Tax reproduce the filing's fiscal identity so
// CreateOrMergeMAFiling writes the same fiscal_key (and correctly labelled vat/tax clean
// columns) as the identity-aware enqueue. Documented deviation from the minimal key set.
type maFilingAcquireJobPayload struct {
	FiscalKey         string `json:"fiscalKey"`
	AcquisitionID     string `json:"acquisitionId"`
	SearchID          string `json:"searchId"`
	BalanceSheetID    string `json:"balanceSheetId"`
	ContextCompanyKey string `json:"contextCompanyKey"`
	TaxCode           string `json:"taxCode"`
	VAT               string `json:"vat"`
	Tax               string `json:"tax"`
}

// maFilingIngestJobPayload is the filing_ingest job's args. Enqueued by the acquire on
// completion; F5 owns its dispatch (the worker leaves it 'pending' until then).
type maFilingIngestJobPayload struct {
	FilingID          string `json:"filingId"`
	FiscalKey         string `json:"fiscalKey"`
	ContextCompanyKey string `json:"contextCompanyKey"`
}

// maFilingSearchResultRow is one serialized DocuEngine result stored in
// ma_filing_search.results: the opaque per-request result_id, the decoded balance-sheet
// facts, and the raw data for defensive re-reads. The result_id is only a hint for the
// UI/audit — the acquire always remaps the target balanceSheetId to a result id FROM THE
// LIVE request it is working (result ids are per-request and never reused across requests).
type maFilingSearchResultRow struct {
	ResultID         string          `json:"resultId"`
	BalanceSheetID   string          `json:"balanceSheetId"`
	BalanceSheetType string          `json:"balanceSheetType"`
	BalanceSheetDate string          `json:"balanceSheetDate"`
	Raw              json.RawMessage `json:"raw,omitempty"`
}

// maVendorTaxCode is the value DocuEngine's search field expects: the codice fiscale, i.e.
// the tax code when present else the VAT (both normalized the same way as the finder).
// Distinct from fiscal_key (which prefers the IT-stripped VAT) by design.
func maVendorTaxCode(vat, tax string) string {
	if t := normalizeMAFiscalValue(tax); t != "" {
		return t
	}
	return normalizeMAFiscalValue(vat)
}

// maDocuErrorDefinitive classifies a DocuEngine error. A typed business error (200
// envelope, success=false) or a 4xx APIError (except 429) means the request was not
// created / cannot succeed ⇒ definitive. Everything else (network/timeout, 5xx, 429) is
// ambiguous ⇒ the outcome is indeterminate and must go to `unknown`, never be re-issued.
func maDocuErrorDefinitive(err error) bool {
	var docuErr *openapiit.DocuEngineError
	if errors.As(err, &docuErr) {
		return true
	}
	var apiErr *openapiit.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusTooManyRequests {
			return false
		}
		return apiErr.StatusCode >= 400 && apiErr.StatusCode < 500
	}
	return false
}

// ---------------------------------------------------------------------------
// Enqueue (service entry points; wired to HTTP in F8).
// ---------------------------------------------------------------------------

// startFilingSearch opens a durable DocuEngine search for a fiscal identity: it writes the
// intent row (F3) BEFORE any vendor call, then enqueues a filing_search job pre-leased to
// this instance. The mig-115 inflight index on payload->>'fiscalKey' deduplicates
// concurrent searches for the same identity; on a dedup the freshly-created intent row is
// a harmless orphan (no job references it, so no vendor call ever runs on it).
//
// taxCode is the DocuEngine search value (the codice fiscale — see maVendorTaxCode);
// callers derive it and fiscalKey from the raw identity (buildMAFiscalKey / maVendorTaxCode).
func (s *maService) startFilingSearch(ctx context.Context, fiscalKey, taxCode, subject, email string) (string, error) {
	if s.store == nil || s.filing == nil {
		return "", errMAStoreUnavailable
	}
	if s.openapiit == nil {
		return "", errMAOpenAPIITUnavailable
	}
	fiscalKey = strings.TrimSpace(fiscalKey)
	if fiscalKey == "" {
		return "", fmt.Errorf("%w: empty fiscal key", errMAStrategyInvalid)
	}
	taxCode = normalizeMAFiscalValue(taxCode)
	if taxCode == "" {
		return "", fmt.Errorf("%w: empty tax code", errMAStrategyInvalid)
	}
	searchID, err := s.filing.CreateMAFilingSearchIntent(ctx, fiscalKey, subject, email)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(maFilingSearchJobPayload{FiscalKey: fiscalKey, SearchID: searchID, TaxCode: taxCode})
	if err != nil {
		return "", fmt.Errorf("marshal ma filing search payload: %w", err)
	}
	_, created, err := s.store.EnqueueMAJob(ctx, maJobEnqueue{
		JobType: maJobTypeFilingSearch,
		Status:  maJobStatusPending,
		Subject: subject,
		Email:   email,
		Payload: payload,
		Owner:   s.owner,
	})
	if err != nil {
		return "", err
	}
	if !created {
		logging.FromContext(ctx).Info("binocolo filing search deduplicated",
			"component", "binocolo", "operation", "ma_filing_search",
			"fiscal_key", fiscalKey, "orphan_search_id", searchID)
	}
	return searchID, nil
}

// startFilingAcquisitions enqueues one durable filing_acquire per requested balance sheet
// of an already-completed search. Each acquisition intent (F3) is written BEFORE its job,
// carrying origin=docuengine + balance_sheet_id + a result_id hint. The mig-115 inflight
// index on (fiscalKey, balanceSheetId) deduplicates; a dedup leaves a harmless orphan
// intent.
//
// vat/tax are the raw fiscal identity; fiscal_key and the vendor taxCode are derived from
// them (matching startFilingSearch). Only balance sheets present in the search results are
// enqueued. Returns the created acquisition ids.
func (s *maService) startFilingAcquisitions(ctx context.Context, searchID string, balanceSheetIDs []string, vat, tax, contextCompanyKey, subject, email string) ([]string, error) {
	if s.store == nil || s.filing == nil {
		return nil, errMAStoreUnavailable
	}
	if s.openapiit == nil {
		return nil, errMAOpenAPIITUnavailable
	}
	searchID = strings.TrimSpace(searchID)
	if searchID == "" {
		return nil, fmt.Errorf("%w: search id", errMAStrategyInvalid)
	}
	fiscalKey := buildMAFiscalKey(vat, tax)
	if fiscalKey == "" {
		return nil, fmt.Errorf("%w: empty fiscal identity", errMAStrategyInvalid)
	}
	taxCode := maVendorTaxCode(vat, tax)
	search, err := s.filing.GetMAFilingSearch(ctx, searchID)
	if err != nil {
		return nil, err
	}
	if search == nil {
		return nil, fmt.Errorf("%w: search not found", errMAStrategyInvalid)
	}
	if search.Status != maFilingSearchResults && search.Status != maFilingSearchConsumed {
		return nil, fmt.Errorf("%w: search has no results", errMAStrategyInvalid)
	}
	if search.FiscalKey != fiscalKey {
		return nil, fmt.Errorf("%w: search identity mismatch", errMAStrategyInvalid)
	}
	resultsByBSID, err := maFilingResultsByBalanceSheet(search.Results)
	if err != nil {
		return nil, err
	}
	vatClean := normalizeMAFiscalValue(vat)
	taxClean := normalizeMAFiscalValue(tax)
	acquisitionIDs := make([]string, 0, len(balanceSheetIDs))
	for _, bsid := range balanceSheetIDs {
		bsid = strings.TrimSpace(bsid)
		if bsid == "" {
			continue
		}
		row, ok := resultsByBSID[bsid]
		if !ok {
			logging.FromContext(ctx).Warn("binocolo filing acquire skipped unknown balance sheet",
				"component", "binocolo", "operation", "ma_filing_acquire",
				"search_id", searchID, "balance_sheet_id", bsid)
			continue
		}
		acqID, err := s.filing.CreateMAFilingAcquisitionIntent(ctx, maFilingAcquisitionCreate{
			Origin:            maFilingOriginDocuEngine,
			BalanceSheetID:    bsid,
			ResultID:          row.ResultID,
			ContextCompanyKey: contextCompanyKey,
			ActorSubject:      subject,
			ActorEmail:        email,
		})
		if err != nil {
			return nil, err
		}
		payload, err := json.Marshal(maFilingAcquireJobPayload{
			FiscalKey:         fiscalKey,
			AcquisitionID:     acqID,
			SearchID:          searchID,
			BalanceSheetID:    bsid,
			ContextCompanyKey: contextCompanyKey,
			TaxCode:           taxCode,
			VAT:               vatClean,
			Tax:               taxClean,
		})
		if err != nil {
			return nil, fmt.Errorf("marshal ma filing acquire payload: %w", err)
		}
		_, created, err := s.store.EnqueueMAJob(ctx, maJobEnqueue{
			JobType: maJobTypeFilingAcquire,
			Status:  maJobStatusPending,
			Subject: subject,
			Email:   email,
			Payload: payload,
			Owner:   s.owner,
		})
		if err != nil {
			return nil, err
		}
		if !created {
			logging.FromContext(ctx).Info("binocolo filing acquire deduplicated",
				"component", "binocolo", "operation", "ma_filing_acquire",
				"fiscal_key", fiscalKey, "balance_sheet_id", bsid, "orphan_acquisition_id", acqID)
			continue
		}
		acquisitionIDs = append(acquisitionIDs, acqID)
	}
	return acquisitionIDs, nil
}

// enqueueFilingIngest enqueues the OCR/parse ingest for a downloaded filing. F5 owns the
// dispatch; the worker leaves the row 'pending' until then. Pre-leased and deduplicated on
// payload->>'filingId' (mig 115).
func (s *maService) enqueueFilingIngest(ctx context.Context, filingID, fiscalKey, contextCompanyKey, subject, email string) error {
	payload, err := json.Marshal(maFilingIngestJobPayload{FilingID: filingID, FiscalKey: fiscalKey, ContextCompanyKey: contextCompanyKey})
	if err != nil {
		return fmt.Errorf("marshal ma filing ingest payload: %w", err)
	}
	_, _, err = s.store.EnqueueMAJob(ctx, maJobEnqueue{
		JobType: maJobTypeFilingIngest,
		Status:  maJobStatusPending,
		Subject: subject,
		Email:   email,
		Payload: payload,
		Owner:   s.owner,
	})
	return err
}

// ---------------------------------------------------------------------------
// filing_search job.
// ---------------------------------------------------------------------------

// runFilingSearchJob owns the operation trace for one tick of a filing_search job (one
// trace per run — completed here). Returns the trace id (correlated to the job row) and
// the work error verbatim, so the worker sees errMAFilingPollPending.
func (s *maService) runFilingSearchJob(ctx context.Context, job maJob) (string, error) {
	trace, err := s.startTrace(ctx, maTraceStart{
		Operation:        "ma_filing_search",
		CreatedBySubject: job.CreatedBySubject,
		CreatedByEmail:   job.CreatedByEmail,
		Request:          job.Payload,
	})
	if err != nil {
		return "", err
	}
	ctx = withMATrace(ctx, trace)
	workErr := s.filingSearchWork(ctx, job)
	s.completeFilingTrace(ctx, workErr)
	return trace.id, workErr
}

func (s *maService) filingSearchWork(ctx context.Context, job maJob) error {
	var payload maFilingSearchJobPayload
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decode ma filing search payload: %w", err)
		}
	}
	searchID := strings.TrimSpace(payload.SearchID)
	if searchID == "" {
		return fmt.Errorf("%w: search id", errMAStrategyInvalid)
	}
	docu := s.docuEngine()
	if docu == nil {
		return errMAOpenAPIITUnavailable
	}
	search, err := s.filing.GetMAFilingSearch(ctx, searchID)
	if err != nil {
		return err
	}
	if search == nil {
		return fmt.Errorf("%w: search %s not found", errMAStrategyInvalid, searchID)
	}
	// Resume ALWAYS from the durable row status, never from an in-flight call.
	switch search.Status {
	case maFilingSearchIntent:
		return s.filingSearchIntent(ctx, docu, search, payload)
	case maFilingSearchRequested:
		return s.filingSearchRequested(ctx, docu, search, payload)
	case maFilingSearchUnknown:
		return s.filingSearchReconcile(ctx, docu, search, payload)
	case maFilingSearchResults, maFilingSearchConsumed, maFilingSearchFailed:
		return nil // terminal for the search job (results ready / consumed / failed recorded)
	default:
		return fmt.Errorf("%w: unexpected search status %q", errMAStrategyInvalid, search.Status)
	}
}

// filingSearchIntent opens the DocuEngine request for a search. BEFORE paying for a POST
// it reconciles against existing requests (free ListRequests, name + window + taxCode,
// excluding request ids already owned by other rows): a unique orphan (e.g. a prior
// attempt whose persist failed after a paid POST) is ADOPTED instead of paying again;
// multiple candidates ⇒ unknown; none ⇒ POST /requests (state NEW, taxCode in search.field0
// so readableSearch is populated for later reconciliation). A definitive POST error ⇒
// fail; an ambiguous one ⇒ unknown; a POST that succeeds but whose persist fails ⇒ unknown
// (never left at 'intent', which would re-POST) so the next tick adopts the created request.
func (s *maService) filingSearchIntent(ctx context.Context, docu maDocuEngine, search *maFilingSearch, payload maFilingSearchJobPayload) error {
	if strings.TrimSpace(s.filingDocumentID) == "" {
		if err := s.filing.FailMAFilingSearch(ctx, search.ID, "docuengine channel not configured"); err != nil {
			return err
		}
		return nil
	}
	// Pre-POST reconciliation: adopt an already-created request rather than pay for a new
	// one. Even a double persist-failure cannot produce a second paid request.
	exclude, _ := s.filing.ListMAFilingReferencedRequestIDs(ctx, search.ID, "")
	matches, err := s.reconcileFilingRequestMatches(ctx, docu, payload.TaxCode, search.UpdatedAt, exclude)
	if err != nil {
		return err // transient list/get failure → poll-pending, no POST yet
	}
	switch len(matches) {
	case 1:
		if perr := s.filing.SetMAFilingSearchRequested(ctx, search.ID, matches[0].ID); perr != nil {
			return s.filingSearchPersistFailedUnknown(ctx, search.ID, matches[0].ID, perr)
		}
		s.filingTrace(ctx, "docuengine_adopt_request", maTraceEventSucceeded, map[string]any{"search_id": search.ID, "request_id": matches[0].ID}, "")
		return s.filingSearchAdvance(ctx, docu, search.ID, payload.TaxCode, matches[0])
	case 0:
		// No existing request → pay for a fresh POST below.
	default:
		if uerr := s.filing.SetMAFilingSearchUnknown(ctx, search.ID, "multiple candidate requests on reconcile"); uerr != nil {
			return uerr
		}
		return errMAFilingPollPending
	}

	s.filingTrace(ctx, "docuengine_create_request", maTraceEventStarted, map[string]any{"search_id": search.ID, "fiscal_key": search.FiscalKey}, "")
	req, err := docu.CreateRequest(ctx, docuSearchCreateBody(s.filingDocumentID, payload.TaxCode))
	if err != nil {
		s.filingTrace(ctx, "docuengine_create_request", maTraceEventFailed, nil, err.Error())
		if maDocuErrorDefinitive(err) {
			if ferr := s.filing.FailMAFilingSearch(ctx, search.ID, err.Error()); ferr != nil {
				return ferr
			}
			return nil
		}
		if uerr := s.filing.SetMAFilingSearchUnknown(ctx, search.ID, err.Error()); uerr != nil {
			return uerr
		}
		return errMAFilingPollPending
	}
	requestID := strings.TrimSpace(req.ID)
	if requestID == "" {
		if uerr := s.filing.SetMAFilingSearchUnknown(ctx, search.ID, "create request returned no id"); uerr != nil {
			return uerr
		}
		return errMAFilingPollPending
	}
	if err := s.filing.SetMAFilingSearchRequested(ctx, search.ID, requestID); err != nil {
		return s.filingSearchPersistFailedUnknown(ctx, search.ID, requestID, err)
	}
	s.filingTrace(ctx, "docuengine_create_request", maTraceEventSucceeded, map[string]any{"request_id": requestID}, "")
	// The request is NEW; the requested path (next tick) observes the vendor state and
	// submits the search.
	return errMAFilingPollPending
}

// filingSearchPersistFailedUnknown handles a POST that succeeded but whose request-id
// persist failed: it moves the row to 'unknown' (best-effort, logging the persist error) so
// the next tick reconciles and adopts the already-created request — never re-POSTing.
func (s *maService) filingSearchPersistFailedUnknown(ctx context.Context, searchID, requestID string, persistErr error) error {
	logging.FromContext(ctx).Warn("binocolo filing search persist requested failed",
		"component", "binocolo", "operation", "ma_filing_search",
		"search_id", searchID, "request_id", requestID, "error", persistErr)
	if uerr := s.filing.SetMAFilingSearchUnknown(ctx, searchID, "persist requested failed"); uerr != nil {
		return uerr
	}
	return errMAFilingPollPending
}

// filingSearchRequested observes the vendor state and advances. GET failures are transient
// polls (never a re-mutation).
func (s *maService) filingSearchRequested(ctx context.Context, docu maDocuEngine, search *maFilingSearch, payload maFilingSearchJobPayload) error {
	requestID := strings.TrimSpace(search.DocuEngineRequestID)
	if requestID == "" {
		if err := s.filing.SetMAFilingSearchUnknown(ctx, search.ID, "requested without request id"); err != nil {
			return err
		}
		return errMAFilingPollPending
	}
	req, err := s.docuGetRequest(ctx, docu, requestID)
	if err != nil {
		return err
	}
	return s.filingSearchAdvance(ctx, docu, search.ID, payload.TaxCode, req)
}

// filingSearchReconcile recovers an `unknown` search. With a request id (ambiguous PATCH)
// it observes the vendor state and adopts back to `requested` — the SubmitSearch is
// re-issued ONLY if the GET shows the request is still NEW (never blind). Without an id
// (ambiguous POST) it matches a request via ListRequests + timestamp window +
// readableSearch.taxCode; a unique match is adopted, otherwise the row stays `unknown`.
func (s *maService) filingSearchReconcile(ctx context.Context, docu maDocuEngine, search *maFilingSearch, payload maFilingSearchJobPayload) error {
	requestID := strings.TrimSpace(search.DocuEngineRequestID)
	if requestID != "" {
		req, err := s.docuGetRequest(ctx, docu, requestID)
		if err != nil {
			return err
		}
		if err := s.filing.AdoptMAFilingSearchRequest(ctx, search.ID, requestID); err != nil {
			return err
		}
		return s.filingSearchAdvance(ctx, docu, search.ID, payload.TaxCode, req)
	}
	exclude, _ := s.filing.ListMAFilingReferencedRequestIDs(ctx, search.ID, "")
	matches, err := s.reconcileFilingRequestMatches(ctx, docu, payload.TaxCode, search.UpdatedAt, exclude)
	if err != nil {
		return err
	}
	if len(matches) != 1 {
		return errMAFilingPollPending // zero/multiple matches: stay unknown, keep polling
	}
	if err := s.filing.AdoptMAFilingSearchRequest(ctx, search.ID, matches[0].ID); err != nil {
		return err
	}
	return s.filingSearchAdvance(ctx, docu, search.ID, payload.TaxCode, matches[0])
}

// filingSearchAdvance branches on the observed vendor state (used from both `requested`
// and the id-based reconcile so there is one GET per run). It only issues the PATCH SEARCH
// when the request is still NEW.
func (s *maService) filingSearchAdvance(ctx context.Context, docu maDocuEngine, searchID, taxCode string, req openapiit.DocuRequest) error {
	switch {
	case req.State == openapiit.DocuStateCancelled:
		reason := strings.TrimSpace(req.CancellationReason)
		if reason == "" {
			reason = "cancelled"
		}
		if err := s.filing.FailMAFilingSearch(ctx, searchID, reason); err != nil {
			return err
		}
		return nil
	case len(req.Results) > 0 || req.State == openapiit.DocuStateDone:
		return s.filingSearchStoreResults(ctx, searchID, req.Results)
	case req.State == openapiit.DocuStateNew:
		s.filingTrace(ctx, "docuengine_submit_search", maTraceEventStarted, map[string]any{"request_id": req.ID}, "")
		submitted, err := docu.SubmitSearch(ctx, req.ID, taxCode)
		if err != nil {
			s.filingTrace(ctx, "docuengine_submit_search", maTraceEventFailed, nil, err.Error())
			if maDocuErrorDefinitive(err) {
				if ferr := s.filing.FailMAFilingSearch(ctx, searchID, err.Error()); ferr != nil {
					return ferr
				}
				return nil
			}
			// Ambiguous PATCH, request id persisted: unknown → reconcile via GetRequest,
			// which re-submits ONLY if the GET still shows NEW.
			if uerr := s.filing.SetMAFilingSearchUnknown(ctx, searchID, err.Error()); uerr != nil {
				return uerr
			}
			return errMAFilingPollPending
		}
		s.filingTrace(ctx, "docuengine_submit_search", maTraceEventSucceeded, map[string]any{"state": submitted.State}, "")
		if len(submitted.Results) > 0 || submitted.State == openapiit.DocuStateDone {
			return s.filingSearchStoreResults(ctx, searchID, submitted.Results)
		}
		return errMAFilingPollPending
	default:
		return errMAFilingPollPending // SEARCH/WAIT, no results yet: keep polling
	}
}

func (s *maService) filingSearchStoreResults(ctx context.Context, searchID string, results []openapiit.DocuResult) error {
	serialized, err := marshalMAFilingSearchResults(results)
	if err != nil {
		return err
	}
	if err := s.filing.SetMAFilingSearchResults(ctx, searchID, serialized); err != nil {
		return err
	}
	s.filingTrace(ctx, "ma_filing_search_results", maTraceEventSucceeded, map[string]any{"search_id": searchID, "result_count": len(results)}, "")
	return nil
}

// ---------------------------------------------------------------------------
// filing_acquire job.
// ---------------------------------------------------------------------------

func (s *maService) runFilingAcquireJob(ctx context.Context, job maJob) (string, error) {
	trace, err := s.startTrace(ctx, maTraceStart{
		Operation:        "ma_filing_acquire",
		CreatedBySubject: job.CreatedBySubject,
		CreatedByEmail:   job.CreatedByEmail,
		Request:          job.Payload,
	})
	if err != nil {
		return "", err
	}
	ctx = withMATrace(ctx, trace)
	workErr := s.filingAcquireWork(ctx, job)
	s.completeFilingTrace(ctx, workErr)
	return trace.id, workErr
}

func (s *maService) filingAcquireWork(ctx context.Context, job maJob) error {
	var payload maFilingAcquireJobPayload
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decode ma filing acquire payload: %w", err)
		}
	}
	acquisitionID := strings.TrimSpace(payload.AcquisitionID)
	if acquisitionID == "" {
		return fmt.Errorf("%w: acquisition id", errMAStrategyInvalid)
	}
	docu := s.docuEngine()
	if docu == nil {
		return errMAOpenAPIITUnavailable
	}
	acq, err := s.filing.GetMAFilingAcquisition(ctx, acquisitionID)
	if err != nil {
		return err
	}
	if acq == nil {
		return fmt.Errorf("%w: acquisition %s not found", errMAStrategyInvalid, acquisitionID)
	}
	switch acq.Status {
	case maFilingAcquisitionIntent:
		return s.filingAcquireIntent(ctx, docu, acq, payload)
	case maFilingAcquisitionRequested, maFilingAcquisitionDownloaded:
		// downloaded = crashed between download and CompleteMAFiling; re-observe the
		// request and re-run the (idempotent) download+ingest tail.
		return s.filingAcquireRequested(ctx, docu, acq, payload)
	case maFilingAcquisitionUnknown:
		return s.filingAcquireReconcile(ctx, docu, acq, payload)
	case maFilingAcquisitionDone, maFilingAcquisitionFailed:
		return nil
	default:
		return fmt.Errorf("%w: unexpected acquisition status %q", errMAStrategyInvalid, acq.Status)
	}
}

// filingAcquireIntent picks the request to use. The acquisition reuses the search's
// request ONLY when it already consumed it (a prior attempt) or wins the CAS now; a lost
// claim (or no reusable search) opens a fresh request. The consumed-by-me short-circuit is
// what stops a retry from re-claiming or opening a duplicate paid request.
func (s *maService) filingAcquireIntent(ctx context.Context, docu maDocuEngine, acq *maFilingAcquisition, payload maFilingAcquireJobPayload) error {
	searchID := strings.TrimSpace(payload.SearchID)
	var search *maFilingSearch
	if searchID != "" {
		var err error
		search, err = s.filing.GetMAFilingSearch(ctx, searchID)
		if err != nil {
			return err
		}
	}
	if search != nil {
		if search.ConsumedByAcquisitionID == acq.ID {
			// A prior attempt already won the claim for THIS acquisition: never re-claim.
			return s.filingAcquireUseSearchRequest(ctx, docu, acq, search, payload.TaxCode)
		}
		if search.Status == maFilingSearchResults {
			won, err := s.filing.ClaimMAFilingSearchConsumed(ctx, search.ID, acq.ID)
			if err != nil {
				return err
			}
			if won {
				return s.filingAcquireUseSearchRequest(ctx, docu, acq, search, payload.TaxCode)
			}
		}
	}
	// Lost the claim / search consumed by another acquisition / no reusable search: open a
	// fresh DocuEngine request of our own.
	return s.filingAcquireCreateRequest(ctx, docu, acq, payload.TaxCode)
}

func (s *maService) filingAcquireUseSearchRequest(ctx context.Context, docu maDocuEngine, acq *maFilingAcquisition, search *maFilingSearch, taxCode string) error {
	requestID := strings.TrimSpace(search.DocuEngineRequestID)
	if requestID == "" {
		// A consumed 'results' search without a request id should not happen; open our own
		// request rather than reference a missing one.
		return s.filingAcquireCreateRequest(ctx, docu, acq, taxCode)
	}
	if err := s.filing.SetMAFilingAcquisitionRequested(ctx, acq.ID, requestID); err != nil {
		return err
	}
	s.filingTrace(ctx, "ma_filing_acquire_reuse_search", maTraceEventSucceeded, map[string]any{"acquisition_id": acq.ID, "request_id": requestID}, "")
	// The requested path selects our balanceSheetId from this request's live results.
	return errMAFilingPollPending
}

// filingAcquireCreateRequest opens the acquisition's OWN DocuEngine request (the lost-claim
// / no-reusable-search path). Like the search intent it reconciles BEFORE paying: a unique
// orphan matching the taxCode and not owned by another row is adopted; multiple ⇒ unknown;
// none ⇒ POST. A persist failure after a successful POST goes to 'unknown' (never left at
// 'intent', which would re-POST). Adoptable orphans are always in NEW/SEARCH state — a
// result is only ever selected after the request id is persisted (referenced) — so
// adopting one and selecting our own balanceSheetId is safe.
func (s *maService) filingAcquireCreateRequest(ctx context.Context, docu maDocuEngine, acq *maFilingAcquisition, taxCode string) error {
	if strings.TrimSpace(s.filingDocumentID) == "" {
		if err := s.filing.FailMAFilingAcquisition(ctx, acq.ID, "docuengine channel not configured"); err != nil {
			return err
		}
		return nil
	}
	exclude, _ := s.filing.ListMAFilingReferencedRequestIDs(ctx, "", acq.ID)
	matches, err := s.reconcileFilingRequestMatches(ctx, docu, taxCode, acq.UpdatedAt, exclude)
	if err != nil {
		return err // transient list/get failure → poll-pending, no POST yet
	}
	switch len(matches) {
	case 1:
		if perr := s.filing.SetMAFilingAcquisitionRequested(ctx, acq.ID, matches[0].ID); perr != nil {
			return s.filingAcquirePersistFailedUnknown(ctx, acq.ID, matches[0].ID, perr)
		}
		s.filingTrace(ctx, "docuengine_adopt_request", maTraceEventSucceeded, map[string]any{"acquisition_id": acq.ID, "request_id": matches[0].ID}, "")
		return errMAFilingPollPending // requested path advances from the adopted request
	case 0:
		// No existing request → pay for a fresh POST below.
	default:
		if uerr := s.filing.SetMAFilingAcquisitionUnknown(ctx, acq.ID, "multiple candidate requests on reconcile"); uerr != nil {
			return uerr
		}
		return errMAFilingPollPending
	}

	s.filingTrace(ctx, "docuengine_create_request", maTraceEventStarted, map[string]any{"acquisition_id": acq.ID}, "")
	req, err := docu.CreateRequest(ctx, docuSearchCreateBody(s.filingDocumentID, taxCode))
	if err != nil {
		s.filingTrace(ctx, "docuengine_create_request", maTraceEventFailed, nil, err.Error())
		if maDocuErrorDefinitive(err) {
			if ferr := s.filing.FailMAFilingAcquisition(ctx, acq.ID, err.Error()); ferr != nil {
				return ferr
			}
			return nil
		}
		if uerr := s.filing.SetMAFilingAcquisitionUnknown(ctx, acq.ID, err.Error()); uerr != nil {
			return uerr
		}
		return errMAFilingPollPending
	}
	requestID := strings.TrimSpace(req.ID)
	if requestID == "" {
		if uerr := s.filing.SetMAFilingAcquisitionUnknown(ctx, acq.ID, "create request returned no id"); uerr != nil {
			return uerr
		}
		return errMAFilingPollPending
	}
	if err := s.filing.SetMAFilingAcquisitionRequested(ctx, acq.ID, requestID); err != nil {
		return s.filingAcquirePersistFailedUnknown(ctx, acq.ID, requestID, err)
	}
	s.filingTrace(ctx, "docuengine_create_request", maTraceEventSucceeded, map[string]any{"request_id": requestID}, "")
	return errMAFilingPollPending
}

// filingAcquirePersistFailedUnknown mirrors filingSearchPersistFailedUnknown for the
// acquisition: a POST that succeeded but whose request-id persist failed goes to 'unknown'
// so the next tick adopts the already-created request instead of re-POSTing.
func (s *maService) filingAcquirePersistFailedUnknown(ctx context.Context, acquisitionID, requestID string, persistErr error) error {
	logging.FromContext(ctx).Warn("binocolo filing acquire persist requested failed",
		"component", "binocolo", "operation", "ma_filing_acquire",
		"acquisition_id", acquisitionID, "request_id", requestID, "error", persistErr)
	if uerr := s.filing.SetMAFilingAcquisitionUnknown(ctx, acquisitionID, "persist requested failed"); uerr != nil {
		return uerr
	}
	return errMAFilingPollPending
}

func (s *maService) filingAcquireRequested(ctx context.Context, docu maDocuEngine, acq *maFilingAcquisition, payload maFilingAcquireJobPayload) error {
	requestID := strings.TrimSpace(acq.DocuEngineRequestID)
	if requestID == "" {
		if err := s.filing.SetMAFilingAcquisitionUnknown(ctx, acq.ID, "requested without request id"); err != nil {
			return err
		}
		return errMAFilingPollPending
	}
	req, err := s.docuGetRequest(ctx, docu, requestID)
	if err != nil {
		return err
	}
	return s.filingAcquireAdvance(ctx, docu, acq, req, payload)
}

// filingAcquireReconcile recovers an `unknown` acquisition. With a request id it observes
// the vendor state and adopts back to `requested` (never re-issuing a PATCH the GET shows
// already landed). Without an id (ambiguous own POST) it matches via ListRequests as the
// search does.
func (s *maService) filingAcquireReconcile(ctx context.Context, docu maDocuEngine, acq *maFilingAcquisition, payload maFilingAcquireJobPayload) error {
	requestID := strings.TrimSpace(acq.DocuEngineRequestID)
	if requestID != "" {
		req, err := s.docuGetRequest(ctx, docu, requestID)
		if err != nil {
			return err
		}
		if err := s.filing.AdoptMAFilingAcquisitionRequest(ctx, acq.ID, requestID); err != nil {
			return err
		}
		return s.filingAcquireAdvance(ctx, docu, acq, req, payload)
	}
	exclude, _ := s.filing.ListMAFilingReferencedRequestIDs(ctx, "", acq.ID)
	matches, err := s.reconcileFilingRequestMatches(ctx, docu, payload.TaxCode, acq.UpdatedAt, exclude)
	if err != nil {
		return err
	}
	if len(matches) != 1 {
		return errMAFilingPollPending
	}
	if err := s.filing.AdoptMAFilingAcquisitionRequest(ctx, acq.ID, matches[0].ID); err != nil {
		return err
	}
	return s.filingAcquireAdvance(ctx, docu, acq, matches[0], payload)
}

// filingAcquireAdvance branches on the observed vendor request (shared by the requested
// path and the id-based reconcile). It always remaps the target balanceSheetId to a result
// id IN THIS request (result ids are per-request), and only SelectResult/SubmitSearch when
// the observed state requires it — so a retry after a landed PATCH never re-issues it.
func (s *maService) filingAcquireAdvance(ctx context.Context, docu maDocuEngine, acq *maFilingAcquisition, req openapiit.DocuRequest, payload maFilingAcquireJobPayload) error {
	if req.State == openapiit.DocuStateCancelled {
		reason := strings.TrimSpace(req.CancellationReason)
		if reason == "" {
			reason = "cancelled"
		}
		if err := s.filing.FailMAFilingAcquisition(ctx, acq.ID, reason); err != nil {
			return err
		}
		return nil
	}
	if req.ResultID != nil && strings.TrimSpace(*req.ResultID) != "" {
		// A result is already selected on this request.
		if req.State == openapiit.DocuStateDone {
			return s.filingAcquireDownload(ctx, docu, acq, req, payload)
		}
		return errMAFilingPollPending // selected, document still generating
	}
	if len(req.Results) > 0 {
		resultID := docuResultIDForBalanceSheet(req.Results, payload.BalanceSheetID)
		if resultID == "" {
			if err := s.filing.FailMAFilingAcquisition(ctx, acq.ID, "balance sheet not in request results"); err != nil {
				return err
			}
			return nil
		}
		return s.filingAcquireSelect(ctx, docu, acq, req.ID, resultID)
	}
	if req.State == openapiit.DocuStateNew {
		return s.filingAcquireSubmitSearch(ctx, docu, acq, req.ID, payload.TaxCode)
	}
	return errMAFilingPollPending // SEARCH/WAIT, no results yet
}

func (s *maService) filingAcquireSubmitSearch(ctx context.Context, docu maDocuEngine, acq *maFilingAcquisition, requestID, taxCode string) error {
	s.filingTrace(ctx, "docuengine_submit_search", maTraceEventStarted, map[string]any{"request_id": requestID}, "")
	if _, err := docu.SubmitSearch(ctx, requestID, taxCode); err != nil {
		s.filingTrace(ctx, "docuengine_submit_search", maTraceEventFailed, nil, err.Error())
		if maDocuErrorDefinitive(err) {
			if ferr := s.filing.FailMAFilingAcquisition(ctx, acq.ID, err.Error()); ferr != nil {
				return ferr
			}
			return nil
		}
		if uerr := s.filing.SetMAFilingAcquisitionUnknown(ctx, acq.ID, err.Error()); uerr != nil {
			return uerr
		}
		return errMAFilingPollPending
	}
	s.filingTrace(ctx, "docuengine_submit_search", maTraceEventSucceeded, nil, "")
	// Results appear on a later poll; the requested path selects them then.
	return errMAFilingPollPending
}

func (s *maService) filingAcquireSelect(ctx context.Context, docu maDocuEngine, acq *maFilingAcquisition, requestID, resultID string) error {
	s.filingTrace(ctx, "docuengine_select_result", maTraceEventStarted, map[string]any{"request_id": requestID}, "")
	if _, err := docu.SelectResult(ctx, requestID, resultID); err != nil {
		s.filingTrace(ctx, "docuengine_select_result", maTraceEventFailed, nil, err.Error())
		if maDocuErrorDefinitive(err) {
			if ferr := s.filing.FailMAFilingAcquisition(ctx, acq.ID, err.Error()); ferr != nil {
				return ferr
			}
			return nil
		}
		// Ambiguous PATCH: unknown → reconcile via GetRequest, which re-selects ONLY if the
		// GET shows no result id registered yet.
		if uerr := s.filing.SetMAFilingAcquisitionUnknown(ctx, acq.ID, err.Error()); uerr != nil {
			return uerr
		}
		return errMAFilingPollPending
	}
	s.filingTrace(ctx, "docuengine_select_result", maTraceEventSucceeded, nil, "")
	// Selected; the next poll observes DONE and downloads.
	return errMAFilingPollPending
}

// filingAcquireDownload fetches the signed PDF (download is unpaid), verifies its md5,
// stores the blob, creates/merges the filing, marks the acquisition done, and enqueues the
// ingest. A transient download failure polls again; a persistent md5 mismatch fails the
// acquisition (never re-charges — the request was already paid).
func (s *maService) filingAcquireDownload(ctx context.Context, docu maDocuEngine, acq *maFilingAcquisition, req openapiit.DocuRequest, payload maFilingAcquireJobPayload) error {
	s.filingTrace(ctx, "docuengine_list_documents", maTraceEventStarted, map[string]any{"request_id": req.ID}, "")
	docs, err := docu.ListRequestDocuments(ctx, req.ID)
	if err != nil {
		s.filingTrace(ctx, "docuengine_list_documents", maTraceEventFailed, nil, err.Error())
		return fmt.Errorf("docuengine list documents %s failed: %v: %w", req.ID, err, errMAFilingPollPending)
	}
	download, ok := pickMAFilingDownload(docs)
	if !ok {
		return errMAFilingPollPending // DONE but the document is not materialized yet
	}
	pdf, md5Hex, err := s.downloadAndVerifyMAFiling(ctx, docu, download)
	if err != nil {
		if errors.Is(err, errMAFilingMD5Mismatch) {
			if ferr := s.filing.FailMAFilingAcquisition(ctx, acq.ID, "download md5 mismatch"); ferr != nil {
				return ferr
			}
			return nil
		}
		return err // transient download error (wrapped as poll-pending)
	}
	// requested → downloaded (no-op if a prior attempt already reached downloaded).
	if err := s.filing.SetMAFilingAcquisitionDownloaded(ctx, acq.ID); err != nil {
		return err
	}
	sha := sha256.Sum256(pdf)
	if err := s.filing.InsertMAFilingBlob(ctx, md5Hex, hex.EncodeToString(sha[:]), pdf); err != nil {
		return err
	}
	closing, bsType := docuResultDeposit(req.Results, payload.BalanceSheetID)
	filingID, _, err := s.filing.CreateOrMergeMAFiling(ctx, maFilingCreate{
		VAT:              payload.VAT,
		Tax:              payload.Tax,
		ClosingDate:      closing,
		BalanceSheetID:   payload.BalanceSheetID,
		BalanceSheetType: bsType,
		BlobMD5:          md5Hex,
		CreatedBySubject: acq.ActorSubject,
		CreatedByEmail:   acq.ActorEmail,
	})
	if err != nil {
		return err
	}
	if err := s.filing.CompleteMAFilingAcquisition(ctx, acq.ID, filingID); err != nil {
		return err
	}
	s.filingTrace(ctx, "ma_filing_acquire_completed", maTraceEventSucceeded, map[string]any{"acquisition_id": acq.ID, "filing_id": filingID, "blob_md5": md5Hex}, "")
	// Enqueue OCR/parse ingest (F5 owns dispatch). Best-effort: the acquisition is already
	// done and the filing exists — a failed enqueue must not fail the (paid) acquire.
	if err := s.enqueueFilingIngest(ctx, filingID, payload.FiscalKey, payload.ContextCompanyKey, acq.ActorSubject, acq.ActorEmail); err != nil {
		logging.FromContext(ctx).Warn("binocolo filing ingest enqueue failed",
			"component", "binocolo", "operation", "ma_filing_acquire",
			"acquisition_id", acq.ID, "filing_id", filingID, "error", err)
	}
	return nil
}

// downloadAndVerifyMAFiling downloads the PDF and checks its md5 against the vendor digest.
// A transient download error polls again; a persistent mismatch (after one re-download)
// returns errMAFilingMD5Mismatch. Returns the bytes and canonical (hex) md5 on success.
func (s *maService) downloadAndVerifyMAFiling(ctx context.Context, docu maDocuEngine, download openapiit.DocuDownload) ([]byte, string, error) {
	want, err := openapiit.CanonicalMD5FromBase64(download.MD5)
	if err != nil {
		// Can't establish the expected digest: don't trust the bytes, but the download is
		// unpaid — treat as a transient poll (a later listing may carry a valid md5).
		return nil, "", fmt.Errorf("canonical md5 failed: %v: %w", err, errMAFilingPollPending)
	}
	for attempt := 0; attempt < 2; attempt++ {
		pdf, derr := docu.DownloadFile(ctx, download.DownloadURL)
		if derr != nil {
			return nil, "", fmt.Errorf("download file failed: %v: %w", derr, errMAFilingPollPending)
		}
		got := fmt.Sprintf("%x", md5.Sum(pdf))
		if got == want {
			return pdf, got, nil
		}
		logging.FromContext(ctx).Warn("binocolo filing download md5 mismatch",
			"component", "binocolo", "operation", "ma_filing_acquire",
			"want", want, "got", got, "attempt", attempt+1)
	}
	return nil, "", errMAFilingMD5Mismatch
}

// ---------------------------------------------------------------------------
// Shared helpers.
// ---------------------------------------------------------------------------

// docuGetRequest performs GET /requests/{id} with tracing, wrapping any failure as a
// poll-pending — a GET is idempotent, so a transient failure is a wait, not a re-mutation.
func (s *maService) docuGetRequest(ctx context.Context, docu maDocuEngine, requestID string) (openapiit.DocuRequest, error) {
	s.filingTrace(ctx, "docuengine_get_request", maTraceEventStarted, map[string]any{"request_id": requestID}, "")
	req, err := docu.GetRequest(ctx, requestID)
	if err != nil {
		s.filingTrace(ctx, "docuengine_get_request", maTraceEventFailed, nil, err.Error())
		return openapiit.DocuRequest{}, fmt.Errorf("docuengine get request %s failed: %v: %w", requestID, err, errMAFilingPollPending)
	}
	s.filingTrace(ctx, "docuengine_get_request", maTraceEventSucceeded, map[string]any{"state": req.State}, "")
	return req, nil
}

// reconcileFilingRequestMatches shortlists the DocuEngine requests that could be an intent's
// already-created request. The list carries no server-side filter, so it filters by
// name=="Bilancio Ottico" + a ±maFilingReconcileWindow window around the anchor + a
// per-request GET on readableSearch.taxCode, and drops any request id in the exclude set
// (already owned by another row). Callers decide: exactly one ⇒ adopt; zero ⇒ POST (intent)
// or stay unknown (reconcile); more than one ⇒ unknown (never picked heuristically).
func (s *maService) reconcileFilingRequestMatches(ctx context.Context, docu maDocuEngine, taxCode string, anchor time.Time, exclude map[string]bool) ([]openapiit.DocuRequest, error) {
	s.filingTrace(ctx, "docuengine_list_requests", maTraceEventStarted, nil, "")
	summaries, err := docu.ListRequests(ctx)
	if err != nil {
		s.filingTrace(ctx, "docuengine_list_requests", maTraceEventFailed, nil, err.Error())
		return nil, fmt.Errorf("docuengine list requests failed: %v: %w", err, errMAFilingPollPending)
	}
	wantTax := normalizeMAFiscalValue(taxCode)
	lo := anchor.Add(-maFilingReconcileWindow).Unix()
	hi := anchor.Add(maFilingReconcileWindow).Unix()
	var matches []openapiit.DocuRequest
	for _, summary := range summaries {
		if exclude[summary.ID] {
			continue // a request already owned by another search/acquisition row
		}
		if !strings.EqualFold(strings.TrimSpace(summary.Name), docuBilancioOtticoName) {
			continue
		}
		if ts := summary.Timestamps.Max(); ts != 0 && (ts < lo || ts > hi) {
			continue // outside the intent window
		}
		req, err := docu.GetRequest(ctx, summary.ID)
		if err != nil {
			continue // a transient per-candidate GET must not abort the whole sweep
		}
		if normalizeMAFiscalValue(docuReadableTaxCode(req)) == wantTax {
			matches = append(matches, req)
		}
	}
	s.filingTrace(ctx, "docuengine_reconcile", maTraceEventInfo, map[string]any{"candidate_matches": len(matches)}, "")
	return matches, nil
}

// completeFilingTrace closes the per-run trace: a nil error or a poll-pending is a
// successful tick (the poll itself succeeded); anything else is a failure.
func (s *maService) completeFilingTrace(ctx context.Context, workErr error) {
	if workErr == nil || errors.Is(workErr, errMAFilingPollPending) {
		_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusSucceeded, HTTPStatus: http.StatusOK})
		return
	}
	_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusFailed, ErrorMessage: workErr.Error()})
}

// markFilingJobUnknownOnPollTimeout is the worker's cleanup when the WALL-CLOCK poll window
// elapses: it moves the business row to `unknown` (never `failed`) — the paid vendor request
// may still complete, and a later reconciliation can adopt it. Idempotent: the store setters
// only transition intent/requested rows, so an already-unknown row is untouched.
func (s *maService) markFilingJobUnknownOnPollTimeout(ctx context.Context, job maJob) {
	if s.filing == nil {
		return
	}
	switch job.JobType {
	case maJobTypeFilingSearch:
		var payload maFilingSearchJobPayload
		if len(job.Payload) > 0 {
			_ = json.Unmarshal(job.Payload, &payload)
		}
		if strings.TrimSpace(payload.SearchID) != "" {
			_ = s.filing.SetMAFilingSearchUnknown(ctx, payload.SearchID, "poll timeout")
		}
	case maJobTypeFilingAcquire:
		var payload maFilingAcquireJobPayload
		if len(job.Payload) > 0 {
			_ = json.Unmarshal(job.Payload, &payload)
		}
		if strings.TrimSpace(payload.AcquisitionID) != "" {
			_ = s.filing.SetMAFilingAcquisitionUnknown(ctx, payload.AcquisitionID, "poll timeout")
		}
	}
}

// failFilingJobIfNotUnknown is the worker's terminal cleanup for an INFRASTRUCTURE-error
// give-up (attempts exhausted): fail the business row UNLESS it is `unknown`, which is never
// resolved heuristically (it awaits a future manual/dev reconciliation).
func (s *maService) failFilingJobIfNotUnknown(ctx context.Context, job maJob, code string) {
	if s.filing == nil {
		return
	}
	switch job.JobType {
	case maJobTypeFilingSearch:
		var payload maFilingSearchJobPayload
		if len(job.Payload) > 0 {
			_ = json.Unmarshal(job.Payload, &payload)
		}
		if strings.TrimSpace(payload.SearchID) == "" {
			return
		}
		search, err := s.filing.GetMAFilingSearch(ctx, payload.SearchID)
		if err != nil || search == nil || search.Status == maFilingSearchUnknown {
			return
		}
		_ = s.filing.FailMAFilingSearch(ctx, payload.SearchID, code)
	case maJobTypeFilingAcquire:
		var payload maFilingAcquireJobPayload
		if len(job.Payload) > 0 {
			_ = json.Unmarshal(job.Payload, &payload)
		}
		if strings.TrimSpace(payload.AcquisitionID) == "" {
			return
		}
		acq, err := s.filing.GetMAFilingAcquisition(ctx, payload.AcquisitionID)
		if err != nil || acq == nil || acq.Status == maFilingAcquisitionUnknown {
			return
		}
		_ = s.filing.FailMAFilingAcquisition(ctx, payload.AcquisitionID, code)
	}
}

// filingTrace records one ordered trace event for a vendor call (best-effort — a trace
// failure never blocks the paid path).
func (s *maService) filingTrace(ctx context.Context, eventType, status string, metadata map[string]any, errMsg string) {
	event := maTraceEventWrite{
		EventType:      eventType,
		ExternalSystem: "docuengine",
		Status:         status,
		Error:          errMsg,
	}
	if len(metadata) > 0 {
		event.Metadata = maTraceJSON(metadata)
	}
	_ = s.traceEvent(ctx, event)
}

// docuSearchCreateBody is the POST /requests body for the "Bilancio Ottico": documentId +
// state NEW (only NEW is allowed in POST) + the taxCode recorded in search.field0. The
// taxCode at POST populates the vendor's readableSearch even before the PATCH SEARCH
// launches the search, which is what lets an ambiguous-POST reconciliation (no request id)
// match the request by readableSearch.taxCode.
func docuSearchCreateBody(documentID, taxCode string) openapiit.DocuRequestCreate {
	return openapiit.DocuRequestCreate{
		DocumentID: documentID,
		State:      openapiit.DocuStateNew,
		Search:     map[string]string{"field0": taxCode},
	}
}

// marshalMAFilingSearchResults serializes the vendor results for ma_filing_search.results:
// opaque result_id + defensively-decoded balance-sheet facts + raw data.
func marshalMAFilingSearchResults(results []openapiit.DocuResult) ([]byte, error) {
	rows := make([]maFilingSearchResultRow, 0, len(results))
	for _, r := range results {
		row := maFilingSearchResultRow{ResultID: r.ID, Raw: r.Data}
		if bs, ok := r.BalanceSheet(); ok {
			row.BalanceSheetID = bs.BalanceSheetID
			row.BalanceSheetType = bs.BalanceSheetTypeCode
			row.BalanceSheetDate = bs.BalanceSheetDate
		}
		rows = append(rows, row)
	}
	out, err := json.Marshal(rows)
	if err != nil {
		return nil, fmt.Errorf("marshal ma filing search results: %w", err)
	}
	return out, nil
}

// maFilingResultsByBalanceSheet indexes stored search results by balanceSheetId (used at
// enqueue to validate requested balance sheets and capture the result_id hint).
func maFilingResultsByBalanceSheet(raw json.RawMessage) (map[string]maFilingSearchResultRow, error) {
	out := map[string]maFilingSearchResultRow{}
	if len(raw) == 0 {
		return out, nil
	}
	var rows []maFilingSearchResultRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode ma filing search results: %w", err)
	}
	for _, row := range rows {
		if row.BalanceSheetID != "" {
			out[row.BalanceSheetID] = row
		}
	}
	return out, nil
}

// docuResultIDForBalanceSheet remaps a balanceSheetId to the opaque result id IN a specific
// request's live results. Result ids are per-request handles — never reused across
// requests — so this is always done against the request being acted on.
func docuResultIDForBalanceSheet(results []openapiit.DocuResult, balanceSheetID string) string {
	balanceSheetID = strings.TrimSpace(balanceSheetID)
	for _, r := range results {
		if bs, ok := r.BalanceSheet(); ok && bs.BalanceSheetID == balanceSheetID {
			return r.ID
		}
	}
	return ""
}

// docuResultDeposit reads the deposit's closing date + balance-sheet type for a
// balanceSheetId from a request's results (for CreateOrMergeMAFiling). Missing/unparseable
// values are returned as (nil, "").
func docuResultDeposit(results []openapiit.DocuResult, balanceSheetID string) (*time.Time, string) {
	balanceSheetID = strings.TrimSpace(balanceSheetID)
	for _, r := range results {
		bs, ok := r.BalanceSheet()
		if !ok || bs.BalanceSheetID != balanceSheetID {
			continue
		}
		var closing *time.Time
		if t, ok := parseMAFilingDate(bs.BalanceSheetDate); ok {
			closing = &t
		}
		bsType := bs.BalanceSheetTypeCode
		if bsType == "" {
			bsType = bs.BalanceSheetTypeDescription
		}
		return closing, bsType
	}
	return nil, ""
}

// docuReadableTaxCode reads the tax code echoed back in a request's readableSearch (the
// only per-request identity available for the unknown-without-id reconciliation match).
func docuReadableTaxCode(req openapiit.DocuRequest) string {
	for _, key := range []string{"taxCode", "field0"} {
		if v := strings.TrimSpace(req.ReadableSearch[key]); v != "" {
			return v
		}
	}
	return ""
}

// pickMAFilingDownload selects the first downloadable document (the "Bilancio Ottico" has
// one PDF per request).
func pickMAFilingDownload(docs []openapiit.DocuDownload) (openapiit.DocuDownload, bool) {
	for _, d := range docs {
		if strings.TrimSpace(d.DownloadURL) != "" {
			return d, true
		}
	}
	return openapiit.DocuDownload{}, false
}

// parseMAFilingDate parses a DocuEngine balanceSheetDate (ISO8601Z or bare date).
func parseMAFilingDate(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
