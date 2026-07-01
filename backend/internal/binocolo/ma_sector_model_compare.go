package binocolo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/sciacco/mrsmith/internal/platform/llm"
)

// maSectorCompareConcurrency bounds the parallel analyst calls in the model-comparison
// replay. Higher than the web-validation throttle (analyst-only, no Brave/scraper to
// saturate) so a per-session run completes under the 60s HTTP WriteTimeout.
const maSectorCompareConcurrency = 20

// UC2 model comparison harness. The whole UC2 pipeline (embedder + reranker + analyst)
// currently runs on a single provider (Fireworks). To pick a backup analyst on a
// different provider WITHOUT re-running the expensive web pipeline, this replays the
// analyst step over already-persisted, human-labeled sessions: same prompt + same
// reconstructed input, only the model varies. Each model is scored against the ground
// truth (keep/forse/scarta) so the operator can compare accuracy, the recall-safety
// metric (keepLeak = true-keep terminally rejected), token cost, and latency side by side.
//
// Fidelity caveat: the persisted summary keeps the distilled company description + the
// matched concepts, but NOT the raw web snippets (those live only in the write-only
// trace). So the replayed analyst sees a slightly thinner input than production
// (description + concepts + perimeter, no snippets). The comparison is still apples-to-
// apples — every model gets the identical reconstructed input — it just measures the
// models relative to each other, not the absolute production number.

// SectorModelCompareRequest selects the labeled sessions to replay over and the models to
// compare. Empty ModelIDs falls back to the single default analyst model.
type SectorModelCompareRequest struct {
	SessionIDs []string `json:"sessionIds"`
	ModelIDs   []string `json:"modelIds"`
	// MaxTokens, when > 0, overrides max_tokens for every analyst call — needed to give
	// reasoning models (which spend budget on hidden reasoning) enough room to finish
	// their JSON. 0 uses each model's configured value / the analyst default.
	MaxTokens int `json:"maxTokens,omitempty"`
	// PerCallTimeoutMs, when > 0, caps each analyst call: a slower call is abandoned and
	// counted as a failure instead of stalling the whole (synchronous, 60s-bounded)
	// request. Lets the harness stay robust to high-tail-latency reasoning models and
	// turns "too slow to use" into a measurable failure rate.
	PerCallTimeoutMs int `json:"perCallTimeoutMs,omitempty"`
	// DropConceptMatches, when true, omits the per-company embed+rerank concept matches
	// (target/distractor + in-perimeter + discriminators) from the analyst payload —
	// everything else (description, perimeter, snippets) held constant. An ablation to
	// measure how much the embedded-KB grounding actually contributes to the LLM analyst.
	DropConceptMatches bool `json:"dropConceptMatches,omitempty"`
	// IncludeSnippets re-gathers each company's neutral web snippets (from its persisted
	// resolved domain) and feeds them to the analyst, making the replayed input
	// production-faithful — the persisted summary keeps the distilled description + concepts
	// but NOT the raw snippets. Cached per domain across requests to bound Brave cost. The
	// snippets are re-fetched now, so they may have drifted since the original validation.
	IncludeSnippets bool `json:"includeSnippets,omitempty"`
	// PromptText / PromptID override the analyst SYSTEM PROMPT for this run, so a prompt
	// revision can be A/B'd on labeled data with the model held fixed (mirror of how
	// ModelIDs A/Bs the model with the prompt fixed). PromptText is used verbatim (no DB
	// row needed — draft a prompt and score it before migrating it); PromptID resolves a
	// stored prompt by id; empty falls back to the default analyst prompt. PromptText wins.
	PromptText string `json:"promptText,omitempty"`
	PromptID   string `json:"promptId,omitempty"`
}

// SectorModelTokens is the summed token usage across all replayed calls for one model.
type SectorModelTokens struct {
	Prompt     int `json:"prompt"`
	Completion int `json:"completion"`
	Total      int `json:"total"`
}

