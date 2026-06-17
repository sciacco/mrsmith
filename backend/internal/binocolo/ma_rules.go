package binocolo

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	errMAStrategyInvalid = errors.New("invalid ma strategy")
	atecoCodePattern     = regexp.MustCompile(`^[A-Z0-9]+(\.[A-Z0-9]+)*$`)
)

func validateMAStrategy(input MAStrategySpec) (MAStrategySpec, error) {
	strategy := input
	strategy.Title = cleanText(strategy.Title, 120)
	strategy.SectorDescription = cleanText(strategy.SectorDescription, 300)
	strategy.TerritoryLabel = cleanText(strategy.TerritoryLabel, 160)
	strategy.ActivityStatus = strings.ToUpper(strings.TrimSpace(strategy.ActivityStatus))
	if strategy.ActivityStatus == "" {
		strategy.ActivityStatus = "ATTIVA"
	}
	if _, ok := companySearchActivityStatuses[strategy.ActivityStatus]; !ok {
		return MAStrategySpec{}, fmt.Errorf("%w: activity status", errMAStrategyInvalid)
	}
	if strategy.SectorDescription == "" {
		return MAStrategySpec{}, fmt.Errorf("%w: sector description", errMAStrategyInvalid)
	}

	provinces := make([]string, 0, len(strategy.Provinces))
	seenProvinces := map[string]struct{}{}
	for _, raw := range strategy.Provinces {
		province, ok := normalizeProvince(raw)
		if !ok {
			return MAStrategySpec{}, fmt.Errorf("%w: province", errMAStrategyInvalid)
		}
		if province == "" {
			continue
		}
		if _, exists := seenProvinces[province]; exists {
			continue
		}
		seenProvinces[province] = struct{}{}
		provinces = append(provinces, province)
	}
	sort.Strings(provinces)
	strategy.Provinces = provinces
	if strategy.TerritoryLabel == "" && len(strategy.Provinces) > 0 {
		strategy.TerritoryLabel = strings.Join(strategy.Provinces, ", ")
	}

	if strategy.TurnoverAround != nil {
		min, max := turnoverAroundRange(*strategy.TurnoverAround)
		if strategy.TurnoverMin == nil {
			strategy.TurnoverMin = &min
		}
		if strategy.TurnoverMax == nil {
			strategy.TurnoverMax = &max
		}
	}
	if err := validateOptionalRange(strategy.TurnoverMin, strategy.TurnoverMax, "turnover"); err != nil {
		return MAStrategySpec{}, err
	}
	if err := validateOptionalRange(strategy.EmployeeMin, strategy.EmployeeMax, "employees"); err != nil {
		return MAStrategySpec{}, err
	}
	strategy.SearchLimit = normalizeMASearchLimit(strategy.SearchLimit)

	candidates := make([]MAAtecoCandidate, 0, len(strategy.AtecoCandidates))
	seenAteco := map[string]struct{}{}
	for _, raw := range strategy.AtecoCandidates {
		code := normalizeAtecoCode(raw.Code)
		if code == "" {
			continue
		}
		if !atecoCodePattern.MatchString(code) {
			return MAStrategySpec{}, fmt.Errorf("%w: ateco", errMAStrategyInvalid)
		}
		dedupeKey := strings.ReplaceAll(code, ".", "")
		if _, exists := seenAteco[dedupeKey]; exists {
			continue
		}
		seenAteco[dedupeKey] = struct{}{}
		candidates = append(candidates, MAAtecoCandidate{
			Code:        code,
			Description: cleanText(raw.Description, 180),
			Rationale:   cleanText(raw.Rationale, 240),
		})
	}
	strategy.AtecoCandidates = candidates
	strategy.ExcludedAteco = normalizeAtecoCodeList(strategy.ExcludedAteco, 16)

	strategy.Keywords = cleanStringList(strategy.Keywords, 12, 80)
	strategy.MissingCriteria = cleanStringList(strategy.MissingCriteria, 12, 120)
	strategy.Rationale = cleanText(strategy.Rationale, 600)
	strategy.ExpandedClassification = cleanText(strategy.ExpandedClassification, 240)
	strategy.SelectedStrategy = normalizeMAStrategyType(strategy.SelectedStrategy)
	strategy.ScoringCriteria = normalizeMAScoringCriteria(strategy.ScoringCriteria)
	strategy.Thesis = normalizeMAThesis(strategy.Thesis)
	strategy.LegalForms = normalizeMALegalForms(strategy.LegalForms)
	strategy.SignalWeights = normalizeMASignalWeights(strategy.SignalWeights)

	return strategy, nil
}

