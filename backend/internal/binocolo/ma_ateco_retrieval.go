package binocolo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/llm"
)

const (
	// ATECO retrieval method (ma_parameter, value_type='text'). "embedding" uses the
	// curated-KB concept retrieval (use case 1); "llm" forces the legacy hierarchy
	// resolver. The embedding path also falls back to the resolver automatically
	// when the KB/embedder is unavailable, so this is a kill switch, not a gate.
	maAtecoRetrievalMethodParameter = "ateco_retrieval_method"
	maAtecoRetrievalMethodEmbedding = "embedding"
	maAtecoRetrievalMethodLLM       = "llm"

	// Calibration knobs (ma_parameter, value_type='number'), with code defaults.
	// Thresholds are RELATIVE to the top concept's cosine, never absolute: validated
	// retrieval shows the bullseye cosine drifting per query (≈0.58 vs ≈0.73), so a
	// fixed floor would reject good matches. RelThreshold keeps a concept when its
	// cosine >= top*RelThreshold; CoreRatio splits core vs weak at top*CoreRatio;
	// Floor is a tiny absolute guard that rejects "no real match" (top below it).
	maAtecoRetrievalRelThresholdKey = "ateco_retrieval_rel_threshold"
	maAtecoRetrievalCoreRatioKey    = "ateco_retrieval_core_ratio"
	maAtecoRetrievalFloorKey        = "ateco_retrieval_floor"
	maAtecoRetrievalCapKey          = "ateco_retrieval_cap"

	maAtecoRetrievalRelThresholdDefault = 0.75
	maAtecoRetrievalCoreRatioDefault    = 0.92
	maAtecoRetrievalFloorDefault        = 0.30
	maAtecoRetrievalCapDefault          = 12

	// Scope of the query-side embedding instruction in mrsmith.llm_prompt. Qwen3 is
	// instruction-aware: the query is wrapped "Instruct: …\nQuery: …" while the KB
	// documents are embedded raw. The instruction is the mechanism, not decoration.
	maModelScopeAtecoEmbed = "ma_strategy_ateco_embed"

	maAtecoEmbedDefaultInstruct = "Data una descrizione di settore aziendale per una ricerca M&A, recupera il concetto di business corrispondente."
)

type maAtecoRetrievalConfig struct {
	Method       string
	RelThreshold float64
	CoreRatio    float64
	Floor        float64
	Cap          int
}

func maAtecoRetrievalConfigFromParameters(params []MAParameter) maAtecoRetrievalConfig {
	cfg := maAtecoRetrievalConfig{
		Method:       maAtecoRetrievalMethodEmbedding,
		RelThreshold: maAtecoRetrievalRelThresholdDefault,
		CoreRatio:    maAtecoRetrievalCoreRatioDefault,
		Floor:        maAtecoRetrievalFloorDefault,
		Cap:          maAtecoRetrievalCapDefault,
	}
	values := make(map[string]string, len(params))
	for _, p := range params {
		values[p.Key] = p.Value
	}
	if v := strings.ToLower(strings.TrimSpace(values[maAtecoRetrievalMethodParameter])); v == maAtecoRetrievalMethodLLM || v == maAtecoRetrievalMethodEmbedding {
		cfg.Method = v
	}
	if v, ok := maParamFloat(values, maAtecoRetrievalRelThresholdKey); ok && v > 0 && v <= 1 {
		cfg.RelThreshold = v
	}
	if v, ok := maParamFloat(values, maAtecoRetrievalCoreRatioKey); ok && v > 0 && v <= 1 {
		cfg.CoreRatio = v
	}
	if v, ok := maParamFloat(values, maAtecoRetrievalFloorKey); ok && v >= 0 && v < 1 {
		cfg.Floor = v
	}
	if v, ok := maParamFloat(values, maAtecoRetrievalCapKey); ok && v >= 1 {
		cfg.Cap = int(v)
	}
	return cfg
}