// SectorModelResult is one model's score over the labeled cases.
type SectorModelResult struct {
	ModelID   string `json:"modelId"`
	ModelName string `json:"modelName,omitempty"`
	Model     string `json:"model,omitempty"`
	Error     string `json:"error,omitempty"` // model could not be resolved/used at all

	Evaluable    int                       `json:"evaluable"` // labeled cases the model produced a bucket for
	Correct      int                       `json:"correct"`
	Accuracy     float64                   `json:"accuracy"`
	KeepLeak     int                       `json:"keepLeak"`              // true-keep predicted scarta — the recall-safety violation
	KeepRecall   float64                   `json:"keepRecall"`            // keep correctly kept / true keeps
	ScartaRecall float64                   `json:"scartaRecall"`          // scarta correctly dropped / true scartas
	Confusion    map[string]map[string]int `json:"confusion"`             // [label][predicted]
	Failures     int                       `json:"failures"`              // per-case call errors
	SampleError  string                    `json:"sampleError,omitempty"` // first call error (why a backup model breaks)
	Tokens       SectorModelTokens         `json:"tokens"`
	AvgLatencyMS int64                     `json:"avgLatencyMs"`
}

// SectorModelCompareItem is one company across all models, for the disagreement view.
type SectorModelCompareItem struct {
	SessionID   string            `json:"sessionId"`
	CompanyKey  string            `json:"companyKey"`
	CompanyName string            `json:"companyName"`
	Label       string            `json:"label"`
	Buckets     map[string]string `json:"buckets"` // modelId -> keep/forse/scarta | "error"
	Agree       bool              `json:"agree"`   // all models that produced a bucket agree
}

// SectorModelCompareReport is the full side-by-side comparison payload.
type SectorModelCompareReport struct {
	SessionIDs []string                 `json:"sessionIds"`
	Cases      int                      `json:"cases"` // labeled+validated companies replayed
	Models     []SectorModelResult      `json:"models"`
	Items      []SectorModelCompareItem `json:"items"`
}

// sectorCompareCase is one replayable company: the reconstructed analyst input + the
// human label. Built once, reused across every model.
type sectorCompareCase struct {
	sessionID   string
	companyKey  string
	companyName string
	label       string
	target      MATarget
	strategy    MAStrategySpec
	evidence    maCompanyEvidence
	class       maSectorClassification
}

// sectorCompareModel pairs a resolved model with its accumulating result. A non-empty err
// means resolution failed and the model is skipped from the replay (but still reported).
type sectorCompareModel struct {
	resolved llm.Model
	err      string
	result   SectorModelResult
}

// sectorCompareCell is one (model, case) call outcome.
type sectorCompareCell struct {
	bucket string
	usage  llm.Usage
	ms     int64
	err    error
}

