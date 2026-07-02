package binocolo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// maGatedSearchJobPayload is the gated-search job's self-contained args, validated
// synchronously at enqueue (estimate freshness + surface cap). The strategy version
// is pinned on the job row, so the worker runs exactly what was confirmed.
type maGatedSearchJobPayload struct {
	StrategyType   string `json:"strategy_type"`
	Limit          int    `json:"limit"`
	EstimatedCount int    `json:"estimated_count"`
}

// maAssociateDomainJobPayload is the associate_domain job's args: which held company and
// which operator-supplied domain to re-gate it with.
type maAssociateDomainJobPayload struct {
	CompanyKey string `json:"company_key"`
	Domain     string `json:"domain"`
}

// maGatedScoreStats records what the enrich_score stage did, for the trace.
type maGatedScoreStats struct {
	Enriched     int // survivors that paid IT-advanced this pass
	Reused       int // survivors already advanced on a prior pass (no re-charge)
	Skipped      int // salta (reject) targets kept identity-only, never charged
	ManualReview int // domain-unresolved targets held for manual domain, never charged
	Unscored     int // survivors whose advanced fetch failed → left as forse/identity
}

// enqueueGatedSearch is the synchronous half of the gated-search flow. It runs the
// same fast validation as execute (estimate freshness, strategy pin) but swaps the
// budget gate for the SURFACE CAP: with "tutto automatico" spend, the cap is the one
// hard governor, because the gate pays ~€0.02/company across the whole surface. The
// heavy multi-stage work runs in the worker (runGatedSearchJob), so the request never
// times out. Coexists with execute as a distinct mode (one in-flight job per type).
func (s *maService) enqueueGatedSearch(ctx context.Context, sessionID string, req MAExecuteSessionRequest, subject, email string, inline bool) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	if s.openapiit == nil {
		return MASessionDetail{}, errMAOpenAPIITUnavailable
	}
	if s.brave == nil {
		return MASessionDetail{}, errMABraveUnavailable
	}
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return MASessionDetail{}, err
	}
	if err := ensureMASessionOperational(detail.Session); err != nil {
		return MASessionDetail{}, err
	}
	strategyVersion := detail.Strategy
	if req.Strategy != nil {
		strategy, err := validateMAStrategy(*req.Strategy)
		if err != nil {
			return MASessionDetail{}, err
		}
		strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategy, nil, false)
		if err != nil {
			return MASessionDetail{}, err
		}
		strategyVersion, err = s.store.AddMAStrategyVersion(ctx, sessionID, strategy, email)
		if err != nil {
			return MASessionDetail{}, err
		}
		detail, err = s.store.GetMASession(ctx, sessionID)
		if err != nil {
			return MASessionDetail{}, err
		}
	}
	if strategyVersion == nil {
		return MASessionDetail{}, fmt.Errorf("%w: strategy", errMAStrategyInvalid)
	}

	strategyType := normalizeMAStrategyType(req.StrategyType)
	if strategyType == "" {
		strategyType = detail.Session.SelectedStrategy
	}
	if strategyType == "" {
		strategyType = strategyVersion.Strategy.SelectedStrategy
	}
	if strategyType == "" {
		strategyType = chooseSelectedStrategyFromEstimates(detail.Estimates, estimateTotal(detail.Estimates, maStrategyTypeATECO) > 0)
	}
	estimatedCount := estimateTotal(detail.Estimates, strategyType)
	if len(detail.Estimates) == 0 || estimatedCount == 0 {
		return MASessionDetail{}, fmt.Errorf("%w: estimate required", errMAStrategyInvalid)
	}
	// The real (queued) endpoint enforces execute's "run exactly what you estimated"
	// contract: the confirmed limit must equal the pinned SearchLimit and match every
	// per-combo estimate row. The test/inline path relaxes it — it runs the funnel on an
	// EXISTING session's estimate, defaulting the limit to the pinned SearchLimit (so the
	// dry-run cache matches) and letting the caller sub-sample a smaller, cheaper surface.
	var limit int
	if inline {
		limit = normalizeMASearchLimit(req.Limit)
		if req.Limit <= 0 {
			limit = strategyVersion.Strategy.SearchLimit
			if limit <= 0 {
				limit = maDefaultSearchLimit
			}
		}
	} else {
		limit = normalizeMASearchLimit(req.Limit)
		if limit != strategyVersion.Strategy.SearchLimit {
			return MASessionDetail{}, fmt.Errorf("%w: stale estimate", errMAStrategyInvalid)
		}
		if !estimatesMatchSearchLimit(detail.Estimates, strategyType, limit) {
			return MASessionDetail{}, fmt.Errorf("%w: stale estimate", errMAStrategyInvalid)
		}
	}
	// Surface cap: the admitted surface is min(limit, available). Reject up front when
	// it exceeds the cap — the gate spends across the WHOLE surface, so this is the
	// governor. No spend happens on rejection.
	pricing := s.loadPricing(ctx)
	admitted := limit
	if estimatedCount < admitted {
		admitted = estimatedCount
	}
	if pricing.SurfaceCap > 0 && admitted > pricing.SurfaceCap {
		return MASessionDetail{}, errMAEstimateTooLarge
	}

	payload, err := json.Marshal(maGatedSearchJobPayload{StrategyType: strategyType, Limit: limit, EstimatedCount: estimatedCount})
	if err != nil {
		return MASessionDetail{}, fmt.Errorf("marshal ma gated search payload: %w", err)
	}

	// Inline (dev/test only, off-queue): the shared ma_job queue can be claimed by a
	// foreign worker running stale code, which on this paid pipeline means double spend.
	// Inline runs the whole funnel in THIS process via a detached goroutine and writes no
	// ma_job row — nothing to steal. No durability/resume; used by the test page. Guard on
	// session status so a re-trigger while a run is in flight cannot double-charge.
	if inline {
		if detail.Session.Status == maSessionStatusRunning {
			return MASessionDetail{}, fmt.Errorf("%w: gated search already running", errMAStrategyInvalid)
		}
		if err := s.store.MarkMASessionExecuting(ctx, sessionID); err != nil {
			return MASessionDetail{}, err
		}
		job := maJob{
			JobType:           maJobTypeGatedSearch,
			SessionID:         sessionID,
			StrategyVersionID: strategyVersion.ID,
			Status:            maJobStatusRunning,
			Payload:           payload,
			CreatedBySubject:  subject,
			CreatedByEmail:    email,
		}
		go func() {
			bg := context.WithoutCancel(ctx)
			if _, err := s.runGatedSearchJob(bg, job); err != nil {
				logging.FromContext(bg).Error("binocolo inline gated search failed",
					"component", "binocolo", "operation", "ma_gated_search_inline",
					"session_id", sessionID, "error", err)
			}
		}()
		return s.getSession(ctx, sessionID)
	}

	_, created, err := s.store.EnqueueMAJob(ctx, maJobEnqueue{
		JobType:           maJobTypeGatedSearch,
		SessionID:         sessionID,
		StrategyVersionID: strategyVersion.ID,
		Subject:           subject,
		Email:             email,
		Payload:           payload,
		Owner:             s.owner,
	})
	if err != nil {
		return MASessionDetail{}, err
	}
	if created {
		if err := s.store.MarkMASessionExecuting(ctx, sessionID); err != nil {
			return MASessionDetail{}, err
		}
	}
	return s.getSession(ctx, sessionID)
}

