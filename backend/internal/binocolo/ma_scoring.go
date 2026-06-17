package binocolo

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// maSignalContext carries the decoded vendor data a signal needs to score one target.
type maSignalContext struct {
	target   MATarget
	object   map[string]any
	fin      maFinancials
	holders  []maShareholder
	strategy MAStrategySpec
	now      time.Time
	thesis   string
}

// scoreMATargetsV2 scores and ranks a run's targets with the catalog/thesis model.
// Two passes are required because percentile signals (trend, productivity) rank
// each target against the whole population. Weights are re-normalized per target
// over the signals that actually have data (missing data lowers confidence, never
// penalizes the score). Returns the targets sorted by score desc, company name.
func scoreMATargetsV2(targets []MATarget, strategy MAStrategySpec, now time.Time) []MATarget {
	catalog := maSignalCatalog()
	nominal := maSignalNominalWeights(strategy)
	thesis := normalizeMAThesis(strategy.Thesis)

	contexts := make([]maSignalContext, len(targets))
	samples := make([]map[string]maSignalSample, len(targets))
	percentileRaws := map[string][]float64{}

	// Pass 1: decode payloads, evaluate every intended signal, gather percentile raws.
	for i := range targets {
		object, err := decodeVendorObject(targets[i].VendorPayload)
		if err != nil {
			object = nil
		}
		ctx := maSignalContext{
			target:   targets[i],
			object:   object,
			strategy: strategy,
			now:      now,
			thesis:   thesis,
		}
		if object != nil {
			ctx.fin = extractFinancials(object)
			ctx.holders = extractShareholders(object)
		}
		contexts[i] = ctx
		row := make(map[string]maSignalSample, len(catalog))
		for _, signal := range catalog {
			if _, ok := nominal[signal.ID]; !ok {
				continue
			}
			sample := signal.Measure(ctx)
			row[signal.ID] = sample
			if signal.Percentile && sample.Applicable {
				percentileRaws[signal.ID] = append(percentileRaws[signal.ID], sample.Raw)
			}
		}
		samples[i] = row
	}
	for id := range percentileRaws {
		sort.Float64s(percentileRaws[id])
	}

	// Pass 2: convert to graded sub-scores, re-normalize, blend, derive confidence.
	for i := range targets {
		type contribution struct {
			signal maScoringSignal
			weight float64
			score  float64
			label  string
			active bool
		}
		contributions := make([]contribution, 0, len(catalog))
		var activeWeight, intendedWeight float64
		for _, signal := range catalog {
			weight, ok := nominal[signal.ID]
			if !ok {
				continue
			}
			intendedWeight += weight
			sample := samples[i][signal.ID]
			contrib := contribution{signal: signal, weight: weight, label: sample.Label}
			if sample.Applicable {
				contrib.active = true
				if signal.Percentile {
					contrib.score = percentileRank(percentileRaws[signal.ID], sample.Raw)
				} else {
					contrib.score = sample.Score
				}
				activeWeight += weight
			}
			contributions = append(contributions, contrib)
		}

		evidence := make([]MATargetEvidence, 0, len(contributions))
		missing := make([]string, 0)
		blended := 0.0
		for _, contrib := range contributions {
			status := maEvidenceMissing
			points := 0.0
			if contrib.active && activeWeight > 0 {
				final := contrib.weight / activeWeight
				points = final * contrib.score * 100
				blended += final * contrib.score
				if contrib.score >= 0.66 {
					status = maEvidenceMatch
				} else {
					status = maEvidencePartial
				}
			} else {
				missing = append(missing, contrib.signal.Label)
			}
			evidence = append(evidence, MATargetEvidence{
				Criterion: contrib.signal.ID,
				Family:    contrib.signal.Family,
				Status:    status,
				Label:     contrib.signal.Label,
				Value:     contrib.label,
				Points:    math.Round(points*10) / 10,
				Weight:    math.Round(contrib.weight*10) / 10,
			})
		}

		coverage := 0.0
		if intendedWeight > 0 {
			coverage = activeWeight / intendedWeight
		}
		confidence := maConfidenceLabel(coverage)
		flags := computeMAFlags(contexts[i])
		viability, knockout := computeViability(contexts[i])

		targets[i].Score = int(math.Round(blended * viability * 100))
		if targets[i].Score > 100 {
			targets[i].Score = 100
		}
		targets[i].Confidence = confidence
		if knockout {
			targets[i].MatchState = maMatchStateOutside
		} else {
			targets[i].MatchState = maMatchStateFromConfidence(confidence)
		}
		targets[i].Evidence = evidence
		targets[i].Flags = flags
		targets[i].MissingCriteria = cleanStringList(missing, 20, 80)
		targets[i].Rationale = maRationaleV2(thesis, evidence, flags)
	}

	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].Score == targets[j].Score {
			return targets[i].CompanyName < targets[j].CompanyName
		}
		return targets[i].Score > targets[j].Score
	})
	return targets
}

