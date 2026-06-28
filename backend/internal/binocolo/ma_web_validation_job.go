package binocolo

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/brave"
)

const (
	maWebValidationDefaultLimit       = 25
	maWebValidationMaxLimit           = 100
	maWebValidationDefaultDomainCount = 20
	// maWebValidationEvidenceCount is how many results each NEUTRAL self-description
	// probe pulls from the company's own site (no strategy terms — UC2 gathers what
	// the company says about itself, then classifies, instead of confirming the
	// strategy's expectations).
	maWebValidationEvidenceCount = 5
)

// maNeutralEvidenceProbes are the deliberately strategy-agnostic site queries used
// to surface a company's self-description. They never mention the strategy sector,
// so the evidence cannot be confirmation-biased toward the expected answer.
var maNeutralEvidenceProbes = []string{"chi siamo", "servizi soluzioni", "cosa facciamo"}

type maWebValidationJobPayload struct {
	Limit              int  `json:"limit"`
	Force              bool `json:"force"`
	IncludeIdentifiers bool `json:"includeIdentifiers"`
	AnalyzeWithLLM     bool `json:"analyzeWithLLM"`
	DomainCount        int  `json:"domainCount"`
	KeywordCount       int  `json:"keywordCount"`
	Rank               bool `json:"rank"`
}

func (s *maService) enqueueWebValidation(ctx context.Context, sessionID string, req MAWebValidationEnrichRequest, subject, email string) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
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
	if detail.Strategy == nil {
		return MASessionDetail{}, fmt.Errorf("%w: strategy", errMAStrategyInvalid)
	}
	if len(detail.Targets) == 0 {
		return MASessionDetail{}, fmt.Errorf("%w: targets required", errMAStrategyInvalid)
	}
	payload := normalizeMAWebValidationPayload(req)
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return MASessionDetail{}, fmt.Errorf("marshal ma web validation payload: %w", err)
	}
	if _, err := s.store.EnqueueMAJob(ctx, maJobEnqueue{
		JobType:           maJobTypeWebValidation,
		SessionID:         sessionID,
		StrategyVersionID: detail.Strategy.ID,
		Status:            maJobStatusPending,
		Subject:           subject,
		Email:             email,
		Payload:           rawPayload,
	}); err != nil {
		return MASessionDetail{}, err
	}
	return s.getSession(ctx, sessionID)
}

func (s *maService) runWebValidationJob(ctx context.Context, job maJob) (string, error) {
	trace, err := s.startTrace(ctx, maTraceStart{
		Operation:        "ma_session_web_validation",
		SessionID:        job.SessionID,
		CreatedBySubject: job.CreatedBySubject,
		CreatedByEmail:   job.CreatedByEmail,
		Request:          job.Payload,
	})
	if err != nil {
		return "", err
	}
	ctx = withMATrace(ctx, trace)
	if workErr := s.webValidationJobWork(ctx, job); workErr != nil {
		_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusFailed, ErrorMessage: workErr.Error()})
		return trace.id, workErr
	}
	_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusSucceeded})
	return trace.id, nil
}