func maParamFloat(values map[string]string, key string) (float64, bool) {
	raw, ok := values[key]
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

func (s *maService) loadAtecoRetrievalConfig(ctx context.Context) maAtecoRetrievalConfig {
	if s.store == nil {
		return maAtecoRetrievalConfigFromParameters(nil)
	}
	params, err := s.store.ListMAParameters(ctx)
	if err != nil {
		return maAtecoRetrievalConfigFromParameters(nil)
	}
	return maAtecoRetrievalConfigFromParameters(params)
}

func (s *maService) loadAtecoEmbedInstruction(ctx context.Context) (string, string) {
	if s.llmp == nil {
		return maAtecoEmbedDefaultInstruct, ""
	}
	prompt, err := s.llmp.ResolvePrompt(ctx, maModelScopeAtecoEmbed, "")
	if err != nil || strings.TrimSpace(prompt.Prompt) == "" {
		return maAtecoEmbedDefaultInstruct, ""
	}
	return strings.TrimSpace(prompt.Prompt), prompt.ID
}

// buildMAEmbeddingQuery distills the analyst's positive sector intent into the
// text embedded for retrieval. Exclusions are dropped (positiveSectorText) — they
// surface later as distractor low-similarity / excluded ATECO, not as needles.
func buildMAEmbeddingQuery(intent MAIntent) string {
	parts := []string{}
	for _, sector := range intent.Sectors.Include {
		if text := cleanText(positiveSectorText(sector.Text), 200); text != "" {
			parts = append(parts, text)
		}
	}
	return cleanText(strings.Join(parts, "; "), 400)
}

type kbScored struct {
	concept *kbConcept
	cosine  float64
}

// retrieveMAIntentAtecoEmbedding maps the analyst's sector intent to ATECO
// candidates deterministically: embed the intent (query-side instruction) and
// cosine-rank it against the curated KB concepts, then resolve the matched
// concepts' fit mapping to ATECO codes. ok=false means the embedding path is not
// usable (no KB/embedder, drift, API failure) and the caller must fall back to the
// LLM hierarchy resolver; ok=true means the result is authoritative (even when it
// is a confident no-match -> empty candidates + a missing-criterion note).
func (s *maService) retrieveMAIntentAtecoEmbedding(ctx context.Context, intent MAIntent, allowed map[string]AtecoCode, cfg maAtecoRetrievalConfig) (candidates []MAAtecoCandidate, missing []string, ok bool, err error) {
	if s.llmp == nil || s.kb == nil || s.ateco == nil {
		return nil, nil, false, nil
	}
	queryText := buildMAEmbeddingQuery(intent)
	if queryText == "" {
		// Exclusion-only intent: no positive perimeter to embed. Authoritative.
		if len(intent.Sectors.Exclude) > 0 {
			missing = append(missing, "Esclusione settoriale senza settore incluso: serve un perimetro positivo")
		}
		return nil, missing, true, nil
	}

	instruction, instructionPromptID := s.loadAtecoEmbedInstruction(ctx)
	scored, embModel, usage, ok, err := s.matchKBConcepts(ctx, instruction, queryText, false)
	if err != nil || !ok {
		return nil, nil, false, err
	}

	top := scored[0].cosine
	if top < cfg.Floor {
		s.recordAtecoRetrievalTrace(ctx, queryText, instruction, instructionPromptID, embModel, cfg, top, nil, nil, usage)
		missing = append(missing, "Settore non riconosciuto dalla base di conoscenza ATECO")
		return nil, missing, true, nil
	}
	relCut := top * cfg.RelThreshold
	coreCut := top * cfg.CoreRatio
	matched := make([]kbScored, 0, cfg.Cap)
	for _, sc := range scored {
		if sc.cosine < relCut {
			break
		}
		matched = append(matched, sc)
		if len(matched) >= cfg.Cap {
			break
		}
	}

	candidates, resolveMissing, err := s.resolveKBFitToCandidates(ctx, matched, coreCut, allowed)
	if err != nil {
		return nil, nil, false, err
	}
	missing = append(missing, resolveMissing...)
	s.recordAtecoRetrievalTrace(ctx, queryText, instruction, instructionPromptID, embModel, cfg, top, matched, candidates, usage)
	return candidates, missing, true, nil
}

// matchKBConcepts embeds `text` (wrapped with the query-side instruction, Qwen
// asymmetry) and returns the KB concepts scored by cosine, sorted descending.
// includeDistractors controls whether distractor concepts are scored: UC1 ATECO
// retrieval excludes them, UC2 sector classification includes them so an off-target
// company can land on a distractor. Runs the drift guard (single embedding model +
// dimension across the KB) and embeds with that exact model. ok=false means the
// KB/embedder is unavailable and the caller should fall back. Shared by UC1 and UC2.
func (s *maService) matchKBConcepts(ctx context.Context, instruction, text string, includeDistractors bool) ([]kbScored, llm.EmbeddingModel, llm.Usage, bool, error) {
	if s.llmp == nil || s.kb == nil {
		return nil, llm.EmbeddingModel{}, llm.Usage{}, false, nil
	}
	concepts, err := s.kb.LoadKBConcepts(ctx)
	if err != nil {
		return nil, llm.EmbeddingModel{}, llm.Usage{}, false, err
	}
	cands := make([]*kbConcept, 0, len(concepts))
	for i := range concepts {
		if len(concepts[i].Vector) == 0 {
			continue
		}
		if !includeDistractors && concepts[i].Kind == "distractor" {
			continue
		}
		cands = append(cands, &concepts[i])
	}
	if len(cands) == 0 {
		return nil, llm.EmbeddingModel{}, llm.Usage{}, false, nil
	}
	modelID, dim, err := kbModelConsensus(cands)
	if err != nil {
		return nil, llm.EmbeddingModel{}, llm.Usage{}, false, err
	}
	embModel, err := s.llmp.ResolveEmbeddingModel(ctx, modelID)
	if err != nil {
		return nil, llm.EmbeddingModel{}, llm.Usage{}, false, err
	}
	if embModel.Dimension != dim {
		return nil, llm.EmbeddingModel{}, llm.Usage{}, false, fmt.Errorf("kb vector dim %d != embedding model dim %d", dim, embModel.Dimension)
	}
	queryInput := text
	if strings.TrimSpace(instruction) != "" {
		queryInput = fmt.Sprintf("Instruct: %s\nQuery: %s", instruction, text)
	}
	vecs, usage, err := s.llmp.Embed(ctx, embModel, []string{queryInput})
	if err != nil {
		return nil, llm.EmbeddingModel{}, llm.Usage{}, false, err
	}
	if len(vecs) != 1 || len(vecs[0]) != dim {
		return nil, llm.EmbeddingModel{}, llm.Usage{}, false, fmt.Errorf("embedding returned %d vectors (dim mismatch)", len(vecs))
	}
	q := vecs[0]
	scored := make([]kbScored, 0, len(cands))
	for _, c := range cands {
		scored = append(scored, kbScored{concept: c, cosine: kbCosine(q, c.Vector)})
	}
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].cosine > scored[j].cosine })
	return scored, embModel, usage, true, nil
}

