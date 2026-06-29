package binocolo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/llm"
)

// Use case 2 — web sector classification. Given a target company's NEUTRAL web
// self-description, classify it against the SAME curated KB concept space used by
// UC1 (targets + distractors), then judge it against the strategy's perimeter.
// This replaces the old keyword/negative-term/magic-threshold pipeline: an
// off-target company lands on a distractor concept (e.g. web_agency) and is
// rejected WITHOUT any hardcoded negative term — the bias cure.

const (
	// Distiller (company self-description -> 1-3 neutral sentences) and the UC2
	// embed/rerank instructions live in the centralized registry; the reranker
	// itself is mrsmith.rerank_model at scope ma_sector_classification (migration 062).
	maModelScopeCompanyRepresentation = "ma_company_representation"
	maModelScopeSectorEmbed           = "ma_sector_embed"

	// Compiled fallbacks (overridable via mrsmith.llm_prompt), so the path runs
	// before the UC2 prompts are seeded.
	maSectorEmbedDefaultInstruct  = "Data l'autodescrizione di un'azienda, recupera il concetto di business corrispondente."
	maSectorRerankDefaultInstruct = "Data l'autodescrizione di un'azienda, valuta se l'azienda opera nel concetto di business indicato."

	maCompanyRepresentationMaxToken = 800
	maCompanyEvidenceTextCap        = 2400
	maCompanyDescriptionCap         = 600

	// maCompanyRepresentationJSONInstruction is appended to the distiller system
	// prompt so the call works in json_object mode regardless of the seeded prompt
	// revision (Fireworks requires the literal word "json" in the messages). Forcing
	// structured output is what stops cheap reasoning models from leaking their
	// chain-of-thought into the description (the analyst is clean for the same reason).
	maCompanyRepresentationJSONInstruction = "\n\nRispondi ESCLUSIVAMENTE con un oggetto JSON valido nella forma {\"description\": \"<descrizione neutra e fattuale dell'azienda, 1-3 frasi in italiano>\"}. Nessun altro testo, nessun markdown, nessun ragionamento."
)

// UC2 calibration (ma_parameter, value_type='number'). Thresholds are ABSOLUTE:
// the reranker returns a softmax-normalized yes-probability (0-1) comparable across
// companies, so — unlike UC1's relative cosine — fixed cutoffs are meaningful.
const (
	maSectorConfirmProbKey   = "sector_confirm_prob"
	maSectorRejectMarginKey  = "sector_reject_margin"
	maSectorAmbiguityBandKey = "sector_ambiguity_band"
	maSectorNoSignalFloorKey = "sector_no_signal_floor"
	maSectorRerankCapKey     = "sector_rerank_cap"

	// Calibrated 2026-06-29 on the corrected (2-way-softmax) reranker scale, whose
	// P(yes) tops out ~0.3 for a strong match. Anchors: NETX64 (true positive,
	// in-perimeter 0.296 -> confirm) vs Coherency (off-perimeter, in-perimeter 0.142
	// -> ambiguous/weak). Overridable via the sector_* ma_parameter rows.
	maSectorConfirmProbDefault   = 0.25
	maSectorRejectMarginDefault  = 0.10
	maSectorAmbiguityBandDefault = 0.15
	maSectorNoSignalFloorDefault = 0.08
	maSectorRerankCapDefault     = 8
)

type maSectorVerdict string

const (
	maSectorConfirm   maSectorVerdict = "confirm"   // on-perimeter target, high confidence
	maSectorReject    maSectorVerdict = "reject"    // distractor clearly wins -> off-target
	maSectorWeak      maSectorVerdict = "weak"      // legitimate target but off the strategy perimeter
	maSectorAmbiguous maSectorVerdict = "ambiguous" // contrasting/under-threshold -> escalate to LLM (Step 3)
	maSectorNoSignal  maSectorVerdict = "no_signal" // no usable self-description -> escalate/deprioritize
)

type maSectorClassConfig struct {
	ConfirmProb   float64
	RejectMargin  float64
	AmbiguityBand float64
	NoSignalFloor float64
	RerankCap     int
}