func (s *maService) webValidationJobWork(ctx context.Context, job maJob) error {
	if s.brave == nil {
		return errMABraveUnavailable
	}
	payload := maWebValidationJobPayload{
		Limit:          maWebValidationDefaultLimit,
		AnalyzeWithLLM: true,
		DomainCount:    maWebValidationDefaultDomainCount,
		KeywordCount:   maWebValidationEvidenceCount,
		Rank:           true,
	}
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decode ma web validation payload: %w", err)
		}
	}
	payload = normalizeMAWebValidationPayload(MAWebValidationEnrichRequest{
		Limit:              payload.Limit,
		Force:              payload.Force,
		IncludeIdentifiers: payload.IncludeIdentifiers,
		AnalyzeWithLLM:     boolPtr(payload.AnalyzeWithLLM),
		DomainCount:        payload.DomainCount,
		KeywordCount:       payload.KeywordCount,
		Rank:               boolPtr(payload.Rank),
	})

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
	if err := s.linkTrace(ctx, maTraceLink{SessionID: job.SessionID, StrategyVersionID: version.ID}); err != nil {
		return err
	}

	targets := selectMAWebValidationTargets(detail.Targets, version.Strategy, payload, s.now(), payload.Limit)
	processed := 0
	skipped := 0
	failed := 0
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_web_validation_started",
		Status:    maTraceEventStarted,
		Metadata: maTraceJSON(map[string]any{
			"session_id":          job.SessionID,
			"strategy_version_id": version.ID,
			"candidate_count":     len(targets),
			"force":               payload.Force,
			"limit":               payload.Limit,
			"method":              "concept",
		}),
	})

	for _, work := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		target := work.Target
		if !payload.Force && reusableMAWebValidation(target.WebValidation, work.InputHash, work.InputHash, s.now()) {
			skipped++
			continue
		}

		body, buildErr := s.buildMAWebValidation(ctx, target, version.Strategy, work.InputHash, payload, job.CreatedBySubject, job.CreatedByEmail)
		if buildErr != nil {
			body = failedMAWebValidationRequest(target, work.InputHash, buildErr)
			failed++
		}
		if _, err := s.upsertTargetWebValidation(ctx, job.SessionID, body, job.CreatedBySubject, job.CreatedByEmail); err != nil {
			return err
		}
		processed++
	}

	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_web_validation_completed",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"session_id":   job.SessionID,
			"processed":    processed,
			"skipped":      skipped,
			"failed":       failed,
			"target_count": len(targets),
		}),
	})
	return nil
}

// buildMAWebValidation runs the UC2 concept pipeline for one target: resolve the
// official domain (kept), gather NEUTRAL self-description evidence, classify it
// against the curated KB concepts (embedding + reranker, Step 2), and — only when
// the deterministic verdict is ambiguous/no_signal — escalate to the LLM analyst.
// The result maps onto the existing MAWebValidation contract.
func (s *maService) buildMAWebValidation(
	ctx context.Context,
	target MATarget,
	strategy MAStrategySpec,
	inputHash string,
	payload maWebValidationJobPayload,
	subject string,
	email string,
) (MAWebValidationUpsertRequest, error) {
	domainResponse, err := s.resolveDomainCandidates(ctx, DomainResolutionRequest{
		CompanyName: target.CompanyName,
		VATCode:     optionalIdentifier(payload.IncludeIdentifiers, target.VATCode),
		TaxCode:     optionalIdentifier(payload.IncludeIdentifiers, target.TaxCode),
		Town:        target.Town,
		Province:    target.Province,
		Count:       payload.DomainCount,
	})
	if err != nil {
		return MAWebValidationUpsertRequest{}, fmt.Errorf("domain resolution: %w", err)
	}
	selectedDomain := chooseMAWebValidationDomain(domainResponse.Candidates)
	if selectedDomain == nil {
		decision := &CandidateMatchFinalDecision{
			InitialMatchState:  target.MatchState,
			DeterministicScore: target.Score,
			WebValidationState: "domain_unresolved",
			FinalAction:        "needs_domain_review",
			Confidence:         "bassa",
			Reason:             "Nessun dominio ufficiale credibile risolto.",
			Reasons:            []string{"Nessun dominio ufficiale credibile risolto."},
		}
		return s.assembleWebValidationRequest(target, inputHash, domainResponse, nil, nil, maSectorClassification{}, nil, "", decision), nil
	}

	evidence, evidenceRuns := s.gatherNeutralEvidence(ctx, selectedDomain.Domain, payload.KeywordCount, subject, email)

	classification, classErr := s.classifyCompanySector(ctx, evidence, strategy, subject, email)
	classError := ""
	if classErr != nil {
		// Embedder/KB unavailable: cannot classify. Surface as analysis_unavailable so
		// the analyst reviews it; never silently confirm/reject.
		classError = cleanText(classErr.Error(), 180)
		decision := &CandidateMatchFinalDecision{
			InitialMatchState:  target.MatchState,
			DeterministicScore: target.Score,
			WebValidationState: "analysis_unavailable",
			FinalAction:        "needs_business_validation",
			Confidence:         "bassa",
			Reason:             "Classificazione settore non disponibile (embedder/KB).",
			Reasons:            []string{"Classificazione settore non disponibile: " + classError},
		}
		return s.assembleWebValidationRequest(target, inputHash, domainResponse, selectedDomain, evidenceRuns, maSectorClassification{}, nil, classError, decision), nil
	}

	var analysis *CandidateMatchAnalysisResponse
	analysisErr := ""
	if payload.AnalyzeWithLLM && sectorVerdictNeedsLLM(classification.Verdict) {
		result, err := s.analyzeSectorAmbiguity(ctx, target, strategy, evidence, classification, subject, email)
		if err != nil {
			analysisErr = cleanText(err.Error(), 180)
		} else {
			analysis = &result
		}
	}

	decision := sectorFinalDecision(target, classification, analysis, analysisErr)
	s.recordSectorClassificationTrace(ctx, target, classification, decision, analysis != nil)
	return s.assembleWebValidationRequest(target, inputHash, domainResponse, selectedDomain, evidenceRuns, classification, analysis, analysisErr, decision), nil
}

