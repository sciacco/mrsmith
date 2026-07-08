package binocolo

import (
	"fmt"
	"sort"
	"strings"
)

// UC2 deterministic-layer REPLAY analysis. This is the offline answer to the three
// questions raised in the independent IR review of the embed+rerank classifier —
// computed ENTIRELY from already-persisted concept scores + human labels
// (wv.Summary.Concepts / DeterministicVerdict vs ma_sector_eval_label). No embedder,
// no reranker, no Brave, no scrape: a full re-derivation is sub-second, so threshold
// experiments stop costing a 30-minute live run.
//
//	R1 — is the "distractor beats the best in-perimeter target" signal a real defect,
//	     or mostly off-perimeter companies correctly looking off-perimeter? (slice the
//	     flag by human label: if it concentrates on true-scarta rows it's the system
//	     working, not a bug.)
//	R3 — what is the deterministic CONFIRM / REJECT precision & recall on THIS run? The
//	     "confirm has 0% precision" claim that justifies always-escalating confirm was
//	     measured on the pre-2-way-softmax scale; recompute it on the live verdicts.
//	R2 — the decision question: can a RELATIVE (margin-based) rule lift scarta-recall
//	     toward the LLM's while NEVER auto-rejecting a true keep (keepLeak==0)? The
//	     current rule reads the reranker softmax as an ABSOLUTE threshold; the review
//	     argues the usable signal is relative (target-vs-distractor ordering + margin).
//	     A grid search proves or kills the recoverable-reject hypothesis cheaply.

// SectorReplayReport is attached to the eval report (omitempty) when there are labeled
// rows. It is descriptive only — it changes no production verdict.
type SectorReplayReport struct {
	Rows int `json:"rows"` // labeled + validated rows carrying concepts

	// R1.
	DistractorByLabel map[string]SectorReplayDistractorSlice `json:"distractorByLabel"`

	// R3 — over the persisted deterministic verdicts.
	ConfirmPrecision SectorReplayPR `json:"confirmPrecision"`
	RejectPrecision  SectorReplayPR `json:"rejectPrecision"`

	// R2.
	Baseline SectorReplayRule   `json:"baseline"`           // current absolute rule (persisted verdicts)
	Best     SectorReplayRule   `json:"best"`               // best relative rule with keepLeak==0
	Frontier []SectorReplayRule `json:"frontier,omitempty"` // feasible rules by descending scarta-recall
}

// SectorReplayDistractorSlice is R1's per-label breakdown of distractorBeatsTarget.
type SectorReplayDistractorSlice struct {
	Total          int `json:"total"`          // labeled rows with this label
	DistractorWins int `json:"distractorWins"` // ...where a distractor outranks the best in-perimeter target
}

// SectorReplayPR is precision/recall for one persisted verdict against its target bucket.
type SectorReplayPR struct {
	Fired     int     `json:"fired"`     // rows that emitted this verdict
	Correct   int     `json:"correct"`   // ...whose human label matched the target bucket
	TrueTotal int     `json:"trueTotal"` // rows whose human label IS the target bucket
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
}

// SectorReplayRule is one decision rule's confusion + headline metrics over the labeled
// rows. keepLeak (true-keep predicted scarta) is the hard recall constraint: the whole
// point of a deterministic reject gate is that it never silently discards a real target.
type SectorReplayRule struct {
	Name       string  `json:"name"`
	AbsFloor   float64 `json:"absFloor"`
	RejMargin  float64 `json:"rejMargin"`
	ConfMargin float64 `json:"confMargin"`

	Confusion       map[string]map[string]int `json:"confusion"` // [label][predicted]
	KeepLeak        int                       `json:"keepLeak"`  // true-keep rows predicted scarta (MUST be 0)
	KeepRecall      float64                   `json:"keepRecall"`
	ScartaRecall    float64                   `json:"scartaRecall"`
	ScartaPrecision float64                   `json:"scartaPrecision"`
	Accuracy        float64                   `json:"accuracy"`
}

type sectorReplayInput struct {
	label                string
	concepts             []maConceptScore
	deterministicVerdict string
	origin               string
}

