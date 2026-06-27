package binocolo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/llm"
)

const candidateMatchAnalysisMaxToken = 1800

// CandidateMatchAnalysisRequest is the lab-only payload produced by TestPage's
// evidence pipeline. It contains only deterministic target data and already
// collected web evidence; the LLM must not browse or resolve domains.
type CandidateMatchAnalysisRequest struct {
	Target         MATarget                      `json:"target"`
	KeywordSet     CandidateMatchKeywordSet      `json:"keywordSet"`
	DomainResponse DomainResolutionResponse      `json:"domainResponse"`
	SelectedDomain *DomainResolutionCandidate    `json:"selectedDomain,omitempty"`
	EvidenceRuns   []CandidateMatchEvidenceRun   `json:"evidenceRuns"`
	Summary        CandidateMatchEvidenceSummary `json:"summary"`
}

type CandidateMatchKeywordSet struct {
	IntentLabel   string   `json:"intentLabel"`
	CoreTerms     []string `json:"coreTerms"`
	AdjacentTerms []string `json:"adjacentTerms"`
	NegativeTerms []string `json:"negativeTerms"`
	Sources       []string `json:"sources"`
}

type CandidateMatchEvidenceRun struct {
	Bucket      string             `json:"bucket"`
	Term        string             `json:"term"`
	Response    *WebSearchResponse `json:"response,omitempty"`
	Error       string             `json:"error,omitempty"`
	ResultCount int                `json:"resultCount"`
	BestScore   *int               `json:"bestScore,omitempty"`
	Matched     bool               `json:"matched"`
}

type CandidateMatchEvidenceSummary struct {
	Score           int    `json:"score"`
	Confidence      string `json:"confidence"`
	CoreMatches     int    `json:"coreMatches"`
	AdjacentMatches int    `json:"adjacentMatches"`
	NegativeMatches int    `json:"negativeMatches"`
}

type CandidateMatchAnalysisResponse struct {
	Verdict           string                       `json:"verdict"`
	Confidence        string                       `json:"confidence"`
	SectorFit         string                       `json:"sectorFit"`
	BusinessFit       string                       `json:"businessFit"`
	EvidenceFor       []string                     `json:"evidenceFor"`
	EvidenceAgainst   []string                     `json:"evidenceAgainst"`
	NegativeSignals   []string                     `json:"negativeSignals"`
	MissingEvidence   []string                     `json:"missingEvidence"`
	ConceptAliases    []CandidateMatchConceptAlias `json:"conceptAliases"`
	RecommendedAction string                       `json:"recommendedAction"`
	Rationale         string                       `json:"rationale"`
}

type CandidateMatchConceptAlias struct {
	Term           string `json:"term"`
	MatchedConcept string `json:"matchedConcept"`
	Evidence       string `json:"evidence"`
}

func (h *Handler) handleTestCandidateMatchAnalysis(w http.ResponseWriter, r *http.Request) {
	var body CandidateMatchAnalysisRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if strings.TrimSpace(body.Target.CompanyName) == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_company_name")
		return
	}
	if body.SelectedDomain == nil || strings.TrimSpace(body.SelectedDomain.Domain) == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_selected_domain")
		return
	}

	subject, email := companySearchRefreshActor(r.Context())
	analysis, err := h.ma.analyzeCandidateMatch(r.Context(), body, subject, email)
	if err != nil {
		h.maFailure(w, r, "candidate_match_analysis", err)
		return
	}
	httputil.JSON(w, http.StatusOK, analysis)
}

func (s *maService) analyzeCandidateMatch(ctx context.Context, input CandidateMatchAnalysisRequest, subject, email string) (CandidateMatchAnalysisResponse, error) {
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

	curated := curateCandidateMatchInput(input)
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
		"target_id":       input.Target.ID,
		"session_id":      input.Target.SessionID,
		"company_key":     input.Target.CompanyKey,
		"selected_domain": input.SelectedDomain.Domain,
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
	return analysis, nil
}

