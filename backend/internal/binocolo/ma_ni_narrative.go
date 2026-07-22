package binocolo

// Nota-integrativa NARRATIVE reading (issue #80, Fase N1). A SEPARATE LLM analysis, distinct
// from the proposals reading (ma_ni_reading.go): it extracts the non-numeric CONTEXT facts —
// attribuzioni di risultato, rischi, piani, tratti di profilo — that are invisible to the
// prospetti and to the PFN bridge, WITHOUT ever proposing an adjustment or touching a number.
// It runs in the ingest's ni_reading stage AFTER the proposals reading (same tick, best-effort:
// a narrative failure never changes the filing's ready|degraded outcome), and can be regenerated
// on demand via a durable filing_narrative ma_job that works only on the persisted OCR pages
// (never re-OCR, never a vendor call — LLM only). Every observation is mechanically grounded —
// its verbatim quote must exist on the cited page (±1, reassembled tables inlined) via the SAME
// citation mechanic the proposals reading uses (maNIQuoteGrounding) — so a hallucinated fact is
// discarded with telemetry, never stored. Observations are IMMUTABLE and there are no analyst
// decisions (read-only facts). The store methods live in ma_ni_narrative_store.go.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sciacco/mrsmith/internal/platform/llm"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const (
	// maModelScopeNINarrative is the registry (app, scope) the narrative reading resolves its
	// model + prompt against (mig 118 seeds it). DISTINCT from ma_ni_reading — the narrative is a
	// separate, cheaper analysis and MUST NOT perturb the proposals prompt/model.
	maModelScopeNINarrative = "ma_ni_narrative"

	// maNINarrativeSectionCharBudget is the char ceiling for accreting narrative sections into a
	// single LLM call. GENEROUS relative to the proposals reading (12k): the narrative reads the
	// whole discursive story (NI + relazione sulla gestione) for context, so a larger window keeps
	// related paragraphs together in one call and produces fewer, richer calls. A lone section
	// larger than the budget is still its own chunk (chunkMANISections never splits a section, so
	// a citation is never severed across calls).
	maNINarrativeSectionCharBudget = 24000

	// maNINarrativeMaxTokens is the generous per-call output ceiling. When a chunk's completion
	// hits it (usage.CompletionTokens >= this), the run is flagged truncated (telemetry): the
	// narrative may be incomplete, surfaced on the run so the UI/operator can regenerate.
	maNINarrativeMaxTokens = 8000

	// Narrative observation types (mig 118 CHECK is canonical). Copy in the UI is Italian B2B.
	maNITipoAttribuzione = "attribuzione"
	maNITipoRischio      = "rischio"
	maNITipoPiano        = "piano"
	maNITipoProfilo      = "profilo"
)

// maNIValidTipo is the closed vocabulary of narrative observation types; anything else is
// discarded (an LLM out-of-vocabulary tipo is never stored).
var maNIValidTipo = map[string]bool{
	maNITipoAttribuzione: true,
	maNITipoRischio:      true,
	maNITipoPiano:        true,
	maNITipoProfilo:      true,
}

// maNIObservationLLM is one observation exactly as the mig-118 prompt emits it (camelCase).
// Parsed defensively, then validated (tipo vocabulary + citation grounding) into a stored
// maNIObservation. NO amount fields — the narrative is qualitative by design.
type maNIObservationLLM struct {
	Tipo   string `json:"tipo"`
	Claim  string `json:"claim"`
	Quote  string `json:"quote"`
	PageNo *int   `json:"pageNo"`
}

