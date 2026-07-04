package binocolo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sciacco/mrsmith/internal/platform/llm"
	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/openapiit"
)

// maDeepWorkerStore is the persistence the deep-dive worker needs. *SQLStore
// satisfies it; kept narrow so the worker stays testable and decoupled.
type maDeepWorkerStore interface {
	ListMADeepJobs(ctx context.Context, limit int, workerID string) ([]maDeepJob, error)
	AcquireMADeepLease(ctx context.Context, companyKey, workerID string, leaseSeconds int) (bool, error)
	ClaimMADeepQueued(ctx context.Context, companyKey string) (bool, error)
	SetMADeepVendorRequest(ctx context.Context, companyKey, vendorRequestID string) error
	InsertMADeepVintage(ctx context.Context, companyKey, balanceSheetDate string, turnoverYear *int, payload json.RawMessage) error
	maDeepFamilyStore
	SaveMADeepReady(ctx context.Context, companyKey string, result maDeepResult) error
	MarkMADeepFailed(ctx context.Context, companyKey, errorCode string) error
	BumpMADeepAttempt(ctx context.Context, companyKey string) (int, error)
}

// maDeepWorker drives the async IT-full deep-dive as a DB-backed state machine:
// queued -> (CreateITFullRequest) running -> (CheckITRequest) ready | failed.
// State lives in ma_deep_analysis, so a process restart resumes naturally — the
// first tick picks up any queued/running rows. No in-memory queue to lose.
type maDeepWorker struct {
	id        string
	store     maDeepWorkerStore
	openapiit *openapiit.Client
	llmp      maLLMProvider
	pricing   func(ctx context.Context) maPricing
	interval  time.Duration
	batch     int
}

func newMADeepWorker(store maDeepWorkerStore, client *openapiit.Client, llmp maLLMProvider, pricing func(context.Context) maPricing) *maDeepWorker {
	worker := &maDeepWorker{
		id:        uuid.NewString(),
		store:     store,
		openapiit: client,
		llmp:      llmp,
		pricing:   pricing,
		interval:  5 * time.Second,
		batch:     16,
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
	jobs, err := w.store.ListMADeepJobs(ctx, w.batch, w.id)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo deep worker list jobs failed", "component", "binocolo", "operation", "ma_deep_worker", "error", err)
		return
	}
	for _, job := range jobs {
		w.process(ctx, job)
	}
}

