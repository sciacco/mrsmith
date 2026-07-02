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
	params   maScoringParams
}

// maScoringParams carries the configurable (ma_parameter) levers that influence
// scoring. Kept separate from the additive signal catalog so scoreMATargetsV2
// stays a pure, deterministic function of its inputs: the call site reads the
// values from the parameter store (maScoringParamsFromPricing), tests pass them
// explicitly. Zero values fall back to the compiled defaults inside the measure
// functions, so a zero-valued params is safe.
type maScoringParams struct {
	// ThesisFitHoldingFactor in (0,1]: the multiplier applied to a
	// holding-controlled company under the succession thesis. 1.0 disables it.
	ThesisFitHoldingFactor float64
	// Ancore delle rampe ASSOLUTE dei segnali economici (sostituiscono i
	// percentili pool-relativi, migrazione 081). TrendCagrFloor < 0 (es. -0.10):
	// a quel declino il sotto-score trend tocca 0.1; a crescita 0 vale 0.5; a
	// TrendCagrTop (> 0) vale 1.0, lineare nei tratti intermedi.
	TrendCagrFloor float64
	TrendCagrTop   float64
	// Produttività (€/dipendente): floor → 0.1, top → 1.0, lineare in mezzo.
	ProductivityFloorEUR float64
	ProductivityTopEUR   float64
}