func curateCandidateMatchInput(input CandidateMatchAnalysisRequest) map[string]any {
	type resultSnippet struct {
		Title    string   `json:"title"`
		URL      string   `json:"url"`
		Hostname string   `json:"hostname"`
		Score    *int     `json:"score,omitempty"`
		Snippets []string `json:"snippets"`
	}
	type runSnippet struct {
		Bucket      string          `json:"bucket"`
		Term        string          `json:"term"`
		ResultCount int             `json:"resultCount"`
		BestScore   *int            `json:"bestScore,omitempty"`
		Matched     bool            `json:"matched"`
		Error       string          `json:"error,omitempty"`
		Results     []resultSnippet `json:"results,omitempty"`
	}
	runs := make([]runSnippet, 0, len(input.EvidenceRuns))
	for _, run := range input.EvidenceRuns {
		item := runSnippet{
			Bucket:      cleanText(run.Bucket, 40),
			Term:        cleanText(run.Term, 120),
			ResultCount: run.ResultCount,
			BestScore:   run.BestScore,
			Matched:     run.Matched,
			Error:       cleanText(run.Error, 180),
		}
		if run.Response != nil {
			for _, result := range run.Response.Results {
				if len(item.Results) >= 3 {
					break
				}
				snippets := make([]string, 0, min(2, len(result.Snippets)))
				for _, snippet := range result.Snippets {
					if len(snippets) >= 2 {
						break
					}
					if cleaned := cleanText(snippet, 360); cleaned != "" {
						snippets = append(snippets, cleaned)
					}
				}
				item.Results = append(item.Results, resultSnippet{
					Title:    cleanText(result.Title, 160),
					URL:      cleanText(result.URL, 240),
					Hostname: cleanText(result.Hostname, 120),
					Score:    result.Score,
					Snippets: snippets,
				})
			}
		}
		runs = append(runs, item)
	}

	target := map[string]any{
		"id":                      input.Target.ID,
		"companyName":             input.Target.CompanyName,
		"province":                input.Target.Province,
		"town":                    input.Target.Town,
		"activityStatus":          input.Target.ActivityStatus,
		"turnover":                input.Target.Turnover,
		"turnoverYear":            input.Target.TurnoverYear,
		"employees":               input.Target.Employees,
		"atecoCode":               input.Target.AtecoCode,
		"atecoDescription":        input.Target.AtecoDescription,
		"deterministicScore":      input.Target.Score,
		"matchState":              input.Target.MatchState,
		"deterministicConfidence": input.Target.Confidence,
		"rating":                  input.Target.Rating,
		"rationale":               input.Target.Rationale,
		"missingCriteria":         input.Target.MissingCriteria,
		"evidence":                input.Target.Evidence,
	}

	selectedDomain := map[string]any{}
	if input.SelectedDomain != nil {
		selectedDomain = map[string]any{
			"domain":     input.SelectedDomain.Domain,
			"score":      input.SelectedDomain.Score,
			"confidence": input.SelectedDomain.Confidence,
			"reasons":    input.SelectedDomain.Reasons,
		}
	}

	return map[string]any{
		"target":         target,
		"keywordSet":     input.KeywordSet,
		"selectedDomain": selectedDomain,
		"evidenceRuns":   runs,
		"summary":        input.Summary,
		"instructions": map[string]any{
			"doNotBrowse":                     true,
			"doNotChooseAlternativeDomain":    true,
			"doNotOverrideDeterministicScore": true,
			"interpretSemanticAliases":        true,
		},
	}
}

func parseCandidateMatchAnalysis(content string) (CandidateMatchAnalysisResponse, error) {
	raw := extractJSONObject(content)
	if raw == "" {
		return CandidateMatchAnalysisResponse{}, fmt.Errorf("candidate match analyst returned non-JSON content: %s", truncateRunes(strings.TrimSpace(content), 200))
	}
	var out CandidateMatchAnalysisResponse
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return CandidateMatchAnalysisResponse{}, fmt.Errorf("candidate match analyst JSON parse: %w (content: %s)", err, truncateRunes(strings.TrimSpace(content), 200))
	}
	out.Verdict = normalizeAllowed(out.Verdict, "unclear", "strong_match", "match", "weak_match", "no_match", "unclear")
	out.Confidence = normalizeAllowed(out.Confidence, "bassa", "alta", "media", "bassa")
	out.RecommendedAction = normalizeAllowed(out.RecommendedAction, "review", "confirm", "review", "downgrade", "reject")
	out.SectorFit = cleanText(out.SectorFit, 700)
	out.BusinessFit = cleanText(out.BusinessFit, 700)
	out.Rationale = cleanText(out.Rationale, 900)
	out.EvidenceFor = cleanStringList(out.EvidenceFor, 8, 280)
	out.EvidenceAgainst = cleanStringList(out.EvidenceAgainst, 8, 280)
	out.NegativeSignals = cleanStringList(out.NegativeSignals, 8, 280)
	out.MissingEvidence = cleanStringList(out.MissingEvidence, 8, 280)
	if out.ConceptAliases == nil {
		out.ConceptAliases = []CandidateMatchConceptAlias{}
	}
	if len(out.ConceptAliases) > 8 {
		out.ConceptAliases = out.ConceptAliases[:8]
	}
	for i := range out.ConceptAliases {
		out.ConceptAliases[i].Term = cleanText(out.ConceptAliases[i].Term, 120)
		out.ConceptAliases[i].MatchedConcept = cleanText(out.ConceptAliases[i].MatchedConcept, 180)
		out.ConceptAliases[i].Evidence = cleanText(out.ConceptAliases[i].Evidence, 240)
	}
	return out, nil
}

func normalizeAllowed(value, fallback string, allowed ...string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, item := range allowed {
		if value == item {
			return value
		}
	}
	return fallback
}
