package binocolo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// maJobWorkerStore is the persistence the async job worker needs. *SQLStore
// satisfies it; kept narrow so the worker stays testable and decoupled.
type maJobWorkerStore interface {
	ListMAJobs(ctx context.Context, limit int, owner, workerID string, jobTypes []string) ([]maJob, error)
	AcquireMAJobLease(ctx context.Context, jobID, owner, workerID string, leaseSeconds int) (bool, error)
	ClaimMAJobQueued(ctx context.Context, jobID string) (bool, error)
	SetMAJobTrace(ctx context.Context, jobID, traceID string) error
	CompleteMAJob(ctx context.Context, jobID string) error
	FailMAJob(ctx context.Context, jobID, errorCode string) error
	BumpMAJobAttempt(ctx context.Context, jobID string) (int, error)
	TouchMAJob(ctx context.Context, jobID string) error
	MarkMASessionEstimateFailed(ctx context.Context, sessionID string) error
	MarkMASessionExecuteFailed(ctx context.Context, sessionID string) error
}

// maJobWorker drives the async session-job queue (estimate today; execute next)
// as a DB-backed state machine: queued -> running -> ready | failed. State lives
// in binocolo.ma_job, so a process restart resumes naturally — the first tick
// picks up queued/running rows. Same lease/claim mechanics as the deep worker,
// correct with multiple replicas on one DB.
type maJobWorker struct {
	// id is this worker process's ephemeral lease identity (a fresh uuid per start);
	// owner is this instance's stable identity (config InstanceOwner). A row is
	// claimable when it belongs to this owner (or is legacy owner-NULL) and is
	// unleased / pre-leased to the owner / already held by this worker / expired.
	id       string
	owner    string
	svc      *maService
	store    maJobWorkerStore
	interval time.Duration
	batch    int
	jobTypes []string
	// ticks counts elapsed ticks so the deposited-filing orphan sweep runs only every
	// maFilingIngestSweepEveryTicks (cheap, rare) rather than every tick.
	ticks int
}

func newMAJobWorker(svc *maService, store maJobWorkerStore, owner string) *maJobWorker {
	return &maJobWorker{
		id:       uuid.NewString(),
		owner:    owner,
		svc:      svc,
		store:    store,
		interval: 2 * time.Second,
		batch:    16,
		jobTypes: []string{
			maJobTypeEstimate,
			maJobTypeExecute,
			maJobTypeWebValidation,
			maJobTypeGatedSearch,
			maJobTypeAssociateDomain,
			maJobTypeManualAdd,
			maJobTypeCardDomainVerify,
			maJobTypeFilingSearch,
			maJobTypeFilingAcquire,
			maJobTypeFilingIngest, // F5 activates the ingest dispatch (F4 only enqueued it).
		},
	}
}

func (w *maJobWorker) run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.tick(ctx) // resume sweep on start
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *maJobWorker) tick(ctx context.Context) {
	jobs, err := w.store.ListMAJobs(ctx, w.batch, w.owner, w.id, w.jobTypes)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo job worker list failed", "component", "binocolo", "operation", "ma_job_worker", "error", err)
		return
	}
	for _, job := range jobs {
		w.process(ctx, job)
	}
	// Periodic re-aggancio of orphaned filing_ingest work (QA-F4 gap): a filing left in a
	// non-terminal ingest state with no job inflight is re-enqueued (idempotent via the
	// mig-115 inflight index). Rare relative to the drain, so it does not slow the tick.
	w.ticks++
	if w.ticks%maFilingIngestSweepEveryTicks == 0 {
		w.svc.sweepFilingIngestOrphans(ctx)
	}
}

