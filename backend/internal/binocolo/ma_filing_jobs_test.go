package binocolo

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/openapiit"
)

// fakeFilingPDF is the payload the fake DocuEngine "downloads"; its md5 is echoed by
// ListRequestDocuments so the download verification passes.
var fakeFilingPDF = []byte("%PDF-1.4 fake bilancio ottico")

// The single approved test for Fase 4 (issue #78): an ambiguous outcome AFTER a paid
// POST/PATCH NEVER produces a second call for the same intent — the job goes to `unknown`
// and the retry reconciles from the persisted state (GetRequest / ListRequests), never a
// blind re-issue. Assertions count vendor calls per method, not just states.

// ---------------------------------------------------------------------------
// Fakes: in-memory filing store (mirrors the SQL status gating) + call-counting
// DocuEngine seam. No real DB, no real network — the whole point.
// ---------------------------------------------------------------------------

type fakeFilingStore struct {
	searches        map[string]*maFilingSearch
	acquisitions    map[string]*maFilingAcquisition
	blobs           map[string]bool
	filings         int
	seq             int
	clock           time.Time
	insertBlobCalls int
	failBlobFirst   bool // when true, the first InsertMAFilingBlob returns an error (finalize-retry test)
}

func newFakeFilingStore() *fakeFilingStore {
	return &fakeFilingStore{
		searches:     map[string]*maFilingSearch{},
		acquisitions: map[string]*maFilingAcquisition{},
		blobs:        map[string]bool{},
		clock:        time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC),
	}
}

func (f *fakeFilingStore) nextID(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%d", prefix, f.seq)
}

func (f *fakeFilingStore) CreateMAFilingSearchIntent(_ context.Context, fiscalKey, subject, email string) (string, error) {
	id := f.nextID("search")
	f.searches[id] = &maFilingSearch{ID: id, FiscalKey: fiscalKey, Status: maFilingSearchIntent, SearchedBySubject: subject, SearchedByEmail: email, CreatedAt: f.clock, UpdatedAt: f.clock}
	return id, nil
}

func (f *fakeFilingStore) GetMAFilingSearch(_ context.Context, id string) (*maFilingSearch, error) {
	s, ok := f.searches[id]
	if !ok {
		return nil, nil
	}
	clone := *s
	return &clone, nil
}

func (f *fakeFilingStore) SetMAFilingSearchRequested(_ context.Context, id, requestID string) error {
	if s := f.searches[id]; s != nil && s.Status == maFilingSearchIntent {
		s.Status = maFilingSearchRequested
		s.DocuEngineRequestID = requestID
		s.UpdatedAt = f.clock
	}
	return nil
}

func (f *fakeFilingStore) SetMAFilingSearchResults(_ context.Context, id string, results []byte) error {
	if s := f.searches[id]; s != nil && s.Status == maFilingSearchRequested {
		s.Status = maFilingSearchResults
		s.Results = append(json.RawMessage(nil), results...)
		s.UpdatedAt = f.clock
	}
	return nil
}

func (f *fakeFilingStore) SetMAFilingSearchUnknown(_ context.Context, id, _ string) error {
	if s := f.searches[id]; s != nil && (s.Status == maFilingSearchIntent || s.Status == maFilingSearchRequested) {
		s.Status = maFilingSearchUnknown
		s.UpdatedAt = f.clock
	}
	return nil
}

func (f *fakeFilingStore) FailMAFilingSearch(_ context.Context, id, _ string) error {
	if s := f.searches[id]; s != nil && s.Status != maFilingSearchConsumed {
		s.Status = maFilingSearchFailed
		s.UpdatedAt = f.clock
	}
	return nil
}

func (f *fakeFilingStore) AdoptMAFilingSearchRequest(_ context.Context, id, requestID string) error {
	if s := f.searches[id]; s != nil && s.Status == maFilingSearchUnknown {
		s.Status = maFilingSearchRequested
		s.DocuEngineRequestID = requestID
		s.UpdatedAt = f.clock
	}
	return nil
}