// runGatedSearchJob executes a queued gated-search job off the request path, owning
// the operation trace for the run.
func (s *maService) runGatedSearchJob(ctx context.Context, job maJob) (string, error) {
	trace, err := s.startTrace(ctx, maTraceStart{
		Operation:        "ma_session_gated_search",
		SessionID:        job.SessionID,
		CreatedBySubject: job.CreatedBySubject,
		CreatedByEmail:   job.CreatedByEmail,
		Request:          job.Payload,
	})
	if err != nil {
		return "", err
	}
	ctx = withMATrace(ctx, trace)
	if workErr := s.gatedSearchJobWork(ctx, job); workErr != nil {
		_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusFailed, ErrorMessage: workErr.Error()})
		return trace.id, workErr
	}
	_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusSucceeded, HTTPStatus: http.StatusOK})
	return trace.id, nil
}

// gatedSearchJobWork is the multi-stage orchestrator: surface → address → UC2 gate →
// advanced+score on survivors → ready. It has NO explicit stage column: each stage is
// idempotent and derives its resume point from durable rows, so a crashed/reclaimed
// job re-runs cheaply without re-charging:
//   - address: skipped when targets already exist; a run with 0 targets means a prior
//     attempt crashed mid-address → ABANDON (like execute) rather than re-charge.
//   - gate: reuses validateMATargetsBatch, whose per-company freshness guard re-charges
//     only the un-validated tail.
//   - enrich_score: MarkMATargetAdvancedEnriched persists each €0.10 fetch immediately,
//     so a re-run skips already-advanced survivors (the enrichment_level guard).
//
// Returning an error triggers a worker retry; because every stage is idempotent that is
// safe. The one residual re-charge window is a crash mid-address after runExecution
// charged but before ReplaceMATargets committed — caught by the abandon guard on the
// next attempt (never re-charged, but the address spend of that attempt is lost). This
// matches the execute contract.
func (s *maService) gatedSearchJobWork(ctx context.Context, job maJob) error {
	var payload maGatedSearchJobPayload
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decode ma gated search payload: %w", err)
		}
	}
	detail, err := s.store.GetMASession(ctx, job.SessionID)
	if err != nil {
		return err
	}
	if err := ensureMASessionOperational(detail.Session); err != nil {
		return err
	}
	version, err := s.store.GetMAStrategyVersion(ctx, job.SessionID, job.StrategyVersionID)
	if err != nil {
		return err
	}
	strategyType := normalizeMAStrategyType(payload.StrategyType)
	limit := normalizeMASearchLimit(payload.Limit)
	pricing := s.loadPricing(ctx)
	if pricing.SurfaceCap > 0 && limit > pricing.SurfaceCap {
		limit = pricing.SurfaceCap
	}

	// Canonicalize the pinned strategy once (deterministic, no spend) so the gate
	// perimeter and scoring use the same curated AtecoCandidates on every attempt,
	// exactly like execute. The address stage expands FROM this for the search.
	strategy := version.Strategy
	strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategy, nil, false)
	if err != nil {
		return err // pure/cheap, no spend → safe to retry
	}

	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_gated_search_started",
		Status:    maTraceEventStarted,
		Metadata:  maTraceJSON(map[string]any{"session_id": job.SessionID, "strategy_version_id": version.ID, "strategy_type": strategyType, "limit": limit, "surface_cap": pricing.SurfaceCap}),
	})

	// ---- Stage: address (identity-only fetch on the whole surface) ----
	// Reuse persisted address targets ONLY when they were fetched for the SAME strategy.
	// A strategy switch (e.g. the test page re-running ateco→expanded on a session that
	// already ran) must re-fetch the whole surface — reusing the prior strategy's targets
	// is exactly the "expanded still shows the ateco surface" bug. ReplaceMATargets wipes
	// the session's targets, so the switch is clean; same-strategy reuse preserves the
	// resume/idempotency contract (no re-charge).
	runID := ""
	reuseTargets := len(detail.Targets) > 0 &&
		gatedRunStrategyType(detail.Runs, detail.Targets[0].RunID) == strategyType
	if reuseTargets {
		runID = detail.Targets[0].RunID
	} else {
		// Fresh address fetch: no targets yet, or a strategy switch that must replace the
		// prior surface. A run already 'running' here means a prior attempt crashed
		// mid-address before committing any targets for this strategy → abandon rather
		// than re-charge. On a first strategy switch the prior run is already completed,
		// so this passes through; it only fires on a genuine crashed/in-flight attempt.
		inflight, err := s.store.HasRunningMAExecution(ctx, job.SessionID)
		if err != nil {
			return err
		}
		if inflight {
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType: "ma_gated_search_abandoned",
				Status:    maTraceEventFailed,
				Metadata:  maTraceJSON(map[string]any{"session_id": job.SessionID, "reason": "run in flight with no committed targets for requested strategy (worker likely crashed mid-address)"}),
			})
			return s.store.AbandonMASessionExecution(ctx, job.SessionID, "gated_search_interrupted")
		}

		run, err := s.store.CreateMAExecutionRun(ctx, maExecutionRunCreate{
			SessionID:         job.SessionID,
			StrategyVersionID: version.ID,
			StrategyType:      strategyType,
			EstimatedCount:    payload.EstimatedCount,
		})
		if err != nil {
			return err
		}
		_ = s.linkTrace(ctx, maTraceLink{SessionID: job.SessionID, StrategyVersionID: version.ID, ExecutionRunID: run.ID})
		// Best-effort from here: a run exists, so failures are recorded on the run and
		// the job completes (no retry that would re-charge the address fetch). Expand the
		// canonicalized strategy (cache-backed dry-runs, effectively free) for the search.
		expanded, err := s.expandStrategyAteco(ctx, strategy, job.CreatedBySubject, job.CreatedByEmail)
		if err != nil {
			_ = s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusFailed, 0, maErrorCode(err))
			return nil
		}
		expanded, err = s.expandStrategyExpansion(ctx, expanded, job.CreatedBySubject, job.CreatedByEmail)
		if err != nil {
			_ = s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusFailed, 0, maErrorCode(err))
			return nil
		}
		addressTargets, execErr := s.runExecution(ctx, expanded, strategyType, limit, job.CreatedBySubject, job.CreatedByEmail, maEnrichmentAddress)
		if execErr != nil {
			_ = s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusFailed, 0, maErrorCode(execErr))
			return nil
		}
		for index := range addressTargets {
			addressTargets[index].SessionID = job.SessionID
			addressTargets[index].RunID = run.ID
		}
		if err := s.store.ReplaceMATargets(ctx, job.SessionID, run.ID, addressTargets); err != nil {
			_ = s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusFailed, 0, "store_error")
			return nil
		}
		runID = run.ID
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_gated_address_completed",
			Status:    maTraceEventSucceeded,
			Metadata:  maTraceJSON(map[string]any{"session_id": job.SessionID, "run_id": run.ID, "target_count": len(addressTargets)}),
		})
		detail, err = s.store.GetMASession(ctx, job.SessionID)
		if err != nil {
			return err
		}
	}

	// ---- Stage: gate (UC2 keep/forse/salta over all address targets) ----
	// Idempotent via per-company freshness reuse, so a retry re-gates only the tail.
	gatePayload := normalizeMAWebValidationPayload(MAWebValidationEnrichRequest{Limit: limit})
	// The gate MUST cover the WHOLE address surface. normalizeMAWebValidationPayload clamps
	// to the standalone endpoint's 100-cap (maWebValidationMaxLimit), but here total gate
	// cost is already bounded upstream by the surface cap, and any address target left
	// un-gated would fall through enrichAndScoreSurvivors as a recall-safe forse and pay
	// Advanced WITHOUT a verdict. So gate every persisted target — this is what lets the
	// surface cap scale past 100 (the gate batch ceiling must scale WITH the surface cap).
	gatePayload.Limit = len(detail.Targets)
	if err := s.validateMATargetsBatch(ctx, job.SessionID, version.ID, strategy, detail.Targets, gatePayload, job.CreatedBySubject, job.CreatedByEmail); err != nil {
		return err // gate is idempotent → safe to retry
	}
	detail, err = s.store.GetMASession(ctx, job.SessionID)
	if err != nil {
		return err
	}

	// ---- Stage: enrich_score (advanced €0.10 + scoring on survivors only) ----
	merged, scoredCount, stats, err := s.enrichAndScoreSurvivors(ctx, detail.Targets, strategy, job.SessionID, runID)
	if err != nil {
		_ = s.store.CompleteMAExecutionRun(ctx, runID, maRunStatusFailed, 0, maErrorCode(err))
		return nil
	}
	if err := s.store.ReplaceMATargets(ctx, job.SessionID, runID, merged); err != nil {
		_ = s.store.CompleteMAExecutionRun(ctx, runID, maRunStatusFailed, 0, "store_error")
		return nil
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_gated_enrich_completed",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": job.SessionID, "run_id": runID, "enriched": stats.Enriched, "reused": stats.Reused, "salta": stats.Skipped, "manual_review": stats.ManualReview, "unscored": stats.Unscored, "scored": scoredCount}),
	})

	// ---- Stage: ready ----
	if err := s.store.CompleteMAExecutionRun(ctx, runID, maRunStatusCompleted, scoredCount, ""); err != nil {
		return err
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_gated_search_completed",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": job.SessionID, "run_id": runID, "result_count": scoredCount, "target_count": len(merged)}),
	})
	return nil
}