func (w *maJobWorker) process(ctx context.Context, job maJob) {
	// Per-row lease: only the owning worker advances a row, so multiple replicas (or
	// devs sharing one staging DB) never run the same job twice.
	owned, err := w.store.AcquireMAJobLease(ctx, job.ID, w.owner, w.id, maJobLeaseSeconds)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo job worker lease failed", "component", "binocolo", "job_id", job.ID, "error", err)
		return
	}
	if !owned {
		return // another worker owns this row right now
	}
	if job.Status == maJobStatusQueued || job.Status == maJobStatusPending {
		claimed, err := w.store.ClaimMAJobQueued(ctx, job.ID)
		if err != nil {
			logging.FromContext(ctx).Warn("binocolo job worker claim failed", "component", "binocolo", "job_id", job.ID, "error", err)
			return
		}
		if !claimed {
			return // another worker/replica claimed it first
		}
	}

	var traceID string
	switch job.JobType {
	case maJobTypeEstimate:
		traceID, err = w.svc.runEstimateJob(ctx, job)
	case maJobTypeExecute:
		traceID, err = w.svc.runExecuteJob(ctx, job)
	case maJobTypeWebValidation:
		traceID, err = w.svc.runWebValidationJob(ctx, job)
	case maJobTypeGatedSearch:
		traceID, err = w.svc.runGatedSearchJob(ctx, job)
	case maJobTypeAssociateDomain:
		traceID, err = w.svc.runAssociateDomainJob(ctx, job)
	case maJobTypeManualAdd:
		traceID, err = w.svc.runManualAddJob(ctx, job)
	case maJobTypeCardDomainVerify:
		traceID, err = w.svc.runCardDomainVerifyJob(ctx, job)
	case maJobTypeFilingSearch:
		traceID, err = w.svc.runFilingSearchJob(ctx, job)
	case maJobTypeFilingAcquire:
		traceID, err = w.svc.runFilingAcquireJob(ctx, job)
	case maJobTypeFilingIngest:
		traceID, err = w.svc.runFilingIngestJob(ctx, job)
	default:
		logging.FromContext(ctx).Warn("binocolo job worker skipped unknown job type", "component", "binocolo", "job_id", job.ID, "job_type", job.JobType)
		return
	}
	if traceID != "" {
		_ = w.store.SetMAJobTrace(ctx, job.ID, traceID)
	}
	if err != nil {
		// A filing poll-pending is not a failure: the vendor result/DONE is not ready
		// yet (or an 'unknown' outcome is being reconciled). Leave the row processing and
		// re-run next tick, bounded by the poll budget — never the terminal give-up path.
		if errors.Is(err, errMAFilingPollPending) {
			w.pollAgainOrTimeout(ctx, job)
			return
		}
		w.retryOrFail(ctx, job, classifyMAJobError(err, job.JobType))
		return
	}
	if err := w.store.CompleteMAJob(ctx, job.ID); err != nil {
		logging.FromContext(ctx).Warn("binocolo job worker complete failed", "component", "binocolo", "job_id", job.ID, "error", err)
	}
}

// retryOrFail bumps the attempt counter and either leaves the row running (the
// next tick re-runs it from scratch; estimate work is cheap and cache-backed) or,
// past the cap, marks the job failed and moves the session out of the in-flight
// 'estimating' state so the UI stops waiting.
func (w *maJobWorker) retryOrFail(ctx context.Context, job maJob, code string) {
	attempts, err := w.store.BumpMAJobAttempt(ctx, job.ID)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo job worker bump failed", "component", "binocolo", "job_id", job.ID, "error", err)
		return
	}
	if attempts < maJobMaxAttempts {
		logging.FromContext(ctx).Warn("binocolo job worker retry", "component", "binocolo", "job_id", job.ID, "job_type", job.JobType, "attempt", attempts, "error_code", code)
		return // leave running; reclaimed and re-run on the next tick
	}
	logging.FromContext(ctx).Error("binocolo job worker giving up", "component", "binocolo", "job_id", job.ID, "job_type", job.JobType, "attempts", attempts, "error_code", code)
	if err := w.store.FailMAJob(ctx, job.ID, code); err != nil {
		logging.FromContext(ctx).Warn("binocolo job worker fail failed", "component", "binocolo", "job_id", job.ID, "error", err)
	}
	switch job.JobType {
	case maJobTypeEstimate:
		if err := w.store.MarkMASessionEstimateFailed(ctx, job.SessionID); err != nil {
			logging.FromContext(ctx).Warn("binocolo job worker session-fail failed", "component", "binocolo", "session_id", job.SessionID, "error", err)
		}
	case maJobTypeExecute, maJobTypeGatedSearch:
		if err := w.store.MarkMASessionExecuteFailed(ctx, job.SessionID); err != nil {
			logging.FromContext(ctx).Warn("binocolo job worker session-fail failed", "component", "binocolo", "session_id", job.SessionID, "error", err)
		}
	case maJobTypeAssociateDomain, maJobTypeManualAdd:
		// The session was 'completed' before the per-target remedy; a failed remedy must not
		// destroy that — release it back to 'completed' (results intact), not 'failed'.
		w.svc.releaseAssociateSession(ctx, job.SessionID)
	case maJobTypeCardDomainVerify:
		w.svc.recordCardDomainUnverifiable(ctx, job)
	case maJobTypeFilingSearch, maJobTypeFilingAcquire:
		// Terminal give-up on an infrastructure error (attempts exhausted): fail the business
		// row, but NEVER resolve an 'unknown' vendor outcome heuristically — unknown stays
		// unknown for a future manual/dev reconciliation.
		w.svc.failFilingJobIfNotUnknown(ctx, job, code)
	case maJobTypeFilingIngest:
		// Terminal give-up on an infra error: fail the filing UNLESS it is identity_blocked
		// (awaits a manual override). There is no 'unknown' state for the ingest.
		w.svc.failFilingIngestIfNotBlocked(ctx, job, code)
	}
}