func (f *fakeFilingStore) ClaimMAFilingSearchConsumed(_ context.Context, searchID, acquisitionID string) (bool, error) {
	s := f.searches[searchID]
	if s != nil && s.Status == maFilingSearchResults && s.ConsumedByAcquisitionID == "" {
		s.Status = maFilingSearchConsumed
		s.ConsumedByAcquisitionID = acquisitionID
		s.UpdatedAt = f.clock
		return true, nil
	}
	return false, nil
}

func (f *fakeFilingStore) CreateMAFilingAcquisitionIntent(_ context.Context, in maFilingAcquisitionCreate) (string, error) {
	id := f.nextID("acq")
	f.acquisitions[id] = &maFilingAcquisition{ID: id, Origin: in.Origin, Status: maFilingAcquisitionIntent, ResultID: in.ResultID, BalanceSheetID: in.BalanceSheetID, ContextCompanyKey: in.ContextCompanyKey, ActorSubject: in.ActorSubject, ActorEmail: in.ActorEmail, CreatedAt: f.clock, UpdatedAt: f.clock}
	return id, nil
}

func (f *fakeFilingStore) GetMAFilingAcquisition(_ context.Context, id string) (*maFilingAcquisition, error) {
	a, ok := f.acquisitions[id]
	if !ok {
		return nil, nil
	}
	clone := *a
	return &clone, nil
}

func (f *fakeFilingStore) SetMAFilingAcquisitionRequested(_ context.Context, id, requestID string) error {
	if a := f.acquisitions[id]; a != nil && a.Status == maFilingAcquisitionIntent {
		a.Status = maFilingAcquisitionRequested
		a.DocuEngineRequestID = requestID
		a.UpdatedAt = f.clock
	}
	return nil
}

func (f *fakeFilingStore) SetMAFilingAcquisitionDownloaded(_ context.Context, id string) error {
	if a := f.acquisitions[id]; a != nil && a.Status == maFilingAcquisitionRequested {
		a.Status = maFilingAcquisitionDownloaded
		a.UpdatedAt = f.clock
	}
	return nil
}

func (f *fakeFilingStore) CompleteMAFilingAcquisition(_ context.Context, id, filingID string) error {
	if a := f.acquisitions[id]; a != nil && a.Status != maFilingAcquisitionDone && a.Status != maFilingAcquisitionFailed {
		a.Status = maFilingAcquisitionDone
		a.FilingID = filingID
		a.UpdatedAt = f.clock
	}
	return nil
}

func (f *fakeFilingStore) FailMAFilingAcquisition(_ context.Context, id, errMsg string) error {
	if a := f.acquisitions[id]; a != nil && a.Status != maFilingAcquisitionDone {
		a.Status = maFilingAcquisitionFailed
		a.Error = errMsg
		a.UpdatedAt = f.clock
	}
	return nil
}

func (f *fakeFilingStore) SetMAFilingAcquisitionUnknown(_ context.Context, id, errMsg string) error {
	if a := f.acquisitions[id]; a != nil && (a.Status == maFilingAcquisitionIntent || a.Status == maFilingAcquisitionRequested) {
		a.Status = maFilingAcquisitionUnknown
		a.Error = errMsg
		a.UpdatedAt = f.clock
	}
	return nil
}

func (f *fakeFilingStore) AdoptMAFilingAcquisitionRequest(_ context.Context, id, requestID string) error {
	if a := f.acquisitions[id]; a != nil && a.Status == maFilingAcquisitionUnknown {
		a.Status = maFilingAcquisitionRequested
		a.DocuEngineRequestID = requestID
		a.UpdatedAt = f.clock
	}
	return nil
}

func (f *fakeFilingStore) InsertMAFilingBlob(_ context.Context, md5Hex, _ string, _ []byte) error {
	f.insertBlobCalls++
	if f.failBlobFirst && f.insertBlobCalls == 1 {
		return errors.New("insert blob transient failure")
	}
	f.blobs[md5Hex] = true
	return nil
}

func (f *fakeFilingStore) CreateOrMergeMAFiling(_ context.Context, _ maFilingCreate) (string, bool, error) {
	f.filings++
	return f.nextID("filing"), false, nil
}