// percentileRank returns the fraction of the population at or below v (ties shared).
func percentileRank(sorted []float64, v float64) float64 {
	n := len(sorted)
	if n <= 1 {
		return 0.5
	}
	less := 0
	equal := 0
	for _, x := range sorted {
		switch {
		case x < v:
			less++
		case x == v:
			equal++
		}
	}
	return (float64(less) + 0.5*float64(equal)) / float64(n)
}

func maConfidenceLabel(coverage float64) string {
	switch {
	case coverage >= 0.8:
		return "alta"
	case coverage >= 0.5:
		return "media"
	default:
		return "bassa"
	}
}

func maMatchStateFromConfidence(confidence string) string {
	if confidence == "alta" {
		return maMatchStateMatch
	}
	return maMatchStatePartial
}

const (
	maViabilityKnockoutFloor  = 0.25
	maBalancePhysiologicalGap = 2 // years; a filing lag up to here is normal, no penalty
)

// computeViability collapses the fit blend toward zero for distressed / dormant
// companies and reports whether the target should be knocked out to
// fuori_criterio. Unlike the fit signals it lives OUTSIDE the coverage
// re-normalization: missing or zero financial health is a penalty here, not a
// neutral drop — so a company that lacks the data to compute solidity (e.g. no
// revenue) can no longer have that weight quietly redistributed onto the signals
// it scores well on. Confidence stays a pure coverage measure; viability drives
// the score. Reads the same payload signals as computeMAFlags, whose warning
// flags are the human-readable "why" behind a low score.
func computeViability(c maSignalContext) (factor float64, knockout bool) {
	// Activity status — ceased is a hard knockout.
	if vendorBool(c.object, "taxCodeCeased") {
		return 0, true
	}
	statusFactor := 1.0
	if status := strings.ToUpper(strings.TrimSpace(c.target.ActivityStatus)); status != "" && status != "ATTIVA" {
		statusFactor = 0.2
	}

	// Balance freshness/presence — a filing lag up to maBalancePhysiologicalGap
	// years is physiological (in 2026 a 2024 balance is normal) and not penalized;
	// the bilancio_datato flag still surfaces it as information.
	balanceFactor := 1.0
	if c.fin.Turnover == nil {
		balanceFactor = 0.3
	} else if c.fin.LastYear > 0 {
		switch gap := c.now.Year() - c.fin.LastYear; {
		case gap <= maBalancePhysiologicalGap:
			balanceFactor = 1.0
		case gap == 3:
			balanceFactor = 0.8
		case gap == 4:
			balanceFactor = 0.6
		case gap == 5:
			balanceFactor = 0.4
		default:
			balanceFactor = 0.25
		}
	}

	// Equity soundness — mirrors the patrimonio_netto flags.
	equityFactor := 1.0
	switch {
	case c.fin.NetWorth != nil && *c.fin.NetWorth < 0:
		equityFactor = 0.3
	case c.fin.NetWorth != nil && c.fin.PrevNetWorth != nil &&
		*c.fin.PrevNetWorth > 0 && float64(*c.fin.NetWorth) < maPatrimonioErosionFactor*float64(*c.fin.PrevNetWorth):
		equityFactor = 0.6
	}

	factor = statusFactor * balanceFactor * equityFactor
	return factor, factor < maViabilityKnockoutFloor
}