func sectorVerdictNeedsLLM(verdict maSectorVerdict) bool {
	return verdict == maSectorAmbiguous || verdict == maSectorNoSignal
}

// gatherNeutralEvidence collects a company's self-description from its own site
// using strategy-agnostic probes. Returns the evidence corpus (for classification)
// and per-probe runs (for the persisted contract / UI).
func (s *maService) gatherNeutralEvidence(ctx context.Context, domain string, count int, subject, email string) (maCompanyEvidence, []CandidateMatchEvidenceRun) {
	evidence := maCompanyEvidence{Domain: domain}
	runs := make([]CandidateMatchEvidenceRun, 0, len(maNeutralEvidenceProbes))
	seen := map[string]struct{}{}
	add := func(text string) {
		cleaned := cleanText(text, 360)
		if cleaned == "" {
			return
		}
		key := strings.ToLower(cleaned)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		evidence.Snippets = append(evidence.Snippets, cleaned)
	}
	for _, probe := range maNeutralEvidenceProbes {
		run := CandidateMatchEvidenceRun{Bucket: "neutral", Term: probe}
		response, err := s.searchDomainEvidence(ctx, WebSearchRequest{
			Domain:   domain,
			Keywords: strings.Fields(probe),
			Count:    count,
			Rank:     false,
		}, subject, email)
		if err != nil {
			run.Error = cleanText(err.Error(), 180)
		} else {
			responseCopy := response
			run.Response = &responseCopy
			run.ResultCount = len(response.Results)
			run.Matched = run.ResultCount > 0
			for _, result := range response.Results {
				add(result.Title)
				for _, snippet := range result.Snippets {
					add(snippet)
				}
			}
		}
		runs = append(runs, run)
	}
	return evidence, runs
}

// sectorFinalDecision maps the concept verdict (+ optional analyst override) onto
// the MAWebValidation lifecycle, with NO magic thresholds — the verdict already
// carries the calibrated decision. The analyst only matters when the deterministic
// verdict was ambiguous/no_signal.
func sectorFinalDecision(target MATarget, class maSectorClassification, analysis *CandidateMatchAnalysisResponse, analysisErr string) *CandidateMatchFinalDecision {
	webScore := clampScore(class.TopProb * 100)
	reasons := []string{}
	if class.Reason != "" {
		reasons = append(reasons, class.Reason)
	}
	reasons = append(reasons, sectorConceptReasons(class)...)

	state, action, confidence := "unclear", "needs_business_validation", class.Confidence
	switch class.Verdict {
	case maSectorConfirm:
		state, action = "confirmed", "confirm"
	case maSectorReject:
		state, action = "rejected", "reject"
	case maSectorWeak:
		state, action = "deprioritized", "deprioritize"
	case maSectorAmbiguous, maSectorNoSignal:
		if analysis != nil {
			state, action, confidence = analystToLifecycle(analysis)
			if analysis.Verdict != "" {
				reasons = append([]string{"Giudizio LLM: " + analysis.Verdict + " (" + analysis.RecommendedAction + ")."}, reasons...)
			}
		} else {
			state = "analysis_unavailable"
			action = "needs_business_validation"
			if confidence == "" {
				confidence = "bassa"
			}
			if strings.TrimSpace(analysisErr) != "" {
				reasons = append([]string{"Analyst non disponibile: " + analysisErr}, reasons...)
			} else {
				reasons = append([]string{"Classificazione non decisiva: validazione business richiesta."}, reasons...)
			}
		}
	}
	if confidence == "" {
		confidence = "bassa"
	}

	decision := &CandidateMatchFinalDecision{
		InitialMatchState:  target.MatchState,
		DeterministicScore: target.Score,
		WebScore:           webScore,
		WebValidationState: state,
		FinalAction:        action,
		Confidence:         confidence,
		Reason:             firstNonEmpty(class.Reason, "Classificazione settore concept-based."),
		Reasons:            cleanStringList(reasons, 12, 280),
	}
	if analysis != nil {
		decision.AnalystVerdict = analysis.Verdict
		decision.AnalystAction = analysis.RecommendedAction
	}
	return decision
}

