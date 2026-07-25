package binocolo

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// UC2 sector-classification eval harness. The labeling workflow is inverted: the
// operator runs web validation over a whole session (the existing batch enrich),
// then reviews the system's keep/forse/scarta calls and confirms or reclassifies
// each. The human label is ground truth (persisted in ma_sector_eval_label, separate
// from the re-runnable prediction); the report compares prediction vs label and
// surfaces the metrics that matter — accuracy, LLM-escalation rate, and the
// "a distractor outranks the best in-perimeter target" defect (the Gerico case).

// MASectorEvalLabel is one human ground-truth label for a company in a session.
type MASectorEvalLabel struct {
	CompanyKey     string    `json:"companyKey"`
	Label          string    `json:"label"` // keep | forse | scarta
	Note           string    `json:"note,omitempty"`
	LabeledByEmail string    `json:"labeledByEmail,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// SectorEvalLabelRequest upserts (or clears, with empty Label) a ground-truth label.
type SectorEvalLabelRequest struct {
	CompanyKey string `json:"companyKey"`
	Label      string `json:"label"`
	Note       string `json:"note,omitempty"`
}

// SectorEvalItem is one company row with the three competing predictions and the human
// label (if any):
//   - A (deterministic): embed+rerank verdict alone, LLM off.
//   - B (LLM-always): what the analyst chose for THIS company — present for every row in
//     an LLM-on-all run, empty if the analyst did not run.
//   - C (final): the production hybrid (deterministic + LLM only on ambiguous/no_signal).
type SectorEvalItem struct {
	CompanyKey       string `json:"companyKey"`
	CompanyName      string `json:"companyName"`
	Domain           string `json:"domain,omitempty"`
	AtecoDescription string `json:"atecoDescription,omitempty"`
	SelfDescription  string `json:"selfDescription,omitempty"`
	Validated        bool   `json:"validated"`

	// Domain-resolution diagnostics (read-only, from the persisted DomainResponse).
	// resolved = un dominio è stato scelto; retrieval_fail = 0 candidati (il sito non
	// emerge); acceptance_fail = candidati trovati ma tutti scartati dal cancello.
	DomainOutcome           string           `json:"domainOutcome,omitempty"`
	DomainConfidence        string           `json:"domainConfidence,omitempty"`
	IdentityState           string           `json:"identityState,omitempty"` // verified|vouched|assumed, ""=legacy/unresolved (mig 083)
	GroupSiteHint           *MAGroupSiteHint `json:"groupSiteHint,omitempty"` // brand-compatible candidate with another entity's P.IVA (policy B1)
	DomainScore             int              `json:"domainScore,omitempty"`
	DomainCandidateCount    int              `json:"domainCandidateCount"`
	BestCandidateDomain     string           `json:"bestCandidateDomain,omitempty"`
	BestCandidateScore      int              `json:"bestCandidateScore,omitempty"`
	BestCandidateConfidence string           `json:"bestCandidateConfidence,omitempty"`

	// A — embed+rerank only (no LLM).
	DeterministicVerdict string `json:"deterministicVerdict,omitempty"` // confirm/reject/weak/ambiguous/no_signal
	DeterministicBucket  string `json:"deterministicBucket,omitempty"`  // keep | forse | scarta

	// B — the LLM analyst's call on this company (LLM-on-all). Empty if it did not run.
	LLMVerdict string `json:"llmVerdict,omitempty"`
	LLMAction  string `json:"llmAction,omitempty"`
	LLMBucket  string `json:"llmBucket,omitempty"` // keep | forse | scarta

	// C — production hybrid final decision.
	FinalState  string `json:"finalState,omitempty"` // web_validation_state
	FinalAction string `json:"finalAction,omitempty"`
	FinalBucket string `json:"finalBucket,omitempty"` // keep | forse | scarta

	Escalated             bool             `json:"escalated"` // deterministic verdict needed the LLM
	DistractorBeatsTarget bool             `json:"distractorBeatsTarget"`
	TopConcepts           []maConceptScore `json:"topConcepts,omitempty"`

	Label string `json:"label,omitempty"` // ground truth
	Note  string `json:"note,omitempty"`
}

// SectorEvalPredictorMetrics is accuracy + confusion for one predictor over the rows
// that are both labeled and have a prediction from it.
type SectorEvalPredictorMetrics struct {
	Evaluable int                       `json:"evaluable"`
	Correct   int                       `json:"correct"`
	Accuracy  float64                   `json:"accuracy"`
	Confusion map[string]map[string]int `json:"confusion"` // [label][predicted]
}

// SectorEvalDomainMetrics measures how effective domain resolution is, over validated
// targets. The retrieval/acceptance split says WHERE recall breaks: retrievalFail = the
// site never surfaced (fix = query/probing); acceptanceFail = it surfaced but the gate
// rejected it (fix = relax the gate).
type SectorEvalDomainMetrics struct {
	Resolved       int     `json:"resolved"`
	RetrievalFail  int     `json:"retrievalFail"`
	AcceptanceFail int     `json:"acceptanceFail"`
	ResolutionRate float64 `json:"resolutionRate"`

	// Identity-asymmetry measurement (phase b): how certain was the domain identity
	// on resolved companies, and — the decision-driving cut — what identity level
	// backs each REJECT verdict. Keys: verified|vouched|assumed|legacy.
	IdentityStates   map[string]int `json:"identityStates,omitempty"`
	RejectByIdentity map[string]int `json:"rejectByIdentity,omitempty"`

	// GroupSiteSuspected counts validations (resolved AND unresolved) whose
	// resolution saw a brand-compatible candidate with another entity's P.IVA —
	// the population policy B1's operator remedy will serve.
	GroupSiteSuspected int `json:"groupSiteSuspected,omitempty"`
}

// SectorEvalMetrics aggregates the three predictors plus the operational stats that
// answer the four questions: (1) deterministic accuracy, (2) escalation rate, (3) the
// LLM column itself + its accuracy, (4) the deterministic-vs-LLM head-to-head.
type SectorEvalMetrics struct {
	Targets   int `json:"targets"`
	Validated int `json:"validated"`
	Labeled   int `json:"labeled"`

	Domain SectorEvalDomainMetrics `json:"domain"`

	Deterministic SectorEvalPredictorMetrics `json:"deterministic"` // A
	LLM           SectorEvalPredictorMetrics `json:"llm"`           // B
	Final         SectorEvalPredictorMetrics `json:"final"`         // C

	Escalations    int     `json:"escalations"`    // deterministic verdict ∈ {ambiguous, no_signal}
	EscalationRate float64 `json:"escalationRate"` // over validated

	DistractorBeatsTarget int `json:"distractorBeatsTarget"`

	// LLMVsDeterministic is the head-to-head over rows that are labeled and have BOTH a
	// deterministic and an LLM bucket. It is the decision stat for "should we trust the
	// LLM completely": llmOnly = the KB was wrong and the LLM rescued it; detOnly = the
	// KB was right and the LLM would have broken it.
	LLMVsDeterministic map[string]int `json:"llmVsDeterministic"` // bothCorrect|detOnly|llmOnly|bothWrong
}

// SectorEvalPerimeter is the search perimeter (the strategy) that defines WHAT this
// session looked for. Embedded in the report so the copied JSON is self-contained — the
// keep/forse/scarta labels only mean anything relative to this perimeter.
type SectorEvalPerimeter struct {
	Title             string             `json:"title,omitempty"`
	SectorDescription string             `json:"sectorDescription,omitempty"`
	Thesis            string             `json:"thesis,omitempty"`
	TerritoryLabel    string             `json:"territoryLabel,omitempty"`
	Provinces         []string           `json:"provinces,omitempty"`
	LegalForms        []string           `json:"legalForms,omitempty"`
	ActivityStatus    string             `json:"activityStatus,omitempty"`
	AtecoCandidates   []MAAtecoCandidate `json:"atecoCandidates,omitempty"`
	Keywords          []string           `json:"keywords,omitempty"`
	TurnoverAround    *int               `json:"turnoverAround,omitempty"`
	TurnoverMin       *int               `json:"turnoverMin,omitempty"`
	TurnoverMax       *int               `json:"turnoverMax,omitempty"`
	EmployeeMin       *int               `json:"employeeMin,omitempty"`
	EmployeeMax       *int               `json:"employeeMax,omitempty"`
}

// SectorEvalReport is the full eval payload (copyable JSON for the Test page tab).
type SectorEvalReport struct {
	SessionID string              `json:"sessionId"`
	Perimeter SectorEvalPerimeter `json:"perimeter"`
	Items     []SectorEvalItem    `json:"items"`
	Metrics   SectorEvalMetrics   `json:"metrics"`
	// Replay is the offline R1/R2/R3 re-analysis of the deterministic layer over the
	// labeled rows (no live calls). Nil when nothing is labeled. See ma_sector_replay.go.
	Replay *SectorReplayReport `json:"replay,omitempty"`
}

var sectorEvalBuckets = []string{"keep", "forse", "scarta"}

func newSectorConfusion() map[string]map[string]int {
	m := map[string]map[string]int{}
	for _, l := range sectorEvalBuckets {
		m[l] = map[string]int{}
	}
	return m
}

func (s *maService) setSectorEvalLabel(ctx context.Context, sessionID string, body SectorEvalLabelRequest, subject, email string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	// normalizeMACompanyKey, non solo TrimSpace: la chiave arriva dal client e
	// finisce dritta in ma_sector_eval_label.company_key. Senza il maiuscolo, un
	// client che la rimandasse in altra forma scriverebbe una riga che nessuna
	// lettura ritrova — l'etichetta sparirebbe in silenzio — e una chiave che non
	// esiste in ma_company. È l'unico writer del sottosistema che non
	// normalizzava.
	key := normalizeMACompanyKey(body.CompanyKey)
	if key == "" {
		return fmt.Errorf("%w: companyKey", errMAStrategyInvalid)
	}
	label := strings.TrimSpace(body.Label)
	switch label {
	case "", "keep", "forse", "scarta":
	default:
		return fmt.Errorf("%w: label", errMAStrategyInvalid)
	}
	return s.store.UpsertMASectorEvalLabel(ctx, sessionID, key, label, cleanText(body.Note, 280), subject, email)
}

func (s *maService) sectorEvalReport(ctx context.Context, sessionID string) (SectorEvalReport, error) {
	if s.store == nil {
		return SectorEvalReport{}, errMAStoreUnavailable
	}
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return SectorEvalReport{}, err
	}
	labels, err := s.store.ListMASectorEvalLabels(ctx, sessionID)
	if err != nil {
		return SectorEvalReport{}, err
	}

	metrics := SectorEvalMetrics{
		Domain:             SectorEvalDomainMetrics{IdentityStates: map[string]int{}, RejectByIdentity: map[string]int{}},
		Deterministic:      SectorEvalPredictorMetrics{Confusion: newSectorConfusion()},
		LLM:                SectorEvalPredictorMetrics{Confusion: newSectorConfusion()},
		Final:              SectorEvalPredictorMetrics{Confusion: newSectorConfusion()},
		LLMVsDeterministic: map[string]int{"bothCorrect": 0, "detOnly": 0, "llmOnly": 0, "bothWrong": 0},
	}
	items := make([]SectorEvalItem, 0, len(detail.Targets))
	replayInputs := make([]sectorReplayInput, 0, len(labels))

	for _, t := range detail.Targets {
		metricTarget := t.Origin != maTargetOriginManual
		item := SectorEvalItem{
			CompanyKey:       t.CompanyKey,
			CompanyName:      t.CompanyName,
			AtecoDescription: t.AtecoDescription,
		}
		if lbl, ok := labels[t.CompanyKey]; ok {
			item.Label = lbl.Label
			item.Note = lbl.Note
		}
		if metricTarget {
			metrics.Targets++
			if item.Label != "" {
				metrics.Labeled++
			}
		}

		if wv := t.WebValidation; wv != nil {
			item.Validated = true
			if metricTarget {
				metrics.Validated++
			}
			item.Domain = wv.SelectedDomain
			item.SelfDescription = wv.Summary.CompanyDescription

			item.DomainCandidateCount = len(wv.DomainResponse.Candidates)
			switch {
			case wv.SelectedDomain != "":
				item.DomainOutcome = "resolved"
				item.DomainConfidence = wv.DomainConfidence
				if wv.DomainScore != nil {
					item.DomainScore = *wv.DomainScore
				}
				if metricTarget {
					metrics.Domain.Resolved++
				}
			case item.DomainCandidateCount == 0:
				item.DomainOutcome = "retrieval_fail"
				if metricTarget {
					metrics.Domain.RetrievalFail++
				}
			default:
				item.DomainOutcome = "acceptance_fail"
				best := wv.DomainResponse.Candidates[0] // sorted desc by score
				item.BestCandidateDomain = best.Domain
				item.BestCandidateScore = best.Score
				item.BestCandidateConfidence = best.Confidence
				if metricTarget {
					metrics.Domain.AcceptanceFail++
				}
			}

			item.DeterministicVerdict = wv.Summary.DeterministicVerdict
			item.DeterministicBucket = deterministicVerdictToBucket(wv.Summary.DeterministicVerdict)

			item.LLMVerdict = wv.Summary.LLMVerdictAll
			item.LLMAction = wv.Summary.LLMActionAll
			item.LLMBucket = analystToBucket(wv.Summary.LLMVerdictAll, wv.Summary.LLMActionAll)

			item.FinalState = wv.WebValidationState
			item.FinalAction = wv.FinalAction
			item.FinalBucket = sectorActionToBucket(wv.FinalAction)

			// Identity-asymmetry cut (phase b): identity level over resolved
			// companies, and per-REJECT — the number that decides whether "reject
			// suppresses only from verified/vouched" is affordable. "" = legacy
			// row (pre-083); unresolved rows carry no domain to have identity on.
			if wv.SelectedDomain != "" {
				item.IdentityState = wv.IdentityState
				identityKey := wv.IdentityState
				if identityKey == "" {
					identityKey = "legacy"
				}
				if metricTarget {
					metrics.Domain.IdentityStates[identityKey]++
					if item.FinalBucket == maGatedBucketReject {
						metrics.Domain.RejectByIdentity[identityKey]++
					}
				}
			}
			// Group-site suspicion (policy B1): counted OUTSIDE the resolved guard —
			// the hint matters most on unresolved rows, which are the queue.
			if hint := wv.DomainResponse.GroupSiteHint; hint != nil {
				item.GroupSiteHint = hint
				if metricTarget {
					metrics.Domain.GroupSiteSuspected++
				}
			}

			item.Escalated = sectorVerdictNeedsLLM(maSectorVerdict(wv.Summary.DeterministicVerdict))
			if metricTarget && item.Escalated {
				metrics.Escalations++
			}
			item.TopConcepts = topSectorConcepts(wv.Summary.Concepts, 5)
			if sectorDistractorBeatsTarget(wv.Summary.Concepts) {
				item.DistractorBeatsTarget = true
				if metricTarget {
					metrics.DistractorBeatsTarget++
				}
			}
			// Collect the FULL concept set (not the capped TopConcepts) for the offline
			// replay analysis — only for labeled rows, which are the only ones it scores.
			if item.Label != "" {
				replayInputs = append(replayInputs, sectorReplayInput{
					label:                item.Label,
					concepts:             wv.Summary.Concepts,
					deterministicVerdict: wv.Summary.DeterministicVerdict,
					origin:               t.Origin,
				})
			}
		}

		if metricTarget && item.Label != "" && item.Validated {
			scoreSectorPredictor(&metrics.Deterministic, item.Label, item.DeterministicBucket)
			scoreSectorPredictor(&metrics.LLM, item.Label, item.LLMBucket)
			scoreSectorPredictor(&metrics.Final, item.Label, item.FinalBucket)

			if item.DeterministicBucket != "" && item.LLMBucket != "" {
				detOK := item.DeterministicBucket == item.Label
				llmOK := item.LLMBucket == item.Label
				switch {
				case detOK && llmOK:
					metrics.LLMVsDeterministic["bothCorrect"]++
				case detOK && !llmOK:
					metrics.LLMVsDeterministic["detOnly"]++
				case !detOK && llmOK:
					metrics.LLMVsDeterministic["llmOnly"]++
				default:
					metrics.LLMVsDeterministic["bothWrong"]++
				}
			}
		}
		items = append(items, item)
	}

	finalizeSectorPredictor(&metrics.Deterministic)
	finalizeSectorPredictor(&metrics.LLM)
	finalizeSectorPredictor(&metrics.Final)
	if metrics.Validated > 0 {
		metrics.EscalationRate = round4(float64(metrics.Escalations) / float64(metrics.Validated))
		metrics.Domain.ResolutionRate = round4(float64(metrics.Domain.Resolved) / float64(metrics.Validated))
	}

	perimeter := SectorEvalPerimeter{}
	if detail.Strategy != nil {
		st := detail.Strategy.Strategy
		perimeter = SectorEvalPerimeter{
			Title:             st.Title,
			SectorDescription: st.SectorDescription,
			Thesis:            st.Thesis,
			TerritoryLabel:    st.TerritoryLabel,
			Provinces:         st.Provinces,
			LegalForms:        st.LegalForms,
			ActivityStatus:    st.ActivityStatus,
			AtecoCandidates:   st.AtecoCandidates,
			Keywords:          st.Keywords,
			TurnoverAround:    st.TurnoverAround,
			TurnoverMin:       st.TurnoverMin,
			TurnoverMax:       st.TurnoverMax,
			EmployeeMin:       st.EmployeeMin,
			EmployeeMax:       st.EmployeeMax,
		}
	}

	return SectorEvalReport{
		SessionID: sessionID,
		Perimeter: perimeter,
		Items:     items,
		Metrics:   metrics,
		Replay:    computeSectorReplay(replayInputs),
	}, nil
}

// scoreSectorPredictor records one (label, prediction) pair for a predictor. Rows where
// the predictor produced no bucket (e.g. the LLM did not run) are skipped, so each
// predictor's accuracy is over the rows it actually decided.
func scoreSectorPredictor(m *SectorEvalPredictorMetrics, label, bucket string) {
	if bucket == "" {
		return
	}
	m.Evaluable++
	m.Confusion[label][bucket]++
	if label == bucket {
		m.Correct++
	}
}

func finalizeSectorPredictor(m *SectorEvalPredictorMetrics) {
	if m.Evaluable > 0 {
		m.Accuracy = round4(float64(m.Correct) / float64(m.Evaluable))
	}
}

// deterministicVerdictToBucket maps the raw embed+rerank verdict onto keep/forse/scarta.
// Without the LLM, weak/ambiguous/no_signal are all "undecided" -> forse. An empty verdict
// (domain unresolved / KB unavailable) yields "" so the row is excluded from A's accuracy.
func deterministicVerdictToBucket(verdict string) string {
	switch strings.TrimSpace(verdict) {
	case "confirm":
		return "keep"
	case "reject":
		return "scarta"
	case "weak", "ambiguous", "no_signal":
		return "forse"
	default:
		return ""
	}
}

// analystToBucket mirrors analystToLifecycle's action mapping onto keep/forse/scarta.
// Empty verdict AND action means the analyst did not run on this company -> "".
func analystToBucket(verdict, action string) string {
	verdict = strings.TrimSpace(verdict)
	action = strings.TrimSpace(action)
	if verdict == "" && action == "" {
		return ""
	}
	switch {
	case action == "confirm" && (verdict == "strong_match" || verdict == "match"):
		return "keep"
	case action == "reject" || verdict == "no_match":
		return "scarta"
	default: // downgrade, weak_match, unclear, ...
		return "forse"
	}
}

// sectorActionToBucket maps the pipeline's final action to the keep/forse/scarta
// label space the operator uses, so prediction and ground truth are comparable.
func sectorActionToBucket(action string) string {
	switch strings.TrimSpace(action) {
	case "confirm":
		return "keep"
	case "reject":
		return "scarta"
	default: // downgrade, review, needs_domain_review, ...
		return "forse"
	}
}

func topSectorConcepts(concepts []maConceptScore, n int) []maConceptScore {
	if len(concepts) <= n {
		return concepts
	}
	return concepts[:n]
}

// sectorDistractorBeatsTarget flags the Gerico defect: a distractor outranks the best
// in-perimeter target. Computed over the persisted top concepts (already rerank-sorted).
func sectorDistractorBeatsTarget(concepts []maConceptScore) bool {
	bestDist, bestTarget := 0.0, 0.0
	sawTarget := false
	for _, c := range concepts {
		if c.Kind == "distractor" {
			if c.RerankProb > bestDist {
				bestDist = c.RerankProb
			}
			continue
		}
		if c.InStrategy {
			sawTarget = true
			if c.RerankProb > bestTarget {
				bestTarget = c.RerankProb
			}
		}
	}
	return sawTarget && bestDist > bestTarget
}

func round4(v float64) float64 {
	return float64(int(v*10000+0.5)) / 10000
}