type sectorReplayBests struct {
	bestDistractor float64
	bestInStrategy float64
}

func sectorReplayComputeBests(concepts []maConceptScore) sectorReplayBests {
	var b sectorReplayBests
	for _, c := range concepts {
		if c.Kind == "distractor" {
			if c.RerankProb > b.bestDistractor {
				b.bestDistractor = c.RerankProb
			}
			continue
		}
		if c.InStrategy && c.RerankProb > b.bestInStrategy {
			b.bestInStrategy = c.RerankProb
		}
	}
	return b
}

// sectorReplayPredict is the candidate RELATIVE rule: reject when a distractor beats the
// best in-perimeter target by rejMargin (and clears a low absolute floor); confirm when
// the in-perimeter target beats the best distractor by confMargin; otherwise defer
// (forse → LLM). reject is checked first (we are probing the reject gate), but the
// keepLeak==0 constraint downstream discards any rule where that order rejects a true keep.
func sectorReplayPredict(b sectorReplayBests, absFloor, rejMargin, confMargin float64) string {
	if b.bestDistractor > 0 && b.bestDistractor >= absFloor && b.bestDistractor-b.bestInStrategy >= rejMargin {
		return "scarta"
	}
	if b.bestInStrategy > 0 && b.bestInStrategy >= absFloor && b.bestInStrategy-b.bestDistractor >= confMargin {
		return "keep"
	}
	return "forse"
}

func sectorReplayRuleFromConfusion(name string, absFloor, rejMargin, confMargin float64, conf map[string]map[string]int, rows int) SectorReplayRule {
	totalKeep := conf["keep"]["keep"] + conf["keep"]["forse"] + conf["keep"]["scarta"]
	totalScarta := conf["scarta"]["keep"] + conf["scarta"]["forse"] + conf["scarta"]["scarta"]
	predScarta := conf["keep"]["scarta"] + conf["forse"]["scarta"] + conf["scarta"]["scarta"]
	correct := conf["keep"]["keep"] + conf["forse"]["forse"] + conf["scarta"]["scarta"]
	r := SectorReplayRule{
		Name:       name,
		AbsFloor:   absFloor,
		RejMargin:  rejMargin,
		ConfMargin: confMargin,
		Confusion:  conf,
		KeepLeak:   conf["keep"]["scarta"],
	}
	if totalKeep > 0 {
		r.KeepRecall = round4(float64(conf["keep"]["keep"]) / float64(totalKeep))
	}
	if totalScarta > 0 {
		r.ScartaRecall = round4(float64(conf["scarta"]["scarta"]) / float64(totalScarta))
	}
	if predScarta > 0 {
		r.ScartaPrecision = round4(float64(conf["scarta"]["scarta"]) / float64(predScarta))
	}
	if rows > 0 {
		r.Accuracy = round4(float64(correct) / float64(rows))
	}
	return r
}

func sectorReplayEvaluate(name string, inputs []sectorReplayInput, absFloor, rejMargin, confMargin float64) SectorReplayRule {
	conf := newSectorConfusion()
	for _, in := range inputs {
		pred := sectorReplayPredict(sectorReplayComputeBests(in.concepts), absFloor, rejMargin, confMargin)
		conf[in.label][pred]++
	}
	return sectorReplayRuleFromConfusion(name, absFloor, rejMargin, confMargin, conf, len(inputs))
}

// sectorReplayBaseline reproduces the CURRENT (absolute-threshold) production rule from
// the persisted deterministic verdicts, so the relative rules have a like-for-like
// reference. An empty/unresolved verdict is treated as undecided (forse).
func sectorReplayBaseline(inputs []sectorReplayInput) SectorReplayRule {
	conf := newSectorConfusion()
	for _, in := range inputs {
		bucket := deterministicVerdictToBucket(in.deterministicVerdict)
		if bucket == "" {
			bucket = "forse"
		}
		conf[in.label][bucket]++
	}
	return sectorReplayRuleFromConfusion("current (absolute thresholds, persisted verdicts)",
		maSectorConfirmProbDefault, maSectorRejectMarginDefault, maSectorRejectMarginDefault, conf, len(inputs))
}