func (w *maDeepWorker) process(ctx context.Context, job maDeepJob) {
	// Per-row lease: only the owning worker advances a row. Without it, a worker that
	// is NOT the claimer would see a just-claimed row as running-without-id and wrongly
	// fail it (missing_request_id), and several workers would regenerate the LLM brief
	// in parallel. This is what makes the worker correct with multiple instances on one
	// DB (shared staging across devs, k8s replicas, rolling-deploy overlap).
	owned, err := w.store.AcquireMADeepLease(ctx, job.CompanyKey, w.id, maDeepLeaseSeconds)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo deep worker lease failed", "component", "binocolo", "company_key", job.CompanyKey, "error", err)
		return
	}
	if !owned {
		return // another worker owns this row right now
	}
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
		// Archive the vintage FIRST: the payload is paid, and ma_deep_analysis keeps
		// only the latest one per company (a future refresh overwrites it). Best-effort:
		// a vintage failure (e.g. migration 091 not applied yet) must not block the
		// analysis; ON CONFLICT makes re-polls of the same filing a no-op.
		if date, year, ok := deepVintageKey(resp.Data); ok {
			if err := w.store.InsertMADeepVintage(ctx, job.CompanyKey, date, year, resp.Data); err != nil {
				logging.FromContext(ctx).Warn("binocolo deep worker vintage insert failed", "component", "binocolo", "company_key", job.CompanyKey, "error", err)
			}
		} else {
			logging.FromContext(ctx).Warn("binocolo deep worker vintage key unreadable", "component", "binocolo", "company_key", job.CompanyKey)
		}
		pricing := w.pricing(ctx)
		// Percorso condiviso con recompute (Fase 3): lettura CEE, suggerimento
		// famiglia (mai sovrascrive la ratifica), soglie per famiglia, scorecard
		// con flag, multiplo famiglia→prefisso→TOTAL e valuation con caveat.
		scorecard, reading, family, ratified := computeMADeepScorecard(ctx, w.store, resp.Data, job.CompanyKey, job.VATCode, job.TaxCode, pricing)
		if scorecard == nil || (scorecard.Turnover == nil && scorecard.Ebitda == nil && !scorecardHasMetric(scorecard)) {
			// Paid call returned, but no KPI could be read — keep the payload but
			// flag it so a path mismatch is diagnosable on the first real run.
			logging.FromContext(ctx).Warn("binocolo deep worker empty scorecard", "component", "binocolo", "company_key", job.CompanyKey)
		}
		var valuation *MADeepValuation
		if scorecard != nil {
			valuation = resolveMADeepValuation(ctx, w.store, scorecard, reading, family, ratified, pricing)
		}
		result := maDeepResult{Payload: resp.Data, Scorecard: scorecard, Valuation: valuation, CostEUR: pricing.CostFull}
		if w.llmp != nil && scorecard != nil {
			brief, modelID, promptID, err := w.generateBrief(ctx, resp.Data, scorecard, valuation)
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

// briefLLMRetryPolicy controls how the async brief call survives transient
// provider errors (429/5xx). The brief is a paid, best-effort step: on failure
// the scorecard is saved WITHOUT a brief (a silent gap until manual regeneration),
// so an async job that can afford to wait should back off and retry rather than
// drop the brief on a passing rate-limit — which can hit any aggregator
// (OpenRouter, Fireworks) under load. Interactive callers keep the SDK's own
// 2-retry default; this second, longer loop is only for the batch worker.
type briefLLMRetryPolicy struct {
	maxRetries  int
	backoffBase time.Duration
	backoffMax  time.Duration
}

const (
	briefLLMMaxRetriesDefault  = 4
	briefLLMBackoffBaseDefault = 2 * time.Second
	briefLLMBackoffMaxDefault  = 30 * time.Second
)

// briefLLMRetryPolicyFromEnv reads the (optional) ops tunables; zero-config uses
// the defaults. BINOCOLO_BRIEF_LLM_MAX_RETRIES=0 disables the outer loop.
func briefLLMRetryPolicyFromEnv() briefLLMRetryPolicy {
	p := briefLLMRetryPolicy{
		maxRetries:  briefLLMMaxRetriesDefault,
		backoffBase: briefLLMBackoffBaseDefault,
		backoffMax:  briefLLMBackoffMaxDefault,
	}
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("BINOCOLO_BRIEF_LLM_MAX_RETRIES"))); err == nil && v >= 0 {
		p.maxRetries = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("BINOCOLO_BRIEF_LLM_BACKOFF_MS"))); err == nil && v > 0 {
		p.backoffBase = time.Duration(v) * time.Millisecond
	}
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("BINOCOLO_BRIEF_LLM_BACKOFF_MAX_MS"))); err == nil && v > 0 {
		p.backoffMax = time.Duration(v) * time.Millisecond
	}
	return p
}