// resolveKBFitToCandidates turns the matched concepts into ATECO candidates with
// intent-relative fit. Iterating highest-cosine-first, an in_kb code takes the fit
// of the first (best) concept that claims it (core if cosine>=coreCut, else weak);
// later, lower-cosine concepts cannot override it (highest cosine wins). Excluded
// codes become excluded prefixes UNLESS a matched concept includes the same code
// as core (the exclusion safety net). Includes are resolved against the catalog
// and remembered in `allowed` so canonicalize(requireAllowed=true) accepts them;
// excluded codes are kept raw (canonicalize treats them as subtree prefixes).
func (s *maService) resolveKBFitToCandidates(ctx context.Context, matched []kbScored, coreCut float64, allowed map[string]AtecoCode) ([]MAAtecoCandidate, []string, error) {
	type incEntry struct {
		code   string
		sc     string
		fit    string
		cosine float64
		source string
	}
	// Pass 1: includes with intent-relative fit. Highest-cosine concept first, so the
	// first concept to claim a code fixes its fit (highest cosine wins).
	includes := make([]incEntry, 0)
	seenInc := map[string]bool{}
	coreSearchCodes := map[string]bool{}
	for _, m := range matched {
		fit := maFitWeak
		if m.cosine >= coreCut {
			fit = maFitCore
		}
		for _, raw := range m.concept.InKB {
			sc := atecoSearchCode(raw)
			if sc == "" || seenInc[sc] {
				continue
			}
			seenInc[sc] = true
			includes = append(includes, incEntry{code: raw, sc: sc, fit: fit, cosine: m.cosine, source: m.concept.ID})
			if fit == maFitCore {
				coreSearchCodes[sc] = true
			}
		}
	}
	// Pass 2: exclude set, skipping codes a matched concept includes as core (the
	// safety net). A non-core include on an excluded code does NOT save it.
	excludes := make([]string, 0)
	seenExc := map[string]bool{}
	for _, m := range matched {
		for _, raw := range m.concept.Excluded {
			sc := atecoSearchCode(raw)
			if sc == "" || seenExc[sc] {
				continue
			}
			if coreSearchCodes[sc] {
				continue // a core include outranks an exclusion on the same code
			}
			seenExc[sc] = true
			excludes = append(excludes, raw)
		}
	}

	// Pass 3: emit candidates. A weak include shadowed by an exclusion yields to the
	// exclusion (exclusion wins unless core-included).
	candidates := make([]MAAtecoCandidate, 0, len(includes)+len(excludes))
	missing := []string{}
	for _, inc := range includes {
		if seenExc[inc.sc] && inc.fit != maFitCore {
			continue
		}
		resolved, err := s.ateco.ResolveAtecoCode(ctx, inc.code)
		if errors.Is(err, errAtecoCodeNotFound) {
			missing = append(missing, "Codice ATECO della KB non presente nel catalogo: "+inc.code)
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		rememberAllowedAteco(allowed, resolved)
		candidates = append(candidates, MAAtecoCandidate{
			Code:        resolved.Codice,
			Description: resolved.Titolo,
			Rationale:   fmt.Sprintf("Concetto KB %s (coseno %.2f)", inc.source, inc.cosine),
			Fit:         inc.fit,
		})
	}
	for _, raw := range excludes {
		code := normalizeAtecoCode(raw)
		if code == "" {
			continue
		}
		candidates = append(candidates, MAAtecoCandidate{
			Code:      code,
			Rationale: "Escluso dal perimetro (base di conoscenza ATECO)",
			Fit:       maFitExcluded,
		})
	}
	return candidates, missing, nil
}

func (s *maService) recordAtecoRetrievalTrace(ctx context.Context, queryText, instruction, instructionPromptID string, model llm.EmbeddingModel, cfg maAtecoRetrievalConfig, top float64, matched []kbScored, candidates []MAAtecoCandidate, usage llm.Usage) {
	matchedMeta := make([]map[string]any, 0, len(matched))
	coreCut := top * cfg.CoreRatio
	for _, m := range matched {
		fit := maFitWeak
		if m.cosine >= coreCut {
			fit = maFitCore
		}
		matchedMeta = append(matchedMeta, map[string]any{
			"concept_id": m.concept.ID,
			"name":       m.concept.Name,
			"cosine":     math.Round(m.cosine*10000) / 10000,
			"fit":        fit,
		})
	}
	coreCount, weakCount, excludedCount := 0, 0, 0
	for _, c := range candidates {
		switch normalizeMAFit(c.Fit) {
		case maFitCore:
			coreCount++
		case maFitWeak:
			weakCount++
		case maFitExcluded:
			excludedCount++
		}
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType:      "ma_ateco_retrieval",
		ExternalSystem: "fireworks",
		Status:         maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"query":                 queryText,
			"instruction":           instruction,
			"instruction_prompt_id": instructionPromptID,
			"embedding_model_id":    model.ID,
			"embedding_model":       model.Model,
			"dimension":             model.Dimension,
			"top_cosine":            math.Round(top*10000) / 10000,
			"rel_threshold":         cfg.RelThreshold,
			"core_ratio":            cfg.CoreRatio,
			"floor":                 cfg.Floor,
			"cap":                   cfg.Cap,
			"matched_count":         len(matched),
			"candidate_core":        coreCount,
			"candidate_weak":        weakCount,
			"candidate_excluded":    excludedCount,
			"prompt_tokens":         usage.PromptTokens,
			"total_tokens":          usage.TotalTokens,
		}),
		Response: maTraceJSON(map[string]any{
			"matched":    matchedMeta,
			"candidates": candidates,
		}),
	})
}