func (f *fakeFilingStore) ListMAFilingReferencedRequestIDs(_ context.Context, excludeSearchID, excludeAcquisitionID string) (map[string]bool, error) {
	out := map[string]bool{}
	for id, s := range f.searches {
		if id == excludeSearchID {
			continue
		}
		if s.DocuEngineRequestID != "" {
			out[s.DocuEngineRequestID] = true
		}
	}
	for id, a := range f.acquisitions {
		if id == excludeAcquisitionID {
			continue
		}
		if a.DocuEngineRequestID != "" {
			out[a.DocuEngineRequestID] = true
		}
	}
	return out, nil
}

// fakeDocuEngine counts calls per method and lets each test script the paid mutations
// (create/submit/select). GET/list are pure reads over the in-memory request state.
type fakeDocuEngine struct {
	createCalls, getCalls, submitCalls, selectCalls, listCalls, docCalls, dlCalls int
	seq                                                                           int
	requests                                                                      map[string]*openapiit.DocuRequest
	createFn                                                                      func(f *fakeDocuEngine, body openapiit.DocuRequestCreate) (openapiit.DocuRequest, error)
	submitFn                                                                      func(f *fakeDocuEngine, id, taxCode string) (openapiit.DocuRequest, error)
	selectFn                                                                      func(f *fakeDocuEngine, id, resultID string) (openapiit.DocuRequest, error)
}

func newFakeDocuEngine() *fakeDocuEngine {
	return &fakeDocuEngine{requests: map[string]*openapiit.DocuRequest{}}
}

func (f *fakeDocuEngine) newRequest(taxCode string) *openapiit.DocuRequest {
	f.seq++
	id := fmt.Sprintf("req-%d", f.seq)
	r := &openapiit.DocuRequest{ID: id, Name: docuBilancioOtticoName, State: openapiit.DocuStateNew, ReadableSearch: map[string]string{"taxCode": taxCode}}
	f.requests[id] = r
	return r
}

func (f *fakeDocuEngine) CreateRequest(_ context.Context, body openapiit.DocuRequestCreate) (openapiit.DocuRequest, error) {
	f.createCalls++
	if f.createFn != nil {
		return f.createFn(f, body)
	}
	r := f.newRequest(body.Search["field0"])
	return *r, nil
}

func (f *fakeDocuEngine) GetRequest(_ context.Context, id string) (openapiit.DocuRequest, error) {
	f.getCalls++
	r, ok := f.requests[id]
	if !ok {
		return openapiit.DocuRequest{}, &openapiit.APIError{Service: "docuengine", StatusCode: http.StatusNotFound}
	}
	return *r, nil
}

func (f *fakeDocuEngine) SubmitSearch(_ context.Context, id, taxCode string) (openapiit.DocuRequest, error) {
	f.submitCalls++
	if f.submitFn != nil {
		return f.submitFn(f, id, taxCode)
	}
	if r, ok := f.requests[id]; ok {
		r.State = openapiit.DocuStateSearch
		if r.ReadableSearch == nil {
			r.ReadableSearch = map[string]string{}
		}
		r.ReadableSearch["taxCode"] = taxCode
		return *r, nil
	}
	return openapiit.DocuRequest{}, &openapiit.APIError{StatusCode: http.StatusNotFound}
}

func (f *fakeDocuEngine) SelectResult(_ context.Context, id, resultID string) (openapiit.DocuRequest, error) {
	f.selectCalls++
	if f.selectFn != nil {
		return f.selectFn(f, id, resultID)
	}
	if r, ok := f.requests[id]; ok {
		rid := resultID
		r.ResultID = &rid
		r.State = openapiit.DocuStateDone
		return *r, nil
	}
	return openapiit.DocuRequest{}, &openapiit.APIError{StatusCode: http.StatusNotFound}
}

func (f *fakeDocuEngine) ListRequests(_ context.Context) ([]openapiit.DocuRequestSummary, error) {
	f.listCalls++
	out := make([]openapiit.DocuRequestSummary, 0, len(f.requests))
	for _, r := range f.requests {
		out = append(out, openapiit.DocuRequestSummary{ID: r.ID, Name: r.Name, State: r.State})
	}
	return out, nil
}