func maSectorClassConfigFromParameters(params []MAParameter) maSectorClassConfig {
	cfg := maSectorClassConfig{
		ConfirmProb:   maSectorConfirmProbDefault,
		RejectMargin:  maSectorRejectMarginDefault,
		AmbiguityBand: maSectorAmbiguityBandDefault,
		NoSignalFloor: maSectorNoSignalFloorDefault,
		RerankCap:     maSectorRerankCapDefault,
	}
	values := make(map[string]string, len(params))
	for _, p := range params {
		values[p.Key] = p.Value
	}
	if v, ok := maParamFloat(values, maSectorConfirmProbKey); ok && v > 0 && v <= 1 {
		cfg.ConfirmProb = v
	}
	if v, ok := maParamFloat(values, maSectorRejectMarginKey); ok && v >= 0 && v < 1 {
		cfg.RejectMargin = v
	}
	if v, ok := maParamFloat(values, maSectorAmbiguityBandKey); ok && v >= 0 && v < 1 {
		cfg.AmbiguityBand = v
	}
	if v, ok := maParamFloat(values, maSectorNoSignalFloorKey); ok && v >= 0 && v < 1 {
		cfg.NoSignalFloor = v
	}
	if v, ok := maParamFloat(values, maSectorRerankCapKey); ok && v >= 1 {
		cfg.RerankCap = int(v)
	}
	return cfg
}

func (s *maService) loadSectorClassConfig(ctx context.Context) maSectorClassConfig {
	if s.store == nil {
		return maSectorClassConfigFromParameters(nil)
	}
	params, err := s.store.ListMAParameters(ctx)
	if err != nil {
		return maSectorClassConfigFromParameters(nil)
	}
	return maSectorClassConfigFromParameters(params)
}

// maCompanyEvidence is the NEUTRAL self-description corpus gathered in Step 3
// (titles + snippets from site:domain "chi siamo"/"servizi"/homepage probes).
type maCompanyEvidence struct {
	Domain   string
	Snippets []string
}

