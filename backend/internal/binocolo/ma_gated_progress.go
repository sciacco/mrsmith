package binocolo

import (
	"context"
	"fmt"
)

func (s *maService) gatedProgress(ctx context.Context, sessionID string) (MAGatedProgressResponse, error) {
	if s.store == nil {
		return MAGatedProgressResponse{}, errMAStoreUnavailable
	}
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return MAGatedProgressResponse{}, err
	}

	run := latestMAGatedRun(detail.Runs)
	targets := targetsForRun(detail.Targets, run.ID)
	out := MAGatedProgressResponse{
		Stage:   "ready",
		Surface: MAGatedProgressSurface{Expected: run.EstimatedCount, Fetched: len(targets)},
		Gate:    MAGatedProgressGate{Total: len(targets)},
		Run: MAGatedProgressRunSummary{
			StartedAt:   run.StartedAt,
			CompletedAt: run.CompletedAt,
			ErrorCode:   run.ErrorCode,
		},
	}

	for _, target := range targets {
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
	for _, target := range targets {
		if target.EnrichmentLevel == maEnrichmentAdvanced {
			out.Enrich.Enriched++
		}
	}

	switch {
	case run.Status == maRunStatusFailed || detail.Session.Status == maSessionStatusFailed:
		out.Stage = "failed"
	case run.Status == maRunStatusCompleted || detail.Session.Status == maSessionStatusCompleted:
		out.Stage = "ready"
	case detail.Session.Status == maSessionStatusRunning && len(targets) == 0:
		out.Stage = "address"
	case detail.Session.Status == maSessionStatusRunning && out.Gate.Processed < out.Gate.Total:
		out.Stage = "gate"
	case detail.Session.Status == maSessionStatusRunning:
		out.Stage = "enrich"
	}
	return out, nil
}

func (s *maService) verificationQueue(ctx context.Context, sessionID string) (MAVerificationQueueResponse, error) {
	if s.store == nil {
		return MAVerificationQueueResponse{}, errMAStoreUnavailable
	}
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return MAVerificationQueueResponse{}, err
	}
	run := latestMAGatedRun(detail.Runs)
	targets := targetsForRun(detail.Targets, run.ID)
	running := detail.Session.Status == maSessionStatusRunning
	out := MAVerificationQueueResponse{Items: []MAVerificationQueueItem{}}
	for _, target := range targets {
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