// kbModelConsensus returns the single embedding_model_id shared by all concepts
// and the vector dimension, erroring if rows disagree (mixed model = incompatible
// vectors) or carry no model id. The dimension is checked for uniformity too.
func kbModelConsensus(concepts []*kbConcept) (string, int, error) {
	modelID := ""
	dim := 0
	for _, c := range concepts {
		if strings.TrimSpace(c.EmbeddingModelID) == "" {
			return "", 0, fmt.Errorf("kb concept %q has no embedding_model_id", c.ID)
		}
		if modelID == "" {
			modelID = c.EmbeddingModelID
			dim = len(c.Vector)
			continue
		}
		if c.EmbeddingModelID != modelID {
			return "", 0, fmt.Errorf("kb concepts span multiple embedding models (%s vs %s)", modelID, c.EmbeddingModelID)
		}
		if len(c.Vector) != dim {
			return "", 0, fmt.Errorf("kb concept %q vector dim %d != %d", c.ID, len(c.Vector), dim)
		}
	}
	if modelID == "" || dim == 0 {
		return "", 0, fmt.Errorf("kb has no usable embedded concepts")
	}
	return modelID, dim, nil
}

// kbCosine is the cosine similarity of two vectors, tolerant of non-normalized
// inputs. Returns 0 on length mismatch or a zero-norm vector.
func kbCosine(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