// runMANINarrative performs one narrative reading pass over the filing's ACTIVE processing run
// and persists the observations on a fresh narrative run. Shared by BOTH the ingest stage (best-
// effort, after the proposals reading) and the filing_narrative regeneration job. Contract:
//   - resolve model/prompt FIRST; a missing binding (mig 118 not applied) returns llmConfigError
//     WITHOUT creating a run — so the failure is legible (POST → clear error) and leaves no
//     spurious run (the GET keeps showing "no narrative");
//   - only then open a NEW 'running' run (superseding the prior), read the NI + relazione sulla
//     gestione sections on the PERSISTED pages (never re-OCR), call the LLM once per section
//     chunk, mechanically validate + persist the observations, and finalize the run to 'ready';
//   - a chat/parse/store error is returned RAW with the run left 'running' — the caller decides
//     (both callers mark the run 'failed', regenerable): a narrative failure is terminal, never a
//     silent success and never an effect on the filing outcome.
func (s *maService) runMANINarrative(ctx context.Context, filing *maFiling, requestID string) error {
	if s.llmp == nil || s.filing == nil {
		return errMAStoreUnavailable
	}
	processingRunID := strings.TrimSpace(filing.ActiveProcessingRunID)
	if processingRunID == "" {
		return fmt.Errorf("%w: processing run assente per la lettura narrativa", errMAStrategyInvalid)
	}
	// Registry model/prompt (scope ma_ni_narrative): resolve BEFORE creating any run so a missing
	// binding fails clean (no orphan run). Their uuids are the run's soft-refs (contract QA-F1).
	model, err := s.llmp.ResolveModel(ctx, maModelScopeNINarrative, "")
	if err != nil {
		return llmConfigError(err)
	}
	prompt, err := s.llmp.ResolvePrompt(ctx, maModelScopeNINarrative, "")
	if err != nil {
		return llmConfigError(err)
	}
	client, err := s.llmp.ClientForModel(ctx, model)
	if err != nil {
		return err
	}

	pages, err := s.filing.GetMAFilingPages(ctx, processingRunID)
	if err != nil {
		return err
	}
	sections := sectionMAFilingNI(pages) // NI + relazione sulla gestione (the post-prospetti narrative)

	// New 'running' run (supersedes the prior non-superseded run) — the observable «in corso».
	runID, err := s.filing.CreateMANINarrativeRun(ctx, filing.ID, processingRunID, prompt.ID, model.ID, requestID)
	if err != nil {
		return err
	}
	s.filingIngestTrace(ctx, "narrative_run", maFilingSysInternal, maTraceEventSucceeded, map[string]any{"filing_id": filing.ID, "run_id": runID, "sections": len(sections)}, "")

	company := map[string]any{}
	if filing.VATClean != "" {
		company["vat"] = filing.VATClean
	}
	if filing.TaxClean != "" {
		company["tax"] = filing.TaxClean
	}
	if filing.ClosingDate != nil {
		company["baselineExercise"] = maDateValue(*filing.ClosingDate)
	}

	// One LLM call per chunk of narrative sections; OR the per-chunk truncation flags.
	var llmObs []maNIObservationLLM
	truncated := false
	for _, chunk := range chunkMANISections(sections, maNINarrativeSectionCharBudget) {
		chunkObs, chunkTruncated, cerr := s.readMANINarrativeChunk(ctx, model, prompt, client, requestID, filing.ID, runID, company, chunk)
		if cerr != nil {
			return cerr // raw LLM/parse error; run left 'running', caller marks it 'failed' (regenerable)
		}
		llmObs = append(llmObs, chunkObs...)
		truncated = truncated || chunkTruncated
	}

	// Mechanical validation (tipo vocabulary + citation grounding) then immutable persist.
	idx := buildMANIPageIndex(pages)
	observations, discarded := s.buildMANIObservations(ctx, llmObs, idx)
	inserted, err := s.filing.InsertMANIObservations(ctx, runID, observations)
	if err != nil {
		return err
	}
	s.filingIngestTrace(ctx, "narrative_validated", maFilingSysInternal, maTraceEventInfo, map[string]any{
		"run_id": runID, "candidates": len(llmObs), "kept": len(observations),
		"inserted": inserted, "discarded": discarded, "truncated": truncated,
	}, "")

	if err := s.filing.SetMANINarrativeRunStatus(ctx, runID, maNINarrativeStatusReady, truncated); err != nil {
		return err
	}
	s.filingIngestTrace(ctx, "narrative_done", maFilingSysInternal, maTraceEventSucceeded, map[string]any{"filing_id": filing.ID, "run_id": runID, "observations": len(observations)}, "")
	return nil
}

