package binocolo

import (
	"sort"
	"strings"
	"time"
)

// Scoring v2 — acquisition thesis, signal families and the vetted signal catalog.
//
// The ranking score is a weighted blend of curated signals grouped in three
// families. The acquisition thesis (inferred by the LLM, overridable by the
// analyst) sets the family weights and the direction of ambivalent signals.
// The LLM never invents signals or data paths: it selects a thesis and may
// nudge weights of catalog signals via strategy.SignalWeights.

const (
	maThesisSuccession    = "successione"
	maThesisGrowth        = "crescita"
	maThesisConsolidation = "consolidamento"
	maThesisTuckIn        = "tuck_in"
	maThesisGeneric       = "generico"
)

const (
	maFamilyFit      = "aderenza"
	maFamilyDeal     = "opportunita"
	maFamilyEconomic = "economico"
)

const (
	maSignalAtecoPrecision         = "ateco_precision"
	maSignalTurnoverProximity      = "turnover_proximity"
	maSignalKeywordMatch           = "keyword_match"
	maSignalOwnershipConcentration = "ownership_concentration"
	maSignalCompanyAge             = "company_age"
	maSignalLegalForm              = "legal_form"
	maSignalTurnoverTrend          = "turnover_trend"
	maSignalProductivity           = "productivity"
	maSignalEquitySolidity         = "equity_solidity"
	maSignalSuccessionOwner        = "succession_owner"
)

// maScoringSignal is one catalog entry. Base is the signal's share within its
// family (the family weight from the thesis is split proportionally across the
// intended signals of that family). Percentile signals are ranked across the
// run population in a second pass; absolute signals score themselves in [0,1].
type maScoringSignal struct {
	ID         string
	Family     string
	Label      string
	Base       int
	Percentile bool
	Measure    func(c maSignalContext) maSignalSample
	intended   func(strategy MAStrategySpec) bool
}

// Intended reports whether the signal makes sense for the given strategy
// (preconditions present and not removed by an explicit categorical constraint).
func (s maScoringSignal) Intended(strategy MAStrategySpec) bool {
	if s.intended == nil {
		return true
	}
	return s.intended(strategy)
}

type maSignalSample struct {
	Applicable bool
	Score      float64 // [0,1] for absolute signals
	Raw        float64 // metric ranked across the population for percentile signals
	Label      string
}

func maSignalCatalog() []maScoringSignal {
	return []maScoringSignal{
		{ID: maSignalAtecoPrecision, Family: maFamilyFit, Label: "Aderenza ATECO", Base: 20, Measure: measureAtecoPrecision, intended: func(s MAStrategySpec) bool { return len(s.AtecoCandidates) > 0 }},
		{ID: maSignalTurnoverProximity, Family: maFamilyFit, Label: "Vicinanza dimensione", Base: 15, Measure: measureTurnoverProximity, intended: func(s MAStrategySpec) bool { return maTurnoverIdeal(s) > 0 }},
		{ID: maSignalKeywordMatch, Family: maFamilyFit, Label: "Aderenza settore", Base: 5, Measure: measureKeywordMatch, intended: func(s MAStrategySpec) bool { return len(s.Keywords) > 0 || s.SectorDescription != "" }},
		{ID: maSignalOwnershipConcentration, Family: maFamilyDeal, Label: "Concentrazione proprietà", Base: 12, Measure: measureOwnershipConcentration},
		{ID: maSignalCompanyAge, Family: maFamilyDeal, Label: "Anzianità", Base: 10, Measure: measureCompanyAge},
		{ID: maSignalLegalForm, Family: maFamilyDeal, Label: "Forma giuridica", Base: 8, Measure: measureLegalForm, intended: func(s MAStrategySpec) bool { return len(s.LegalForms) == 0 }},
		{ID: maSignalSuccessionOwner, Family: maFamilyDeal, Label: "Ricambio generazionale", Base: 12, Measure: measureSuccessionOwner, intended: func(s MAStrategySpec) bool { return normalizeMAThesis(s.Thesis) == maThesisSuccession }},
		{ID: maSignalTurnoverTrend, Family: maFamilyEconomic, Label: "Trend fatturato", Base: 12, Percentile: true, Measure: measureTurnoverTrend},
		{ID: maSignalProductivity, Family: maFamilyEconomic, Label: "Produttività", Base: 8, Percentile: true, Measure: measureProductivity},
		{ID: maSignalEquitySolidity, Family: maFamilyEconomic, Label: "Solidità patrimoniale", Base: 10, Measure: measureEquitySolidity},
	}
}