func (f *fakeDocuEngine) ListRequestDocuments(_ context.Context, id string) ([]openapiit.DocuDownload, error) {
	f.docCalls++
	sum := md5.Sum(fakeFilingPDF)
	return []openapiit.DocuDownload{{
		FileName:    id + "_1.pdf",
		MimeType:    "application/pdf",
		DownloadURL: "https://signed/" + id,
		MD5:         base64.StdEncoding.EncodeToString(sum[:]),
	}}, nil
}

func (f *fakeDocuEngine) DownloadFile(_ context.Context, _ string) ([]byte, error) {
	f.dlCalls++
	return fakeFilingPDF, nil
}

// ---------------------------------------------------------------------------
// Test helpers.
// ---------------------------------------------------------------------------

func newFilingTestService(filing *fakeFilingStore, docu *fakeDocuEngine) *maService {
	return &maService{
		// store is a no-op fake: the direct-work-function tests never set a trace in ctx
		// (so tracing is skipped), but the acquire finalize enqueues filing_ingest, which
		// needs a non-nil store.
		store:            &fakeMAWorkspaceStore{},
		filing:           filing,
		docu:             docu,
		filingDocumentID: "doc-bilancio",
		now:              func() time.Time { return time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC) },
	}
}

func filingSearchTestJob(searchID, fiscalKey, taxCode string) maJob {
	payload, _ := json.Marshal(maFilingSearchJobPayload{FiscalKey: fiscalKey, SearchID: searchID, TaxCode: taxCode})
	return maJob{JobType: maJobTypeFilingSearch, Payload: payload}
}

func filingAcquireTestJob(acqID, searchID, fiscalKey, bsid, taxCode string) maJob {
	payload, _ := json.Marshal(maFilingAcquireJobPayload{FiscalKey: fiscalKey, AcquisitionID: acqID, SearchID: searchID, BalanceSheetID: bsid, TaxCode: taxCode, Tax: taxCode})
	return maJob{JobType: maJobTypeFilingAcquire, Payload: payload}
}