// enrichAndScoreSurvivors is the isolated, idempotent enrich+score step. Given the
// session's persisted (address-level) targets, each carrying a gate final_action, it:
//   - keeps salta (reject) targets identity-only, never charged;
//   - keeps manual_review (domain-unresolved) targets identity-only, never auto-charged —
//     they await a manually associated domain before they can be processed;
//   - for keep/forse survivors not yet advanced, fetches IT-advanced by VAT (€0.10),
//     persists the payload immediately (MarkMATargetAdvancedEnriched) so a re-run skips
//     it, and re-parses financials; a per-company fetch failure is tolerated (that
//     company stays a forse, identity-only — recall-safe, never silently dropped);
//   - scores ONLY the advanced survivors together (scoring is set-relative, so ranking
//     the survivors is exactly the intent).
//
// It returns the full merged target set ready for ReplaceMATargets (survivors scored,
// salta + un-enriched forse identity-only), the scored count, and stats.
func (s *maService) enrichAndScoreSurvivors(ctx context.Context, targets []MATarget, strategy MAStrategySpec, sessionID, runID string) ([]MATarget, int, maGatedScoreStats, error) {
	pricing := s.loadPricing(ctx)
	scoringParams := maScoringParamsFromPricing(pricing)

	plan := planGatedEnrichment(targets)
	var toScore []MATarget // advanced survivors, ready to score
	var carried []MATarget // salta + manual_review + un-enriched forse, kept identity-only
	stats := maGatedScoreStats{Skipped: len(plan.Carry), ManualReview: len(plan.ManualReview), Reused: len(plan.Reuse)}

	// salta (reject): identity-only, never charged.
	for _, target := range plan.Carry {
		target.SessionID = sessionID
		target.RunID = runID
		target.EnrichmentLevel = maEnrichmentAddress
		carried = append(carried, target)
	}
	// manual_review (domain unresolved): identity-only, held for manual domain association.
	// Same spend safety as reject (never auto-charged), kept distinct only for telemetry
	// and so the UI can surface them as actionable (associate a domain → auto-process).
	for _, target := range plan.ManualReview {
		target.SessionID = sessionID
		target.RunID = runID
		target.EnrichmentLevel = maEnrichmentAddress
		carried = append(carried, target)
	}
	// survivors already advanced on a prior pass: re-scored for free, never re-charged.
	for _, target := range plan.Reuse {
		target.SessionID = sessionID
		target.RunID = runID
		toScore = append(toScore, target)
	}
	// keep/forse survivors not yet advanced: pay IT-advanced now, persisting each fetch
	// immediately so a re-run skips it (the enrichment_level guard).
	for _, target := range plan.ToEnrich {
		target.SessionID = sessionID
		target.RunID = runID
		enriched, err := s.enrichTargetAdvanced(ctx, target)
		if err != nil {
			// Tolerate: this survivor stays a forse (identity-only), never dropped.
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType: "ma_gated_enrich_target_failed",
				Status:    maTraceEventFailed,
				Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "company": target.CompanyName, "vat": target.VATCode}),
				Error:     err.Error(),
			})
			target.EnrichmentLevel = maEnrichmentAddress
			carried = append(carried, target)
			stats.Unscored++
			continue
		}
		if err := s.store.MarkMATargetAdvancedEnriched(ctx, target.ID, enriched.VendorPayload); err != nil {
			return nil, 0, stats, err
		}
		toScore = append(toScore, enriched)
		stats.Enriched++
	}

	scored := scoreMATargetsV2(toScore, strategy, scoringParams, s.now())
	merged := make([]MATarget, 0, len(scored)+len(carried))
	merged = append(merged, scored...)
	merged = append(merged, carried...)
	return merged, len(scored), stats, nil
}