// normalizeMAThesis clamps the thesis to the supported enum, defaulting to generic.
func normalizeMAThesis(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case maThesisSuccession:
		return maThesisSuccession
	case maThesisGrowth:
		return maThesisGrowth
	case maThesisConsolidation:
		return maThesisConsolidation
	case maThesisTuckIn, "tuckin", "tuck-in":
		return maThesisTuckIn
	default:
		return maThesisGeneric
	}
}

// thesisFamilyWeights returns the family weight budget (summing to 100) for a thesis.
func thesisFamilyWeights(thesis string) map[string]int {
	switch normalizeMAThesis(thesis) {
	case maThesisSuccession:
		return map[string]int{maFamilyFit: 30, maFamilyDeal: 50, maFamilyEconomic: 20}
	case maThesisGrowth:
		return map[string]int{maFamilyFit: 35, maFamilyDeal: 20, maFamilyEconomic: 45}
	case maThesisConsolidation:
		return map[string]int{maFamilyFit: 50, maFamilyDeal: 25, maFamilyEconomic: 25}
	case maThesisTuckIn:
		return map[string]int{maFamilyFit: 45, maFamilyDeal: 30, maFamilyEconomic: 25}
	default:
		return map[string]int{maFamilyFit: 40, maFamilyDeal: 30, maFamilyEconomic: 30}
	}
}

// maSignalNominalWeights computes the session-level nominal weight (out of 100)
// of each intended signal: family weight split proportionally by Base across the
// intended signals of the family. Applies the LLM's optional per-signal overrides.
func maSignalNominalWeights(strategy MAStrategySpec) map[string]float64 {
	catalog := maSignalCatalog()
	familyWeights := thesisFamilyWeights(strategy.Thesis)

	// Effective base per intended signal (override scaled relative to default).
	base := map[string]float64{}
	familyBaseSum := map[string]float64{}
	for _, signal := range catalog {
		if !signal.Intended(strategy) {
			continue
		}
		b := float64(signal.Base)
		if override, ok := strategy.SignalWeights[signal.ID]; ok && override > 0 {
			b = float64(override)
		}
		base[signal.ID] = b
		familyBaseSum[signal.Family] += b
	}

	// Redistribute the weight of any family with no intended signals across
	// the families that do, proportionally to their nominal budget.
	activeFamilyBudget := 0
	for family, weight := range familyWeights {
		if familyBaseSum[family] > 0 {
			activeFamilyBudget += weight
		}
	}
	if activeFamilyBudget == 0 {
		return map[string]float64{}
	}

	out := map[string]float64{}
	for _, signal := range catalog {
		b, ok := base[signal.ID]
		if !ok || familyBaseSum[signal.Family] == 0 {
			continue
		}
		familyWeight := float64(familyWeights[signal.Family]) * 100.0 / float64(activeFamilyBudget)
		out[signal.ID] = familyWeight * b / familyBaseSum[signal.Family]
	}
	return out
}

// legalFormTier mirrors company_legal_forms.acquisition_tier (migration 033):
// 1 = capital companies (clean shares), 2 = partnerships, 3 = individual/PF,
// 4 = co-ops/consortia/public bodies/foundations/other.
func legalFormTier(code string) int {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "SP", "AU", "SR", "SU", "RR", "RS", "SD", "SV":
		return 1
	case "SN", "AS", "AA", "SE", "SF", "SI":
		return 2
	case "DI", "PF", "IF", "CE":
		return 3
	default:
		return 4
	}
}

func maKnownSignalIDs() map[string]struct{} {
	out := map[string]struct{}{}
	for _, signal := range maSignalCatalog() {
		out[signal.ID] = struct{}{}
	}
	return out
}

var maLegalFormCodePattern = func() func(string) bool {
	return func(code string) bool {
		if len(code) != 2 {
			return false
		}
		for _, r := range code {
			if r < 'A' || r > 'Z' {
				return false
			}
		}
		return true
	}
}()

