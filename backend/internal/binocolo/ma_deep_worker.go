package binocolo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/openapiit"
	"github.com/sciacco/mrsmith/internal/platform/openrouter"
)

// maDeepWorkerStore is the persistence the deep-dive worker needs. *SQLStore
// satisfies it; kept narrow so the worker stays testable and decoupled.
type maDeepWorkerStore interface {
	ListMADeepJobs(ctx context.Context, limit int) ([]maDeepJob, error)
	ClaimMADeepQueued(ctx context.Context, companyKey string) (bool, error)
	SetMADeepVendorRequest(ctx context.Context, companyKey, vendorRequestID string) error
	ResolveSectorMultiple(ctx context.Context, ateco string) (*sectorMultiple, error)
	ResolveMAModel(ctx context.Context, scope, modelID string) (maLLMModel, error)
	ResolveMAPrompt(ctx context.Context, scope, promptID string) (maLLMPrompt, error)
	SaveMADeepReady(ctx context.Context, companyKey string, result maDeepResult) error
	MarkMADeepFailed(ctx context.Context, companyKey, errorCode string) error
	BumpMADeepAttempt(ctx context.Context, companyKey string) (int, error)
}

// maDeepWorker drives the async IT-full deep-dive as a DB-backed state machine:
// queued -> (CreateITFullRequest) running -> (CheckITRequest) ready | failed.
// State lives in ma_deep_analysis, so a process restart resumes naturally — the
// first tick picks up any queued/running rows. No in-memory queue to lose.
type maDeepWorker struct {
	store     maDeepWorkerStore
	openapiit *openapiit.Client
	ai        maAIClient
	pricing   func(ctx context.Context) maPricing
	interval  time.Duration
	batch     int
}

func newMADeepWorker(store maDeepWorkerStore, client *openapiit.Client, ai *openrouter.Client, pricing func(context.Context) maPricing) *maDeepWorker {
	worker := &maDeepWorker{
		store:     store,
		openapiit: client,
		pricing:   pricing,
		interval:  5 * time.Second,
		batch:     16,
	}
	// Guard against a typed-nil interface: leave ai unset (nil) when no client,
	// so w.ai != nil correctly skips brief generation.
	if ai != nil {
		worker.ai = ai
	}
	return worker
}

func (w *maDeepWorker) run(ctx context.Context) {
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

func (w *maDeepWorker) tick(ctx context.Context) {
	jobs, err := w.store.ListMADeepJobs(ctx, w.batch)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo deep worker list jobs failed", "component", "binocolo", "operation", "ma_deep_worker", "error", err)
		return
	}
	for _, job := range jobs {
		w.process(ctx, job)
	}
}

func (w *maDeepWorker) process(ctx context.Context, job maDeepJob) {
	switch job.Status {
	case maDeepStatusQueued:
		ident := strings.TrimSpace(job.VATCode)
		if ident == "" {
			ident = strings.TrimSpace(job.TaxCode)
		}
		if ident == "" {
			_ = w.store.MarkMADeepFailed(ctx, job.CompanyKey, "missing_identifier")
			return
		}
		// Atomic claim BEFORE the paid POST: only the worker/replica that flips
		// queued->running proceeds, so CreateITFullRequest (the charge) runs once.
		claimed, err := w.store.ClaimMADeepQueued(ctx, job.CompanyKey)
		if err != nil {
			logging.FromContext(ctx).Warn("binocolo deep worker claim failed", "component", "binocolo", "company_key", job.CompanyKey, "error", err)
			return
		}
		if !claimed {
			return // another worker/replica claimed it first
		}
		resp, err := w.openapiit.Company().CreateITFullRequest(ctx, ident, openapiit.CompanyPostBody{})
		if err != nil {
			// Claimed (running) already, so do NOT re-POST (would re-charge): fail it.
			_ = w.store.MarkMADeepFailed(ctx, job.CompanyKey, "create_failed")
			return
		}
		requestID := strings.TrimSpace(resp.Data.ID)
		if requestID == "" {
			_ = w.store.MarkMADeepFailed(ctx, job.CompanyKey, "no_request_id")
			return
		}
		if err := w.store.SetMADeepVendorRequest(ctx, job.CompanyKey, requestID); err != nil {
			logging.FromContext(ctx).Warn("binocolo deep worker set request id failed", "component", "binocolo", "company_key", job.CompanyKey, "error", err)
		}
	case maDeepStatusRunning:
		if strings.TrimSpace(job.VendorRequestID) == "" {
			// No request id can ever appear (lost between a successful POST and its
			// persist): fail fast instead of burning the whole poll budget.
			_ = w.store.MarkMADeepFailed(ctx, job.CompanyKey, "missing_request_id")
			return
		}
		resp, err := w.openapiit.Company().CheckITRequest(ctx, job.VendorRequestID)
		if err != nil {
			w.retryOrFail(ctx, job, "check_failed")
			return
		}
		if !deepPayloadReady(resp.Data) {
			w.retryOrFail(ctx, job, "poll_timeout")
			return
		}
		scorecard := buildMADeepScorecard(resp.Data)
		if scorecard == nil || (scorecard.Turnover == nil && scorecard.Ebitda == nil && !scorecardHasMetric(scorecard)) {
			// Paid call returned, but no KPI could be read — keep the payload but
			// flag it so a path mismatch is diagnosable on the first real run.
			logging.FromContext(ctx).Warn("binocolo deep worker empty scorecard", "component", "binocolo", "company_key", job.CompanyKey)
		}
		pricing := w.pricing(ctx)
		var valuation *MADeepValuation
		if scorecard != nil {
			multiple, err := w.store.ResolveSectorMultiple(ctx, scorecard.AtecoCode)
			if err != nil {
				logging.FromContext(ctx).Warn("binocolo deep worker sector multiple failed", "component", "binocolo", "company_key", job.CompanyKey, "error", err)
			} else {
				valuation = buildMADeepValuation(scorecard, multiple, pricing)
			}
		}
		result := maDeepResult{Payload: resp.Data, Scorecard: scorecard, Valuation: valuation, CostEUR: pricing.CostFull}
		if w.ai != nil && scorecard != nil {
			brief, modelID, promptID, err := w.generateBrief(ctx, scorecard, valuation)
			if err != nil {
				// Brief is best-effort: a failure must not lose the paid scorecard.
				logging.FromContext(ctx).Warn("binocolo deep worker brief failed", "component", "binocolo", "company_key", job.CompanyKey, "error", err)
			} else {
				result.Brief = brief
				result.ModelID = modelID
				result.PromptID = promptID
			}
		}
		if err := w.store.SaveMADeepReady(ctx, job.CompanyKey, result); err != nil {
			logging.FromContext(ctx).Warn("binocolo deep worker save failed", "component", "binocolo", "company_key", job.CompanyKey, "error", err)
		}
	}
}