// maConceptScore is one KB concept the company matched, with embedding recall
// (cosine) and reranker precision (yes-probability), plus whether the concept is
// in the strategy's perimeter.
type maConceptScore struct {
	ID         string  `json:"conceptId"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Cosine     float64 `json:"cosine"`
	RerankProb float64 `json:"rerankProb"`
	InStrategy bool    `json:"inStrategy"`
}

type maSectorClassification struct {
	CompanyDescription string           `json:"companyDescription"`
	Concepts           []maConceptScore `json:"concepts"`
	StrategyConcepts   []string         `json:"strategyConcepts"`
	Verdict            maSectorVerdict  `json:"verdict"`
	TopProb            float64          `json:"topProb"`
	RerankApplied      bool             `json:"rerankApplied"`
	Confidence         string           `json:"confidence"`
	Reason             string           `json:"reason"`
}

// classifyCompanySector runs the UC2 cascade: distil a neutral company description,
// classify it against the KB concepts (embedding recall + reranker precision), and
// judge it against the strategy's perimeter concepts. The verdict is deterministic;
// `ambiguous`/`no_signal` are the hand-off to the LLM tie-breaker (Step 3). An error
// means the embedder/KB is unavailable and the caller should fall back to legacy.
func (s *maService) classifyCompanySector(ctx context.Context, evidence maCompanyEvidence, strategy MAStrategySpec, subject, email string) (maSectorClassification, error) {
	description := s.representCompany(ctx, evidence, subject, email)
	return s.classifyCompanyDescription(ctx, description, strategy)
}

// classifyCompanyDescription is the description-in classification core: derive the
// strategy perimeter concepts, classify the company description against the KB
// (embedding + reranker), and compute the verdict. Split out from
// classifyCompanySector so the lab probe can feed a pasted description directly.
func (s *maService) classifyCompanyDescription(ctx context.Context, description string, strategy MAStrategySpec) (maSectorClassification, error) {
	cfg := s.loadSectorClassConfig(ctx)
	if strings.TrimSpace(description) == "" {
		return maSectorClassification{
			CompanyDescription: description,
			Verdict:            maSectorNoSignal,
			Confidence:         "bassa",
			Reason:             "Nessuna autodescrizione web disponibile per la classificazione.",
		}, nil
	}

	strategyConcepts, err := s.deriveStrategyConcepts(ctx, strategy)
	if err != nil {
		// Strategy perimeter underivable -> treat as empty (every match off-perimeter);
		// the classification still distinguishes target from distractor.
		strategyConcepts = map[string]bool{}
	}

	concepts, rerankApplied, err := s.classifyCompanyConcepts(ctx, description, strategyConcepts, cfg)
	if err != nil {
		return maSectorClassification{}, err
	}

	verdict, topProb, confidence, reason := sectorVerdict(concepts, cfg, rerankApplied)
	return maSectorClassification{
		CompanyDescription: description,
		Concepts:           concepts,
		StrategyConcepts:   sortedKeys(strategyConcepts),
		Verdict:            verdict,
		TopProb:            topProb,
		RerankApplied:      rerankApplied,
		Confidence:         confidence,
		Reason:             reason,
	}, nil
}

// SectorClassificationTestRequest is the lab probe input (Test page): classify one
// company against a sector intent, in isolation from the funnel. Two modes:
//   - existing target: set SessionID + TargetID — the sector comes from the session
//     strategy, the company from the target (domain resolution + neutral evidence,
//     i.e. the exact production path minus persistence);
//   - ad-hoc: set SectorDescription plus the company as raw CompanyDescription
//     (fastest, skips Brave + distiller), Snippets, or a Domain (gathers via Brave).
type SectorClassificationTestRequest struct {
	SectorDescription  string   `json:"sectorDescription,omitempty"`
	SessionID          string   `json:"sessionId,omitempty"`
	TargetID           string   `json:"targetId,omitempty"`
	CompanyName        string   `json:"companyName,omitempty"`
	Domain             string   `json:"domain,omitempty"`
	Snippets           []string `json:"snippets,omitempty"`
	CompanyDescription string   `json:"companyDescription,omitempty"`
	Analyze            bool     `json:"analyze,omitempty"`
}

type SectorClassificationTestResponse struct {
	CompanyName    string                          `json:"companyName,omitempty"`
	SelectedDomain string                          `json:"selectedDomain,omitempty"`
	Evidence       []string                        `json:"evidence"`
	Classification maSectorClassification          `json:"classification"`
	Analysis       *CandidateMatchAnalysisResponse `json:"analysis,omitempty"`
	FinalDecision  *CandidateMatchFinalDecision    `json:"finalDecision,omitempty"`
}

// testSectorClassification runs the UC2 concept pipeline on demand for the Test
// page. It mirrors the production path (neutral evidence -> distiller -> embed +
// rerank -> verdict -> optional analyst -> decision) but never persists. In
// session/target mode it pulls the sector from the session strategy and the company
// from the target (the exact production builder, read-only); in ad-hoc mode the
// company is supplied directly.
func (s *maService) testSectorClassification(ctx context.Context, req SectorClassificationTestRequest, subject, email string) (SectorClassificationTestResponse, error) {
	var (
		strategy       MAStrategySpec
		target         MATarget
		description    string
		evidenceUsed   []string
		selectedDomain string
	)

	if strings.TrimSpace(req.SessionID) != "" && strings.TrimSpace(req.TargetID) != "" {
		if s.store == nil {
			return SectorClassificationTestResponse{}, errMAStoreUnavailable
		}
		detail, err := s.store.GetMASession(ctx, req.SessionID)
		if err != nil {
			return SectorClassificationTestResponse{}, err
		}
		if detail.Strategy == nil {
			return SectorClassificationTestResponse{}, fmt.Errorf("%w: strategy", errMAStrategyInvalid)
		}
		strategy = detail.Strategy.Strategy
		found := false
		for _, item := range detail.Targets {
			if item.ID == req.TargetID {
				target = item
				found = true
				break
			}
		}
		if !found {
			return SectorClassificationTestResponse{}, fmt.Errorf("%w: target non trovato nella sessione", errMAStrategyInvalid)
		}
		if s.brave == nil {
			return SectorClassificationTestResponse{}, errMABraveUnavailable
		}
		domainResponse, err := s.resolveDomainCandidates(ctx, DomainResolutionRequest{
			CompanyName: target.CompanyName,
			VATCode:     target.VATCode,
			TaxCode:     target.TaxCode,
			Town:        target.Town,
			Province:    target.Province,
			Count:       maWebValidationDefaultDomainCount,
		})
		if err != nil {
			return SectorClassificationTestResponse{}, err
		}
		chosen := chooseMAWebValidationDomain(domainResponse.Candidates)
		if chosen == nil {
			decision := &CandidateMatchFinalDecision{
				InitialMatchState:  target.MatchState,
				DeterministicScore: target.Score,
				WebValidationState: "domain_unresolved",
				FinalAction:        "needs_domain_review",
				Confidence:         "bassa",
				Reason:             "Nessun dominio ufficiale credibile risolto.",
				Reasons:            []string{"Nessun dominio ufficiale credibile risolto."},
			}
			return SectorClassificationTestResponse{
				CompanyName:    target.CompanyName,
				Evidence:       []string{},
				Classification: maSectorClassification{Verdict: maSectorNoSignal, Confidence: "bassa", Reason: "Nessun dominio ufficiale credibile risolto."},
				FinalDecision:  decision,
			}, nil
		}
		selectedDomain = chosen.Domain
		evidence, _ := s.gatherNeutralEvidence(ctx, chosen.Domain, maWebValidationEvidenceCount, subject, email)
		evidenceUsed = evidence.Snippets
		description = s.representCompany(ctx, evidence, subject, email)
	} else {
		strategy = MAStrategySpec{SectorDescription: cleanText(req.SectorDescription, 400)}
		target = MATarget{CompanyName: cleanText(req.CompanyName, 200)}
		description = cleanText(req.CompanyDescription, maCompanyDescriptionCap)
		if description == "" {
			evidence := maCompanyEvidence{Domain: strings.TrimSpace(req.Domain)}
			switch {
			case len(req.Snippets) > 0:
				for _, snippet := range req.Snippets {
					if cleaned := cleanText(snippet, 360); cleaned != "" {
						evidence.Snippets = append(evidence.Snippets, cleaned)
					}
				}
			case strings.TrimSpace(req.Domain) != "":
				if s.brave == nil {
					return SectorClassificationTestResponse{}, errMABraveUnavailable
				}
				gathered, _ := s.gatherNeutralEvidence(ctx, req.Domain, maWebValidationEvidenceCount, subject, email)
				evidence = gathered
			default:
				return SectorClassificationTestResponse{}, fmt.Errorf("%w: serve companyDescription, snippets o domain", errMAStrategyInvalid)
			}
			selectedDomain = evidence.Domain
			evidenceUsed = evidence.Snippets
			description = s.representCompany(ctx, evidence, subject, email)
		}
	}

	class, err := s.classifyCompanyDescription(ctx, description, strategy)
	if err != nil {
		return SectorClassificationTestResponse{}, err
	}
	var analysis *CandidateMatchAnalysisResponse
	if req.Analyze && sectorVerdictNeedsLLM(class.Verdict) {
		result, aErr := s.analyzeSectorAmbiguity(ctx, target, strategy, maCompanyEvidence{Snippets: evidenceUsed}, class, subject, email)
		if aErr == nil {
			analysis = &result
		}
	}
	decision := sectorFinalDecision(target, class, analysis, "")
	return SectorClassificationTestResponse{
		CompanyName:    target.CompanyName,
		SelectedDomain: selectedDomain,
		Evidence:       evidenceUsed,
		Classification: class,
		Analysis:       analysis,
		FinalDecision:  decision,
	}, nil
}

// classifyCompanyConcepts embeds the company description over the KB concepts
// (targets + distractors), keeps the top-K by cosine, then reranks them for
// precision (query=description, document=concept text). rerankApplied=false when
// the reranker is unavailable — the caller's verdict then degrades to LLM. err is
// returned only when the embedder/KB is down.
func (s *maService) classifyCompanyConcepts(ctx context.Context, description string, strategyConcepts map[string]bool, cfg maSectorClassConfig) ([]maConceptScore, bool, error) {
	embInstruction := s.loadSectorEmbedInstruction(ctx)
	scored, _, _, ok, err := s.matchKBConcepts(ctx, embInstruction, description, true)
	if err != nil || !ok {
		return nil, false, err
	}
	k := cfg.RerankCap
	if k <= 0 || k > len(scored) {
		k = len(scored)
	}
	cands := scored[:k]

	rerankApplied := false
	var probs []float64
	if rerankModel, rErr := s.llmp.ResolveRerankModel(ctx, maModelScopeSectorClassification, ""); rErr == nil {
		docs := make([]string, len(cands))
		for i, c := range cands {
			docs[i] = c.concept.EmbeddingText
		}
		p, _, perr := s.llmp.Rerank(ctx, rerankModel, s.loadSectorRerankInstruction(ctx), description, docs)
		if perr == nil && len(p) == len(cands) {
			probs = p
			rerankApplied = true
		}
	}

	out := make([]maConceptScore, len(cands))
	for i, c := range cands {
		prob := 0.0
		if rerankApplied {
			prob = probs[i]
		}
		out[i] = maConceptScore{
			ID:         c.concept.ID,
			Name:       c.concept.Name,
			Kind:       c.concept.Kind,
			Cosine:     c.cosine,
			RerankProb: prob,
			InStrategy: strategyConcepts[c.concept.ID],
		}
	}
	if rerankApplied {
		sort.SliceStable(out, func(i, j int) bool { return out[i].RerankProb > out[j].RerankProb })
	}
	return out, rerankApplied, nil
}

// deriveStrategyConcepts marks which KB concepts fall inside the strategy's perimeter.
// The perimeter is defined by ATECO overlap: a concept is in-perimeter iff one of its
// in_kb ATECO codes is among the strategy's (non-excluded) ATECO candidates — exactly
// the explicit criteria used to select the strategy's targets, matched exactly (no
// prefix), using the KB's own concept<->ATECO tagging.
//
// This replaces deriving the perimeter from embedding similarity to the
// sector-description text, which was systematically misaligned with the ATECO scope:
// it dropped in-scope concepts (security/infra sub-domains like identity_access,
// soc_mdr, backup_dr that share the strategy's codes) and added out-of-scope ones
// (telecom/ISP/hardware/wholesale whose codes are in a different division). That
// misalignment wrongly declassed legitimate candidates (e.g. a cybersecurity firm
// topping on identity_access, which shares the strategy's 62.20.20). Falls back to the
// embedding derivation only when the strategy carries no ATECO codes (free-text /
// non-ATECO strategies) or the KB is unavailable.
func (s *maService) deriveStrategyConcepts(ctx context.Context, strategy MAStrategySpec) (map[string]bool, error) {
	codes := map[string]bool{}
	for _, cand := range strategy.AtecoCandidates {
		if cand.Fit == maFitExcluded {
			continue
		}
		if sc := atecoSearchCode(cand.Code); sc != "" {
			codes[sc] = true
		}
	}
	if len(codes) == 0 || s.kb == nil {
		return s.deriveStrategyConceptsByEmbedding(ctx, strategy)
	}
	concepts, err := s.kb.LoadKBConcepts(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, c := range concepts {
		for _, raw := range c.InKB {
			if codes[atecoSearchCode(raw)] {
				out[c.ID] = true
				break
			}
		}
	}
	if len(out) == 0 {
		// Strategy codes matched no concept (unexpected, since strategy codes derive
		// from concepts) — fall back rather than render an empty perimeter that would
		// push every target off-perimeter.
		return s.deriveStrategyConceptsByEmbedding(ctx, strategy)
	}
	return out, nil
}

// deriveStrategyConceptsByEmbedding is the fallback perimeter: concepts semantically
// near the strategy's sector description (UC1 instruction + relative threshold). Used
// only when the strategy has no ATECO codes to anchor the perimeter exactly.
func (s *maService) deriveStrategyConceptsByEmbedding(ctx context.Context, strategy MAStrategySpec) (map[string]bool, error) {
	sector := cleanText(strategy.SectorDescription, 400)
	out := map[string]bool{}
	if sector == "" {
		return out, nil
	}
	instruction, _ := s.loadAtecoEmbedInstruction(ctx)
	scored, _, _, ok, err := s.matchKBConcepts(ctx, instruction, sector, false)
	if err != nil || !ok || len(scored) == 0 {
		return out, err
	}
	cfg := s.loadAtecoRetrievalConfig(ctx)
	top := scored[0].cosine
	if top < cfg.Floor {
		return out, nil
	}
	relCut := top * cfg.RelThreshold
	for i, sc := range scored {
		if sc.cosine < relCut || i >= cfg.Cap {
			break
		}
		out[sc.concept.ID] = true
	}
	return out, nil
}

// sectorVerdict applies the concept-driven decision over ABSOLUTE rerank
// probabilities (softmax-calibrated). Without a reranker (rerankApplied=false) the
// embedding alone is not treated as decisive for a confirm/reject — it escalates.
func sectorVerdict(concepts []maConceptScore, cfg maSectorClassConfig, rerankApplied bool) (maSectorVerdict, float64, string, string) {
	if len(concepts) == 0 {
		return maSectorNoSignal, 0, "bassa", "Nessun concetto KB corrispondente."
	}
	topProb := concepts[0].RerankProb
	if !rerankApplied {
		return maSectorAmbiguous, topProb, "bassa", "Reranker non disponibile: classificazione non decisiva, serve giudizio LLM."
	}

	bestTargetProb, bestDistractorProb, bestInStrategyProb := 0.0, 0.0, 0.0
	for _, c := range concepts {
		if c.Kind == "distractor" {
			if c.RerankProb > bestDistractorProb {
				bestDistractorProb = c.RerankProb
			}
			continue
		}
		if c.RerankProb > bestTargetProb {
			bestTargetProb = c.RerankProb
		}
		if c.InStrategy && c.RerankProb > bestInStrategyProb {
			bestInStrategyProb = c.RerankProb
		}
	}

	switch {
	case topProb < cfg.NoSignalFloor:
		return maSectorNoSignal, topProb, "bassa", "Segnale troppo debole per classificare il settore."
	case bestDistractorProb >= cfg.ConfirmProb && bestDistractorProb > bestTargetProb+cfg.RejectMargin:
		return maSectorReject, topProb, sectorConfidence(bestDistractorProb), "Profilo coerente con un concetto off-target (distrattore)."
	case bestInStrategyProb >= cfg.ConfirmProb && bestInStrategyProb >= bestDistractorProb+cfg.RejectMargin:
		return maSectorConfirm, topProb, sectorConfidence(bestInStrategyProb), "Profilo coerente con il perimetro della strategia."
	case bestTargetProb >= cfg.ConfirmProb && bestTargetProb > bestDistractorProb+cfg.RejectMargin:
		return maSectorWeak, topProb, "media", "Azienda IT legittima ma fuori dal perimetro della strategia."
	default:
		return maSectorAmbiguous, topProb, "bassa", "Segnali contrastanti o sotto soglia: serve giudizio LLM."
	}
}

// sectorConfidence grades the verdict's confidence on the corrected reranker scale
// (P(yes) tops out ~0.3 for a strong match), so the cutoffs are far below a textbook
// probability scale. Bands sit around the confirm threshold (0.25).
func sectorConfidence(prob float64) string {
	switch {
	case prob >= 0.28:
		return "alta"
	case prob >= 0.16:
		return "media"
	default:
		return "bassa"
	}
}

// representCompany distils the neutral web evidence into a short company
// self-description, in the register of the KB concept texts. Best-effort: when the
// distiller model/prompt is unconfigured or the call fails, it falls back to the
// cleaned, concatenated snippets so the pipeline still runs.
func (s *maService) representCompany(ctx context.Context, evidence maCompanyEvidence, subject, email string) string {
	fallback := buildCompanyEvidenceText(evidence)
	if s.llmp == nil || fallback == "" {
		return fallback
	}
	model, err := s.llmp.ResolveModel(ctx, maModelScopeCompanyRepresentation, "")
	if err != nil {
		return fallback
	}
	prompt, err := s.llmp.ResolvePrompt(ctx, maModelScopeCompanyRepresentation, "")
	if err != nil {
		return fallback
	}
	client, err := s.llmp.ClientForModel(ctx, model)
	if err != nil {
		return fallback
	}
	input, err := json.Marshal(map[string]any{"domain": evidence.Domain, "snippets": evidence.Snippets})
	if err != nil {
		return fallback
	}
	reqParams := model.RawParams()
	if _, ok := reqParams["max_tokens"]; !ok {
		reqParams["max_tokens"] = maCompanyRepresentationMaxToken
	}
	// Guarantee the json_object contract only when the seeded prompt doesn't already
	// request it (migration 064), to avoid duplicating the instruction.
	systemContent := prompt.Prompt
	if !strings.Contains(strings.ToLower(systemContent), "\"description\"") {
		systemContent += maCompanyRepresentationJSONInstruction
	}
	chatReq := llm.ChatRequest{
		Model:          model.Model,
		Params:         reqParams,
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
		Messages: []llm.Message{
			{Role: "system", Content: systemContent},
			{Role: "user", Content: string(input)},
		},
	}
	resp, chatErr := client.Chat(ctx, chatReq)
	usageRaw, _ := json.Marshal(resp.Usage)
	requestBody, _ := llm.BuildRequestBody(chatReq)
	requestRaw, _ := json.Marshal(requestBody)
	audit := llm.CallAudit{
		App:          maApp,
		Scope:        maModelScopeCompanyRepresentation,
		ProviderID:   model.ProviderID,
		ModelID:      model.ID,
		PromptID:     prompt.ID,
		Model:        model.Model,
		Request:      requestRaw,
		Usage:        usageRaw,
		ActorSubject: subject,
		ActorEmail:   email,
	}
	if chatErr != nil {
		audit.Status = "failed"
		audit.ErrorMessage = chatErr.Error()
	} else if respRaw, mErr := json.Marshal(map[string]any{"content": resp.Content}); mErr == nil {
		audit.Response = respRaw
	}
	_ = s.llmp.RecordAudit(ctx, audit)
	if chatErr != nil {
		return fallback
	}
	if description := parseCompanyRepresentation(resp.Content); description != "" {
		return description
	}
	return fallback
}

// parseCompanyRepresentation extracts the distilled description from the model's
// JSON output ({"description": "..."}). If the output is not JSON (e.g. the call ran
// against an older non-JSON prompt and the model returned plain text), it falls back
// to the raw content. Returns "" when nothing usable is found.
func parseCompanyRepresentation(content string) string {
	if raw := extractJSONObject(content); raw != "" {
		var obj struct {
			Description string `json:"description"`
		}
		if err := json.Unmarshal([]byte(raw), &obj); err == nil {
			if d := cleanText(obj.Description, maCompanyDescriptionCap); d != "" {
				return d
			}
		}
	}
	return cleanText(content, maCompanyDescriptionCap)
}

func buildCompanyEvidenceText(evidence maCompanyEvidence) string {
	parts := make([]string, 0, len(evidence.Snippets))
	for _, snippet := range evidence.Snippets {
		if cleaned := cleanText(snippet, 360); cleaned != "" {
			parts = append(parts, cleaned)
		}
	}
	return cleanText(strings.Join(parts, " "), maCompanyEvidenceTextCap)
}

func (s *maService) loadSectorEmbedInstruction(ctx context.Context) string {
	return s.loadInstructionOrDefault(ctx, maModelScopeSectorEmbed, maSectorEmbedDefaultInstruct)
}

func (s *maService) loadSectorRerankInstruction(ctx context.Context) string {
	return s.loadInstructionOrDefault(ctx, maModelScopeSectorClassification, maSectorRerankDefaultInstruct)
}

func (s *maService) loadInstructionOrDefault(ctx context.Context, scope, fallback string) string {
	if s.llmp == nil {
		return fallback
	}
	prompt, err := s.llmp.ResolvePrompt(ctx, scope, "")
	if err != nil || strings.TrimSpace(prompt.Prompt) == "" {
		return fallback
	}
	return strings.TrimSpace(prompt.Prompt)
}

// analyzeSectorAmbiguity is the LLM tie-breaker, invoked ONLY when the
// deterministic concept verdict is ambiguous/no_signal (decision #4). It is fed the
// distilled company description, the ranked concept matches (with kind + in-perimeter
// flags) and the neutral evidence — never strategy-seeded keywords — and returns the
// existing analyst response contract. Best-effort: the caller degrades to
// needs_business_validation when this errors.
func (s *maService) analyzeSectorAmbiguity(ctx context.Context, target MATarget, strategy MAStrategySpec, evidence maCompanyEvidence, class maSectorClassification, subject, email string) (CandidateMatchAnalysisResponse, error) {
	if s.llmp == nil {
		return CandidateMatchAnalysisResponse{}, errMAOpenRouterUnavailable
	}
	model, err := s.llmp.ResolveModel(ctx, maModelScopeCandidateMatchAnalyst, "")
	if err != nil {
		return CandidateMatchAnalysisResponse{}, llmConfigError(err)
	}
	prompt, err := s.llmp.ResolvePrompt(ctx, maModelScopeCandidateMatchAnalyst, "")
	if err != nil {
		return CandidateMatchAnalysisResponse{}, llmConfigError(err)
	}
	client, err := s.llmp.ClientForModel(ctx, model)
	if err != nil {
		return CandidateMatchAnalysisResponse{}, llmConfigError(err)
	}

	concepts := make([]map[string]any, 0, len(class.Concepts))
	for i, c := range class.Concepts {
		if i >= 6 {
			break
		}
		concepts = append(concepts, map[string]any{
			"name":       c.Name,
			"kind":       c.Kind,
			"rerankProb": c.RerankProb,
			"inStrategy": c.InStrategy,
		})
	}
	snippets := evidence.Snippets
	if len(snippets) > 12 {
		snippets = snippets[:12]
	}
	curated := map[string]any{
		"company": map[string]any{
			"name":               target.CompanyName,
			"atecoDescription":   target.AtecoDescription,
			"matchState":         target.MatchState,
			"deterministicScore": target.Score,
			"selfDescription":    class.CompanyDescription,
		},
		"strategy": map[string]any{
			"sector":            cleanText(strategy.SectorDescription, 400),
			"perimeterConcepts": class.StrategyConcepts,
		},
		"conceptMatches": concepts,
		"webEvidence":    snippets,
		"instructions": map[string]any{
			"doNotBrowse":              true,
			"decideSectorMembership":   true,
			"distractorMeansOffTarget": true,
		},
	}
	payload, err := json.Marshal(curated)
	if err != nil {
		return CandidateMatchAnalysisResponse{}, err
	}
	reqParams := model.RawParams()
	if _, ok := reqParams["max_tokens"]; !ok {
		reqParams["max_tokens"] = candidateMatchAnalysisMaxToken
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
	resp, chatErr := client.Chat(ctx, chatReq)
	usageRaw, _ := json.Marshal(resp.Usage)
	requestBody, _ := llm.BuildRequestBody(chatReq)
	requestRaw, _ := json.Marshal(requestBody)
	contextRaw, _ := json.Marshal(map[string]any{
		"target_id":   target.ID,
		"session_id":  target.SessionID,
		"company_key": target.CompanyKey,
		"verdict":     string(class.Verdict),
	})
	audit := llm.CallAudit{
		App:          maApp,
		Scope:        maModelScopeCandidateMatchAnalyst,
		ProviderID:   model.ProviderID,
		ModelID:      model.ID,
		PromptID:     prompt.ID,
		Model:        model.Model,
		Request:      requestRaw,
		Usage:        usageRaw,
		Context:      contextRaw,
		ActorSubject: subject,
		ActorEmail:   email,
	}
	if chatErr != nil {
		audit.Status = "failed"
		audit.ErrorMessage = chatErr.Error()
	} else if respRaw, mErr := json.Marshal(map[string]any{"content": resp.Content}); mErr == nil {
		audit.Response = respRaw
	}
	_ = s.llmp.RecordAudit(ctx, audit)
	if chatErr != nil {
		return CandidateMatchAnalysisResponse{}, chatErr
	}

	analysis, err := parseCandidateMatchAnalysis(resp.Content)
	if err != nil {
		return CandidateMatchAnalysisResponse{}, err
	}
	analysis.ModelID = model.ID
	analysis.PromptID = prompt.ID
	analysis.Model = model.Model
	return analysis, nil
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
