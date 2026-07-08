package binocolo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sciacco/mrsmith/internal/platform/openapiit"
)

// maManualAddJobPayload is the manual_add job args: a syntactically valid VAT/tax
// identifier and, optionally, an analyst-supplied official domain already normalized
// with the same routine used by associate-domain remediation.
type maManualAddJobPayload struct {
	VATCode string `json:"vat_code"`
	Domain  string `json:"domain,omitempty"`
}

// enqueueManualAdd validates the existing session and inserts a rollout-safe durable
// manual_add job. The ma_job partial unique index is (session_id, job_type), so manual
// inserts are intentionally serialized per session: a second in-flight manual add does
// not create another paid job.
func (s *maService) enqueueManualAdd(ctx context.Context, sessionID, vatCode, domain, subject, email string, lean bool) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	if s.openapiit == nil {
		return MASessionDetail{}, errMAOpenAPIITUnavailable
	}
	vatCode = normalizeMAVATOrTax(vatCode)
	if vatCode == "" {
		return MASessionDetail{}, fmt.Errorf("%w: vat", errMAStrategyInvalid)
	}
	domain = strings.TrimSpace(domain)
	if domain != "" {
		normalized, ok := normalizeDomain(domain)
		if !ok {
			return MASessionDetail{}, fmt.Errorf("%w: domain", errMAStrategyInvalid)
		}
		domain = normalized
	} else if s.brave == nil {
		return MASessionDetail{}, errMABraveUnavailable
	}

	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return MASessionDetail{}, err
	}
	if err := ensureMASessionOperational(detail.Session); err != nil {
		return MASessionDetail{}, err
	}
	if detail.Session.Status == maSessionStatusRunning {
		manualAddJob, err := s.store.LatestMAManualAddJob(ctx, sessionID)
		if err != nil {
			return MASessionDetail{}, err
		}
		if maManualAddJobActive(manualAddJob) {
			return MASessionDetail{}, errMAManualAddInFlight
		}
		return MASessionDetail{}, fmt.Errorf("%w: session busy", errMAStrategyInvalid)
	}
	if detail.Strategy == nil {
		return MASessionDetail{}, fmt.Errorf("%w: strategy", errMAStrategyInvalid)
	}
	if found, retryable := maManualAddExistingTargetRetryability(detail.Targets, vatCode, ""); found && !retryable {
		return MASessionDetail{}, errMATargetAlreadyPresent
	}

	payload, err := json.Marshal(maManualAddJobPayload{VATCode: vatCode, Domain: domain})
	if err != nil {
		return MASessionDetail{}, fmt.Errorf("marshal ma manual add payload: %w", err)
	}
	_, created, err := s.store.EnqueueMAJob(ctx, maJobEnqueue{
		JobType:           maJobTypeManualAdd,
		SessionID:         sessionID,
		StrategyVersionID: detail.Strategy.ID,
		Status:            maJobStatusPending,
		Subject:           subject,
		Email:             email,
		Payload:           payload,
		Owner:             s.owner,
	})
	if err != nil {
		return MASessionDetail{}, err
	}
	if !created {
		return MASessionDetail{}, errMAManualAddInFlight
	}
	if err := s.store.MarkMASessionExecuting(ctx, sessionID); err != nil {
		return MASessionDetail{}, err
	}
	return s.getSessionShape(ctx, sessionID, lean)
}

func (s *maService) runManualAddJob(ctx context.Context, job maJob) (string, error) {
	trace, err := s.startTrace(ctx, maTraceStart{
		Operation:        "ma_target_manual_add",
		SessionID:        job.SessionID,
		CreatedBySubject: job.CreatedBySubject,
		CreatedByEmail:   job.CreatedByEmail,
		Request:          job.Payload,
	})
	if err != nil {
		return "", err
	}
	ctx = withMATrace(ctx, trace)
	if workErr := s.manualAddWork(ctx, job); workErr != nil {
		_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusFailed, ErrorMessage: workErr.Error()})
		return trace.id, workErr
	}
	_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusSucceeded, HTTPStatus: http.StatusOK})
	return trace.id, nil
}