// scoreMATargetsV2 scores and ranks a run's targets with the catalog/thesis model.
// Every signal is ABSOLUTE (the former percentile signals now score on anchored
// ramps), so each target scores independently of the pool: same input → same
// score, across runs and sessions (score_version 3). Weights are re-normalized
// per target over the signals that actually have data (missing data lowers
// confidence, never penalizes the score — the coverage tier, not the number,
// carries the caveat). The blended fit is then scaled by two multiplicative
// factors that live OUTSIDE the re-normalization — viability (distress) and
// thesisFit (structural contradiction of the thesis) — each surfaced as an
// Adjustment so the displayed score reconstructs from the breakdown. Returns the
// targets sorted by score desc, then coverage desc, viability desc, turnover
// desc, company name (the graded sub-scores make exact ties common; alphabetical
// alone would be an arbitrary rank).
func scoreMATargetsV2(targets []MATarget, strategy MAStrategySpec, params maScoringParams, now time.Time) []MATarget {
	catalog := maSignalCatalog()
	nominal := maSignalNominalWeights(strategy)
	thesis := normalizeMAThesis(strategy.Thesis)

	type rankKey struct {
		coverage  float64
		viability float64
		turnover  int
	}
	keys := make([]rankKey, len(targets))

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
			params:   params,
		}
		if object != nil {
			ctx.fin = extractFinancials(object)
			ctx.holders = extractShareholders(object)
		}

		type contribution struct {
			signal maScoringSignal
			weight float64
			score  float64
			label  string
			active bool
		}
		contributions := make([]contribution, 0, len(catalog))
		var activeWeight, intendedWeight float64
		econActive := false
		for _, signal := range catalog {
			weight, ok := nominal[signal.ID]
			if !ok {
				continue
			}
			intendedWeight += weight
			sample := signal.Measure(ctx)
			contrib := contribution{signal: signal, weight: weight, label: sample.Label, score: sample.Score}
			if sample.Applicable {
				contrib.active = true
				activeWeight += weight
				if signal.Family == maFamilyEconomic {
					econActive = true
				}
			}
			contributions = append(contributions, contrib)
		}

		evidence := make([]MATargetEvidence, 0, len(contributions))
		missing := make([]string, 0)
		blended := 0.0
		for _, contrib := range contributions {
			status := maEvidenceMissing
			points := 0.0
			// Active rows report the EFFECTIVE weight: the criterion's share of the
			// re-normalized 100-point budget once the weight of any missing criteria
			// has been redistributed onto the survivors. Since points = effectiveWeight
			// * score, the displayed points can never exceed the displayed weight.
			// Inactive rows keep the nominal weight so the UI shows the foregone
			// budget as "0 / N" rather than a meaningless "0 / 0".
			weight := contrib.weight
			if contrib.active && activeWeight > 0 {
				final := contrib.weight / activeWeight
				points = final * contrib.score * 100
				weight = final * 100
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
				Weight:    math.Round(weight*10) / 10,
			})
		}
		postFilterEvidence, postFilterMissing, postFilterOut := maPostFilterEvidence(ctx)
		evidence = append(evidence, postFilterEvidence...)
		missing = append(missing, postFilterMissing...)

		coverage := 0.0
		if intendedWeight > 0 {
			coverage = activeWeight / intendedWeight
		}
		confidence := maConfidenceLabel(coverage)
		// Zero evidenza economica ≠ alta confidenza: sotto le tesi che pesano poco
		// l'economico (successione: 20/100) un'azienda senza alcun bilancio può
		// comunque superare l'80% di coverage. Il numero resta (assente non
		// penalizza), ma l'etichetta non può dichiarare "alta" senza nemmeno un
		// segnale finanziario attivo: cap a media (resta in lista principale, con
		// lo stile "unsure" e il flag bilancio_assente a spiegare perché).
		if confidence == "alta" && !econActive {
			confidence = "media"
		}
		flags := computeMAFlags(ctx)
		viability, knockout, knockoutReason := computeViability(ctx)
		thesisFit := computeThesisFit(ctx, params.ThesisFitHoldingFactor)

		// Sector gate, with the semantic-gate rescue: an ATECO outside the declared
		// divisions is often just a mis-coded company. If the (paid, semantic) UC2
		// gate already confirmed the company as a keep/forse survivor, hiding it on
		// the cruder 2-digit code would re-reject a semantically confirmed target —
		// the dominant false-negative risk. It stays VISIBLE, flagged, and the read
		// path routes it to the actionable drawer. An explicitly excluded subtree is
		// the analyst's own rule and is never rescued.
		sectorOut := !inSectorPerimeter(strategy, targets[i].AtecoCode)
		if sectorOut && resolveAtecoFit(strategy, targets[i].AtecoCode) != maFitExcluded && maGateSurvivor(targets[i]) {
			sectorOut = false
			flags = append(flags, MATargetFlag{Code: maFlagAtecoFuoriPerimetro, Label: "ATECO fuori perimetro (settore confermato dal gate)", Severity: maFlagWarning})
		}
		if sectorOut {
			flags = append(flags, MATargetFlag{Code: maFlagFuoriSettore, Label: "Fuori perimetro ATECO", Severity: maFlagWarning})
		}
		if knockout {
			flags = append(flags, maKnockoutFlag(knockoutReason))
		}

		targets[i].Score = int(math.Round(blended * viability * thesisFit * 100))
		if targets[i].Score > 100 {
			targets[i].Score = 100
		}
		version := maScoreVersion
		targets[i].ScoreVersion = &version
		targets[i].Confidence = confidence
		// Sector gate and viability knockout both land the target in fuori_criterio
		// (hidden by default in the UI). The gate is sector-only; it does not touch
		// the score, so a revealed off-sector row still shows what it would score.
		if knockout || sectorOut || postFilterOut {
			targets[i].MatchState = maMatchStateOutside
		} else {
			targets[i].MatchState = maMatchStateFromConfidence(confidence)
		}
		targets[i].Evidence = evidence
		targets[i].Adjustments = maScoreAdjustments(viability, thesisFit)
		targets[i].Flags = flags
		targets[i].MissingCriteria = cleanStringList(missing, 20, 80)
		targets[i].Rationale = maRationaleV2(thesis, evidence, flags)
		keys[i] = rankKey{coverage: coverage, viability: viability, turnover: intValue(ctx.fin.Turnover)}
	}

	// Rank: score, then tie-break on substance — coverage (documented beats
	// sparse), viability, turnover — before the alphabetical last resort.
	order := make([]int, len(targets))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		i, j := order[a], order[b]
		if targets[i].Score != targets[j].Score {
			return targets[i].Score > targets[j].Score
		}
		if keys[i].coverage != keys[j].coverage {
			return keys[i].coverage > keys[j].coverage
		}
		if keys[i].viability != keys[j].viability {
			return keys[i].viability > keys[j].viability
		}
		if keys[i].turnover != keys[j].turnover {
			return keys[i].turnover > keys[j].turnover
		}
		return targets[i].CompanyName < targets[j].CompanyName
	})
	ranked := make([]MATarget, len(targets))
	for pos, idx := range order {
		ranked[pos] = targets[idx]
	}
	return ranked
}