// gatedEnrichPlan is the money-critical partition of a gated target set BEFORE any paid
// Advanced fetch: which survivors pay €0.10 now, which are reused for free, and which are
// never charged. Kept pure (no network/store) so the spend rules are unit-tested.
type gatedEnrichPlan struct {
	ToEnrich     []MATarget // keep/forse survivors not yet advanced → pay Advanced now
	Reuse        []MATarget // survivors already advanced → re-scored free, never re-charged
	Carry        []MATarget // scarta (reject) → identity-only, never charged
	ManualReview []MATarget // domain unresolved → held for manual domain, never auto-charged
}

// gatedRunStrategyType returns the strategy_type of the run that owns the session's
// current targets, or "" when the run isn't found. Used to decide whether persisted
// address targets can be reused for the requested strategy: a mismatch (or unknown run)
// forces a fresh surface fetch so a strategy switch isn't silently ignored.
func gatedRunStrategyType(runs []MAExecutionRun, runID string) string {
	if runID == "" {
		return ""
	}
	for _, run := range runs {
		if run.ID == runID {
			return run.StrategyType
		}
	}
	return ""
}

// planGatedEnrichment classifies each target by gate bucket and prior enrichment. The
// invariants it enforces are the whole point of the funnel: reject never pays Advanced;
// a domain-unresolved company is HELD for manual review (never auto-charged, awaiting a
// manually associated domain); an already-advanced survivor is never re-charged; a
// missing gate verdict is a forse (survivor), never dropped.
func planGatedEnrichment(targets []MATarget) gatedEnrichPlan {
	plan := gatedEnrichPlan{}
	for _, target := range targets {
		switch gatedTargetBucket(target) {
		case maGatedBucketReject:
			plan.Carry = append(plan.Carry, target)
		case maGatedBucketManualReview:
			plan.ManualReview = append(plan.ManualReview, target)
		default:
			if target.EnrichmentLevel == maEnrichmentAdvanced {
				plan.Reuse = append(plan.Reuse, target)
			} else {
				plan.ToEnrich = append(plan.ToEnrich, target)
			}
		}
	}
	return plan
}