func normalizeMASearchLimit(value int) int {
	if value <= 0 {
		return maDefaultSearchLimit
	}
	if value > maVendorLimit {
		return maVendorLimit
	}
	return value
}

func normalizeMAScoringCriteria(input []MAScoringCriterion) []MAScoringCriterion {
	out := make([]MAScoringCriterion, 0, len(input))
	seen := map[string]struct{}{}
	for _, raw := range input {
		label := cleanText(raw.Label, 120)
		if label == "" {
			continue
		}
		id := normalizeCriterionID(raw.ID)
		if id == "" {
			id = normalizeCriterionID(label)
		}
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		weight := raw.Weight
		if weight <= 0 {
			weight = 10
		}
		if weight > 40 {
			weight = 40
		}
		out = append(out, MAScoringCriterion{
			ID:          id,
			Label:       label,
			Description: cleanText(raw.Description, 240),
			Weight:      weight,
			Evaluation: MAScoringEvaluation{
				SourcePath: cleanText(raw.Evaluation.SourcePath, 120),
				Operator:   normalizeCriterionOperator(raw.Evaluation.Operator),
				Value:      raw.Evaluation.Value,
				Min:        raw.Evaluation.Min,
				Max:        raw.Evaluation.Max,
				Tolerance:  raw.Evaluation.Tolerance,
				Match:      normalizeCriterionMatch(raw.Evaluation.Match),
			},
			Source: cleanText(raw.Source, 80),
		})
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func normalizeCriterionID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastUnderscore = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
		if b.Len() >= 64 {
			break
		}
	}
	return strings.Trim(b.String(), "_")
}

func normalizeCriterionOperator(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "", "exists", "eq", "equals", "not_equals", "neq", "contains", "starts_with", "gt", "gte", "lt", "lte", "between", "range", "min", "max":
		return value
	default:
		return ""
	}
}

func normalizeCriterionMatch(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "all" {
		return "all"
	}
	return "any"
}

func turnoverAroundRange(value int) (int, int) {
	if value < 0 {
		value = 0
	}
	min := int(math.Round(float64(value) * 0.70))
	max := int(math.Round(float64(value) * 1.30))
	return min, max
}

func validateOptionalRange(min, max *int, label string) error {
	if min != nil && *min < 0 {
		return fmt.Errorf("%w: %s min", errMAStrategyInvalid, label)
	}
	if max != nil && *max < 0 {
		return fmt.Errorf("%w: %s max", errMAStrategyInvalid, label)
	}
	if min != nil && max != nil && *min > *max {
		return fmt.Errorf("%w: %s range", errMAStrategyInvalid, label)
	}
	return nil
}

func normalizeAtecoCode(raw string) string {
	value := strings.ToUpper(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, " ", "")
	return value
}