// normalizeMALegalForms keeps valid two-letter uppercase legal-form codes (deduped, capped).
func normalizeMALegalForms(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, raw := range values {
		code := strings.ToUpper(strings.TrimSpace(raw))
		if !maLegalFormCodePattern(code) {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
		if len(out) >= 8 {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// normalizeMASignalWeights keeps only known catalog signal IDs, clamping weights to 1..40.
func normalizeMASignalWeights(input map[string]int) map[string]int {
	if len(input) == 0 {
		return nil
	}
	known := maKnownSignalIDs()
	out := map[string]int{}
	for id, weight := range input {
		if _, ok := known[id]; !ok {
			continue
		}
		if weight < 1 {
			continue
		}
		if weight > 40 {
			weight = 40
		}
		out[id] = weight
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// maTurnoverIdeal returns the "sweet spot" turnover used by the proximity signal:
// the explicit turnoverAround, else the midpoint of an explicit min/max range.
func maTurnoverIdeal(strategy MAStrategySpec) int {
	if strategy.TurnoverAround != nil && *strategy.TurnoverAround > 0 {
		return *strategy.TurnoverAround
	}
	if strategy.TurnoverMin != nil && strategy.TurnoverMax != nil {
		mid := (*strategy.TurnoverMin + *strategy.TurnoverMax) / 2
		if mid > 0 {
			return mid
		}
	}
	return 0
}

// successionMinAge returns the owner-age threshold for the succession thesis,
// from the strategy when set to a sane value (LLM-extracted), else the default.
// Shared by the succession_owner signal and the ricambio_generazionale flag.
func successionMinAge(strategy MAStrategySpec) int {
	if strategy.SuccessionMinOwnerAge != nil {
		if v := *strategy.SuccessionMinOwnerAge; v >= 30 && v <= 90 {
			return v
		}
	}
	return maSuccessionDefaultMinAge
}

// maBalanceSheet is one year of the vendor balanceSheets.all[] series.
type maBalanceSheet struct {
	Year        int
	Turnover    *int
	NetWorth    *int
	TotalAssets *int
	Employees   *int
}

// maFinancials is the per-target financial view extracted from the vendor payload.
type maFinancials struct {
	Series       []maBalanceSheet // ascending by year, only years with a filed turnover
	LastYear     int
	Turnover     *int
	NetWorth     *int
	TotalAssets  *int
	Employees    *int
	PrevNetWorth *int
}

func extractFinancials(object map[string]any) maFinancials {
	var fin maFinancials
	raw, ok := vendorPath(object, "balanceSheets.all")
	list, _ := raw.([]any)
	if ok && len(list) > 0 {
		all := make([]maBalanceSheet, 0, len(list))
		for _, item := range list {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			year := vendorIntFromValue(entry["year"])
			if year == nil {
				continue
			}
			sheet := maBalanceSheet{
				Year:        *year,
				Turnover:    vendorIntFromValue(entry["turnover"]),
				NetWorth:    vendorIntFromValue(entry["netWorth"]),
				TotalAssets: vendorIntFromValue(entry["totalAssets"]),
				Employees:   vendorIntFromValue(entry["employees"]),
			}
			all = append(all, sheet)
		}
		// Keep only years with a filed turnover for the trend series.
		filed := make([]maBalanceSheet, 0, len(all))
		for _, sheet := range all {
			if sheet.Turnover != nil {
				filed = append(filed, sheet)
			}
		}
		sort.Slice(filed, func(i, j int) bool { return filed[i].Year < filed[j].Year })
		fin.Series = filed
		if n := len(filed); n > 0 {
			last := filed[n-1]
			fin.LastYear = last.Year
			fin.Turnover = last.Turnover
			fin.NetWorth = last.NetWorth
			fin.TotalAssets = last.TotalAssets
			fin.Employees = last.Employees
			if n >= 2 {
				fin.PrevNetWorth = filed[n-2].NetWorth
			}
		}
	}
	// Fallback to balanceSheets.last when the series is absent.
	if fin.Turnover == nil {
		if last, ok := vendorPath(object, "balanceSheets.last"); ok {
			if entry, ok := last.(map[string]any); ok {
				fin.Turnover = vendorIntFromValue(entry["turnover"])
				if fin.NetWorth == nil {
					fin.NetWorth = vendorIntFromValue(entry["netWorth"])
				}
				if fin.TotalAssets == nil {
					fin.TotalAssets = vendorIntFromValue(entry["totalAssets"])
				}
				if fin.Employees == nil {
					fin.Employees = vendorIntFromValue(entry["employees"])
				}
				if fin.LastYear == 0 {
					if year := vendorIntFromValue(entry["year"]); year != nil {
						fin.LastYear = *year
					}
				}
			}
		}
	}
	return fin
}

func vendorIntFromValue(value any) *int {
	if value == nil {
		return nil
	}
	if parsed, ok := vendorNumber(value); ok {
		v := int(parsed)
		return &v
	}
	return nil
}

// maAge returns the company age in years from startDate/registrationDate.
func maAge(object map[string]any, now time.Time) (int, bool) {
	value := firstVendorString(object, "startDate", "registrationDate")
	if value == "" {
		return 0, false
	}
	layout := "2006-01-02"
	if len(value) >= 10 {
		value = value[:10]
	}
	parsed, err := time.Parse(layout, value)
	if err != nil || parsed.After(now) {
		return 0, false
	}
	age := now.Year() - parsed.Year()
	if now.Month() < parsed.Month() || (now.Month() == parsed.Month() && now.Day() < parsed.Day()) {
		age--
	}
	if age < 0 || age > 400 {
		return 0, false
	}
	return age, true
}