func (s *maService) manualAddWork(ctx context.Context, job maJob) error {
	var payload maManualAddJobPayload
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decode ma manual add payload: %w", err)
		}
	}
	vatCode := normalizeMAVATOrTax(payload.VATCode)
	if vatCode == "" {
		return fmt.Errorf("%w: vat", errMAStrategyInvalid)
	}
	domain := strings.TrimSpace(payload.Domain)
	if domain != "" {
		normalized, ok := normalizeDomain(domain)
		if !ok {
			return fmt.Errorf("%w: domain", errMAStrategyInvalid)
		}
		domain = normalized
	} else if s.brave == nil {
		return errMABraveUnavailable
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
	runID := maManualAddRunID(detail)
	if runID == "" {
		strategyType := maManualAddStrategyType(detail, version.Strategy)
		estimatedCount := estimateTotal(detail.Estimates, strategyType)
		if estimatedCount <= 0 {
			estimatedCount = 1
		}
		run, err := s.store.CreateMAExecutionRun(ctx, maExecutionRunCreate{
			SessionID:         job.SessionID,
			StrategyVersionID: version.ID,
			StrategyType:      strategyType,
			EstimatedCount:    estimatedCount,
		})
		if err != nil {
			return err
		}
		runID = run.ID
		_ = s.linkTrace(ctx, maTraceLink{SessionID: job.SessionID, StrategyVersionID: version.ID, ExecutionRunID: runID})
	}
	strategy, err := s.canonicalizeMAStrategyAteco(ctx, version.Strategy, nil, false)
	if err != nil {
		return err
	}

	var target MATarget
	if existing := maFindManualAddTarget(detail.Targets, vatCode, ""); existing != nil {
		target = *existing
	} else {
		target, err = s.fetchManualAddAdvancedTarget(ctx, vatCode, job.SessionID, runID)
		if err != nil {
			return err
		}
		companyKey := normalizeMACompanyKey(maTargetDedupeKey(target))
		if companyKey == "" {
			return fmt.Errorf("%w: company key", errMAStrategyInvalid)
		}
		target.CompanyKey = companyKey
		if existing := maFindManualAddTarget(detail.Targets, vatCode, companyKey); existing != nil {
			target = *existing
		} else if err := s.store.InsertMATarget(ctx, job.SessionID, runID, target); err != nil {
			if !errors.Is(err, errMATargetAlreadyPresent) {
				return err
			}
			detail, err = s.store.GetMASession(ctx, job.SessionID)
			if err != nil {
				return err
			}
			existing := maFindManualAddTarget(detail.Targets, vatCode, companyKey)
			if existing == nil {
				return errMATargetAlreadyPresent
			}
			target = *existing
		}
	}
	companyKey := normalizeMACompanyKey(target.CompanyKey)
	if companyKey == "" {
		companyKey = normalizeMACompanyKey(maTargetDedupeKey(target))
	}
	if companyKey == "" {
		return fmt.Errorf("%w: company key", errMAStrategyInvalid)
	}
	target.SessionID = job.SessionID
	if strings.TrimSpace(target.RunID) == "" {
		target.RunID = runID
	}

	gatePayload := normalizeMAWebValidationPayload(MAWebValidationEnrichRequest{Limit: 1, IncludeIdentifiers: true})
	inputHash := maWebValidationInputHash(target, strategy, gatePayload)
	validationFresh := reusableMAWebValidation(target.WebValidation, inputHash, inputHash, s.now())
	if !validationFresh && target.WebValidation != nil && target.WebValidation.PipelineVersion == maWebValidationPipelineVersion {
		validationFresh = maWebValidationFreshness(s.now(), target.WebValidation.StaleAfter, target.WebValidation.ExpiresAt) == maWebValidationFresh
	}
	if !validationFresh {
		var body MAWebValidationUpsertRequest
		if domain != "" {
			body, err = s.buildMAWebValidationForDomain(ctx, target, strategy, inputHash, gatePayload, domain, "dominio fornito dall'analista", job.CreatedBySubject, job.CreatedByEmail)
		} else {
			body, err = s.buildMAWebValidation(ctx, target, strategy, inputHash, gatePayload, job.CreatedBySubject, job.CreatedByEmail)
		}
		if err != nil {
			return err
		}
		if _, err := s.upsertTargetWebValidation(ctx, job.SessionID, body, job.CreatedBySubject, job.CreatedByEmail); err != nil {
			return err
		}
	}
	if domain != "" {
		s.registerCompanyDomain(ctx, target, domain, maDomainMethodManual, job.CreatedBySubject, job.CreatedByEmail)
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_target_manual_added",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": job.SessionID, "company_key": companyKey, "domain_supplied": domain != ""}),
	})

	detail, err = s.store.GetMASession(ctx, job.SessionID)
	if err != nil {
		return err
	}
	return s.rescoreManualAddTargets(ctx, detail.Targets, strategy, job.SessionID, runID)
}