// enrichTargetAdvanced fetches IT-advanced (€0.10) by VAT/tax code for one target and
// returns a fresh MATarget carrying the advanced financials + payload, with identity
// keys (ID/session/run) preserved from the address-stage row and enrichment level set
// to advanced.
func (s *maService) enrichTargetAdvanced(ctx context.Context, target MATarget) (MATarget, error) {
	if s.openapiit == nil {
		return MATarget{}, errMAOpenAPIITUnavailable
	}
	ident := strings.TrimSpace(target.VATCode)
	if ident == "" {
		ident = strings.TrimSpace(target.TaxCode)
	}
	if ident == "" {
		return MATarget{}, fmt.Errorf("no vat/tax code for %q", target.CompanyName)
	}
	resp, err := s.openapiit.Company().GetITAdvanced(ctx, ident)
	if err != nil {
		return MATarget{}, err
	}
	if len(resp.Data) == 0 {
		return MATarget{}, fmt.Errorf("empty advanced enrichment for %s", ident)
	}
	rows, err := json.Marshal(resp.Data)
	if err != nil {
		return MATarget{}, fmt.Errorf("marshal advanced dataset: %w", err)
	}
	parsed, err := parseMATargetsFromVendorData(rows)
	if err != nil {
		return MATarget{}, err
	}
	if len(parsed) == 0 {
		return MATarget{}, fmt.Errorf("no parsable advanced row for %s", ident)
	}
	enriched := parsed[0]
	// Preserve the ADDRESS-row identity keys (the exact fields maTargetDedupeKey derives
	// company_key from: VendorID→VATCode→TaxCode→CompanyName) plus the row IDs. The gate
	// keyed its ma_target_web_validation on the address company_key; if the advanced
	// payload carried a different/absent VendorID the key would drift, orphan the gate
	// verdict, and make a re-run treat the company as un-gated → re-charge. Financials,
	// ATECO and the payload that scoring reads come from the advanced fetch.
	enriched.ID = target.ID
	enriched.SessionID = target.SessionID
	enriched.RunID = target.RunID
	enriched.CompanyKey = target.CompanyKey
	enriched.VendorID = target.VendorID
	enriched.VATCode = target.VATCode
	enriched.TaxCode = target.TaxCode
	enriched.CompanyName = target.CompanyName
	enriched.EnrichmentLevel = maEnrichmentAdvanced
	// Carry the gate verdict onto the enriched row: scoring reads it for the
	// sector-mismatch rescue (a gate-confirmed survivor with an off-division
	// ATECO must not be re-hidden by the cruder 2-digit gate).
	enriched.WebValidation = target.WebValidation
	return enriched, nil
}