// maGateSurvivor reports whether the semantic gate POSITIVELY confirmed the
// target as a keep/forse survivor. A missing web-validation is not a
// confirmation (the non-gated execute path has none): the rescue must never
// fire on absence of evidence.
func maGateSurvivor(target MATarget) bool {
	if target.WebValidation == nil {
		return false
	}
	switch gatedTargetBucket(target) {
	case maGatedBucketKeep, maGatedBucketForse:
		return true
	default:
		return false
	}
}

// maKnockoutFlag makes the viability knockout reason machine-readable on the
// persisted row, so the read-time routing (maRouteTarget) can send the target
// to the right destination without re-deriving financials.
func maKnockoutFlag(reason string) MATargetFlag {
	label := "Fuori criterio: vitalità"
	switch reason {
	case maViabilityReasonCeased:
		label = "Fuori criterio: cessata"
	case maViabilityReasonInactive:
		label = "Fuori criterio: non attiva"
	case maViabilityReasonDistress:
		label = "Fuori criterio: distress finanziario"
	}
	return MATargetFlag{Code: maFlagKnockoutVitalita + "_" + reason, Label: label, Severity: maFlagWarning}
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
	maPostFilterRevenuePerEmployeeMin = "revenue_per_employee_min"
	maPostFilterMaxShareholders       = "max_shareholders"
)

func maPostFilterEvidence(c maSignalContext) ([]MATargetEvidence, []string, bool) {
	evidence := make([]MATargetEvidence, 0, 2)
	missing := make([]string, 0, 2)
	outside := false

	if c.strategy.RevenuePerEmployeeMin != nil {
		item, missingText, itemOutside := maRevenuePerEmployeeMinEvidence(c, *c.strategy.RevenuePerEmployeeMin)
		evidence = append(evidence, item)
		if missingText != "" {
			missing = append(missing, missingText)
		}
		outside = outside || itemOutside
	}
	if c.strategy.MaxShareholders != nil {
		item, missingText, itemOutside := maMaxShareholdersEvidence(c, *c.strategy.MaxShareholders)
		evidence = append(evidence, item)
		if missingText != "" {
			missing = append(missing, missingText)
		}
		outside = outside || itemOutside
	}

	return evidence, missing, outside
}

func maRevenuePerEmployeeMinEvidence(c maSignalContext, min int) (MATargetEvidence, string, bool) {
	evidence := MATargetEvidence{
		Criterion: maPostFilterRevenuePerEmployeeMin,
		Family:    maFamilyEconomic,
		Label:     "Ricavo per dipendente minimo",
	}
	pair, reason, ok := resolveMARevenueEmployeePair(c)
	if !ok {
		evidence.Status = maEvidenceMissing
		evidence.Value = reason
		return evidence, "Ricavo/dipendente non valutabile: " + reason, false
	}

	ratio := float64(pair.turnover) / float64(pair.employees)
	evidence.Value = maRevenuePerEmployeeValue(ratio, min, pair)
	if ratio < float64(min) {
		evidence.Status = maEvidenceOutside
		return evidence, "", true
	}
	evidence.Status = maEvidenceMatch
	return evidence, "", false
}

type maRevenueEmployeePair struct {
	turnover  int
	employees int
	year      int
	source    string
}

const (
	maRevenueEmployeeSourceBalanceSheet = "bilancio"
	maRevenueEmployeeSourceHeadline     = "headline"
)

func resolveMARevenueEmployeePair(c maSignalContext) (maRevenueEmployeePair, string, bool) {
	if len(c.fin.Series) > 0 {
		return maRevenueEmployeePairFromSeries(c.fin)
	}
	if pair, reason, ok := maRevenueEmployeePairFromValues(c.fin.Turnover, c.fin.Employees, c.fin.LastYear, maRevenueEmployeeSourceBalanceSheet); ok {
		return pair, "", true
	} else if c.fin.Turnover != nil || c.fin.Employees != nil {
		return maRevenueEmployeePair{}, reason, false
	}
	return maRevenueEmployeePairFromValues(c.target.Turnover, c.target.Employees, intValue(c.target.TurnoverYear), maRevenueEmployeeSourceHeadline)
}

