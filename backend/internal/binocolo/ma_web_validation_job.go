package binocolo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/brave"
)

const (
	maWebValidationDefaultLimit        = 25
	maWebValidationMaxLimit            = 100
	maWebValidationDefaultDomainCount  = 10
	maWebValidationDefaultKeywordCount = 5
	maWebValidationMaxKeywordCount     = 20
)

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
		KeywordCount:   maWebValidationDefaultKeywordCount,
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

	targets := selectMAWebValidationTargets(detail.Targets, payload.Limit)
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
		}),
	})

	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		keywordSet := deriveMAWebValidationKeywordSet(version.Strategy, target)
		keywordSetHash := maWebValidationHash(maWebValidationKeywordSetFingerprint(keywordSet))
		inputHash := maWebValidationInputHash(target, keywordSet, payload)
		if !payload.Force && reusableMAWebValidation(target.WebValidation, inputHash, keywordSetHash, s.now()) {
			skipped++
			continue
		}

		body, buildErr := s.buildMAWebValidation(ctx, target, version.Strategy, keywordSet, inputHash, keywordSetHash, payload, job.CreatedBySubject, job.CreatedByEmail)
		if buildErr != nil {
			body = failedMAWebValidationRequest(target, keywordSet, inputHash, keywordSetHash, buildErr)
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

func (s *maService) buildMAWebValidation(
	ctx context.Context,
	target MATarget,
	strategy MAStrategySpec,
	keywordSet CandidateMatchKeywordSet,
	inputHash string,
	keywordSetHash string,
	payload maWebValidationJobPayload,
	subject string,
	email string,
) (MAWebValidationUpsertRequest, error) {
	domainKeywords := append([]string{}, keywordSet.CoreTerms[:min(3, len(keywordSet.CoreTerms))]...)
	domainKeywords = append(domainKeywords, keywordSet.AdjacentTerms[:min(2, len(keywordSet.AdjacentTerms))]...)
	domainResponse, err := s.resolveDomainCandidates(ctx, DomainResolutionRequest{
		CompanyName: target.CompanyName,
		VATCode:     optionalIdentifier(payload.IncludeIdentifiers, target.VATCode),
		TaxCode:     optionalIdentifier(payload.IncludeIdentifiers, target.TaxCode),
		Town:        target.Town,
		Province:    target.Province,
		Keywords:    domainKeywords,
		Count:       payload.DomainCount,
	})
	if err != nil {
		return MAWebValidationUpsertRequest{}, fmt.Errorf("domain resolution: %w", err)
	}

	selectedDomain := chooseMAWebValidationDomain(domainResponse.Candidates)
	evidenceRuns := []CandidateMatchEvidenceRun{}
	if selectedDomain != nil {
		for _, spec := range maWebValidationTermSpecs(keywordSet) {
			run := CandidateMatchEvidenceRun{Bucket: spec.Bucket, Term: spec.Term}
			response, searchErr := s.searchDomainEvidence(ctx, WebSearchRequest{
				Domain:   selectedDomain.Domain,
				Keywords: []string{spec.Term},
				Count:    payload.KeywordCount,
				Rank:     payload.Rank,
			}, subject, email)
			if searchErr != nil {
				run.Error = cleanText(searchErr.Error(), 180)
			} else {
				responseCopy := response
				run.Response = &responseCopy
				run.ResultCount = len(response.Results)
				run.BestScore = bestMAWebSearchScore(response)
				run.Matched = maEvidenceResponseMatched(response)
			}
			evidenceRuns = append(evidenceRuns, run)
		}
	}

	summary := computeMAWebValidationSummary(selectedDomain, keywordSet, evidenceRuns)
	analysisInput := CandidateMatchAnalysisRequest{
		Target:         target,
		KeywordSet:     keywordSet,
		DomainResponse: domainResponse,
		SelectedDomain: selectedDomain,
		EvidenceRuns:   evidenceRuns,
		Summary:        summary,
	}
	var analysis *CandidateMatchAnalysisResponse
	analysisErr := ""
	if selectedDomain != nil && payload.AnalyzeWithLLM {
		result, err := s.analyzeCandidateMatch(ctx, analysisInput, subject, email)
		if err != nil {
			analysisErr = cleanText(err.Error(), 180)
		} else {
			analysis = &result
		}
	}
	finalDecision := reconcileCandidateMatchDecision(analysisInput, analysis, analysisErr)
	if analysis != nil && analysis.FinalDecision != nil {
		finalDecision = analysis.FinalDecision
	}

	return MAWebValidationUpsertRequest{
		PipelineVersion:        maWebValidationPipelineVersion,
		InputHash:              inputHash,
		KeywordSetHash:         keywordSetHash,
		Target:                 target,
		KeywordSet:             keywordSet,
		DomainResponse:         domainResponse,
		SelectedDomain:         selectedDomain,
		EvidenceRuns:           evidenceRuns,
		Summary:                summary,
		CandidateMatchAnalysis: analysis,
		CandidateMatchError:    analysisErr,
		FinalDecision:          *finalDecision,
	}, nil
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
		keywordCount = maWebValidationDefaultKeywordCount
	}
	if keywordCount > maWebValidationMaxKeywordCount {
		keywordCount = maWebValidationMaxKeywordCount
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

func selectMAWebValidationTargets(targets []MATarget, limit int) []MATarget {
	out := make([]MATarget, 0, len(targets))
	for _, target := range targets {
		if target.MatchState == maMatchStateOutside {
			continue
		}
		out = append(out, target)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri := targetRatingValue(out[i])
		rj := targetRatingValue(out[j])
		if ri != rj {
			return ri > rj
		}
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].CompanyName < out[j].CompanyName
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

func failedMAWebValidationRequest(target MATarget, keywordSet CandidateMatchKeywordSet, inputHash, keywordSetHash string, err error) MAWebValidationUpsertRequest {
	domainResponse := DomainResolutionResponse{}
	summary := CandidateMatchEvidenceSummary{
		Score:              0,
		Confidence:         "bassa",
		TotalCoreTerms:     len(keywordSet.CoreTerms),
		TotalAdjacentTerms: len(keywordSet.AdjacentTerms),
		TotalNegativeTerms: len(keywordSet.NegativeTerms),
	}
	input := CandidateMatchAnalysisRequest{Target: target, KeywordSet: keywordSet, DomainResponse: domainResponse, Summary: summary}
	errorText := cleanText(err.Error(), 180)
	return MAWebValidationUpsertRequest{
		PipelineVersion:     maWebValidationPipelineVersion,
		InputHash:           inputHash,
		KeywordSetHash:      keywordSetHash,
		Target:              target,
		KeywordSet:          keywordSet,
		DomainResponse:      domainResponse,
		EvidenceRuns:        []CandidateMatchEvidenceRun{},
		Summary:             summary,
		CandidateMatchError: errorText,
		FinalDecision:       *reconcileCandidateMatchDecision(input, nil, errorText),
	}
}

type maWebValidationTermSpec struct {
	Bucket string
	Term   string
}

func maWebValidationTermSpecs(keywordSet CandidateMatchKeywordSet) []maWebValidationTermSpec {
	out := []maWebValidationTermSpec{}
	for _, term := range keywordSet.CoreTerms[:min(6, len(keywordSet.CoreTerms))] {
		out = append(out, maWebValidationTermSpec{Bucket: "core", Term: term})
	}
	for _, term := range keywordSet.AdjacentTerms[:min(4, len(keywordSet.AdjacentTerms))] {
		out = append(out, maWebValidationTermSpec{Bucket: "adjacent", Term: term})
	}
	for _, term := range keywordSet.NegativeTerms[:min(3, len(keywordSet.NegativeTerms))] {
		out = append(out, maWebValidationTermSpec{Bucket: "negative", Term: term})
	}
	return out
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

func deriveMAWebValidationKeywordSet(strategy MAStrategySpec, target MATarget) CandidateMatchKeywordSet {
	coreTerms := []string{}
	adjacentTerms := []string{}
	negativeTerms := []string{}
	sources := []string{}
	atecoCodes := maWebValidationAtecoCodes(strategy, target)
	sector := cleanText(strategy.SectorDescription, 220)
	if sector != "" {
		sources = append(sources, "settore: "+sector)
	}
	if len(atecoCodes) > 0 {
		sources = append(sources, "ATECO: "+strings.Join(atecoCodes, ", "))
	}
	if target.AtecoDescription != "" {
		sources = append(sources, "target ATECO: "+target.AtecoDescription)
	}
	addMAWebValidationTerms(&coreTerms, strategy.Keywords, 10)
	if maHasAtecoPrefix(atecoCodes, []string{"631010", "6310", "631"}) {
		addMAWebValidationTerms(&coreTerms, []string{"infrastrutture informatiche", "hosting", "cloud infrastructure", "cloud native", "data center"}, 10)
		addMAWebValidationTerms(&adjacentTerms, []string{"cloud", "server", "storage", "backup", "virtualizzazione"}, 8)
	}
	if maHasAtecoPrefix(atecoCodes, []string{"622020", "6220", "622"}) {
		addMAWebValidationTerms(&coreTerms, []string{"gestione strutture informatiche", "gestione infrastrutture IT", "IT operations", "system management", "assistenza sistemistica"}, 10)
		addMAWebValidationTerms(&adjacentTerms, []string{"networking", "monitoraggio", "help desk", "SLA"}, 8)
	}
	if maHasAtecoPrefix(atecoCodes, []string{"629009", "6290", "629"}) {
		addMAWebValidationTerms(&coreTerms, []string{"servizi IT", "servizi informatici", "tecnologie informatiche", "information technology services"}, 10)
		addMAWebValidationTerms(&adjacentTerms, []string{"cybersecurity", "Microsoft 365", "consulenza informatica"}, 8)
	}
	text := strings.ToLower(sector + " " + target.AtecoDescription)
	if strings.Contains(text, "cloud") {
		addMAWebValidationTerm(&coreTerms, "cloud", 10)
	}
	if strings.Contains(text, "hosting") {
		addMAWebValidationTerm(&coreTerms, "hosting", 10)
	}
	if strings.Contains(text, "infrastrutt") {
		addMAWebValidationTerm(&coreTerms, "infrastrutture informatiche", 10)
	}
	if strings.Contains(text, "gestione") {
		addMAWebValidationTerm(&coreTerms, "gestione infrastrutture IT", 10)
	}
	if len(coreTerms) == 0 {
		addMAWebValidationTerms(&coreTerms, []string{"servizi IT", "tecnologie informatiche", "consulenza informatica"}, 10)
		sources = append(sources, "fallback: lente IT ampia")
	}
	addMAWebValidationTerms(&adjacentTerms, []string{"cloud", "backup", "cybersecurity", "networking", "virtualizzazione"}, 8)
	addMAWebValidationTerms(&negativeTerms, []string{"web agency", "marketing digitale", "rivendita hardware", "sviluppo software puro", "formazione informatica"}, 6)
	intentLabel := sector
	if intentLabel == "" {
		intentLabel = "Lente derivata da strategia M&A"
	}
	return CandidateMatchKeywordSet{
		IntentLabel:   intentLabel,
		CoreTerms:     coreTerms,
		AdjacentTerms: adjacentTerms,
		NegativeTerms: negativeTerms,
		Sources:       sources,
	}
}

func addMAWebValidationTerms(list *[]string, values []string, limit int) {
	for _, value := range values {
		addMAWebValidationTerm(list, value, limit)
	}
}

func addMAWebValidationTerm(list *[]string, value string, limit int) {
	term := cleanText(value, 120)
	if term == "" || len(*list) >= limit {
		return
	}
	key := strings.ToLower(term)
	for _, item := range *list {
		if strings.ToLower(item) == key {
			return
		}
	}
	*list = append(*list, term)
}

func maWebValidationAtecoCodes(strategy MAStrategySpec, target MATarget) []string {
	seen := map[string]struct{}{}
	out := []string{}
	add := func(value string) {
		code := maWebValidationAtecoDigits(value)
		if code == "" {
			return
		}
		if _, exists := seen[code]; exists {
			return
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	add(target.AtecoCode)
	for _, candidate := range strategy.AtecoCandidates {
		if normalizeMAFit(candidate.Fit) == maFitExcluded {
			continue
		}
		add(candidate.Code)
	}
	return out
}

func maWebValidationAtecoDigits(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func maHasAtecoPrefix(codes []string, prefixes []string) bool {
	for _, code := range codes {
		for _, prefix := range prefixes {
			if strings.HasPrefix(code, prefix) {
				return true
			}
		}
	}
	return false
}

func computeMAWebValidationSummary(selectedDomain *DomainResolutionCandidate, keywordSet CandidateMatchKeywordSet, runs []CandidateMatchEvidenceRun) CandidateMatchEvidenceSummary {
	matched := func(bucket string) int {
		count := 0
		for _, run := range runs {
			if run.Bucket == bucket && run.Matched {
				count++
			}
		}
		return count
	}
	searched := func(bucket string) int {
		count := 0
		for _, run := range runs {
			if run.Bucket == bucket {
				count++
			}
		}
		return count
	}
	coreMatches := matched("core")
	adjacentMatches := matched("adjacent")
	negativeMatches := matched("negative")
	searchedCore := searched("core")
	searchedAdjacent := searched("adjacent")
	searchedNegative := searched("negative")
	coreEvidenceScore := maBucketEvidenceScore(runs, "core")
	adjacentEvidenceScore := maBucketEvidenceScore(runs, "adjacent")
	sectorEvidenceScore := clampMAWebScore(float64(coreEvidenceScore)*0.72 + float64(adjacentEvidenceScore)*0.28)
	coverageScore := clampMAWebScore(ratioMAWebScore(coreMatches, max(len(keywordSet.CoreTerms), searchedCore))*0.7 + ratioMAWebScore(adjacentMatches, max(len(keywordSet.AdjacentTerms), searchedAdjacent))*0.3)
	domainScore := maDomainEvidenceScore(selectedDomain)
	negativeRate := ratioMAWebScore(negativeMatches, max(1, searchedNegative))
	negativePenalty := clampMAWebScore(float64(negativeMatches)*18 + negativeRate*0.35)
	noNegativeScore := 100 - negativePenalty
	rawScore := float64(sectorEvidenceScore)*0.45 + float64(coverageScore)*0.2 + float64(domainScore)*0.25 + float64(noNegativeScore)*0.1 - float64(negativePenalty)*0.35
	score := 0
	if selectedDomain != nil {
		score = clampMAWebScore(rawScore)
	}
	confidence := "bassa"
	if selectedDomain != nil && score >= 70 && coreMatches >= 2 && negativePenalty < 25 {
		confidence = "alta"
	} else if selectedDomain != nil && score >= 45 && (coreMatches >= 1 || adjacentMatches >= 2) {
		confidence = "media"
	}
	return CandidateMatchEvidenceSummary{
		Score:                 score,
		Confidence:            confidence,
		SectorEvidenceScore:   sectorEvidenceScore,
		CoverageScore:         coverageScore,
		DomainScore:           domainScore,
		NegativePenalty:       negativePenalty,
		CoreMatches:           coreMatches,
		AdjacentMatches:       adjacentMatches,
		NegativeMatches:       negativeMatches,
		SearchedCoreTerms:     searchedCore,
		SearchedAdjacentTerms: searchedAdjacent,
		SearchedNegativeTerms: searchedNegative,
		TotalCoreTerms:        len(keywordSet.CoreTerms),
		TotalAdjacentTerms:    len(keywordSet.AdjacentTerms),
		TotalNegativeTerms:    len(keywordSet.NegativeTerms),
	}
}

func maBucketEvidenceScore(runs []CandidateMatchEvidenceRun, bucket string) int {
	total := 0
	matched := []CandidateMatchEvidenceRun{}
	for _, run := range runs {
		if run.Bucket != bucket {
			continue
		}
		total++
		if run.Matched {
			matched = append(matched, run)
		}
	}
	if total == 0 {
		return 0
	}
	matchedRate := float64(len(matched)) / float64(total)
	quality := 0.0
	if len(matched) > 0 {
		sum := 0
		for _, run := range matched {
			score := 60
			if run.BestScore != nil {
				score = *run.BestScore
			}
			sum += score
		}
		quality = float64(sum) / float64(len(matched))
	}
	return clampMAWebScore(matchedRate*70 + quality*0.3)
}

func maDomainEvidenceScore(selectedDomain *DomainResolutionCandidate) int {
	if selectedDomain == nil {
		return 0
	}
	confidenceScore := 30
	if selectedDomain.Confidence == "alta" {
		confidenceScore = 100
	} else if selectedDomain.Confidence == "media" {
		confidenceScore = 70
	}
	return clampMAWebScore(float64(selectedDomain.Score)*0.75 + float64(confidenceScore)*0.25)
}

func maEvidenceResponseMatched(response WebSearchResponse) bool {
	if bestScore := bestMAWebSearchScore(response); bestScore != nil {
		return *bestScore >= 45
	}
	return !response.Ranked && len(response.Results) > 0
}

func bestMAWebSearchScore(response WebSearchResponse) *int {
	var best *int
	for _, result := range response.Results {
		if result.Score == nil {
			continue
		}
		if best == nil || *result.Score > *best {
			value := *result.Score
			best = &value
		}
	}
	return best
}

func maWebValidationInputHash(target MATarget, keywordSet CandidateMatchKeywordSet, payload maWebValidationJobPayload) string {
	return maWebValidationHash(map[string]any{
		"keywordSet":      maWebValidationKeywordSetFingerprint(keywordSet),
		"options":         maWebValidationInputOptions(payload),
		"pipelineVersion": maWebValidationPipelineVersion,
		"target":          maWebValidationTargetFingerprint(target),
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

func clampMAWebScore(value float64) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return int(value + 0.5)
}

func ratioMAWebScore(value int, total int) float64 {
	if total <= 0 {
		return 0
	}
	score := float64(value) / float64(total) * 100
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
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