// gatedTargetBucket maps a target's gate verdict onto the gated funnel's buckets. It
// layers a manual_review carve-out on top of sectorActionToBucket: a company whose
// official domain could not be resolved (needs_domain_review / domain_unresolved) is NOT
// an auto-processed forse — it is HELD for manual domain association and never pays
// Advanced. This is the decision that stops the domain-unresolved false-positive spend
// (off-thesis companies leaking into Advanced only because their site wasn't found). A
// missing web-validation (gate did not decide at all) stays a recall-safe forse.
func gatedTargetBucket(target MATarget) string {
	if target.WebValidation == nil {
		return maGatedBucketForse
	}
	if target.WebValidation.FinalAction == "needs_domain_review" ||
		target.WebValidation.WebValidationState == "domain_unresolved" {
		return maGatedBucketManualReview
	}
	return sectorActionToBucket(target.WebValidation.FinalAction)
}

// enqueueAssociateDomain is the manual-review remedy. For a company held in manual_review
// (domain unresolved), the operator supplies an official domain; this validates synchronously
// (company exists, domain well-formed, session idle) then enqueues a DURABLE associate_domain
// job pre-leased to this instance's owner — so a foreign worker on the shared queue can't
// steal the paid re-gate+enrich, and a crash resumes (unlike the dev-only inline mode). The
// worker runs runAssociateDomainJob; the caller gets the session in 'running' and polls.
func (s *maService) enqueueAssociateDomain(ctx context.Context, sessionID, companyKey, domain, subject, email string) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	if s.openapiit == nil {
		return MASessionDetail{}, errMAOpenAPIITUnavailable
	}
	if s.brave == nil {
		return MASessionDetail{}, errMABraveUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	if companyKey == "" {
		return MASessionDetail{}, fmt.Errorf("%w: company key", errMAStrategyInvalid)
	}
	if _, ok := normalizeDomain(domain); !ok {
		return MASessionDetail{}, fmt.Errorf("%w: domain", errMAStrategyInvalid)
	}
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return MASessionDetail{}, err
	}
	if err := ensureMASessionOperational(detail.Session); err != nil {
		return MASessionDetail{}, err
	}
	if detail.Session.Status == maSessionStatusRunning {
		return MASessionDetail{}, fmt.Errorf("%w: session busy", errMAStrategyInvalid)
	}
	if detail.Strategy == nil {
		return MASessionDetail{}, fmt.Errorf("%w: strategy", errMAStrategyInvalid)
	}
	found := false
	for _, target := range detail.Targets {
		if normalizeMACompanyKey(target.CompanyKey) == companyKey {
			found = true
			break
		}
	}
	if !found {
		return MASessionDetail{}, fmt.Errorf("%w: target not found", errMAStrategyInvalid)
	}

	payload, err := json.Marshal(maAssociateDomainJobPayload{CompanyKey: companyKey, Domain: domain})
	if err != nil {
		return MASessionDetail{}, fmt.Errorf("marshal ma associate domain payload: %w", err)
	}
	_, created, err := s.store.EnqueueMAJob(ctx, maJobEnqueue{
		JobType:           maJobTypeAssociateDomain,
		SessionID:         sessionID,
		StrategyVersionID: detail.Strategy.ID,
		Subject:           subject,
		Email:             email,
		Payload:           payload,
		Owner:             s.owner,
	})
	if err != nil {
		return MASessionDetail{}, err
	}
	if created {
		if err := s.store.MarkMASessionExecuting(ctx, sessionID); err != nil {
			return MASessionDetail{}, err
		}
	}
	return s.getSession(ctx, sessionID)
}