func analystToLifecycle(analysis *CandidateMatchAnalysisResponse) (state, action, confidence string) {
	confidence = analysis.Confidence
	if confidence == "" {
		confidence = "media"
	}
	switch {
	case analysis.RecommendedAction == "confirm" && (analysis.Verdict == "strong_match" || analysis.Verdict == "match"):
		return "confirmed", "confirm", confidence
	case analysis.RecommendedAction == "reject" || analysis.Verdict == "no_match":
		return "rejected", "reject", confidence
	case analysis.RecommendedAction == "downgrade" || analysis.Verdict == "weak_match":
		return "deprioritized", "deprioritize", confidence
	default:
		return "unclear", "needs_business_validation", confidence
	}
}

func sectorConceptReasons(class maSectorClassification) []string {
	out := []string{}
	for i, c := range class.Concepts {
		if i >= 3 {
			break
		}
		tag := "target"
		if c.Kind == "distractor" {
			tag = "distrattore"
		}
		scope := ""
		if c.InStrategy {
			scope = ", in perimetro"
		}
		out = append(out, fmt.Sprintf("Concetto %s (%s%s): prob %.2f", c.Name, tag, scope, c.RerankProb))
	}
	return out
}

// assembleWebValidationRequest maps the classification + evidence onto the existing
// MAWebValidation contract. The KeywordSet field is repurposed honestly: matched
// target concepts -> CoreTerms, matched distractors -> NegativeTerms (no hardcoded
// negative dogma). Full concept provenance lives in the trace event.
func (s *maService) assembleWebValidationRequest(
	target MATarget,
	inputHash string,
	domainResponse DomainResolutionResponse,
	selectedDomain *DomainResolutionCandidate,
	evidenceRuns []CandidateMatchEvidenceRun,
	class maSectorClassification,
	analysis *CandidateMatchAnalysisResponse,
	analysisErr string,
	decision *CandidateMatchFinalDecision,
) MAWebValidationUpsertRequest {
	if evidenceRuns == nil {
		evidenceRuns = []CandidateMatchEvidenceRun{}
	}
	keywordSet := classificationToKeywordSet(class)
	summary := classificationToSummary(class, decision)
	return MAWebValidationUpsertRequest{
		PipelineVersion:        maWebValidationPipelineVersion,
		InputHash:              inputHash,
		KeywordSetHash:         inputHash,
		Target:                 target,
		KeywordSet:             keywordSet,
		DomainResponse:         domainResponse,
		SelectedDomain:         selectedDomain,
		EvidenceRuns:           evidenceRuns,
		Summary:                summary,
		CandidateMatchAnalysis: analysis,
		CandidateMatchError:    analysisErr,
		FinalDecision:          *decision,
	}
}

func classificationToKeywordSet(class maSectorClassification) CandidateMatchKeywordSet {
	core := []string{}
	negative := []string{}
	for _, c := range class.Concepts {
		if c.Kind == "distractor" {
			negative = append(negative, c.Name)
		} else {
			core = append(core, c.Name)
		}
	}
	intent := strings.TrimSpace(class.CompanyDescription)
	if intent == "" {
		intent = "Classificazione settore concept-based"
	}
	sources := []string{}
	if len(class.StrategyConcepts) > 0 {
		sources = append(sources, "perimetro strategia: "+strings.Join(class.StrategyConcepts, ", "))
	}
	return CandidateMatchKeywordSet{
		IntentLabel:   cleanText(intent, 220),
		CoreTerms:     cleanStringList(core, 12, 120),
		AdjacentTerms: []string{},
		NegativeTerms: cleanStringList(negative, 12, 120),
		Sources:       cleanStringList(sources, 6, 220),
	}
}