func sectorReplayVerdictPR(inputs []sectorReplayInput, verdict, targetBucket string) SectorReplayPR {
	pr := SectorReplayPR{}
	for _, in := range inputs {
		if in.label == targetBucket {
			pr.TrueTotal++
		}
		if strings.TrimSpace(in.deterministicVerdict) == verdict {
			pr.Fired++
			if in.label == targetBucket {
				pr.Correct++
			}
		}
	}
	if pr.Fired > 0 {
		pr.Precision = round4(float64(pr.Correct) / float64(pr.Fired))
	}
	if pr.TrueTotal > 0 {
		pr.Recall = round4(float64(pr.Correct) / float64(pr.TrueTotal))
	}
	return pr
}

func sectorReplayDistractorByLabel(inputs []sectorReplayInput) map[string]SectorReplayDistractorSlice {
	out := map[string]SectorReplayDistractorSlice{}
	for _, l := range sectorEvalBuckets {
		out[l] = SectorReplayDistractorSlice{}
	}
	for _, in := range inputs {
		s := out[in.label]
		s.Total++
		if sectorDistractorBeatsTarget(in.concepts) {
			s.DistractorWins++
		}
		out[in.label] = s
	}
	return out
}

// computeSectorReplay runs R1/R3/R2 over the labeled automatic-search rows. Returns nil
// when there is nothing labeled (so the report omits the section). Manual-origin rows are
// excluded because their gate verdict is informational, not an operational survivor/drop.
func computeSectorReplay(inputs []sectorReplayInput) *SectorReplayReport {
	inputs = automaticSectorReplayInputs(inputs)
	if len(inputs) == 0 {
		return nil
	}
	rep := &SectorReplayReport{
		Rows:              len(inputs),
		DistractorByLabel: sectorReplayDistractorByLabel(inputs),
		ConfirmPrecision:  sectorReplayVerdictPR(inputs, "confirm", "keep"),
		RejectPrecision:   sectorReplayVerdictPR(inputs, "reject", "scarta"),
		Baseline:          sectorReplayBaseline(inputs),
	}

	// R2 grid search over a relative rule family. Low absolute floors (the signal is
	// relative, not a textbook 0-1 probability) crossed with margin sweeps.
	absFloors := []float64{0.0, 0.05, 0.08, 0.10, 0.12}
	rejMargins := []float64{0.0, 0.02, 0.04, 0.06, 0.08, 0.10, 0.12, 0.15}
	confMargins := []float64{0.0, 0.05, 0.10}
	var feasible []SectorReplayRule
	for _, af := range absFloors {
		for _, rm := range rejMargins {
			for _, cm := range confMargins {
				rule := sectorReplayEvaluate("relative", inputs, af, rm, cm)
				if rule.KeepLeak == 0 {
					feasible = append(feasible, rule)
				}
			}
		}
	}
	sort.SliceStable(feasible, func(i, j int) bool {
		if feasible[i].ScartaRecall != feasible[j].ScartaRecall {
			return feasible[i].ScartaRecall > feasible[j].ScartaRecall
		}
		if feasible[i].ScartaPrecision != feasible[j].ScartaPrecision {
			return feasible[i].ScartaPrecision > feasible[j].ScartaPrecision
		}
		return feasible[i].Accuracy > feasible[j].Accuracy
	})
	if len(feasible) > 0 {
		rep.Best = feasible[0]
	}
	// Frontier: distinct (scarta-recall, scarta-precision) operating points, capped.
	seen := map[string]bool{}
	for _, r := range feasible {
		key := fmt.Sprintf("%.4f|%.4f", r.ScartaRecall, r.ScartaPrecision)
		if seen[key] {
			continue
		}
		seen[key] = true
		rep.Frontier = append(rep.Frontier, r)
		if len(rep.Frontier) >= 8 {
			break
		}
	}
	return rep
}

func automaticSectorReplayInputs(inputs []sectorReplayInput) []sectorReplayInput {
	out := make([]sectorReplayInput, 0, len(inputs))
	for _, in := range inputs {
		if in.origin == maTargetOriginManual {
			continue
		}
		out = append(out, in)
	}
	return out
}
