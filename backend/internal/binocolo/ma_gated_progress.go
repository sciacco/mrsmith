package binocolo

import (
	"context"
	"fmt"
)

func (s *maService) gatedProgress(ctx context.Context, sessionID string) (MAGatedProgressResponse, error) {
	if s.store == nil {
		return MAGatedProgressResponse{}, errMAStoreUnavailable
	}
	session, err := s.store.GetMASessionState(ctx, sessionID)
	if err != nil {
		return MAGatedProgressResponse{}, err
	}
	runs, err := s.store.ListMARuns(ctx, sessionID)
	if err != nil {
		return MAGatedProgressResponse{}, err
	}
	rows, err := s.store.ListMATargetRows(ctx, sessionID)
	if err != nil {
		return MAGatedProgressResponse{}, err
	}
	manualAddJob, err := s.store.LatestMAManualAddJob(ctx, sessionID)
	if err != nil {
		return MAGatedProgressResponse{}, err
	}

	run := latestMAGatedRun(runs)
	targets := targetRowsForRun(rows, run.ID)
	automaticTargets := automaticMATargetRows(targets)
	surfaceExpected := run.EstimatedCount
	if maManualOnlyProgressSurface(run, targets, automaticTargets, manualAddJob) {
		surfaceExpected = 0
	}
	out := MAGatedProgressResponse{
		Stage:   "ready",
		Surface: MAGatedProgressSurface{Expected: surfaceExpected, Fetched: len(automaticTargets)},
		Gate:    MAGatedProgressGate{Total: len(automaticTargets)},
		Run: MAGatedProgressRunSummary{
			StartedAt:   run.StartedAt,
			CompletedAt: run.CompletedAt,
			ErrorCode:   run.ErrorCode,
		},
		ManualAdd: manualAddJob,
	}

	for _, row := range automaticTargets {
		target := rowAsTarget(row)
		if target.WebValidation == nil {
			continue
		}
		// A stale validation still counts as processed: progress is derived from
		// durable target facts, not trace events or a separate stage ledger.
		out.Gate.Processed++
		switch gatedTargetBucket(target) {
		case maGatedBucketKeep:
			out.Gate.Buckets.Keep++
			out.Enrich.Survivors++
		case maGatedBucketForse:
			out.Gate.Buckets.Forse++
			out.Enrich.Survivors++
		case maGatedBucketReject:
			out.Gate.Buckets.Scarta++
		case maGatedBucketManualReview:
			out.Gate.Buckets.ManualReview++
		}
	}
	for _, row := range automaticTargets {
		if row.EnrichmentLevel == maEnrichmentAdvanced {
			out.Enrich.Enriched++
		}
	}

	switch {
	case run.Status == maRunStatusFailed || session.Status == maSessionStatusFailed:
		out.Stage = "failed"
	case run.Status == maRunStatusCompleted || session.Status == maSessionStatusCompleted:
		out.Stage = "ready"
	case session.Status == maSessionStatusRunning && len(automaticTargets) == 0:
		out.Stage = "address"
	case session.Status == maSessionStatusRunning && out.Gate.Processed < out.Gate.Total:
		out.Stage = "gate"
	case session.Status == maSessionStatusRunning:
		out.Stage = "enrich"
	}
	return out, nil
}

func (s *maService) verificationQueue(ctx context.Context, sessionID string) (MAVerificationQueueResponse, error) {
	if s.store == nil {
		return MAVerificationQueueResponse{}, errMAStoreUnavailable
	}
	session, err := s.store.GetMASessionState(ctx, sessionID)
	if err != nil {
		return MAVerificationQueueResponse{}, err
	}
	runs, err := s.store.ListMARuns(ctx, sessionID)
	if err != nil {
		return MAVerificationQueueResponse{}, err
	}
	rows, err := s.store.ListMATargetRows(ctx, sessionID)
	if err != nil {
		return MAVerificationQueueResponse{}, err
	}
	run := latestMAGatedRun(runs)
	targets := targetRowsForRun(rows, run.ID)
	running := session.Status == maSessionStatusRunning
	out := MAVerificationQueueResponse{Items: []MAVerificationQueueItem{}}
	for _, row := range targets {
		target := rowAsTarget(row)
		bucket := gatedTargetBucket(target)
		identityOnlySurvivor := target.MatchState == "" && (bucket == maGatedBucketKeep || bucket == maGatedBucketForse)
		if bucket != maGatedBucketManualReview && (running || !identityOnlySurvivor) {
			continue
		}
		kind, detailText, remedies := verificationQueueReason(target, bucket, identityOnlySurvivor)
		out.Items = append(out.Items, MAVerificationQueueItem{
			TargetID:    target.ID,
			CompanyKey:  target.CompanyKey,
			CompanyName: target.CompanyName,
			Province:    target.Province,
			Reason:      MAVerificationQueueReason{Kind: kind, Detail: detailText},
			Remedies:    remedies,
		})
	}
	return out, nil
}