func classificationToSummary(class maSectorClassification, decision *CandidateMatchFinalDecision) CandidateMatchEvidenceSummary {
	coreMatches, negativeMatches := 0, 0
	for _, c := range class.Concepts {
		if c.Kind == "distractor" {
			negativeMatches++
		} else {
			coreMatches++
		}
	}
	score := 0
	if decision != nil {
		score = decision.WebScore
	}
	return CandidateMatchEvidenceSummary{
		Score:           score,
		Confidence:      class.Confidence,
		CoreMatches:     coreMatches,
		NegativeMatches: negativeMatches,
		TotalCoreTerms:  coreMatches,
	}
}

func (s *maService) recordSectorClassificationTrace(ctx context.Context, target MATarget, class maSectorClassification, decision *CandidateMatchFinalDecision, analystUsed bool) {
	concepts := make([]map[string]any, 0, len(class.Concepts))
	for i, c := range class.Concepts {
		if i >= 6 {
			break
		}
		concepts = append(concepts, map[string]any{
			"concept_id":  c.ID,
			"name":        c.Name,
			"kind":        c.Kind,
			"cosine":      math.Round(c.Cosine*10000) / 10000,
			"rerank_prob": math.Round(c.RerankProb*10000) / 10000,
			"in_strategy": c.InStrategy,
		})
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_sector_classification",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"company_key":       target.CompanyKey,
			"company_name":      target.CompanyName,
			"verdict":           string(class.Verdict),
			"final_action":      decision.FinalAction,
			"top_prob":          math.Round(class.TopProb*10000) / 10000,
			"rerank_applied":    class.RerankApplied,
			"analyst_used":      analystUsed,
			"strategy_concepts": class.StrategyConcepts,
		}),
		Response: maTraceJSON(map[string]any{
			"description": class.CompanyDescription,
			"concepts":    concepts,
		}),
	})
}

func (s *maService) resolveDomainCandidates(ctx context.Context, body DomainResolutionRequest) (DomainResolutionResponse, error) {
	if s.brave == nil {
		return DomainResolutionResponse{}, errMABraveUnavailable
	}
	companyName := strings.Join(strings.Fields(strings.TrimSpace(body.CompanyName)), " ")
	if companyName == "" {
		return DomainResolutionResponse{}, fmt.Errorf("%w: company name", errMAStrategyInvalid)
	}
	count := body.Count
	if count <= 0 {
		count = domainResolutionDefaultCount
	}
	if count > domainResolutionMaxCount {
		count = domainResolutionMaxCount
	}

	queries := buildDomainResolutionQueries(body, companyName)
	results := []WebSearchResult{}
	seenResults := map[string]struct{}{}
	executedQueries := []string{}
	candidates := []DomainResolutionCandidate{}
	for _, query := range queries {
		if len(query) > webSearchMaxQueryLen || len(strings.Fields(query)) > webSearchMaxWords {
			continue
		}
		executedQueries = append(executedQueries, query)
		res, err := s.brave.LLMContext(ctx, brave.LLMContextParams{
			Query:      query,
			Count:      count,
			Country:    "it",
			SearchLang: "it",
		})
		if err != nil {
			return DomainResolutionResponse{}, err
		}
		for _, g := range res.Generic {
			host := resultHostname(g.URL, res.Sources)
			if strings.TrimSpace(host) == "" {
				continue
			}
			if _, exists := seenResults[g.URL]; exists {
				continue
			}
			seenResults[g.URL] = struct{}{}
			results = append(results, WebSearchResult{
				Title:    g.Title,
				URL:      g.URL,
				Hostname: host,
				Age:      firstAge(res.Sources[g.URL].Age),
				Snippets: g.Snippets,
			})
		}
		candidates = rankDomainResolutionCandidates(body, companyName, results)
		if hasCredibleDomainResolutionCandidate(candidates) {
			break
		}
	}
	if len(executedQueries) == 0 {
		return DomainResolutionResponse{}, fmt.Errorf("%w: query too long", errMAStrategyInvalid)
	}
	return DomainResolutionResponse{
		Query:      strings.Join(executedQueries, " | "),
		Count:      count,
		Candidates: candidates,
		Results:    results,
	}, nil
}