func maThesisLabel(thesis string) string {
	switch normalizeMAThesis(thesis) {
	case maThesisSuccession:
		return "successione"
	case maThesisGrowth:
		return "crescita"
	case maThesisConsolidation:
		return "consolidamento"
	case maThesisTuckIn:
		return "tuck-in"
	default:
		return "generico"
	}
}

func maRationaleV2(thesis string, evidence []MATargetEvidence, flags []MATargetFlag) string {
	sorted := append([]MATargetEvidence(nil), evidence...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Points > sorted[j].Points })
	parts := []string{"Tesi " + maThesisLabel(thesis)}
	strengths := make([]string, 0, 2)
	for _, item := range sorted {
		if item.Points > 0 && len(strengths) < 2 {
			strengths = append(strengths, strings.ToLower(item.Label))
		}
	}
	if len(strengths) > 0 {
		parts = append(parts, "punti di forza: "+strings.Join(strengths, ", "))
	}
	for _, flag := range flags {
		if flag.Severity == maFlagWarning {
			parts = append(parts, "attenzione: "+strings.ToLower(flag.Label))
			break
		}
	}
	return cleanText(strings.Join(parts, ". "), 320)
}

// --- Signal measures ---------------------------------------------------------

func measureAtecoPrecision(c maSignalContext) maSignalSample {
	code := normalizeAtecoCode(c.target.AtecoCode)
	code = strings.ReplaceAll(code, ".", "")
	if code == "" {
		return maSignalSample{}
	}
	best := 0.0
	for _, candidate := range c.strategy.AtecoCandidates {
		cc := strings.ReplaceAll(normalizeAtecoCode(candidate.Code), ".", "")
		if cc == "" {
			continue
		}
		switch {
		case strings.HasPrefix(code, cc):
			n := len(cc)
			if n > 6 {
				n = 6
			}
			if score := float64(n) / 6.0; score > best {
				best = score
			}
		case len(code) >= 2 && len(cc) >= 2 && code[:2] == cc[:2]:
			if 0.3 > best {
				best = 0.3
			}
		}
	}
	if best == 0 && textIntersects(c.target.AtecoDescription, append([]string{c.strategy.SectorDescription}, c.strategy.Keywords...)) {
		best = 0.15
	}
	return maSignalSample{Applicable: true, Score: best, Label: c.target.AtecoCode}
}

func measureTurnoverProximity(c maSignalContext) maSignalSample {
	ideal := maTurnoverIdeal(c.strategy)
	if ideal <= 0 || c.fin.Turnover == nil {
		return maSignalSample{}
	}
	rel := math.Abs(float64(*c.fin.Turnover)-float64(ideal)) / float64(ideal)
	var score float64
	switch {
	case rel <= 0.10:
		score = 1.0
	case rel <= 0.25:
		score = 0.8
	case rel <= 0.50:
		score = 0.55
	case rel <= 1.0:
		score = 0.3
	default:
		score = 0.15
	}
	return maSignalSample{Applicable: true, Score: score, Label: strconv.Itoa(*c.fin.Turnover)}
}

func measureKeywordMatch(c maSignalContext) maSignalSample {
	haystack := strings.TrimSpace(c.target.AtecoDescription + " " + c.target.CompanyName)
	needles := append([]string{c.strategy.SectorDescription}, c.strategy.Keywords...)
	if textIntersects(haystack, needles) {
		return maSignalSample{Applicable: true, Score: 1.0, Label: "settore coerente"}
	}
	return maSignalSample{Applicable: true, Score: 0.2, Label: "settore debole"}
}