// readMANINarrativeChunk performs one narrative-section-chunk LLM call: builds the
// {company, sections} input, calls chat in json_object mode with a generous max_tokens, records
// the audit (RequestID = the trace id; Context correlates filing/run/section, mirroring the OCR
// and proposals calls), and defensively parses the observations. Returns the parsed observations
// and whether the completion hit the token ceiling (truncation telemetry). A chat/parse error is
// returned raw.
func (s *maService) readMANINarrativeChunk(ctx context.Context, model llm.Model, prompt llm.Prompt, client maAIClient, requestID, filingID, runID string, company map[string]any, chunk []maNISection) ([]maNIObservationLLM, bool, error) {
	sections := make([]map[string]any, 0, len(chunk))
	labels := make([]string, 0, len(chunk))
	for _, sec := range chunk {
		sections = append(sections, map[string]any{
			"label": sec.Label,
			"pages": maNISectionPagesLabel(sec),
			"text":  sec.Markdown,
		})
		labels = append(labels, sec.Label)
	}
	payload, err := json.Marshal(map[string]any{"company": company, "sections": sections})
	if err != nil {
		return nil, false, err
	}
	reqParams := model.RawParams()
	if _, ok := reqParams["max_tokens"]; !ok {
		reqParams["max_tokens"] = maNINarrativeMaxTokens
	}
	chatReq := llm.ChatRequest{
		Model:          model.Model,
		Params:         reqParams,
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
		Messages: []llm.Message{
			{Role: "system", Content: prompt.Prompt},
			{Role: "user", Content: string(payload)},
		},
	}
	s.filingIngestTrace(ctx, "narrative_section", maFilingSysLLM, maTraceEventStarted, map[string]any{"run_id": runID, "sections": labels}, "")
	resp, chatErr := client.Chat(ctx, chatReq)
	usageRaw, _ := json.Marshal(resp.Usage)
	requestBody, _ := llm.BuildRequestBody(chatReq)
	requestRaw, _ := json.Marshal(requestBody)
	audit := llm.CallAudit{
		App:        maApp,
		Scope:      maModelScopeNINarrative,
		ProviderID: model.ProviderID,
		ModelID:    model.ID,
		PromptID:   prompt.ID,
		Model:      model.Model,
		Request:    requestRaw,
		Usage:      usageRaw,
		RequestID:  requestID,
		Context:    maTraceJSON(map[string]any{"filingId": filingID, "runId": runID, "section": strings.Join(labels, ", ")}),
	}
	if chatErr != nil {
		audit.Status = "failed"
		audit.ErrorMessage = chatErr.Error()
	} else if respRaw, mErr := json.Marshal(map[string]any{"content": resp.Content}); mErr == nil {
		audit.Response = respRaw
	}
	_ = s.llmp.RecordAudit(ctx, audit)
	if chatErr != nil {
		s.filingIngestTrace(ctx, "narrative_section", maFilingSysLLM, maTraceEventFailed, map[string]any{"run_id": runID}, chatErr.Error())
		return nil, false, chatErr
	}
	obs, err := parseMANIObservations(resp.Content)
	if err != nil {
		s.filingIngestTrace(ctx, "narrative_section", maFilingSysLLM, maTraceEventFailed, map[string]any{"run_id": runID}, err.Error())
		return nil, false, err
	}
	// Truncation heuristic: no finish_reason on the wire, so a completion at/over the ceiling is
	// treated as truncated. maxTokens comes from the (possibly DB-overridden) request params.
	maxTokens := maNINarrativeMaxTokens
	if v, ok := reqParams["max_tokens"]; ok {
		if n, ok := maAnyToInt(v); ok && n > 0 {
			maxTokens = n
		}
	}
	truncated := resp.Usage.CompletionTokens >= maxTokens
	if truncated {
		logging.FromContext(ctx).Warn("binocolo ni narrative chunk output truncated at token ceiling",
			"component", "binocolo", "operation", "ma_filing_narrative", "run_id", runID,
			"completion_tokens", resp.Usage.CompletionTokens, "max_tokens", maxTokens)
	}
	s.filingIngestTrace(ctx, "narrative_section", maFilingSysLLM, maTraceEventSucceeded, map[string]any{"run_id": runID, "observations": len(obs), "truncated": truncated}, "")
	return obs, truncated, nil
}

// buildMANIObservations turns raw LLM observations into the immutable stored shape, applying the
// mechanical guards in order: (1) tipo out-of-vocabulary ⇒ discard; (2) quote not grounded on the
// cited page (±1, via the shared maNIQuoteGrounding) ⇒ discard. Both discards log telemetry. The
// page number is corrected to the neighbor/deduced page when the quote anchored there. The
// fingerprint is computed on the final values and the batch is deduplicated by fingerprint
// (belt-and-suspenders with the run's UNIQUE index). Returns the kept observations + discard count.
func (s *maService) buildMANIObservations(ctx context.Context, llmObs []maNIObservationLLM, idx maNIPageIndex) ([]maNIObservation, int) {
	var out []maNIObservation
	discarded := 0
	log := logging.FromContext(ctx)
	for _, lo := range llmObs {
		tipo := strings.ToLower(strings.TrimSpace(lo.Tipo))
		if !maNIValidTipo[tipo] {
			discarded++
			log.Warn("binocolo ni observation discarded: tipo out of vocabulary",
				"component", "binocolo", "operation", "ma_filing_narrative", "tipo", tipo)
			continue
		}
		quoteNorm := maNormalizeQuote(lo.Quote)
		grounded, correctedPage, _ := maNIQuoteGrounding(quoteNorm, lo.PageNo, idx)
		if !grounded {
			discarded++
			log.Warn("binocolo ni observation discarded: quote not grounded on cited page",
				"component", "binocolo", "operation", "ma_filing_narrative", "page_no", lo.PageNo)
			continue
		}
		pageNo := lo.PageNo
		if correctedPage != nil {
			pageNo = correctedPage
		}
		out = append(out, maNIObservation{
			Tipo:        tipo,
			Fingerprint: maNIObservationFingerprint(tipo, lo.Claim, lo.Quote, pageNo),
			Claim:       cleanText(lo.Claim, 500),
			Quote:       cleanText(lo.Quote, 1000),
			PageNo:      pageNo,
		})
	}
	return dedupMANIObservationsByFingerprint(out), discarded
}