// normalizeAtecoCodeList keeps well-formed, deduped ATECO codes (by dot-stripped
// key), capped at max. Unlike the candidate list it never errors on a malformed
// entry — an exclusion is best-effort, so a bad code is simply dropped.
func normalizeAtecoCodeList(values []string, max int) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, raw := range values {
		code := normalizeAtecoCode(raw)
		if code == "" || !atecoCodePattern.MatchString(code) {
			continue
		}
		key := strings.ReplaceAll(code, ".", "")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, code)
		if max > 0 && len(out) >= max {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// atecoDivision returns the 2-digit division of an ATECO code (the dot-stripped
// leading pair), or "" if it has fewer than two leading characters.
func atecoDivision(code string) string {
	sc := atecoSearchCode(code)
	if len(sc) < 2 {
		return ""
	}
	return sc[:2]
}

// atecoDivisions returns the distinct 2-digit divisions of the candidates, sorted.
// This is the sector perimeter used both to widen the expanded search (the whole
// division subtree) and to gate scoring (a target outside these divisions is
// fuori_criterio).
func atecoDivisions(candidates []MAAtecoCandidate) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, candidate := range candidates {
		div := atecoDivision(candidate.Code)
		if div == "" {
			continue
		}
		if _, exists := seen[div]; exists {
			continue
		}
		seen[div] = struct{}{}
		out = append(out, div)
	}
	sort.Strings(out)
	return out
}

// positiveSectorText returns the positive part of a sector description, dropping a
// trailing exclusion clause ("… esclusi/escluse/tranne/eccetto …"). The exclusion
// belongs in excludedAteco, not in the keyword needles — otherwise the excluded
// activity matches the sector on its own words. If no marker is present the text is
// returned unchanged.
func positiveSectorText(text string) string {
	lower := strings.ToLower(text)
	cut := len(text)
	for _, marker := range []string{"esclus", "esclud", "tranne", "eccetto", "ad eccezione", "salvo"} {
		if idx := strings.Index(lower, marker); idx >= 0 && idx < cut {
			cut = idx
		}
	}
	return strings.TrimRight(strings.TrimSpace(text[:cut]), ",;:-– ")
}

// significantTokenMatches counts the DISTINCT meaningful tokens (>=4 runes, not a
// generic filler stopword like "servizi"/"attività") drawn from needles that occur
// in haystack. A single shared filler word no longer claims sector adherence: the
// keyword signal requires >=2 such hits for a full match. Reuses the ATECO-search
// stopword set so the bar is the same one the strategy builder already applies.
func significantTokenMatches(haystack string, needles []string) int {
	source := strings.ToLower(haystack)
	seen := map[string]struct{}{}
	count := 0
	for _, needle := range needles {
		for _, token := range strings.Fields(strings.ToLower(needle)) {
			token = strings.TrimFunc(token, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
			if len([]rune(token)) < 4 || atecoSearchStopwords[token] {
				continue
			}
			if _, exists := seen[token]; exists {
				continue
			}
			if strings.Contains(source, token) {
				seen[token] = struct{}{}
				count++
			}
		}
	}
	return count
}

func normalizeMAStrategyType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case maStrategyTypeATECO:
		return maStrategyTypeATECO
	case maStrategyTypeExpanded:
		return maStrategyTypeExpanded
	default:
		return ""
	}
}

func cleanText(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

func cleanStringList(values []string, maxItems int, maxRunes int) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, raw := range values {
		value := cleanText(raw, maxRunes)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
		if maxItems > 0 && len(out) >= maxItems {
			break
		}
	}
	return out
}

func chooseSelectedStrategy(atecoCount int, hasAteco bool) string {
	if hasAteco && atecoCount >= maATECOSuccessThreshold {
		return maStrategyTypeATECO
	}
	return maStrategyTypeExpanded
}

func chooseSelectedStrategyFromEstimates(estimates []MAEstimate, hasAteco bool) string {
	atecoCount := estimateTotal(estimates, maStrategyTypeATECO)
	atecoBroad := estimatesTooBroad(estimates, maStrategyTypeATECO)
	expandedBroad := estimatesTooBroad(estimates, maStrategyTypeExpanded)
	if hasAteco && !atecoBroad && atecoCount >= maATECOSuccessThreshold {
		return maStrategyTypeATECO
	}
	if !expandedBroad && estimatesForStrategy(estimates, maStrategyTypeExpanded) > 0 {
		return maStrategyTypeExpanded
	}
	if hasAteco && !atecoBroad && atecoCount > 0 {
		return maStrategyTypeATECO
	}
	return ""
}