// chatWithBriefRetry runs the brief chat call, retrying on transient provider
// errors (429/5xx) with exponential backoff. Non-retryable errors and context
// cancellation return immediately. The SDK already retries twice at the HTTP
// layer (honoring Retry-After); this adds a longer, batch-appropriate outer loop
// so a paid async brief is not silently dropped on a passing rate-limit.
func chatWithBriefRetry(ctx context.Context, client maAIClient, req llm.ChatRequest) (llm.ChatResponse, error) {
	policy := briefLLMRetryPolicyFromEnv()
	var resp llm.ChatResponse
	var err error
	for attempt := 0; ; attempt++ {
		resp, err = client.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		var apiErr *llm.APIError
		if attempt >= policy.maxRetries || !errors.As(err, &apiErr) || !apiErr.Retryable() {
			return resp, err
		}
		backoff := policy.backoffBase << attempt
		if backoff <= 0 || backoff > policy.backoffMax {
			backoff = policy.backoffMax
		}
		logging.FromContext(ctx).Warn("binocolo deep brief llm retry",
			"component", "binocolo", "operation", "ma_deep_brief",
			"attempt", attempt+1, "status", apiErr.StatusCode, "backoff_ms", backoff.Milliseconds())
		select {
		case <-ctx.Done():
			return resp, ctx.Err()
		case <-time.After(backoff):
		}
	}
}

// generateBrief asks the LLM (scope ma_deep_brief) to narrate the already-computed
// scorecard + valuation. The model receives only the computed numbers and must not
// invent any (thesis-neutral, since the deep analysis is cached globally per company).
func (w *maDeepWorker) generateBrief(ctx context.Context, rawPayload json.RawMessage, scorecard *MADeepScorecard, valuation *MADeepValuation) (*MADeepBrief, string, string, error) {
	model, err := w.llmp.ResolveModel(ctx, maModelScopeDeepBrief, "")
	if err != nil {
		return nil, "", "", err
	}
	prompt, err := w.llmp.ResolvePrompt(ctx, maModelScopeDeepBrief, "")
	if err != nil {
		return nil, "", "", err
	}
	brief, err := buildMADeepBriefLLM(ctx, w.llmp, model, prompt, rawPayload, scorecard, valuation)
	if err != nil {
		return nil, "", "", err
	}
	return brief, model.ID, prompt.ID, nil
}