func measureOwnershipConcentration(c maSignalContext) maSignalSample {
	if len(c.holders) == 0 {
		switch strings.ToUpper(firstVendorString(c.object, "detailedLegalForm.code")) {
		case "AU", "SU":
			return maSignalSample{Applicable: true, Score: 1.0, Label: "socio unico (forma)"}
		}
		return maSignalSample{}
	}
	maxPercent := 0.0
	for _, holder := range c.holders {
		if holder.PercentShare > maxPercent {
			maxPercent = holder.PercentShare
		}
	}
	var score float64
	switch {
	case len(c.holders) == 1 || maxPercent >= 90:
		score = 1.0
	case maxPercent >= 50:
		score = 0.7
	default:
		score = 0.4
	}
	return maSignalSample{Applicable: true, Score: score, Label: shareholderPercentsLabel(c.holders)}
}

func measureCompanyAge(c maSignalContext) maSignalSample {
	if c.thesis == maThesisGeneric {
		return maSignalSample{}
	}
	age, ok := maAge(c.object, c.now)
	if !ok {
		return maSignalSample{}
	}
	var score float64
	if c.thesis == maThesisGrowth {
		switch {
		case age < 8:
			score = 1.0
		case age < 15:
			score = 0.75
		case age < 25:
			score = 0.5
		default:
			score = 0.3
		}
	} else {
		switch {
		case age >= 25:
			score = 1.0
		case age >= 15:
			score = 0.8
		case age >= 8:
			score = 0.5
		default:
			score = 0.3
		}
	}
	return maSignalSample{Applicable: true, Score: score, Label: strconv.Itoa(age) + " anni"}
}

func measureLegalForm(c maSignalContext) maSignalSample {
	code := firstVendorString(c.object, "detailedLegalForm.code")
	if code == "" {
		return maSignalSample{}
	}
	score := 0.1
	switch legalFormTier(code) {
	case 1:
		score = 1.0
	case 2:
		score = 0.6
	case 3:
		score = 0.3
	}
	label := firstVendorString(c.object, "detailedLegalForm.description")
	if label == "" {
		label = code
	}
	return maSignalSample{Applicable: true, Score: score, Label: label}
}

func measureTurnoverTrend(c maSignalContext) maSignalSample {
	series := c.fin.Series
	if len(series) < 2 {
		return maSignalSample{}
	}
	start := len(series) - 3
	if start < 0 {
		start = 0
	}
	first := series[start]
	last := series[len(series)-1]
	if first.Turnover == nil || last.Turnover == nil || *first.Turnover <= 0 {
		return maSignalSample{}
	}
	years := last.Year - first.Year
	if years <= 0 {
		years = 1
	}
	ratio := float64(*last.Turnover) / float64(*first.Turnover)
	cagr := math.Pow(ratio, 1.0/float64(years)) - 1.0
	return maSignalSample{Applicable: true, Raw: cagr, Label: fmt.Sprintf("%+.0f%%/anno", cagr*100)}
}

func measureProductivity(c maSignalContext) maSignalSample {
	if c.fin.Turnover == nil || c.fin.Employees == nil || *c.fin.Employees <= 0 {
		return maSignalSample{}
	}
	ratio := float64(*c.fin.Turnover) / float64(*c.fin.Employees)
	return maSignalSample{Applicable: true, Raw: ratio, Label: fmt.Sprintf("%.0fk/dip", ratio/1000)}
}

func measureEquitySolidity(c maSignalContext) maSignalSample {
	if c.fin.NetWorth == nil {
		return maSignalSample{}
	}
	netWorth := float64(*c.fin.NetWorth)
	var denom float64
	switch {
	case c.fin.TotalAssets != nil && *c.fin.TotalAssets > 0:
		denom = float64(*c.fin.TotalAssets)
	case c.fin.Turnover != nil && *c.fin.Turnover > 0:
		denom = float64(*c.fin.Turnover)
	default:
		return maSignalSample{}
	}
	ratio := netWorth / denom
	var score float64
	switch {
	case ratio <= 0:
		score = 0.0
	case ratio < 0.05:
		score = 0.25
	case ratio < 0.15:
		score = 0.45
	case ratio < 0.30:
		score = 0.7
	default:
		score = 1.0
	}
	return maSignalSample{Applicable: true, Score: score, Label: fmt.Sprintf("%.0f%% PN", ratio*100)}
}