func rowAsTarget(row MATargetRow) MATarget {
	target := MATarget{
		ID:              row.ID,
		RunID:           row.RunID,
		CompanyKey:      row.CompanyKey,
		CompanyName:     row.CompanyName,
		Origin:          row.Origin,
		VATCode:         row.VATCode,
		Province:        row.Province,
		Town:            row.Town,
		AtecoCode:       row.AtecoCode,
		Score:           row.Score,
		ScoreVersion:    row.ScoreVersion,
		MatchState:      row.MatchState,
		Confidence:      row.Confidence,
		Rating:          row.Rating,
		Flags:           row.Flags,
		EnrichmentLevel: row.EnrichmentLevel,
	}
	if row.OutsideRevenuePerEmployeeValue != nil {
		target.Evidence = append(target.Evidence, MATargetEvidence{
			Criterion: maPostFilterRevenuePerEmployeeMin,
			Status:    maEvidenceOutside,
			Value:     *row.OutsideRevenuePerEmployeeValue,
		})
	}
	if row.OutsideMaxShareholdersValue != nil {
		target.Evidence = append(target.Evidence, MATargetEvidence{
			Criterion: maPostFilterMaxShareholders,
			Status:    maEvidenceOutside,
			Value:     *row.OutsideMaxShareholdersValue,
		})
	}
	if row.WebValidation != nil {
		validation := MAWebValidation{
			WebValidationState: row.WebValidation.WebValidationState,
			FinalAction:        row.WebValidation.FinalAction,
			SelectedDomain:     row.WebValidation.SelectedDomain,
			FinalDecision: CandidateMatchFinalDecision{
				Reason: row.WebValidation.FinalDecision.Reason,
			},
		}
		if row.WebValidation.GroupSiteDomain != "" {
			validation.DomainResponse.GroupSiteHint = &MAGroupSiteHint{
				Domain:     row.WebValidation.GroupSiteDomain,
				Identifier: row.WebValidation.GroupSiteIdentifier,
			}
		}
		if row.WebValidation.CandidateCount > 0 {
			validation.DomainResponse.Candidates = make([]DomainResolutionCandidate, row.WebValidation.CandidateCount)
		}
		target.WebValidation = &validation
	}
	return target
}

func verificationQueueReason(target MATarget, bucket string, identityOnlySurvivor bool) (string, string, []string) {
	if target.WebValidation != nil && target.WebValidation.DomainResponse.GroupSiteHint != nil {
		hint := target.WebValidation.DomainResponse.GroupSiteHint
		return "group_site",
			fmt.Sprintf("Possibile sito di gruppo: %s (P.IVA di altra società %s)", hint.Domain, hint.Identifier),
			[]string{"confirm_group_site", "associate_domain", "no_website"}
	}
	if identityOnlySurvivor && (bucket == maGatedBucketKeep || bucket == maGatedBucketForse) {
		return "enrich_failed", "Oltre il gate, analisi non riuscita", []string{"retry_search"}
	}
	if target.WebValidation == nil || len(target.WebValidation.DomainResponse.Candidates) == 0 {
		return "no_candidates", "Nessun candidato web trovato", []string{"associate_domain", "no_website"}
	}
	return "identity_unconfirmed", "Identità non confermata sulle pagine lette", []string{"associate_domain", "no_website"}
}

func latestMAGatedRun(runs []MAExecutionRun) MAExecutionRun {
	if len(runs) == 0 {
		return MAExecutionRun{}
	}
	return runs[0]
}

func targetsForRun(targets []MATarget, runID string) []MATarget {
	if runID == "" {
		return targets
	}
	out := make([]MATarget, 0, len(targets))
	for _, target := range targets {
		if target.RunID == runID {
			out = append(out, target)
		}
	}
	return out
}

func targetRowsForRun(rows []MATargetRow, runID string) []MATargetRow {
	if runID == "" {
		return rows
	}
	out := make([]MATargetRow, 0, len(rows))
	for _, row := range rows {
		if row.RunID == runID {
			out = append(out, row)
		}
	}
	return out
}

func automaticMATargetRows(rows []MATargetRow) []MATargetRow {
	out := make([]MATargetRow, 0, len(rows))
	for _, row := range rows {
		if row.Origin == maTargetOriginManual {
			continue
		}
		out = append(out, row)
	}
	return out
}

func maManualOnlyProgressSurface(run MAExecutionRun, targets, automaticTargets []MATargetRow, manualAddJob *MAManualAddJobProgress) bool {
	if len(automaticTargets) > 0 {
		return false
	}
	if len(targets) > 0 {
		return true
	}
	if manualAddJob == nil {
		return false
	}
	if maManualAddJobActive(manualAddJob) {
		return true
	}
	return run.ID == "" || !manualAddJob.UpdatedAt.Before(run.StartedAt) || !manualAddJob.CreatedAt.Before(run.StartedAt)
}