// buildMADeepBriefLLM runs the rich-brief LLM call for a single company. Shared by the
// deep-dive worker (fresh analyses) and service-level brief regeneration (rolling out a
// new prompt to already-cached companies). Numbers come from scorecard/valuation; the
// curated raw payload supplies qualitative facts. No IT-full call.
func buildMADeepBriefLLM(ctx context.Context, llmp maLLMProvider, model llm.Model, prompt llm.Prompt, rawPayload json.RawMessage, scorecard *MADeepScorecard, valuation *MADeepValuation) (*MADeepBrief, error) {
	client, err := llmp.ClientForModel(ctx, model)
	if err != nil {
		return nil, err
	}
	briefInput := map[string]any{"scorecard": scorecard, "valuation": valuation}
	if company := curateITFullForBrief(rawPayload); company != nil {
		briefInput["company"] = company
	}
	input, err := json.Marshal(briefInput)
	if err != nil {
		return nil, err
	}
	// Sampling params are dynamic, from the model's DB config; the scope default
	// max_tokens applies only when the config omits it. Reasoning models spend part
	// of this budget on hidden reasoning: v4.2 constrains output length, so 8000 is
	// cheap headroom against rare reasoning spikes/truncated JSON on dense cases.
	reqParams := model.RawParams()
	if _, ok := reqParams["max_tokens"]; !ok {
		reqParams["max_tokens"] = 8000
	}
	chatReq := llm.ChatRequest{
		Model:          model.Model,
		Params:         reqParams,
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
		Messages: []llm.Message{
			{Role: "system", Content: prompt.Prompt},
			{Role: "user", Content: string(input)},
		},
	}
	resp, chatErr := client.Chat(ctx, chatReq)
	// Best-effort audit: this runs per-row in a batch, so a failed audit must not
	// abort brief generation. request is the exact wire body (BuildRequestBody),
	// so dynamic params are recorded faithfully; FKs live in their own columns.
	usageRaw, _ := json.Marshal(resp.Usage)
	requestBody, _ := llm.BuildRequestBody(chatReq)
	requestRaw, _ := json.Marshal(requestBody)
	audit := llm.CallAudit{
		App:        maApp,
		Scope:      maModelScopeDeepBrief,
		ProviderID: model.ProviderID,
		ModelID:    model.ID,
		PromptID:   prompt.ID,
		Model:      model.Model,
		Request:    requestRaw,
		Usage:      usageRaw,
	}
	if chatErr != nil {
		audit.Status = "failed"
		audit.ErrorMessage = chatErr.Error()
	} else if respRaw, mErr := json.Marshal(map[string]any{"content": resp.Content}); mErr == nil {
		audit.Response = respRaw
	}
	_ = llmp.RecordAudit(ctx, audit)
	if chatErr != nil {
		return nil, chatErr
	}
	return parseMADeepBrief(resp.Content)
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
	brief.Verdict = normalizeNumbers(cleanText(brief.Verdict, 600))
	brief.BusinessProfile = normalizeNumbers(cleanText(brief.BusinessProfile, 800))
	brief.FinancialReading = normalizeNumbers(cleanText(brief.FinancialReading, 1000))
	// Chiave legacy pre-v3 ("thesisReading" era un nome bugiardo: è la lettura
	// finanziaria): le righe cached la riversano nel campo nuovo.
	brief.ThesisReading = normalizeNumbers(cleanText(brief.ThesisReading, 1000))
	if brief.FinancialReading == "" && brief.ThesisReading != "" {
		brief.FinancialReading = brief.ThesisReading
		brief.ThesisReading = ""
	}
	brief.ValuationRationale = normalizeNumbers(cleanText(brief.ValuationRationale, 800))
	brief.RAG = strings.ToLower(strings.TrimSpace(brief.RAG))
	if len(brief.Strengths) > 6 {
		brief.Strengths = brief.Strengths[:6]
	}
	for i := range brief.Strengths {
		brief.Strengths[i] = normalizeNumbers(cleanText(brief.Strengths[i], 300))
	}
	if len(brief.DDQuestions) > 8 {
		brief.DDQuestions = brief.DDQuestions[:8]
	}
	for i := range brief.DDQuestions {
		brief.DDQuestions[i] = normalizeNumbers(cleanText(brief.DDQuestions[i], 300))
	}
	if len(brief.RedFlags) > 8 {
		brief.RedFlags = brief.RedFlags[:8]
	}
	for i := range brief.RedFlags {
		brief.RedFlags[i].Severity = strings.ToLower(strings.TrimSpace(brief.RedFlags[i].Severity))
		brief.RedFlags[i].Category = strings.ToLower(strings.TrimSpace(brief.RedFlags[i].Category))
		brief.RedFlags[i].Claim = normalizeNumbers(cleanText(brief.RedFlags[i].Claim, 300))
		brief.RedFlags[i].DDQuestion = normalizeNumbers(cleanText(brief.RedFlags[i].DDQuestion, 300))
	}
	return &brief, nil
}

// curateITFullForBrief strips the opaque IIC-coded balance-sheet arrays from the raw
// payload, leaving the human-readable facts (registry, group, governance, public
// tenders, sector, employees, ratios) for the LLM to narrate. Returns nil on bad input.
func curateITFullForBrief(payload json.RawMessage) any {
	if len(payload) == 0 {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return nil
	}
	if inner, ok := obj["data"].(map[string]any); ok {
		obj = inner
	}
	for _, key := range []string{
		"debts", "credits", "netWorth", "inventory", "financialAssets",
		"financialFixedAssets", "tangibleFixedAssets", "intangibleFixedAssets",
		"assetsAggregateValues", "liabilitiesAggregateValues", "riskProvisions",
		"productionCosts", "productionValue", "annualResult", "revenuesFinancialCharges",
		"adjustments", "creditsToShareholders", "incomeStatementAggregateValues",
		"cashEquivalents",
	} {
		delete(obj, key)
	}
	return obj
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