// runAssociateDomainJob executes a queued associate_domain job off the request path, owning
// the operation trace so the paid Advanced fetch + LLM analyst tokens are audited like every
// other spend path. Mirrors runGatedSearchJob's worker signature.
func (s *maService) runAssociateDomainJob(ctx context.Context, job maJob) (string, error) {
	trace, err := s.startTrace(ctx, maTraceStart{
		Operation:        "ma_target_associate_domain",
		SessionID:        job.SessionID,
		CreatedBySubject: job.CreatedBySubject,
		CreatedByEmail:   job.CreatedByEmail,
		Request:          job.Payload,
	})
	if err != nil {
		return "", err
	}
	ctx = withMATrace(ctx, trace)
	if workErr := s.associateDomainWork(ctx, job); workErr != nil {
		_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusFailed, ErrorMessage: workErr.Error()})
		return trace.id, workErr
	}
	_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusSucceeded, HTTPStatus: http.StatusOK})
	return trace.id, nil
}

// associateDomainWork re-gates one company with the forced domain, then enriches+re-scores.
// Idempotent for the worker retry loop: buildMAWebValidationForDomain rebuilds the (cheap)
// gate verdict each attempt, and enrichAssociatedAndRescore only pays Advanced for a survivor
// not already advanced — so a retry after a mid-enrich crash reuses the persisted €0.10, never
// re-charges. Returning an error triggers a retry; final give-up releases the session (worker
// retryOrFail → releaseAssociateSession), so it never sticks in 'running'.
func (s *maService) associateDomainWork(ctx context.Context, job maJob) error {
	var payload maAssociateDomainJobPayload
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decode ma associate domain payload: %w", err)
		}
	}
	companyKey := normalizeMACompanyKey(payload.CompanyKey)
	if companyKey == "" {
		return fmt.Errorf("%w: company key", errMAStrategyInvalid)
	}
	detail, err := s.store.GetMASession(ctx, job.SessionID)
	if err != nil {
		return err
	}
	if err := ensureMASessionOperational(detail.Session); err != nil {
		return err
	}
	version, err := s.store.GetMAStrategyVersion(ctx, job.SessionID, job.StrategyVersionID)
	if err != nil {
		return err
	}
	// Canonicalize the pinned strategy once (deterministic, no spend) so the gate perimeter
	// and scoring use the same curated AtecoCandidates on every attempt.
	strategy, err := s.canonicalizeMAStrategyAteco(ctx, version.Strategy, nil, false)
	if err != nil {
		return err
	}
	var target *MATarget
	for i := range detail.Targets {
		if normalizeMACompanyKey(detail.Targets[i].CompanyKey) == companyKey {
			target = &detail.Targets[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("%w: target not found", errMAStrategyInvalid)
	}
	runID := target.RunID
	if runID == "" {
		return fmt.Errorf("%w: target has no run", errMAStrategyInvalid)
	}

	// Re-gate this one company with the forced domain.
	gatePayload := normalizeMAWebValidationPayload(MAWebValidationEnrichRequest{Limit: 1})
	inputHash := maWebValidationInputHash(*target, strategy, gatePayload)
	body, err := s.buildMAWebValidationForDomain(ctx, *target, strategy, inputHash, gatePayload, payload.Domain, job.CreatedBySubject, job.CreatedByEmail)
	if err != nil {
		return err
	}
	if _, err := s.upsertTargetWebValidation(ctx, job.SessionID, body, job.CreatedBySubject, job.CreatedByEmail); err != nil {
		return err
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_target_domain_associated",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": job.SessionID, "company_key": companyKey, "final_action": body.FinalDecision.FinalAction}),
	})

	// Reload so the target carries the fresh verdict, then enrich (this company only) +
	// re-score the survivor set (which completes the run → session back to 'completed').
	detail, err = s.store.GetMASession(ctx, job.SessionID)
	if err != nil {
		return err
	}
	return s.enrichAssociatedAndRescore(ctx, detail.Targets, strategy, job.SessionID, runID, companyKey)
}