func (s *maService) searchDomainEvidence(ctx context.Context, body WebSearchRequest, subject, email string) (WebSearchResponse, error) {
	if s.brave == nil {
		return WebSearchResponse{}, errMABraveUnavailable
	}
	domain, ok := normalizeDomain(body.Domain)
	if !ok {
		return WebSearchResponse{}, fmt.Errorf("%w: domain", errMAStrategyInvalid)
	}
	keywords := cleanStringList(body.Keywords, 20, 120)
	if len(keywords) == 0 {
		return WebSearchResponse{}, fmt.Errorf("%w: keywords", errMAStrategyInvalid)
	}
	count := body.Count
	if count <= 0 {
		count = webSearchDefaultCount
	}
	if count > webSearchMaxCount {
		count = webSearchMaxCount
	}
	query := "site:" + domain + " " + strings.Join(keywords, " ")
	if len(query) > webSearchMaxQueryLen || len(strings.Fields(query)) > webSearchMaxWords {
		return WebSearchResponse{}, fmt.Errorf("%w: query too long", errMAStrategyInvalid)
	}

	res, err := s.brave.LLMContext(ctx, brave.LLMContextParams{
		Query:      query,
		Count:      count,
		Country:    "it",
		SearchLang: "it",
	})
	if err != nil {
		return WebSearchResponse{}, err
	}
	results := make([]WebSearchResult, 0, len(res.Generic))
	for _, g := range res.Generic {
		host := resultHostname(g.URL, res.Sources)
		if !hostMatchesDomain(host, domain) {
			continue
		}
		results = append(results, WebSearchResult{
			Title:    g.Title,
			URL:      g.URL,
			Hostname: host,
			Age:      firstAge(res.Sources[g.URL].Age),
			Snippets: g.Snippets,
		})
	}

	ranked := false
	rankErr := ""
	if body.Rank && len(results) > 0 {
		scores, scoreErr := s.scoreWebSearchResults(ctx, strings.Join(keywords, " "), results, subject, email)
		if scoreErr != nil {
			rankErr = cleanText(scoreErr.Error(), 180)
		} else {
			for i := range results {
				if sc, ok := scores[i]; ok {
					value := sc
					results[i].Score = &value
				}
			}
			sort.SliceStable(results, func(a, b int) bool {
				return scoreOf(results[a]) > scoreOf(results[b])
			})
			ranked = true
		}
	}
	return WebSearchResponse{Query: query, Count: count, Ranked: ranked, RankError: rankErr, Results: results}, nil
}

func normalizeMAWebValidationPayload(req MAWebValidationEnrichRequest) maWebValidationJobPayload {
	analyzeWithLLM := true
	if req.AnalyzeWithLLM != nil {
		analyzeWithLLM = *req.AnalyzeWithLLM
	}
	rank := true
	if req.Rank != nil {
		rank = *req.Rank
	}
	limit := req.Limit
	if limit <= 0 {
		limit = maWebValidationDefaultLimit
	}
	if limit > maWebValidationMaxLimit {
		limit = maWebValidationMaxLimit
	}
	domainCount := req.DomainCount
	if domainCount <= 0 {
		domainCount = maWebValidationDefaultDomainCount
	}
	if domainCount > domainResolutionMaxCount {
		domainCount = domainResolutionMaxCount
	}
	keywordCount := req.KeywordCount
	if keywordCount <= 0 {
		keywordCount = maWebValidationEvidenceCount
	}
	if keywordCount > webSearchMaxCount {
		keywordCount = webSearchMaxCount
	}
	return maWebValidationJobPayload{
		Limit:              limit,
		Force:              req.Force,
		IncludeIdentifiers: req.IncludeIdentifiers,
		AnalyzeWithLLM:     analyzeWithLLM,
		DomainCount:        domainCount,
		KeywordCount:       keywordCount,
		Rank:               rank,
	}
}

type maWebValidationWorkTarget struct {
	Target    MATarget
	InputHash string
}