func (s *maService) fetchManualAddAdvancedTarget(ctx context.Context, vatCode, sessionID, runID string) (MATarget, error) {
	if s.openapiit == nil {
		return MATarget{}, errMAOpenAPIITUnavailable
	}
	resp, err := s.openapiit.Company().GetITAdvanced(ctx, vatCode)
	if err != nil {
		if maOpenAPIITNotFound(err) {
			return MATarget{}, errMAVATNotFound
		}
		return MATarget{}, err
	}
	if len(resp.Data) == 0 {
		return MATarget{}, errMAVATNotFound
	}
	rows, err := json.Marshal(resp.Data)
	if err != nil {
		return MATarget{}, fmt.Errorf("marshal manual add advanced dataset: %w", err)
	}
	parsed, err := parseMATargetsFromVendorData(rows)
	if err != nil {
		return MATarget{}, err
	}
	if len(parsed) == 0 {
		return MATarget{}, errMAVATNotFound
	}
	target := parsed[0]
	target.ID = uuid.NewString()
	target.SessionID = sessionID
	target.RunID = runID
	target.Origin = maTargetOriginManual
	target.EnrichmentLevel = maEnrichmentAdvanced
	if strings.TrimSpace(target.VATCode) == "" {
		target.VATCode = vatCode
	}
	if strings.TrimSpace(target.TaxCode) == "" && len(vatCode) == 16 {
		target.TaxCode = vatCode
	}
	target.CompanyKey = normalizeMACompanyKey(maTargetDedupeKey(target))
	return target, nil
}

func (s *maService) rescoreManualAddTargets(ctx context.Context, targets []MATarget, strategy MAStrategySpec, sessionID, runID string) error {
	pricing := s.loadPricing(ctx)
	scoringParams := maScoringParamsFromPricing(pricing)
	toScore := make([]MATarget, 0, len(targets))
	carried := make([]MATarget, 0, len(targets))
	for _, target := range targets {
		target.SessionID = sessionID
		target.RunID = runID
		if target.EnrichmentLevel == maEnrichmentAdvanced {
			toScore = append(toScore, target)
		} else {
			carried = append(carried, target)
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
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "run_id": runID, "scored": len(scored), "reason": "manual_add"}),
	})
	return nil
}

func maManualAddRunID(detail MASessionDetail) string {
	for _, target := range detail.Targets {
		if strings.TrimSpace(target.RunID) != "" {
			return target.RunID
		}
	}
	for _, run := range detail.Runs {
		if strings.TrimSpace(run.ID) != "" {
			return run.ID
		}
	}
	return ""
}

func maManualAddStrategyType(detail MASessionDetail, strategy MAStrategySpec) string {
	if value := normalizeMAStrategyType(detail.Session.SelectedStrategy); value != "" {
		return value
	}
	if value := normalizeMAStrategyType(strategy.SelectedStrategy); value != "" {
		return value
	}
	if value := chooseSelectedStrategyFromEstimates(detail.Estimates, len(strategy.AtecoQueryCandidates) > 0); value != "" {
		return value
	}
	if len(strategy.AtecoQueryCandidates) > 0 {
		return maStrategyTypeATECO
	}
	return maStrategyTypeExpanded
}

func maManualAddJobActive(job *MAManualAddJobProgress) bool {
	if job == nil {
		return false
	}
	switch job.Status {
	case maJobStatusQueued, maJobStatusRunning, maJobStatusPending, maJobStatusProcessing:
		return true
	default:
		return false
	}
}

func maManualAddExistingTargetRetryability(targets []MATarget, vatOrTax, companyKey string) (bool, bool) {
	probe := MATarget{VATCode: normalizeMAVATOrTax(vatOrTax), TaxCode: normalizeMAVATOrTax(vatOrTax), CompanyKey: normalizeMACompanyKey(companyKey)}
	found := false
	retryable := false
	for index := range targets {
		if !maManualAddSameTarget(targets[index], probe) {
			continue
		}
		found = true
		if !maManualAddRetryableArtifact(targets[index]) {
			return true, false
		}
		retryable = true
	}
	return found, retryable
}

func maManualAddRetryableArtifact(target MATarget) bool {
	if target.Origin != maTargetOriginManual {
		return false
	}
	return target.WebValidation == nil || strings.TrimSpace(target.MatchState) == ""
}

func maFindManualAddTarget(targets []MATarget, vatOrTax, companyKey string) *MATarget {
	probe := MATarget{VATCode: normalizeMAVATOrTax(vatOrTax), TaxCode: normalizeMAVATOrTax(vatOrTax), CompanyKey: normalizeMACompanyKey(companyKey)}
	for index := range targets {
		if maManualAddSameTarget(targets[index], probe) {
			return &targets[index]
		}
	}
	return nil
}

func maManualAddSameTarget(existing, candidate MATarget) bool {
	existingKeys := maManualAddDedupeKeys(existing)
	candidateKeys := maManualAddDedupeKeys(candidate)
	for key := range existingKeys {
		if _, ok := candidateKeys[key]; ok {
			return true
		}
	}
	return false
}

func maManualAddDedupeKeys(target MATarget) map[string]struct{} {
	out := map[string]struct{}{}
	for _, value := range []string{target.CompanyKey, maTargetDedupeKey(target), target.VendorID, target.VATCode, target.TaxCode} {
		value = normalizeMACompanyKey(value)
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}

func maOpenAPIITNotFound(err error) bool {
	var apiErr *openapiit.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}