func maRevenueEmployeePairFromSeries(fin maFinancials) (maRevenueEmployeePair, string, bool) {
	hasTurnover := false
	hasEmployees := false
	for i := len(fin.Series) - 1; i >= 0; i-- {
		sheet := fin.Series[i]
		if sheet.Turnover != nil && *sheet.Turnover >= 0 {
			hasTurnover = true
		}
		if sheet.Employees != nil && *sheet.Employees > 0 {
			hasEmployees = true
		}
		if sheet.Turnover == nil || *sheet.Turnover < 0 || sheet.Employees == nil || *sheet.Employees <= 0 {
			continue
		}
		pair := maRevenueEmployeePair{
			turnover:  *sheet.Turnover,
			employees: *sheet.Employees,
			year:      sheet.Year,
			source:    maRevenueEmployeeSourceBalanceSheet,
		}
		if fin.LastYear > 0 && sheet.Year > 0 && sheet.Year < fin.LastYear-1 {
			return maRevenueEmployeePair{}, "bilancio coerente troppo datato", false
		}
		return pair, "", true
	}
	return maRevenueEmployeePair{}, maRevenuePerEmployeeMissingReason(hasTurnover, hasEmployees), false
}

func maRevenueEmployeePairFromValues(turnover, employees *int, year int, source string) (maRevenueEmployeePair, string, bool) {
	hasTurnover := turnover != nil && *turnover >= 0
	hasEmployees := employees != nil && *employees > 0
	if !hasTurnover || !hasEmployees {
		return maRevenueEmployeePair{}, maRevenuePerEmployeeMissingReason(hasTurnover, hasEmployees), false
	}
	return maRevenueEmployeePair{
		turnover:  *turnover,
		employees: *employees,
		year:      year,
		source:    source,
	}, "", true
}

func maRevenuePerEmployeeValue(ratio float64, min int, pair maRevenueEmployeePair) string {
	if pair.year > 0 {
		return fmt.Sprintf("%.0f €/dip anno %d (min %d)", ratio, pair.year, min)
	}
	if pair.source == maRevenueEmployeeSourceHeadline {
		return fmt.Sprintf("%.0f €/dip dato sintetico (min %d)", ratio, min)
	}
	return fmt.Sprintf("%.0f €/dip (min %d)", ratio, min)
}

func maRevenuePerEmployeeMissingReason(hasTurnover, hasEmployees bool) string {
	switch {
	case !hasTurnover && !hasEmployees:
		return "fatturato/dipendenti non disponibili"
	case !hasTurnover:
		return "fatturato non disponibile"
	default:
		return "dipendenti non disponibili"
	}
}

func maMaxShareholdersEvidence(c maSignalContext, max int) (MATargetEvidence, string, bool) {
	evidence := MATargetEvidence{
		Criterion: maPostFilterMaxShareholders,
		Family:    maFamilyDeal,
		Label:     "Numero massimo soci",
	}
	count, ok := extractDirectShareholderCount(c.object)
	if !ok {
		evidence.Status = maEvidenceMissing
		evidence.Value = "soci non disponibili"
		return evidence, "Numero soci non valutabile", false
	}

	evidence.Value = fmt.Sprintf("%d soci (max %d)", count, max)
	if count > max {
		evidence.Status = maEvidenceOutside
		return evidence, "", true
	}
	evidence.Status = maEvidenceMatch
	return evidence, "", false
}