func (s *maService) compareSectorEvalModels(ctx context.Context, req SectorModelCompareRequest, subject, email string) (SectorModelCompareReport, error) {
	if s.store == nil {
		return SectorModelCompareReport{}, errMAStoreUnavailable
	}
	if s.llmp == nil {
		return SectorModelCompareReport{}, errMAOpenRouterUnavailable
	}

	sessionIDs := dedupNonEmpty(req.SessionIDs)
	if len(sessionIDs) == 0 {
		return SectorModelCompareReport{}, fmt.Errorf("%w: sessionIds", errMAStrategyInvalid)
	}

	// One prompt for every model — only the model varies (unless a prompt override is
	// given, to A/B a prompt revision with the model fixed).
	var prompt llm.Prompt
	switch {
	case strings.TrimSpace(req.PromptText) != "":
		prompt = llm.Prompt{Prompt: req.PromptText}
	default:
		p, err := s.llmp.ResolvePrompt(ctx, maModelScopeCandidateMatchAnalyst, req.PromptID)
		if err != nil {
			return SectorModelCompareReport{}, llmConfigError(err)
		}
		prompt = p
	}

	models, err := s.resolveCompareModels(ctx, req.ModelIDs)
	if err != nil {
		return SectorModelCompareReport{}, err
	}

	cases, err := s.buildSectorCompareCases(ctx, sessionIDs)
	if err != nil {
		return SectorModelCompareReport{}, err
	}

	// Optionally re-gather the raw web snippets (not persisted) so the replayed analyst sees
	// the production-faithful input. Gathered once per distinct domain and cached.
	if req.IncludeSnippets {
		s.populateCompareSnippets(ctx, cases, subject, email)
	}

	// Result grid: one cell per (model, case). Each goroutine writes its own cell, so no
	// shared-state mutation — errgroup only bounds concurrency. A cell error never aborts
	// the comparison (we want every model fully scored).
	grid := make([][]sectorCompareCell, len(models))
	for mi := range grid {
		grid[mi] = make([]sectorCompareCell, len(cases))
	}
	group, gctx := errgroup.WithContext(ctx)
	// Higher than the web-validation throttle: this endpoint only hits the analyst LLMs
	// (no Brave/scraper), and it must finish a whole session well under the server's 60s
	// WriteTimeout — a synchronous request that overruns loses the entire result.
	group.SetLimit(maSectorCompareConcurrency)
	for mi := range models {
		if models[mi].err != "" {
			continue
		}
		model := models[mi].resolved
		for ci := range cases {
			mi, ci, model := mi, ci, model
			c := cases[ci]
			group.Go(func() error {
				callCtx := gctx
				if req.PerCallTimeoutMs > 0 {
					var cancel context.CancelFunc
					callCtx, cancel = context.WithTimeout(gctx, time.Duration(req.PerCallTimeoutMs)*time.Millisecond)
					defer cancel()
				}
				// Per-goroutine copy so the optional ablation (nil Concepts) never mutates
				// the shared case (the struct copy shares the slice header, but we only
				// reassign the field, never write through it).
				analystClass := c.class
				if req.DropConceptMatches {
					analystClass.Concepts = nil
				}
				start := s.now()
				resp, usage, callErr := s.runSectorAnalyst(callCtx, model, prompt, c.target, c.strategy, c.evidence, analystClass, subject, email, req.MaxTokens)
				cell := sectorCompareCell{usage: usage, ms: s.now().Sub(start).Milliseconds()}
				if callErr != nil {
					cell.err = callErr
				} else {
					cell.bucket = analystToBucket(resp.Verdict, resp.RecommendedAction)
				}
				grid[mi][ci] = cell
				return nil
			})
		}
	}
	_ = group.Wait()

	aggregateSectorCompare(models, cases, grid)

	out := SectorModelCompareReport{
		SessionIDs: sessionIDs,
		Cases:      len(cases),
		Models:     make([]SectorModelResult, len(models)),
		Items:      buildSectorCompareItems(models, cases, grid),
	}
	for mi := range models {
		out.Models[mi] = models[mi].result
	}
	return out, nil
}

// resolveCompareModels resolves each requested model id (scoped to the analyst). An empty
// list falls back to the single default analyst model. Per-model resolution errors are
// captured (not fatal) so a bad id reports as a failed row instead of killing the run.
func (s *maService) resolveCompareModels(ctx context.Context, ids []string) ([]sectorCompareModel, error) {
	ids = dedupNonEmpty(ids)
	if len(ids) == 0 {
		model, err := s.llmp.ResolveModel(ctx, maModelScopeCandidateMatchAnalyst, "")
		if err != nil {
			return nil, llmConfigError(err)
		}
		return []sectorCompareModel{{
			resolved: model,
			result:   SectorModelResult{ModelID: model.ID, ModelName: model.Name, Model: model.Model},
		}}, nil
	}
	out := make([]sectorCompareModel, 0, len(ids))
	for _, id := range ids {
		model, err := s.llmp.ResolveModel(ctx, maModelScopeCandidateMatchAnalyst, id)
		if err != nil {
			out = append(out, sectorCompareModel{
				err:    llmConfigError(err).Error(),
				result: SectorModelResult{ModelID: id, Error: llmConfigError(err).Error()},
			})
			continue
		}
		out = append(out, sectorCompareModel{
			resolved: model,
			result:   SectorModelResult{ModelID: model.ID, ModelName: model.Name, Model: model.Model},
		})
	}
	return out, nil
}