// releaseAssociateSession flips a session left in 'running' by a failed/abandoned association
// back to 'completed', re-completing its run with the current scored count. The prior results
// are intact, so 'completed' — not 'failed' — is the honest state. Called by the worker on a
// final give-up; derives the run from the session's targets.
func (s *maService) releaseAssociateSession(ctx context.Context, sessionID string) {
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return
	}
	runID := ""
	scored := 0
	for _, target := range detail.Targets {
		if runID == "" {
			runID = target.RunID
		}
		if target.EnrichmentLevel == maEnrichmentAdvanced {
			scored++
		}
	}
	if runID == "" {
		return
	}
	_ = s.store.CompleteMAExecutionRun(ctx, runID, maRunStatusCompleted, scored, "")
}

// domainAssociationPlan partitions targets after a manual domain association. Only the
// associated company may pay Advanced this pass; existing advanced survivors are re-scored
// for free and everything else is carried untouched. Kept pure so the tight spend contract
// ("at most ONE company pays") is unit-tested.
type domainAssociationPlan struct {
	Enrich *MATarget  // the associated survivor to pay Advanced now (nil if none)
	Reuse  []MATarget // already-advanced survivors → re-scored for free, never re-charged
	Carry  []MATarget // non-survivors + un-enriched non-associated forse → identity-only, untouched
}

// planDomainAssociationEnrich classifies the target set for the association remedy. The
// invariant: the ONLY target that can newly pay Advanced is the just-associated company,
// and only if it now survives (keep/forse) and isn't already advanced. Every other target
// is either re-scored for free (already advanced) or left untouched — so a per-company
// action never fans out spend across lingering un-enriched survivors.
func planDomainAssociationEnrich(targets []MATarget, companyKey string) domainAssociationPlan {
	plan := domainAssociationPlan{}
	for _, target := range targets {
		associated := normalizeMACompanyKey(target.CompanyKey) == companyKey
		bucket := gatedTargetBucket(target)
		survivor := bucket == maGatedBucketKeep || bucket == maGatedBucketForse
		switch {
		case associated && survivor && target.EnrichmentLevel != maEnrichmentAdvanced:
			t := target
			plan.Enrich = &t
		case survivor && target.EnrichmentLevel == maEnrichmentAdvanced:
			plan.Reuse = append(plan.Reuse, target)
		default:
			plan.Carry = append(plan.Carry, target)
		}
	}
	return plan
}

// enrichAssociatedAndRescore executes the association plan: pay Advanced for the associated
// survivor (persisting immediately, tolerating a fetch failure), re-score the advanced
// survivor set together (set-relative), carry the rest identity-only, and complete the run.
func (s *maService) enrichAssociatedAndRescore(ctx context.Context, targets []MATarget, strategy MAStrategySpec, sessionID, runID, companyKey string) error {
	pricing := s.loadPricing(ctx)
	scoringParams := maScoringParamsFromPricing(pricing)
	plan := planDomainAssociationEnrich(targets, companyKey)

	var toScore, carried []MATarget
	for _, target := range plan.Reuse {
		target.SessionID = sessionID
		target.RunID = runID
		toScore = append(toScore, target)
	}
	for _, target := range plan.Carry {
		target.SessionID = sessionID
		target.RunID = runID
		target.EnrichmentLevel = maEnrichmentAddress
		carried = append(carried, target)
	}
	if plan.Enrich != nil {
		target := *plan.Enrich
		target.SessionID = sessionID
		target.RunID = runID
		enriched, err := s.enrichTargetAdvanced(ctx, target)
		if err != nil {
			// Tolerate: the associated company stays identity-only, never dropped.
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType: "ma_gated_enrich_target_failed",
				Status:    maTraceEventFailed,
				Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "company": target.CompanyName, "vat": target.VATCode}),
				Error:     err.Error(),
			})
			target.EnrichmentLevel = maEnrichmentAddress
			carried = append(carried, target)
		} else {
			if err := s.store.MarkMATargetAdvancedEnriched(ctx, target.ID, enriched.VendorPayload); err != nil {
				return err
			}
			toScore = append(toScore, enriched)
		}
	}

	scored := scoreMATargetsV2(toScore, strategy, scoringParams, s.now())
	merged := make([]MATarget, 0, len(scored)+len(carried))
	merged = append(merged, scored...)
	merged = append(merged, carried...)
	if err := s.store.ReplaceMATargets(ctx, sessionID, runID, merged); err != nil {
		return err
	}
	if err := s.store.CompleteMAExecutionRun(ctx, runID, maRunStatusCompleted, len(scored), ""); err != nil {
		return err
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_gated_enrich_completed",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "run_id": runID, "scored": len(scored), "reason": "domain_association"}),
	})
	return nil
}
