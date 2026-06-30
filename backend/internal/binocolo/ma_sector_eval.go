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

// SectorEvalItem is one company row: the company, the current system prediction, and
// the human label (if any).
type SectorEvalItem struct {
	CompanyKey            string           `json:"companyKey"`
	CompanyName           string           `json:"companyName"`
	Domain                string           `json:"domain,omitempty"`
	AtecoDescription      string           `json:"atecoDescription,omitempty"`
	SelfDescription       string           `json:"selfDescription,omitempty"`
	Validated             bool             `json:"validated"`
	Verdict               string           `json:"verdict,omitempty"` // deterministic web_validation_state
	FinalAction           string           `json:"finalAction,omitempty"`
	PredictedBucket       string           `json:"predictedBucket,omitempty"` // keep | forse | scarta
	AnalystVerdict        string           `json:"analystVerdict,omitempty"`
	AnalystAction         string           `json:"analystAction,omitempty"`
	Escalated             bool             `json:"escalated"`
	DistractorBeatsTarget bool             `json:"distractorBeatsTarget"`
	TopConcepts           []maConceptScore `json:"topConcepts,omitempty"`
	Label                 string           `json:"label,omitempty"` // ground truth
	Note                  string           `json:"note,omitempty"`
	Agreement             string           `json:"agreement"` // match | mismatch | unlabeled | unvalidated
}

// SectorEvalMetrics is the aggregate over evaluable companies (labeled AND validated).
type SectorEvalMetrics struct {
	Targets               int                       `json:"targets"`
	Validated             int                       `json:"validated"`
	Labeled               int                       `json:"labeled"`
	Evaluable             int                       `json:"evaluable"`
	Correct               int                       `json:"correct"`
	Accuracy              float64                   `json:"accuracy"`
	Escalations           int                       `json:"escalations"`
	EscalationRate        float64                   `json:"escalationRate"`
	DistractorBeatsTarget int                       `json:"distractorBeatsTarget"`
	Confusion             map[string]map[string]int `json:"confusion"` // [label][predicted]
}

// SectorEvalReport is the full eval payload (copyable JSON for the Test page tab).
type SectorEvalReport struct {
	SessionID string            `json:"sessionId"`
	Items     []SectorEvalItem  `json:"items"`
	Metrics   SectorEvalMetrics `json:"metrics"`
}

var sectorEvalBuckets = []string{"keep", "forse", "scarta"}

func (s *maService) setSectorEvalLabel(ctx context.Context, sessionID string, body SectorEvalLabelRequest, subject, email string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	key := strings.TrimSpace(body.CompanyKey)
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

	confusion := map[string]map[string]int{}
	for _, l := range sectorEvalBuckets {
		confusion[l] = map[string]int{}
	}
	metrics := SectorEvalMetrics{Confusion: confusion, Labeled: len(labels)}
	items := make([]SectorEvalItem, 0, len(detail.Targets))

	for _, t := range detail.Targets {
		item := SectorEvalItem{
			CompanyKey:       t.CompanyKey,
			CompanyName:      t.CompanyName,
			AtecoDescription: t.AtecoDescription,
			Agreement:        "unvalidated",
		}
		if lbl, ok := labels[t.CompanyKey]; ok {
			item.Label = lbl.Label
			item.Note = lbl.Note
		}
		metrics.Targets++

		if wv := t.WebValidation; wv != nil {
			item.Validated = true
			metrics.Validated++
			item.Domain = wv.SelectedDomain
			item.SelfDescription = wv.Summary.CompanyDescription
			item.Verdict = wv.WebValidationState
			item.FinalAction = wv.FinalAction
			item.PredictedBucket = sectorActionToBucket(wv.FinalAction)
			item.AnalystVerdict = wv.AnalystVerdict
			item.AnalystAction = wv.AnalystAction
			item.Escalated = strings.TrimSpace(wv.AnalystAction) != ""
			if item.Escalated {
				metrics.Escalations++
			}
			item.TopConcepts = topSectorConcepts(wv.Summary.Concepts, 5)
			if sectorDistractorBeatsTarget(wv.Summary.Concepts) {
				item.DistractorBeatsTarget = true
				metrics.DistractorBeatsTarget++
			}
		}

		switch {
		case item.Label == "":
			item.Agreement = "unlabeled"
		case !item.Validated:
			item.Agreement = "unvalidated"
		default:
			metrics.Evaluable++
			confusion[item.Label][item.PredictedBucket]++
			if item.Label == item.PredictedBucket {
				item.Agreement = "match"
				metrics.Correct++
			} else {
				item.Agreement = "mismatch"
			}
		}
		items = append(items, item)
	}

	if metrics.Evaluable > 0 {
		metrics.Accuracy = round4(float64(metrics.Correct) / float64(metrics.Evaluable))
	}
	if metrics.Validated > 0 {
		metrics.EscalationRate = round4(float64(metrics.Escalations) / float64(metrics.Validated))
	}

	return SectorEvalReport{SessionID: sessionID, Items: items, Metrics: metrics}, nil
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