func ageFromItalianTaxCode(taxCode string, now time.Time) (int, bool) {
	code := strings.ToUpper(strings.TrimSpace(taxCode))
	if len(code) != 16 {
		return 0, false
	}
	yearPart := code[6:8]
	monthChar := rune(code[8])
	dayPart := code[9:11]
	year, err := strconv.Atoi(yearPart)
	if err != nil {
		return 0, false
	}
	month := italianTaxCodeMonth(monthChar)
	if month == 0 {
		return 0, false
	}
	day, err := strconv.Atoi(dayPart)
	if err != nil {
		return 0, false
	}
	if day > 40 {
		day -= 40
	}
	if day < 1 || day > 31 {
		return 0, false
	}

	currentYear := now.Year()
	century := (currentYear / 100) * 100
	fullYear := century + year
	if fullYear > currentYear {
		fullYear -= 100
	}
	birth := time.Date(fullYear, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if birth.After(now) {
		return 0, false
	}
	age := currentYear - birth.Year()
	if now.Month() < birth.Month() || (now.Month() == birth.Month() && now.Day() < birth.Day()) {
		age--
	}
	if age < 0 || age > 120 {
		return 0, false
	}
	return age, true
}

func italianTaxCodeMonth(value rune) int {
	switch value {
	case 'A':
		return 1
	case 'B':
		return 2
	case 'C':
		return 3
	case 'D':
		return 4
	case 'E':
		return 5
	case 'H':
		return 6
	case 'L':
		return 7
	case 'M':
		return 8
	case 'P':
		return 9
	case 'R':
		return 10
	case 'S':
		return 11
	case 'T':
		return 12
	default:
		return 0
	}
}

func dedupeMATargets(targets []MATarget) []MATarget {
	out := make([]MATarget, 0, len(targets))
	seen := map[string]struct{}{}
	for _, target := range targets {
		key := maTargetDedupeKey(target)
		if key == "" {
			key = fmt.Sprintf("row:%d", len(out))
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, target)
	}
	return out
}

func maTargetDedupeKey(target MATarget) string {
	for _, value := range []string{target.VendorID, target.VATCode, target.TaxCode} {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value != "" {
			return value
		}
	}
	return strings.ToUpper(strings.TrimSpace(target.CompanyName))
}

func maExportRows(targets []MATarget) [][]any {
	rows := make([][]any, 0, len(targets)+1)
	rows = append(rows, []any{
		"Azienda",
		"Partita IVA",
		"Codice fiscale",
		"Provincia",
		"Comune",
		"Stato",
		"Fatturato",
		"ATECO",
		"Descrizione ATECO",
		"Punteggio",
		"Esito",
		"Preferito",
		"Analisi",
		"EV stimato",
		"Equity stimato",
		"Multiplo",
		"Margine EBITDA",
		"PFN/EBITDA",
		"RAG",
		"Criteri mancanti",
		"Motivazione",
		"Verdetto",
	})
	for _, target := range targets {
		turnover := ""
		if target.Turnover != nil {
			turnover = strconv.Itoa(*target.Turnover)
		}
		deep := maDeepExportFields(target.Deep)
		rows = append(rows, []any{
			target.CompanyName,
			target.VATCode,
			target.TaxCode,
			target.Province,
			target.Town,
			target.ActivityStatus,
			turnover,
			target.AtecoCode,
			target.AtecoDescription,
			target.Score,
			matchStateLabel(target.MatchState),
			maRatingExportLabel(target.Rating),
			deep.analysis,
			deep.ev,
			deep.equity,
			deep.multiple,
			deep.ebitdaMargin,
			deep.pfnEbitda,
			deep.rag,
			strings.Join(target.MissingCriteria, ", "),
			target.Rationale,
			deep.verdict,
		})
	}
	return rows
}

func maRatingExportLabel(rating *int) string {
	if rating == nil {
		return ""
	}
	switch {
	case *rating == maRatingExcluded:
		return "escluso"
	case *rating >= 1 && *rating <= maRatingMax:
		return strings.Repeat("★", *rating)
	default:
		return ""
	}
}

type maDeepExportRow struct {
	analysis, ev, equity, multiple, ebitdaMargin, pfnEbitda, rag, verdict string
}

// maDeepExportFields flattens the deep analysis into export cells, populated only
// when the analysis is ready; pending/absent rows leave the deep columns blank.
func maDeepExportFields(deep *MADeepAnalysis) maDeepExportRow {
	var out maDeepExportRow
	if deep == nil {
		return out
	}
	out.analysis = maDeepStatusExportLabel(deep.Status)
	if deep.Status != maDeepStatusReady {
		return out
	}
	if deep.Scorecard != nil {
		out.rag = maRAGExportLabel(deep.Scorecard.OverallRAG)
		if v := scorecardMetricValue(deep.Scorecard, "ebitda_margin"); v != nil {
			out.ebitdaMargin = fmt.Sprintf("%.1f%%", *v)
		}
		if v := scorecardMetricValue(deep.Scorecard, "pfn_ebitda"); v != nil {
			out.pfnEbitda = fmt.Sprintf("%.1fx", *v)
		}
	}
	if deep.Valuation != nil {
		out.ev = fmt.Sprintf("%.0f - %.0f", deep.Valuation.EVLow, deep.Valuation.EVHigh)
		if deep.Valuation.EquityLow != nil && deep.Valuation.EquityHigh != nil {
			out.equity = fmt.Sprintf("%.0f - %.0f", *deep.Valuation.EquityLow, *deep.Valuation.EquityHigh)
		}
		method := "EV/EBITDA"
		if deep.Valuation.Method == "ev_sales" {
			method = "EV/Sales"
		}
		out.multiple = fmt.Sprintf("%s %.2fx", method, deep.Valuation.Multiple)
	}
	if deep.Brief != nil {
		out.verdict = deep.Brief.Verdict
	}
	return out
}

func scorecardMetricValue(scorecard *MADeepScorecard, key string) *float64 {
	for i := range scorecard.Metrics {
		if scorecard.Metrics[i].Key == key {
			return scorecard.Metrics[i].Value
		}
	}
	return nil
}

func maRAGExportLabel(rag string) string {
	switch rag {
	case maRAGGreen:
		return "solido"
	case maRAGAmber:
		return "attenzione"
	case maRAGRed:
		return "critico"
	default:
		return ""
	}
}

func maDeepStatusExportLabel(status string) string {
	switch status {
	case maDeepStatusQueued:
		return "in coda"
	case maDeepStatusRunning:
		return "in corso"
	case maDeepStatusReady:
		return "pronta"
	case maDeepStatusFailed:
		return "non riuscita"
	default:
		return ""
	}
}

func matchStateLabel(value string) string {
	switch value {
	case maMatchStateMatch:
		return "match"
	case maMatchStatePartial:
		return "match parziale"
	case maMatchStateOutside:
		return "fuori criterio"
	default:
		return value
	}
}

func strategyToRaw(strategy MAStrategySpec) (json.RawMessage, error) {
	raw, err := json.Marshal(strategy)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func rawToStrategy(raw []byte) (MAStrategySpec, error) {
	var strategy MAStrategySpec
	if len(strings.TrimSpace(string(raw))) == 0 {
		return strategy, nil
	}
	if err := json.Unmarshal(raw, &strategy); err != nil {
		return strategy, err
	}
	return validateMAStrategy(strategy)
}