func maPositiveInt(values ...*int) (int, bool) {
	for _, value := range values {
		if value != nil && *value > 0 {
			return *value, true
		}
	}
	return 0, false
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

const (
	maViabilityKnockoutFloor  = 0.25
	maBalancePhysiologicalGap = 2 // years; a filing lag up to here is normal, no penalty
)

// Viability knockout reasons — appended to the knockout flag code so the
// read-time routing can tell a dead/dormant shell (suppress with a count) from
// financial distress (an actionable candidate under a consolidation thesis).
const (
	maViabilityReasonCeased   = "cessata"
	maViabilityReasonInactive = "non_attiva"
	maViabilityReasonDistress = "distress"
)

// Flag codes the scoring layer persists for the read-time routing (maRouteTarget).
const (
	maFlagAtecoFuoriPerimetro = "ateco_fuori_perimetro"
	maFlagFuoriSettore        = "fuori_settore"
	maFlagKnockoutVitalita    = "knockout_vitalita"
)

// computeViability collapses the fit blend toward zero for distressed / dormant
// companies and reports whether the target should be knocked out to
// fuori_criterio, with the dominant reason. Unlike the fit signals it lives
// OUTSIDE the coverage re-normalization — but it punishes only EVIDENCE of
// distress (ceased, dormant, stale filings, negative equity), never the ABSENCE
// of data: an unfiled turnover is physiological for Italian micro filings and
// characteristic of the succession archetype — absence is the coverage tier's
// job (confidence + da_verificare routing), not a distress penalty. Reads the
// same payload signals as computeMAFlags, whose warning flags are the
// human-readable "why" behind a low score.
func computeViability(c maSignalContext) (factor float64, knockout bool, reason string) {
	// Activity status — ceased is a hard knockout.
	if vendorBool(c.object, "taxCodeCeased") {
		return 0, true, maViabilityReasonCeased
	}
	statusFactor := 1.0
	if status := strings.ToUpper(strings.TrimSpace(c.target.ActivityStatus)); status != "" && status != "ATTIVA" {
		statusFactor = 0.2
	}

	// Balance freshness — a filing lag up to maBalancePhysiologicalGap years is
	// physiological (in 2026 a 2024 balance is normal) and not penalized; the
	// bilancio_datato flag still surfaces it as information. A FILED-but-old
	// balance is a real dormancy signal and degrades; a missing turnover is not.
	balanceFactor := 1.0
	if c.fin.Turnover != nil && c.fin.LastYear > 0 {
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
	knockout = factor < maViabilityKnockoutFloor
	if knockout {
		if statusFactor < 1 {
			reason = maViabilityReasonInactive
		} else {
			reason = maViabilityReasonDistress
		}
	}
	return factor, knockout, reason
}

// computeThesisFit returns a multiplicative demotion factor in (0,1] for a target
// that structurally contradicts the acquisition thesis. Like viability it lives
// OUTSIDE the additive blend: missing/ambiguous ownership is neutral (1.0), only
// a clear contradiction demotes — a haircut, never a knockout, so the company
// stays visible and simply ranks lower. Currently only the succession thesis
// defines a contradiction: a company (holding) controls the capital, the
// antithesis of an exiting individual owner. Owner AGE is handled separately by
// the succession_owner signal; this factor reads only the owner TYPE.
func computeThesisFit(c maSignalContext, holdingFactor float64) float64 {
	if c.thesis != maThesisSuccession {
		return 1.0
	}
	holder, ok := dominantHolder(c)
	if !ok {
		return 1.0 // no clear controller (or no shareholder data) → neutral
	}
	if isPersonShareholder(holder) {
		return 1.0 // a natural person controls → on-thesis
	}
	// A company/holding controls. Clamp defensively to (0,1] so a misconfigured
	// parameter can never turn the haircut into a knockout or a bonus.
	if holdingFactor <= 0 || holdingFactor > 1 || math.IsNaN(holdingFactor) {
		holdingFactor = 1 - maThesisFitHoldingHaircutPctDefault/100
	}
	return holdingFactor
}

// dominantHolder returns the controlling shareholder — the sole holder, or the
// one holding >=50% — and whether such a controller was found.
func dominantHolder(c maSignalContext) (maShareholder, bool) {
	if len(c.holders) == 1 {
		return c.holders[0], true
	}
	var best maShareholder
	found := false
	for _, holder := range c.holders {
		if holder.PercentShare >= 50 && (!found || holder.PercentShare > best.PercentShare) {
			best, found = holder, true
		}
	}
	return best, found
}

// maScoreAdjustments surfaces the score multipliers as breakdown rows so the
// displayed score reconstructs from the evidence. Only factors below 1 are
// emitted (a no-op factor stays silent, like the confidence caveat); the
// specific "why" is carried by the warning flags.
func maScoreAdjustments(viability, thesisFit float64) []MATargetAdjustment {
	adjustments := make([]MATargetAdjustment, 0, 2)
	if viability < 1.0 {
		adjustments = append(adjustments, MATargetAdjustment{
			Code:   "viability",
			Label:  "Vitalità ridotta",
			Factor: math.Round(viability*100) / 100,
		})
	}
	if thesisFit < 1.0 {
		adjustments = append(adjustments, MATargetAdjustment{
			Code:   "controllo_holding",
			Label:  "Controllo holding (successione)",
			Factor: math.Round(thesisFit*100) / 100,
		})
	}
	return adjustments
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

// inSectorPerimeter reports whether a target's ATECO falls inside the strategy's
// sector: within one of the declared 2-digit divisions and not under an excluded
// subtree. It is the sector gate — a target outside the perimeter is demoted to
// fuori_criterio (hidden in the UI) no matter how well it scores on the
// sector-blind signals (size, ownership, economics). A sector-less strategy (no
// divisions) gates nothing; a target with no ATECO code is not gated on sector
// (missing data, left to the confidence/coverage measure).
func inSectorPerimeter(strategy MAStrategySpec, atecoCode string) bool {
	if len(strategy.SectorDivisions) == 0 {
		return true
	}
	if resolveAtecoFit(strategy, atecoCode) == maFitExcluded {
		return false
	}
	code := atecoSearchCode(atecoCode)
	if len(code) < 2 {
		return true
	}
	division := code[:2]
	for _, declared := range strategy.SectorDivisions {
		if declared == division {
			return true
		}
	}
	return false
}

// --- Signal measures ---------------------------------------------------------

func measureAtecoPrecision(c maSignalContext) maSignalSample {
	if atecoSearchCode(c.target.AtecoCode) == "" {
		return maSignalSample{}
	}
	// The curated fit tier IS the precision judgment (longest-prefix wins): core is
	// the bullseye, weak is adjacent-but-discounted, neutral is in-scope but
	// unlisted, excluded scores zero (and is gated out of the results anyway).
	var score float64
	switch resolveAtecoFit(c.strategy, c.target.AtecoCode) {
	case maFitCore:
		score = 1.0
	case maFitWeak:
		score = 0.45
	case maFitExcluded:
		score = 0.0
	default:
		score = 0.25
	}
	return maSignalSample{Applicable: true, Score: score, Label: c.target.AtecoCode}
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
	// Only the POSITIVE perimeter feeds the keyword match: an exclusion clause left
	// in sectorDescription ("…esclusi servizi di elaborazione dati contabili") must
	// not let the excluded activity score a full sector match against its own words.
	needles := append([]string{positiveSectorText(c.strategy.SectorDescription)}, c.strategy.Keywords...)
	switch n := significantTokenMatches(haystack, needles); {
	case n >= 2:
		return maSignalSample{Applicable: true, Score: 1.0, Label: "settore coerente"}
	case n == 1:
		return maSignalSample{Applicable: true, Score: 0.6, Label: "settore parziale"}
	default:
		return maSignalSample{Applicable: true, Score: 0.2, Label: "settore debole"}
	}
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

// measureTurnoverTrend scores the revenue trajectory on an ABSOLUTE anchored
// ramp (no pool percentile): the growth metric is the MEDIAN of the annualized
// year-over-year rates across the whole filed series, so a single mis-filed
// year skews an endpoints-CAGR but not this. Same series → same sub-score, in
// any pool.
func measureTurnoverTrend(c maSignalContext) maSignalSample {
	growth, ok := medianAnnualGrowth(c.fin.Series)
	if !ok {
		return maSignalSample{}
	}
	return maSignalSample{Applicable: true, Score: maTrendScore(growth, c.params), Raw: growth, Label: fmt.Sprintf("%+.0f%%/anno", growth*100)}
}

// medianAnnualGrowth returns the median annualized YoY growth over consecutive
// filed years (a >1-year filing gap is annualized geometrically). False when
// fewer than two comparable years exist.
func medianAnnualGrowth(series []maBalanceSheet) (float64, bool) {
	growths := make([]float64, 0, len(series))
	for k := 1; k < len(series); k++ {
		prev, curr := series[k-1], series[k]
		if prev.Turnover == nil || curr.Turnover == nil || *prev.Turnover <= 0 || *curr.Turnover < 0 {
			continue
		}
		gap := curr.Year - prev.Year
		if gap <= 0 {
			gap = 1
		}
		ratio := float64(*curr.Turnover) / float64(*prev.Turnover)
		growths = append(growths, math.Pow(ratio, 1.0/float64(gap))-1.0)
	}
	if len(growths) == 0 {
		return 0, false
	}
	sort.Float64s(growths)
	mid := len(growths) / 2
	if len(growths)%2 == 1 {
		return growths[mid], true
	}
	return (growths[mid-1] + growths[mid]) / 2, true
}

// maTrendScore maps an annual growth rate onto the anchored ramp: floor (a
// decline, e.g. -10%) → 0.1, zero growth → 0.5, top (e.g. +15%) → 1.0, linear
// in between, clamped outside. A declining business can never ride a weak pool
// to the top ("best decliner" is dead).
func maTrendScore(growth float64, params maScoringParams) float64 {
	floor, top := params.TrendCagrFloor, params.TrendCagrTop
	if math.IsNaN(floor) || floor >= 0 {
		floor = maTrendCagrFloorDefault
	}
	if math.IsNaN(top) || top <= 0 {
		top = maTrendCagrTopDefault
	}
	switch {
	case growth <= floor:
		return 0.1
	case growth < 0:
		return 0.1 + (growth-floor)/(-floor)*0.4
	case growth >= top:
		return 1.0
	default:
		return 0.5 + growth/top*0.5
	}
}

// measureProductivity scores €/employee on an absolute anchored ramp (floor →
// 0.1, top → 1.0, linear in between) instead of a pool percentile.
func measureProductivity(c maSignalContext) maSignalSample {
	if c.fin.Turnover == nil || c.fin.Employees == nil || *c.fin.Employees <= 0 {
		return maSignalSample{}
	}
	ratio := float64(*c.fin.Turnover) / float64(*c.fin.Employees)
	return maSignalSample{Applicable: true, Score: maProductivityScore(ratio, c.params), Raw: ratio, Label: fmt.Sprintf("%.0fk/dip", ratio/1000)}
}

func maProductivityScore(ratio float64, params maScoringParams) float64 {
	floor, top := params.ProductivityFloorEUR, params.ProductivityTopEUR
	if floor <= 0 || top <= floor {
		floor, top = maProductivityFloorEURDefault, maProductivityTopEURDefault
	}
	switch {
	case ratio <= floor:
		return 0.1
	case ratio >= top:
		return 1.0
	default:
		return 0.1 + (ratio-floor)/(top-floor)*0.9
	}
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

// measureSuccessionOwner scores the succession readiness of the controlling owner
// by AGE. Applicable only when a natural person controls (>=50%) and the age
// decodes from the tax code; otherwise it drops out and re-normalizes — a
// holding-controlled company is demoted by computeThesisFit, not penalized here.
// The ramp peaks above the threshold and tapers below it, never to zero (a person
// owner always carries some succession relevance). Threshold comes from the
// strategy (LLM-extracted), falling back to maSuccessionDefaultMinAge.
func measureSuccessionOwner(c maSignalContext) maSignalSample {
	age, ok := dominantOwnerAge(c)
	if !ok {
		return maSignalSample{}
	}
	threshold := successionMinAge(c.strategy)
	var score float64
	switch d := age - threshold; {
	case d >= 20:
		score = 1.0
	case d >= 0:
		score = 0.7 + 0.3*float64(d)/20.0
	case d >= -15:
		score = 0.7 + 0.5*float64(d)/15.0 // d<0 → tapers from 0.7 down to 0.2
	default:
		score = 0.1
	}
	return maSignalSample{Applicable: true, Score: score, Label: fmt.Sprintf("socio %d anni", age)}
}
