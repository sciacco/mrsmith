package binocolo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// candidateMatchAnalysisMaxToken bounds the analyst (now the UC2 ambiguity
// tie-breaker in analyzeSectorAmbiguity).
const candidateMatchAnalysisMaxToken = 5000

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
	Score                 int    `json:"score"`
	Confidence            string `json:"confidence"`
	SectorEvidenceScore   int    `json:"sectorEvidenceScore"`
	CoverageScore         int    `json:"coverageScore"`
	DomainScore           int    `json:"domainScore"`
	NegativePenalty       int    `json:"negativePenalty"`
	CoreMatches           int    `json:"coreMatches"`
	AdjacentMatches       int    `json:"adjacentMatches"`
	NegativeMatches       int    `json:"negativeMatches"`
	SearchedCoreTerms     int    `json:"searchedCoreTerms"`
	SearchedAdjacentTerms int    `json:"searchedAdjacentTerms"`
	SearchedNegativeTerms int    `json:"searchedNegativeTerms"`
	TotalCoreTerms        int    `json:"totalCoreTerms"`
	TotalAdjacentTerms    int    `json:"totalAdjacentTerms"`
	TotalNegativeTerms    int    `json:"totalNegativeTerms"`
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
	FinalDecision     *CandidateMatchFinalDecision `json:"finalDecision,omitempty"`
	ModelID           string                       `json:"modelId,omitempty"`
	PromptID          string                       `json:"promptId,omitempty"`
	Model             string                       `json:"model,omitempty"`
}

type CandidateMatchFinalDecision struct {
	InitialMatchState  string   `json:"initialMatchState"`
	DeterministicScore int      `json:"deterministicScore"`
	WebScore           int      `json:"webScore"`
	WebValidationState string   `json:"webValidationState"`
	FinalAction        string   `json:"finalAction"`
	Confidence         string   `json:"confidence"`
	Reason             string   `json:"reason"`
	Reasons            []string `json:"reasons"`
	AnalystVerdict     string   `json:"analystVerdict,omitempty"`
	AnalystAction      string   `json:"analystAction,omitempty"`
}

type CandidateMatchConceptAlias struct {
	Term           string `json:"term"`
	MatchedConcept string `json:"matchedConcept"`
	Evidence       string `json:"evidence"`
}

// handleTestSectorClassification is the UC2 lab probe: classify one company against
// a sector intent in isolation from the funnel (the analog of UC1's `atego smoke`).
// Either pick an existing session target (sessionId+targetId) or pass an ad-hoc
// sector + company.
func (h *Handler) handleTestSectorClassification(w http.ResponseWriter, r *http.Request) {
	var body SectorClassificationTestRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	hasTarget := strings.TrimSpace(body.SessionID) != "" && strings.TrimSpace(body.TargetID) != ""
	if !hasTarget && strings.TrimSpace(body.SectorDescription) == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_sector_description")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	resp, err := h.ma.testSectorClassification(r.Context(), body, subject, email)
	if err != nil {
		h.maFailure(w, r, "sector_classification", err)
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

func candidateMatchLatestStaffCost(target MATarget) *int {
	if len(target.VendorPayload) == 0 {
		return nil
	}
	object, err := decodeVendorObject(target.VendorPayload)
	if err != nil {
		return nil
	}
	value, ok := vendorPath(object, "balanceSheets.last.totalStaffCost")
	if !ok {
		return nil
	}
	return vendorIntFromValue(value)
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