// dedupMANIObservationsByFingerprint keeps the first observation per fingerprint within one batch
// (the run's UNIQUE(run_id, fingerprint) is the durable guard; this keeps the inserted count
// deterministic even against an in-memory/fake store).
func dedupMANIObservationsByFingerprint(observations []maNIObservation) []maNIObservation {
	seen := make(map[string]bool, len(observations))
	out := make([]maNIObservation, 0, len(observations))
	for _, o := range observations {
		if seen[o.Fingerprint] {
			continue
		}
		seen[o.Fingerprint] = true
		out = append(out, o)
	}
	return out
}

// maNIObservationFingerprint is the deterministic dedup key of an observation:
// sha256(tipo | pageNo | sha256(claimNorm) | sha256(quoteNorm)), truncated to 32 hex chars.
// Stable across whitespace/case differences in claim/quote (both normalized before hashing).
func maNIObservationFingerprint(tipo, claim, quote string, pageNo *int) string {
	page := ""
	if pageNo != nil {
		page = strconv.Itoa(*pageNo)
	}
	claimHash := sha256.Sum256([]byte(maNormalizeQuote(claim)))
	quoteHash := sha256.Sum256([]byte(maNormalizeQuote(quote)))
	parts := strings.Join([]string{tipo, page, hex.EncodeToString(claimHash[:]), hex.EncodeToString(quoteHash[:])}, "|")
	sum := sha256.Sum256([]byte(parts))
	return hex.EncodeToString(sum[:])[:32]
}

// parseMANIObservations defensively decodes the prompt's {"observations":[...]} output (tolerating
// ```json fences). An empty body yields no observations (not an error — a truncated/empty
// generation surfaces zero facts rather than failing the run); malformed JSON is an error so the
// caller marks the run failed (regenerable).
func parseMANIObservations(content string) ([]maNIObservationLLM, error) {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return nil, nil
	}
	var decoded struct {
		Observations []maNIObservationLLM `json:"observations"`
	}
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil, fmt.Errorf("decode ni narrative observations: %w", err)
	}
	return decoded.Observations, nil
}

// runMANINarrativeBestEffort runs the narrative reading in the ingest's ni_reading stage AFTER the
// proposals reading (same tick). A failure NEVER changes the filing outcome (ready|degraded decided
// by the proposals): it logs telemetry and leaves the run 'failed' (regenerable). A resolve failure
// (mig 118 not applied) creates NO run, so the narrative is simply absent until the migration lands.
func (s *maService) runMANINarrativeBestEffort(ctx context.Context, filing *maFiling, requestID string) {
	if err := s.runMANINarrative(ctx, filing, requestID); err != nil {
		logging.FromContext(ctx).Warn("binocolo filing ni narrative failed (filing outcome unchanged)",
			"component", "binocolo", "operation", "ma_filing_narrative", "filing_id", filing.ID, "error", err)
		s.filingIngestTrace(ctx, "narrative", maFilingSysLLM, maTraceEventFailed, map[string]any{"filing_id": filing.ID}, err.Error())
		s.failMANINarrativeRunIfRunning(ctx, filing.ID)
	}
}

// failMANINarrativeRunIfRunning marks a filing's current narrative run 'failed' when it is still
// 'running' (regenerable). Best-effort: a lookup/update failure only logs. No-op when there is no
// run (e.g. a resolve failure created none).
func (s *maService) failMANINarrativeRunIfRunning(ctx context.Context, filingID string) {
	if s.filing == nil {
		return
	}
	run, err := s.filing.GetActiveMANINarrativeRun(ctx, filingID)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo filing ni narrative fail lookup failed",
			"component", "binocolo", "operation", "ma_filing_narrative", "filing_id", filingID, "error", err)
		return
	}
	if run == nil || run.Status != maNINarrativeStatusRunning {
		return
	}
	if err := s.filing.SetMANINarrativeRunStatus(ctx, run.ID, maNINarrativeStatusFailed, run.Truncated); err != nil {
		logging.FromContext(ctx).Warn("binocolo filing ni narrative fail update failed",
			"component", "binocolo", "operation", "ma_filing_narrative", "filing_id", filingID, "run_id", run.ID, "error", err)
	}
}