func filingResultData(bsid int, date, typeCode string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"balanceSheetId":%d,"id":%d,"balanceSheetDate":%q,"balanceSheetTypeCode":%q,"balanceSheetTypeDescription":"Bilancio ordinario"}`, bsid, bsid, date, typeCode))
}

func assertPollPending(t *testing.T, tick string, err error) {
	t.Helper()
	if !errors.Is(err, errMAFilingPollPending) {
		t.Fatalf("%s: err = %v, want errMAFilingPollPending", tick, err)
	}
}

func assertJobDone(t *testing.T, tick string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: err = %v, want nil (job complete)", tick, err)
	}
}

// ---------------------------------------------------------------------------
// 1) SEARCH — ambiguous POST => unknown; retry reconciles via ListRequests, never re-POSTs.
// ---------------------------------------------------------------------------

func TestFilingSearchAmbiguousPostNeverRePosts(t *testing.T) {
	const taxCode = "12345678901"
	filing := newFakeFilingStore()
	docu := newFakeDocuEngine()
	// A second, unrelated Bilancio Ottico request must NOT be adopted: the taxCode match is
	// the definitive filter, not merely the name.
	other := docu.newRequest("99999999999")
	other.State = openapiit.DocuStateSearch
	// The POST creates the request server-side but the caller sees an ambiguous (network)
	// error — the request id is lost.
	docu.createFn = func(f *fakeDocuEngine, body openapiit.DocuRequestCreate) (openapiit.DocuRequest, error) {
		f.newRequest(body.Search["field0"])
		return openapiit.DocuRequest{}, errors.New("network timeout after send")
	}
	svc := newFilingTestService(filing, docu)
	ctx := context.Background()

	searchID, _ := filing.CreateMAFilingSearchIntent(ctx, "IT"+taxCode, "", "")
	job := filingSearchTestJob(searchID, "IT"+taxCode, taxCode)

	// Tick 1: ambiguous POST => unknown, no request id captured.
	assertPollPending(t, "tick1", svc.filingSearchWork(ctx, job))
	if docu.createCalls != 1 {
		t.Fatalf("createCalls after tick1 = %d, want 1", docu.createCalls)
	}
	if s := filing.searches[searchID]; s.Status != maFilingSearchUnknown || s.DocuEngineRequestID != "" {
		t.Fatalf("search after tick1 = %+v, want unknown with no request id", s)
	}

	// Tick 2: reconcile via ListRequests + per-candidate GetRequest + taxCode match. The
	// POST is NEVER re-issued.
	assertPollPending(t, "tick2", svc.filingSearchWork(ctx, job))
	if docu.createCalls != 1 {
		t.Fatalf("createCalls after tick2 = %d, want 1 (never re-POST)", docu.createCalls)
	}
	if docu.listCalls == 0 {
		t.Fatalf("listCalls after tick2 = 0, want the reconciliation to call ListRequests")
	}
	s := filing.searches[searchID]
	if s.Status != maFilingSearchRequested {
		t.Fatalf("search after tick2 status = %q, want requested (adopted)", s.Status)
	}
	if s.DocuEngineRequestID != "req-2" {
		t.Fatalf("search after tick2 request id = %q, want the recovered req-2", s.DocuEngineRequestID)
	}
	if docu.submitCalls != 1 {
		t.Fatalf("submitCalls after tick2 = %d, want 1 (search launched once after adoption)", docu.submitCalls)
	}
}

// ---------------------------------------------------------------------------
// 2) SEARCH — ambiguous PATCH (SubmitSearch) with a persisted request id => unknown; the
//    retry uses GetRequest and re-PATCHes ONLY when the GET shows the request still NEW.
// ---------------------------------------------------------------------------

func TestFilingSearchAmbiguousSubmitReconcilesFromGet(t *testing.T) {
	const taxCode = "12345678901"

	// Case A: the ambiguous PATCH DID land (GET shows SEARCH) => zero further PATCH.
	t.Run("get shows search already happened", func(t *testing.T) {
		filing := newFakeFilingStore()
		docu := newFakeDocuEngine()
		docu.submitFn = func(f *fakeDocuEngine, id, tc string) (openapiit.DocuRequest, error) {
			if f.submitCalls == 1 {
				// Landed server-side, but the caller sees an ambiguous error.
				r := f.requests[id]
				r.State = openapiit.DocuStateSearch
				r.ReadableSearch["taxCode"] = tc
				return openapiit.DocuRequest{}, errors.New("timeout")
			}
			r := f.requests[id]
			r.State = openapiit.DocuStateSearch
			return *r, nil
		}
		svc := newFilingTestService(filing, docu)
		ctx := context.Background()
		searchID, _ := filing.CreateMAFilingSearchIntent(ctx, "IT"+taxCode, "", "")
		job := filingSearchTestJob(searchID, "IT"+taxCode, taxCode)

		assertPollPending(t, "tick1", svc.filingSearchWork(ctx, job)) // POST NEW => requested
		assertPollPending(t, "tick2", svc.filingSearchWork(ctx, job)) // GET NEW => PATCH(ambiguous) => unknown
		if filing.searches[searchID].Status != maFilingSearchUnknown {
			t.Fatalf("status after tick2 = %q, want unknown", filing.searches[searchID].Status)
		}
		if docu.submitCalls != 1 {
			t.Fatalf("submitCalls after tick2 = %d, want 1", docu.submitCalls)
		}
		assertPollPending(t, "tick3", svc.filingSearchWork(ctx, job)) // GET shows SEARCH => adopt, no re-PATCH
		if docu.submitCalls != 1 {
			t.Fatalf("submitCalls after tick3 = %d, want 1 (GET showed SEARCH => zero new PATCH)", docu.submitCalls)
		}
		if filing.searches[searchID].Status != maFilingSearchRequested {
			t.Fatalf("status after tick3 = %q, want requested", filing.searches[searchID].Status)
		}
	})

	// Case B: the ambiguous PATCH did NOT land (GET shows NEW) => exactly one re-PATCH.
	t.Run("get shows still new", func(t *testing.T) {
		filing := newFakeFilingStore()
		docu := newFakeDocuEngine()
		docu.submitFn = func(f *fakeDocuEngine, id, tc string) (openapiit.DocuRequest, error) {
			if f.submitCalls == 1 {
				return openapiit.DocuRequest{}, errors.New("timeout") // did NOT land; state stays NEW
			}
			r := f.requests[id]
			r.State = openapiit.DocuStateDone
			r.Results = []openapiit.DocuResult{{ID: "r100", Data: filingResultData(100, "2023-12-31", "710")}}
			return *r, nil
		}
		svc := newFilingTestService(filing, docu)
		ctx := context.Background()
		searchID, _ := filing.CreateMAFilingSearchIntent(ctx, "IT"+taxCode, "", "")
		job := filingSearchTestJob(searchID, "IT"+taxCode, taxCode)

		assertPollPending(t, "tick1", svc.filingSearchWork(ctx, job)) // POST NEW => requested
		assertPollPending(t, "tick2", svc.filingSearchWork(ctx, job)) // GET NEW => PATCH(ambiguous) => unknown
		if docu.submitCalls != 1 {
			t.Fatalf("submitCalls after tick2 = %d, want 1", docu.submitCalls)
		}
		// GET shows NEW => exactly one more PATCH, which lands with results => done.
		assertJobDone(t, "tick3", svc.filingSearchWork(ctx, job))
		if docu.submitCalls != 2 {
			t.Fatalf("submitCalls after tick3 = %d, want 2 (GET showed NEW => one re-PATCH)", docu.submitCalls)
		}
		if filing.searches[searchID].Status != maFilingSearchResults {
			t.Fatalf("status after tick3 = %q, want results", filing.searches[searchID].Status)
		}
	})
}

// ---------------------------------------------------------------------------
// 3) ACQUIRE — ambiguous SelectResult (PATCH) => unknown; the retry resumes from GetRequest
//    and does NOT re-select when the GET shows a result id already registered.
// ---------------------------------------------------------------------------

func TestFilingAcquireAmbiguousSelectNeverReselects(t *testing.T) {
	const taxCode = "12345678901"
	filing := newFakeFilingStore()
	docu := newFakeDocuEngine()
	// A completed search whose request already carries a result for balance sheet 100.
	docu.requests["req-s"] = &openapiit.DocuRequest{
		ID: "req-s", Name: docuBilancioOtticoName, State: openapiit.DocuStateSearch,
		ReadableSearch: map[string]string{"taxCode": taxCode},
		Results:        []openapiit.DocuResult{{ID: "r100", Data: filingResultData(100, "2023-12-31", "710")}},
	}
	filing.searches["s1"] = &maFilingSearch{ID: "s1", FiscalKey: "IT" + taxCode, Status: maFilingSearchResults, DocuEngineRequestID: "req-s", UpdatedAt: filing.clock}
	docu.selectFn = func(f *fakeDocuEngine, id, resultID string) (openapiit.DocuRequest, error) {
		if f.selectCalls == 1 {
			// Landed server-side (result id registered), but the caller sees an ambiguous error.
			r := f.requests[id]
			rid := resultID
			r.ResultID = &rid
			r.State = openapiit.DocuStateWait
			return openapiit.DocuRequest{}, errors.New("timeout")
		}
		r := f.requests[id]
		r.State = openapiit.DocuStateDone
		return *r, nil
	}
	svc := newFilingTestService(filing, docu)
	ctx := context.Background()

	acqID, _ := filing.CreateMAFilingAcquisitionIntent(ctx, maFilingAcquisitionCreate{Origin: maFilingOriginDocuEngine, BalanceSheetID: "100", ResultID: "r100"})
	job := filingAcquireTestJob(acqID, "s1", "IT"+taxCode, "100", taxCode)

	assertPollPending(t, "tick1", svc.filingAcquireWork(ctx, job)) // claim search, reuse request
	if docu.createCalls != 0 {
		t.Fatalf("createCalls after tick1 = %d, want 0 (reused the search request, no new POST)", docu.createCalls)
	}
	assertPollPending(t, "tick2", svc.filingAcquireWork(ctx, job)) // GET => select(ambiguous) => unknown
	if docu.selectCalls != 1 {
		t.Fatalf("selectCalls after tick2 = %d, want 1", docu.selectCalls)
	}
	if filing.acquisitions[acqID].Status != maFilingAcquisitionUnknown {
		t.Fatalf("acquisition after tick2 status = %q, want unknown", filing.acquisitions[acqID].Status)
	}
	assertPollPending(t, "tick3", svc.filingAcquireWork(ctx, job)) // GET shows result id set => no re-select
	if docu.selectCalls != 1 {
		t.Fatalf("selectCalls after tick3 = %d, want 1 (GET showed selection registered => no re-select)", docu.selectCalls)
	}
	if filing.acquisitions[acqID].Status != maFilingAcquisitionRequested {
		t.Fatalf("acquisition after tick3 status = %q, want requested (adopted, awaiting DONE)", filing.acquisitions[acqID].Status)
	}
	if docu.createCalls != 0 {
		t.Fatalf("createCalls total = %d, want 0 (a reused-search acquire never POSTs)", docu.createCalls)
	}
}

// ---------------------------------------------------------------------------
// 4) ACQUIRE — the search claim is won exactly once: two concurrent acquisitions of the
//    same search, the loser opens its OWN request instead of reusing the search's.
// ---------------------------------------------------------------------------

func TestFilingAcquireSearchClaimWonOnce(t *testing.T) {
	const taxCode = "12345678901"
	filing := newFakeFilingStore()
	docu := newFakeDocuEngine()
	docu.requests["req-s"] = &openapiit.DocuRequest{
		ID: "req-s", Name: docuBilancioOtticoName, State: openapiit.DocuStateSearch,
		ReadableSearch: map[string]string{"taxCode": taxCode},
		Results: []openapiit.DocuResult{
			{ID: "r100", Data: filingResultData(100, "2023-12-31", "710")},
			{ID: "r200", Data: filingResultData(200, "2022-12-31", "710")},
		},
	}
	filing.searches["s1"] = &maFilingSearch{ID: "s1", FiscalKey: "IT" + taxCode, Status: maFilingSearchResults, DocuEngineRequestID: "req-s", UpdatedAt: filing.clock}
	svc := newFilingTestService(filing, docu)
	ctx := context.Background()

	acq1, _ := filing.CreateMAFilingAcquisitionIntent(ctx, maFilingAcquisitionCreate{Origin: maFilingOriginDocuEngine, BalanceSheetID: "100", ResultID: "r100"})
	acq2, _ := filing.CreateMAFilingAcquisitionIntent(ctx, maFilingAcquisitionCreate{Origin: maFilingOriginDocuEngine, BalanceSheetID: "200", ResultID: "r200"})
	job1 := filingAcquireTestJob(acq1, "s1", "IT"+taxCode, "100", taxCode)
	job2 := filingAcquireTestJob(acq2, "s1", "IT"+taxCode, "200", taxCode)

	// acq1 wins the CAS and reuses the search's request.
	assertPollPending(t, "acq1", svc.filingAcquireWork(ctx, job1))
	if got := filing.acquisitions[acq1].DocuEngineRequestID; got != "req-s" {
		t.Fatalf("acq1 request id = %q, want the reused search request req-s", got)
	}
	if docu.createCalls != 0 {
		t.Fatalf("createCalls after acq1 = %d, want 0 (winner reuses the search request)", docu.createCalls)
	}
	if filing.searches["s1"].ConsumedByAcquisitionID != acq1 {
		t.Fatalf("search consumed by = %q, want acq1 %q", filing.searches["s1"].ConsumedByAcquisitionID, acq1)
	}

	// acq2 lost the claim => opens its OWN request, never reusing the search's.
	assertPollPending(t, "acq2", svc.filingAcquireWork(ctx, job2))
	if docu.createCalls != 1 {
		t.Fatalf("createCalls after acq2 = %d, want 1 (loser POSTs its own request)", docu.createCalls)
	}
	got := filing.acquisitions[acq2].DocuEngineRequestID
	if got == "" || got == "req-s" {
		t.Fatalf("acq2 request id = %q, want a fresh request distinct from the search's req-s", got)
	}
}

// ---------------------------------------------------------------------------
// 5) ACQUIRE — a store error on the POST-PAYMENT finalize (InsertBlob fails once) must NOT
//    give up nor trigger any second vendor call: the retry resumes the finalize and
//    completes, with the paid-call counts unchanged.
// ---------------------------------------------------------------------------

func TestFilingAcquireFinalizeStoreErrorResumesWithoutRecharge(t *testing.T) {
	const taxCode = "12345678901"
	filing := newFakeFilingStore()
	filing.failBlobFirst = true // the first InsertMAFilingBlob (finalize) fails
	docu := newFakeDocuEngine()
	docu.requests["req-s"] = &openapiit.DocuRequest{
		ID: "req-s", Name: docuBilancioOtticoName, State: openapiit.DocuStateSearch,
		ReadableSearch: map[string]string{"taxCode": taxCode},
		Results:        []openapiit.DocuResult{{ID: "r100", Data: filingResultData(100, "2023-12-31", "710")}},
	}
	filing.searches["s1"] = &maFilingSearch{ID: "s1", FiscalKey: "IT" + taxCode, Status: maFilingSearchResults, DocuEngineRequestID: "req-s", UpdatedAt: filing.clock}
	svc := newFilingTestService(filing, docu)
	ctx := context.Background()

	acqID, _ := filing.CreateMAFilingAcquisitionIntent(ctx, maFilingAcquisitionCreate{Origin: maFilingOriginDocuEngine, BalanceSheetID: "100", ResultID: "r100"})
	job := filingAcquireTestJob(acqID, "s1", "IT"+taxCode, "100", taxCode)

	assertPollPending(t, "tick1", svc.filingAcquireWork(ctx, job)) // claim + reuse search request
	assertPollPending(t, "tick2", svc.filingAcquireWork(ctx, job)) // GET => select => DONE (poll for download)

	// Tick 3: DONE => download + finalize, but InsertBlob fails => a real (non-poll) error.
	err := svc.filingAcquireWork(ctx, job)
	if err == nil || errors.Is(err, errMAFilingPollPending) {
		t.Fatalf("tick3 err = %v, want a real store error (not nil, not poll-pending)", err)
	}
	if filing.acquisitions[acqID].Status != maFilingAcquisitionDownloaded {
		t.Fatalf("acquisition after tick3 status = %q, want downloaded", filing.acquisitions[acqID].Status)
	}

	// Snapshot the paid-call counts: the finalize retry must not re-issue any of them.
	createBefore, submitBefore, selectBefore := docu.createCalls, docu.submitCalls, docu.selectCalls

	// Tick 4: resume from 'downloaded'; InsertBlob now succeeds => filing created, done.
	assertJobDone(t, "tick4", svc.filingAcquireWork(ctx, job))
	if filing.acquisitions[acqID].Status != maFilingAcquisitionDone {
		t.Fatalf("acquisition after tick4 status = %q, want done", filing.acquisitions[acqID].Status)
	}
	if filing.filings != 1 {
		t.Fatalf("filings created = %d, want 1", filing.filings)
	}
	if docu.createCalls != createBefore || docu.submitCalls != submitBefore || docu.selectCalls != selectBefore {
		t.Fatalf("paid vendor calls changed across finalize retry: create %d->%d submit %d->%d select %d->%d",
			createBefore, docu.createCalls, submitBefore, docu.submitCalls, selectBefore, docu.selectCalls)
	}
	// The whole acquisition reused the search request: never a POST, never a submit.
	if docu.createCalls != 0 || docu.submitCalls != 0 {
		t.Fatalf("reused-search acquire made create=%d submit=%d, want 0/0", docu.createCalls, docu.submitCalls)
	}
	if docu.selectCalls != 1 {
		t.Fatalf("selectCalls = %d, want exactly 1 (selected once, never re-selected on finalize retry)", docu.selectCalls)
	}
}