func scorecardHasMetric(scorecard *MADeepScorecard) bool {
	if scorecard == nil {
		return false
	}
	for _, metric := range scorecard.Metrics {
		if metric.Value != nil {
			return true
		}
	}
	return false
}

// generateBrief asks the LLM (scope ma_deep_brief) to narrate the already-computed
// scorecard + valuation. The model receives only the computed numbers and must not
// invent any (thesis-neutral, since the deep analysis is cached globally per company).
func (w *maDeepWorker) generateBrief(ctx context.Context, scorecard *MADeepScorecard, valuation *MADeepValuation) (*MADeepBrief, string, string, error) {
	model, err := w.store.ResolveMAModel(ctx, maModelScopeDeepBrief, "")
	if err != nil {
		return nil, "", "", err
	}
	prompt, err := w.store.ResolveMAPrompt(ctx, maModelScopeDeepBrief, "")
	if err != nil {
		return nil, "", "", err
	}
	payload, err := json.Marshal(map[string]any{"scorecard": scorecard, "valuation": valuation})
	if err != nil {
		return nil, "", "", err
	}
	resp, err := w.ai.Chat(ctx, openrouter.ChatRequest{
		Model:          model.Model,
		Temperature:    0,
		MaxTokens:      900,
		ResponseFormat: &openrouter.ResponseFormat{Type: "json_object"},
		Messages: []openrouter.Message{
			{Role: "system", Content: prompt.Prompt},
			{Role: "user", Content: string(payload)},
		},
	})
	if err != nil {
		return nil, "", "", err
	}
	brief, err := parseMADeepBrief(resp.Content)
	if err != nil {
		return nil, "", "", err
	}
	return brief, model.ID, prompt.ID, nil
}

func parseMADeepBrief(content string) (*MADeepBrief, error) {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return nil, errors.New("empty deep brief response")
	}
	var brief MADeepBrief
	if err := json.Unmarshal([]byte(trimmed), &brief); err != nil {
		return nil, fmt.Errorf("decode deep brief: %w", err)
	}
	brief.Verdict = cleanText(brief.Verdict, 600)
	brief.ThesisReading = cleanText(brief.ThesisReading, 800)
	brief.ThesisFit = cleanText(brief.ThesisFit, 200)
	brief.RAG = strings.ToLower(strings.TrimSpace(brief.RAG))
	if len(brief.RedFlags) > 5 {
		brief.RedFlags = brief.RedFlags[:5]
	}
	for i := range brief.RedFlags {
		brief.RedFlags[i].Severity = strings.ToLower(strings.TrimSpace(brief.RedFlags[i].Severity))
		brief.RedFlags[i].Claim = cleanText(brief.RedFlags[i].Claim, 300)
		brief.RedFlags[i].DDQuestion = cleanText(brief.RedFlags[i].DDQuestion, 300)
	}
	return &brief, nil
}

// retryOrFail bumps the attempt counter and marks the job failed once it exceeds
// the cap; otherwise the row stays queued/running and the next tick retries.
func (w *maDeepWorker) retryOrFail(ctx context.Context, job maDeepJob, code string) {
	attempts, err := w.store.BumpMADeepAttempt(ctx, job.CompanyKey)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo deep worker bump attempt failed", "component", "binocolo", "company_key", job.CompanyKey, "error", err)
		return
	}
	if attempts >= maDeepMaxAttempts {
		_ = w.store.MarkMADeepFailed(ctx, job.CompanyKey, code)
	}
}