func selectMAWebValidationTargets(targets []MATarget, strategy MAStrategySpec, payload maWebValidationJobPayload, now time.Time, limit int) []maWebValidationWorkTarget {
	out := make([]maWebValidationWorkTarget, 0, len(targets))
	for _, target := range targets {
		if target.MatchState == maMatchStateOutside {
			continue
		}
		inputHash := maWebValidationInputHash(target, strategy, payload)
		if !payload.Force && reusableMAWebValidation(target.WebValidation, inputHash, inputHash, now) {
			continue
		}
		out = append(out, maWebValidationWorkTarget{
			Target:    target,
			InputHash: inputHash,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri := targetRatingValue(out[i].Target)
		rj := targetRatingValue(out[j].Target)
		if ri != rj {
			return ri > rj
		}
		if out[i].Target.Score != out[j].Target.Score {
			return out[i].Target.Score > out[j].Target.Score
		}
		return out[i].Target.CompanyName < out[j].Target.CompanyName
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func targetRatingValue(target MATarget) int {
	if target.Rating == nil {
		return 0
	}
	return *target.Rating
}

func reusableMAWebValidation(validation *MAWebValidation, inputHash, keywordSetHash string, now time.Time) bool {
	return validation != nil &&
		validation.PipelineVersion == maWebValidationPipelineVersion &&
		validation.InputHash == inputHash &&
		validation.KeywordSetHash == keywordSetHash &&
		maWebValidationFreshness(now, validation.StaleAfter, validation.ExpiresAt) == maWebValidationFresh
}

func failedMAWebValidationRequest(target MATarget, inputHash string, err error) MAWebValidationUpsertRequest {
	errorText := cleanText(err.Error(), 180)
	decision := &CandidateMatchFinalDecision{
		InitialMatchState:  target.MatchState,
		DeterministicScore: target.Score,
		WebValidationState: "analysis_unavailable",
		FinalAction:        "needs_business_validation",
		Confidence:         "bassa",
		Reason:             "Validazione web fallita.",
		Reasons:            []string{"Errore pipeline: " + errorText},
	}
	return MAWebValidationUpsertRequest{
		PipelineVersion:     maWebValidationPipelineVersion,
		InputHash:           inputHash,
		KeywordSetHash:      inputHash,
		Target:              target,
		KeywordSet:          CandidateMatchKeywordSet{IntentLabel: "Validazione web fallita"},
		DomainResponse:      DomainResolutionResponse{},
		EvidenceRuns:        []CandidateMatchEvidenceRun{},
		Summary:             CandidateMatchEvidenceSummary{Confidence: "bassa"},
		CandidateMatchError: errorText,
		FinalDecision:       *decision,
	}
}

func chooseMAWebValidationDomain(candidates []DomainResolutionCandidate) *DomainResolutionCandidate {
	credibleReasons := map[string]struct{}{
		"host compatibile con ragione sociale": {},
		"dominio citato come sameAs":           {},
		"dominio citato come sito ufficiale":   {},
		"email aziendale su dominio":           {},
	}
	for _, candidate := range candidates {
		if candidate.Confidence == "bassa" || candidate.Score < 45 {
			continue
		}
		for _, reason := range candidate.Reasons {
			if _, ok := credibleReasons[reason]; ok {
				copyCandidate := candidate
				return &copyCandidate
			}
		}
	}
	return nil
}

func maWebValidationInputHash(target MATarget, strategy MAStrategySpec, payload maWebValidationJobPayload) string {
	return maWebValidationHash(map[string]any{
		"pipelineVersion": maWebValidationPipelineVersion,
		"target":          maWebValidationTargetFingerprint(target),
		"sector":          cleanText(strategy.SectorDescription, 400),
		"options":         maWebValidationInputOptions(payload),
	})
}

func maWebValidationInputOptions(payload maWebValidationJobPayload) map[string]any {
	return map[string]any{
		"analyzeWithLLM":     payload.AnalyzeWithLLM,
		"domainCount":        payload.DomainCount,
		"includeIdentifiers": payload.IncludeIdentifiers,
		"keywordCount":       payload.KeywordCount,
		"rank":               payload.Rank,
	}
}

func clampScore(value float64) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return int(value + 0.5)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func optionalIdentifier(enabled bool, value string) string {
	if !enabled {
		return ""
	}
	return value
}

func boolPtr(value bool) *bool {
	return &value
}