// ---------------------------------------------------------------------------
// filing_narrative job: async on-demand regeneration (issue #80).
// ---------------------------------------------------------------------------

// maFilingNarrativeJobPayload is the filing_narrative job's args. Enqueued by the regenerate
// endpoint; deduplicated on payload->>'filingId' (mig 118 inflight index) so a double-click
// enqueues at most one job. It works ONLY on the filing's persisted OCR pages — never re-OCR,
// never a vendor call.
type maFilingNarrativeJobPayload struct {
	FilingID string `json:"filingId"`
}

// runFilingNarrativeJob owns the operation trace for one tick of a filing_narrative job (one
// trace per run, completed here). The trace id becomes the narrative LLM calls' RequestID so the
// trace correlates to the llm_call_audit rows. Returns the trace id + the work error verbatim.
func (s *maService) runFilingNarrativeJob(ctx context.Context, job maJob) (string, error) {
	trace, err := s.startTrace(ctx, maTraceStart{
		Operation:        "ma_filing_narrative",
		CreatedBySubject: job.CreatedBySubject,
		CreatedByEmail:   job.CreatedByEmail,
		Request:          job.Payload,
	})
	if err != nil {
		return "", err
	}
	ctx = withMATrace(ctx, trace)
	workErr := s.filingNarrativeWork(ctx, job, trace.id)
	s.completeFilingTrace(ctx, workErr)
	return trace.id, workErr
}

// filingNarrativeWork regenerates the narrative for the payload's filing. Infrastructure errors
// (store unavailable, filing lookup) are returned to the worker for retry. The narrative
// GENERATION failure (LLM/parse) is swallowed here — logged, the run left 'failed' (regenerable),
// job completed — because a narrative failure is terminal-and-regenerable, never an auto-retry
// spend and never an effect on the filing. A filing that is no longer ready|degraded is a no-op.
func (s *maService) filingNarrativeWork(ctx context.Context, job maJob, requestID string) error {
	if s.filing == nil {
		return errMAStoreUnavailable
	}
	var payload maFilingNarrativeJobPayload
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decode ma filing narrative payload: %w", err)
		}
	}
	filingID := strings.TrimSpace(payload.FilingID)
	if filingID == "" {
		return fmt.Errorf("%w: filing id", errMAStrategyInvalid)
	}
	filing, err := s.filing.GetMAFiling(ctx, filingID)
	if err != nil {
		return err
	}
	if filing == nil {
		return fmt.Errorf("%w: filing %s not found", errMAStrategyInvalid, filingID)
	}
	// Defensive (the endpoint already gates): only a completed pipeline has persisted pages to read.
	if filing.Status != maFilingStatusReady && filing.Status != maFilingStatusDegraded {
		return nil
	}
	if err := s.runMANINarrative(ctx, filing, requestID); err != nil {
		logging.FromContext(ctx).Warn("binocolo filing ni narrative regeneration failed (regenerable)",
			"component", "binocolo", "operation", "ma_filing_narrative", "filing_id", filingID, "error", err)
		s.failMANINarrativeRunIfRunning(ctx, filingID)
	}
	return nil
}

// failMANINarrativeOnGiveUp is the worker's terminal cleanup for a filing_narrative infra give-up:
// mark any still-'running' narrative run 'failed' (regenerable). Reads the filing id from the job
// payload.
func (s *maService) failMANINarrativeOnGiveUp(ctx context.Context, job maJob) {
	if s.filing == nil {
		return
	}
	var payload maFilingNarrativeJobPayload
	if len(job.Payload) > 0 {
		_ = json.Unmarshal(job.Payload, &payload)
	}
	if filingID := strings.TrimSpace(payload.FilingID); filingID != "" {
		s.failMANINarrativeRunIfRunning(ctx, filingID)
	}
}

// maNINarrativeSchemaMissing reports whether err is the Postgres 42P01 (undefined_table) raised
// when the mig-118 narrative tables are not yet applied. The read-only GET tolerates it (status
// "absent") so opening the scheda never 500s before the migration lands — mirroring the ingest
// path, whose registry resolve guard returns BEFORE ever touching these tables.
func maNINarrativeSchemaMissing(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42P01"
}

// maAnyToInt best-effort reads an int from a dynamic JSON param value (a DB-configured max_tokens
// decodes as float64; a literal is int). ok=false for a non-numeric value.
func maAnyToInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}