// buildSectorCompareCases reconstructs the analyst input for every labeled, validated
// company across the sessions: the distilled description + matched concepts (with the
// curated discriminator re-attached from the KB) + the strategy perimeter, exactly the
// shape the production analyst receives (minus raw snippets, which are not persisted).
func (s *maService) buildSectorCompareCases(ctx context.Context, sessionIDs []string) ([]sectorCompareCase, error) {
	contrastByID := s.loadConceptContrast(ctx)
	cases := []sectorCompareCase{}
	for _, sessionID := range sessionIDs {
		detail, err := s.store.GetMASession(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		if detail.Strategy == nil {
			continue
		}
		strategy := detail.Strategy.Strategy
		labels, err := s.store.ListMASectorEvalLabels(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		if len(labels) == 0 {
			continue
		}
		// The perimeter concept ids shown to the analyst (same derivation as production).
		perimeter := []string{}
		if set, derr := s.deriveStrategyConcepts(ctx, strategy); derr == nil {
			perimeter = sortedKeys(set)
		}
		for _, t := range detail.Targets {
			lbl, ok := labels[t.CompanyKey]
			if !ok || strings.TrimSpace(lbl.Label) == "" {
				continue
			}
			wv := t.WebValidation
			if wv == nil || strings.TrimSpace(wv.Summary.CompanyDescription) == "" {
				continue
			}
			concepts := make([]maConceptScore, len(wv.Summary.Concepts))
			for i, c := range wv.Summary.Concepts {
				c.Contrast = contrastByID[c.ID] // re-attach the curated discriminator (json:"-", not persisted)
				concepts[i] = c
			}
			cases = append(cases, sectorCompareCase{
				sessionID:   sessionID,
				companyKey:  t.CompanyKey,
				companyName: t.CompanyName,
				label:       lbl.Label,
				target:      t,
				strategy:    strategy,
				evidence:    maCompanyEvidence{Domain: wv.SelectedDomain},
				class: maSectorClassification{
					CompanyDescription: wv.Summary.CompanyDescription,
					Concepts:           concepts,
					StrategyConcepts:   perimeter,
					Verdict:            maSectorVerdict(wv.Summary.DeterministicVerdict),
				},
			})
		}
	}
	return cases, nil
}

// maSectorCompareSnippetConcurrency bounds the Brave snippet re-gather (Brave-bound, kept
// well below the analyst concurrency to avoid tripping Brave rate limits).
const maSectorCompareSnippetConcurrency = 8

// populateCompareSnippets re-fetches each case's neutral web snippets from its persisted
// resolved domain and stores them on the case (so the analyst sees production-faithful
// input). Best-effort: a missing domain or a failed gather just leaves snippets empty.
// Results are cached on the service per domain so a multi-request run pays Brave once.
func (s *maService) populateCompareSnippets(ctx context.Context, cases []sectorCompareCase, subject, email string) {
	if s.brave == nil {
		return
	}
	gathered := make([][]string, len(cases))
	group, gctx := errgroup.WithContext(ctx)
	group.SetLimit(maSectorCompareSnippetConcurrency)
	for i := range cases {
		domain := strings.TrimSpace(cases[i].evidence.Domain)
		if domain == "" {
			continue
		}
		i := i
		group.Go(func() error {
			if cached, ok := s.compareSnippetCache.Load(domain); ok {
				gathered[i] = cached.([]string)
				return nil
			}
			ev, _ := s.gatherNeutralEvidence(gctx, domain, maWebValidationEvidenceCount, subject, email, nil)
			s.compareSnippetCache.Store(domain, ev.Snippets)
			gathered[i] = ev.Snippets
			return nil
		})
	}
	_ = group.Wait()
	for i := range cases {
		if len(gathered[i]) > 0 {
			cases[i].evidence.Snippets = gathered[i]
		}
	}
}

// loadConceptContrast builds conceptID -> curated discriminator from the KB. Best-effort:
// an unavailable KB just means the analyst runs without discriminators (same for every
// model, so the comparison stays fair).
func (s *maService) loadConceptContrast(ctx context.Context) map[string]string {
	out := map[string]string{}
	if s.kb == nil {
		return out
	}
	concepts, err := s.kb.LoadKBConcepts(ctx)
	if err != nil {
		return out
	}
	for _, c := range concepts {
		if c.ContrastText != "" {
			out[c.ID] = c.ContrastText
		}
	}
	return out
}

// aggregateSectorCompare folds the call grid into per-model metrics.
func aggregateSectorCompare(models []sectorCompareModel, cases []sectorCompareCase, grid [][]sectorCompareCell) {
	for mi := range models {
		if models[mi].err != "" {
			continue
		}
		res := &models[mi].result
		res.Confusion = newSectorConfusion()
		var totLatency int64
		var nLat int
		for ci := range cases {
			cell := grid[mi][ci]
			res.Tokens.Prompt += cell.usage.PromptTokens
			res.Tokens.Completion += cell.usage.CompletionTokens
			res.Tokens.Total += cell.usage.TotalTokens
			if cell.err != nil {
				res.Failures++
				if res.SampleError == "" {
					res.SampleError = cell.err.Error()
				}
				continue
			}
			totLatency += cell.ms
			nLat++
			if cell.bucket == "" {
				continue
			}
			label := cases[ci].label
			res.Evaluable++
			res.Confusion[label][cell.bucket]++
			if label == cell.bucket {
				res.Correct++
			}
			if label == "keep" && cell.bucket == "scarta" {
				res.KeepLeak++
			}
		}
		if res.Evaluable > 0 {
			res.Accuracy = round4(float64(res.Correct) / float64(res.Evaluable))
		}
		keep := res.Confusion["keep"]
		if keepTot := keep["keep"] + keep["forse"] + keep["scarta"]; keepTot > 0 {
			res.KeepRecall = round4(float64(keep["keep"]) / float64(keepTot))
		}
		scarta := res.Confusion["scarta"]
		if scartaTot := scarta["keep"] + scarta["forse"] + scarta["scarta"]; scartaTot > 0 {
			res.ScartaRecall = round4(float64(scarta["scarta"]) / float64(scartaTot))
		}
		if nLat > 0 {
			res.AvgLatencyMS = totLatency / int64(nLat)
		}
	}
}

// buildSectorCompareItems produces the per-company disagreement rows.
func buildSectorCompareItems(models []sectorCompareModel, cases []sectorCompareCase, grid [][]sectorCompareCell) []SectorModelCompareItem {
	items := make([]SectorModelCompareItem, 0, len(cases))
	for ci := range cases {
		item := SectorModelCompareItem{
			SessionID:   cases[ci].sessionID,
			CompanyKey:  cases[ci].companyKey,
			CompanyName: cases[ci].companyName,
			Label:       cases[ci].label,
			Buckets:     map[string]string{},
		}
		distinct := map[string]bool{}
		for mi := range models {
			if models[mi].err != "" {
				continue
			}
			cell := grid[mi][ci]
			bucket := cell.bucket
			if cell.err != nil {
				bucket = "error"
			}
			item.Buckets[models[mi].result.ModelID] = bucket
			if bucket != "" && bucket != "error" {
				distinct[bucket] = true
			}
		}
		item.Agree = len(distinct) <= 1
		items = append(items, item)
	}
	return items
}

func dedupNonEmpty(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