// pollAgainOrTimeout advances a filing job that is still waiting on DocuEngine. The poll
// budget is WALL-CLOCK (created_at + maFilingPollWindow), NOT attempt-based: a "not ready
// yet" tick is expected and must NOT bump the attempt counter (that budget stays reserved
// for real infra errors, including the post-payment finalize). Within the window the row
// is left processing so the next tick re-runs it. Past the window the job ends with a
// poll_timeout and the business row goes to 'unknown' (NOT failed) — the paid vendor
// request may still complete and a later reconciliation can adopt it.
func (w *maJobWorker) pollAgainOrTimeout(ctx context.Context, job maJob) {
	// No attempt bump here: the deadline alone bounds the loop.
	if job.CreatedAt.IsZero() {
		return // no wall-clock anchor (off-DB job); keep polling — created_at is set in prod
	}
	if w.svc.now().Before(job.CreatedAt.Add(maFilingPollWindow)) {
		// Within the poll window: re-run next tick. Bump only updated_at (no attempt change)
		// so a long poll rotates to the back of the shared queue and does not starve other
		// job types under `ORDER BY updated_at LIMIT n`.
		if err := w.store.TouchMAJob(ctx, job.ID); err != nil {
			logging.FromContext(ctx).Warn("binocolo job worker touch failed", "component", "binocolo", "job_id", job.ID, "error", err)
		}
		return
	}
	logging.FromContext(ctx).Error("binocolo job worker giving up polling", "component", "binocolo", "job_id", job.ID, "job_type", job.JobType, "created_at", job.CreatedAt)
	if err := w.store.FailMAJob(ctx, job.ID, "poll_timeout"); err != nil {
		logging.FromContext(ctx).Warn("binocolo job worker fail failed", "component", "binocolo", "job_id", job.ID, "error", err)
	}
	w.svc.markFilingJobUnknownOnPollTimeout(ctx, job)
}

// classifyMAJobError maps an internal error to a short, stable error_code stored
// on the job row (and surfaced for diagnosis), without leaking message detail.
func classifyMAJobError(err error, jobType string) string {
	switch {
	case errors.Is(err, errMAEstimateSuperseded):
		return "superseded"
	case errors.Is(err, errMAStrategyInvalid):
		return "strategy_invalid"
	case errors.Is(err, errMABraveUnavailable):
		return "brave_unavailable"
	case errors.Is(err, errMAOpenAPIITUnavailable):
		return "openapiit_unavailable"
	case errors.Is(err, errMAStoreUnavailable):
		return "store_unavailable"
	default:
		switch jobType {
		case maJobTypeWebValidation:
			return "web_validation_failed"
		case maJobTypeGatedSearch:
			return "gated_search_failed"
		case maJobTypeAssociateDomain:
			return "associate_domain_failed"
		case maJobTypeManualAdd:
			if errors.Is(err, errMAVATNotFound) {
				return "vat_not_found"
			}
			return "manual_add_failed"
		case maJobTypeCardDomainVerify:
			return "card_domain_verify_failed"
		case maJobTypeFilingSearch:
			return "filing_search_failed"
		case maJobTypeFilingAcquire:
			return "filing_acquire_failed"
		case maJobTypeFilingIngest:
			return "filing_ingest_failed"
		default:
			return "estimate_failed"
		}
	}
}
