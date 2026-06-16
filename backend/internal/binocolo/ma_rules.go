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
	atecoCodePattern     = regexp.MustCompile(`^[A-Z]?\d{2}(\.?\d{1,4})?$`)
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

	strategy.Keywords = cleanStringList(strategy.Keywords, 12, 80)
	strategy.MissingCriteria = cleanStringList(strategy.MissingCriteria, 12, 120)
	strategy.Rationale = cleanText(strategy.Rationale, 600)
	strategy.ExpandedClassification = cleanText(strategy.ExpandedClassification, 240)
	strategy.SelectedStrategy = normalizeMAStrategyType(strategy.SelectedStrategy)
	strategy.ScoringCriteria = normalizeMAScoringCriteria(strategy.ScoringCriteria)

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

func scoreMATarget(target MATarget, strategy MAStrategySpec, now time.Time) MATarget {
	score := 0
	missing := make([]string, 0)
	evidence := make([]MATargetEvidence, 0, 8)

	sectorStatus, sectorValue := sectorMatch(target, strategy)
	switch sectorStatus {
	case maEvidenceMatch:
		score += 35
	case maEvidencePartial:
		score += 18
	case maEvidenceMissing:
		missing = append(missing, "settore")
	}
	evidence = append(evidence, MATargetEvidence{
		Criterion:  "sector",
		Status:     sectorStatus,
		Label:      "Settore",
		Value:      sectorValue,
		SourcePath: "atecoClassification",
	})

	turnoverStatus, turnoverValue := turnoverMatch(target, strategy)
	switch turnoverStatus {
	case maEvidenceMatch:
		score += 25
	case maEvidencePartial:
		score += 12
	case maEvidenceMissing:
		missing = append(missing, "fatturato")
	}
	evidence = append(evidence, MATargetEvidence{
		Criterion:  "turnover",
		Status:     turnoverStatus,
		Label:      "Fatturato",
		Value:      turnoverValue,
		SourcePath: "balanceSheets.last.turnover",
	})

	dynamicScore, dynamicMissing, dynamicEvidence := scoreDynamicCriteria(target, strategy.ScoringCriteria)
	score += dynamicScore
	missing = append(missing, dynamicMissing...)
	evidence = append(evidence, dynamicEvidence...)

	completenessStatus := maEvidenceMatch
	completenessValue := "dati principali presenti"
	if len(missing) > 0 {
		completenessStatus = maEvidencePartial
		completenessValue = strings.Join(missing, ", ")
		score += 5
	} else {
		score += 10
	}
	evidence = append(evidence, MATargetEvidence{
		Criterion: "completeness",
		Status:    completenessStatus,
		Label:     "Completezza",
		Value:     completenessValue,
	})

	if score > 100 {
		score = 100
	}
	target.Score = score
	target.MissingCriteria = cleanStringList(append(target.MissingCriteria, missing...), 20, 80)
	target.Evidence = evidence
	target.MatchState = maMatchStateMatch
	for _, item := range evidence {
		if item.Status == maEvidenceOutside {
			target.MatchState = maMatchStateOutside
			break
		}
		if item.Status == maEvidencePartial || item.Status == maEvidenceMissing {
			target.MatchState = maMatchStatePartial
		}
	}
	target.Rationale = buildTargetRationale(target, evidence)
	return target
}

func sectorMatch(target MATarget, strategy MAStrategySpec) (string, string) {
	value := strings.TrimSpace(strings.Join([]string{target.AtecoCode, target.AtecoDescription}, " "))
	if value == "" {
		return maEvidenceMissing, ""
	}
	targetCode := normalizeAtecoCode(target.AtecoCode)
	for _, candidate := range strategy.AtecoCandidates {
		candidateCode := normalizeAtecoCode(candidate.Code)
		if candidateCode != "" && targetCode != "" && strings.HasPrefix(targetCode, strings.ReplaceAll(candidateCode, ".", "")) {
			return maEvidenceMatch, value
		}
	}
	if textIntersects(target.AtecoDescription, append([]string{strategy.SectorDescription}, strategy.Keywords...)) {
		return maEvidencePartial, value
	}
	return maEvidencePartial, value
}

func turnoverMatch(target MATarget, strategy MAStrategySpec) (string, string) {
	if strategy.TurnoverMin == nil && strategy.TurnoverMax == nil {
		return maEvidenceMatch, "criterio non richiesto"
	}
	if target.Turnover == nil {
		return maEvidenceMissing, ""
	}
	value := *target.Turnover
	label := strconv.Itoa(value)
	if strategy.TurnoverMin != nil && value < *strategy.TurnoverMin {
		return maEvidenceOutside, label
	}
	if strategy.TurnoverMax != nil && value > *strategy.TurnoverMax {
		return maEvidenceOutside, label
	}
	return maEvidenceMatch, label
}

func scoreDynamicCriteria(target MATarget, criteria []MAScoringCriterion) (int, []string, []MATargetEvidence) {
	if len(criteria) == 0 {
		return 0, nil, nil
	}
	totalWeight := 0
	for _, criterion := range criteria {
		totalWeight += criterion.Weight
	}
	if totalWeight <= 0 {
		return 0, nil, nil
	}

	score := 0
	missing := make([]string, 0)
	evidence := make([]MATargetEvidence, 0, len(criteria))
	for _, criterion := range criteria {
		status, value, sourcePath := evaluateDynamicCriterion(target, criterion)
		normalizedWeight := int(math.Round(float64(criterion.Weight) / float64(totalWeight) * 30))
		if normalizedWeight <= 0 {
			normalizedWeight = 1
		}
		switch status {
		case maEvidenceMatch:
			score += normalizedWeight
		case maEvidencePartial:
			score += normalizedWeight / 2
		case maEvidenceMissing:
			missing = append(missing, criterion.Label)
		}
		evidence = append(evidence, MATargetEvidence{
			Criterion:  "dynamic_" + criterion.ID,
			Status:     status,
			Label:      criterion.Label,
			Value:      value,
			SourcePath: sourcePath,
		})
	}
	if score > 30 {
		score = 30
	}
	return score, missing, evidence
}

func evaluateDynamicCriterion(target MATarget, criterion MAScoringCriterion) (string, string, string) {
	sourcePath := strings.TrimSpace(criterion.Evaluation.SourcePath)
	if sourcePath == "" {
		return maEvidenceMissing, "non verificabile", ""
	}
	values := dynamicCriterionValues(target, sourcePath)
	if len(values) == 0 {
		return maEvidenceMissing, "", sourcePath
	}
	operator := criterion.Evaluation.Operator
	if operator == "" {
		if criterion.Evaluation.Min != nil || criterion.Evaluation.Max != nil {
			operator = "between"
		} else if criterion.Evaluation.Value != nil {
			operator = "contains"
		} else {
			operator = "exists"
		}
	}
	if operator == "range" {
		operator = "between"
	}
	if operator == "equals" {
		operator = "eq"
	}
	if operator == "neq" {
		operator = "not_equals"
	}
	matches := 0
	verifiable := 0
	labels := make([]string, 0, len(values))
	for _, value := range values {
		label := dynamicValueLabel(value)
		if label != "" {
			labels = append(labels, label)
		}
		ok, canVerify := dynamicValueMatches(value, criterion.Evaluation, operator)
		if !canVerify {
			continue
		}
		verifiable++
		if ok {
			matches++
		}
	}
	if verifiable == 0 {
		return maEvidenceMissing, "non verificabile", sourcePath
	}
	valueLabel := cleanText(strings.Join(labels, ", "), 180)
	if criterion.Evaluation.Match == "all" {
		if matches == verifiable {
			return maEvidenceMatch, valueLabel, sourcePath
		}
		if matches > 0 {
			return maEvidencePartial, valueLabel, sourcePath
		}
		return maEvidenceOutside, valueLabel, sourcePath
	}
	if matches > 0 {
		return maEvidenceMatch, valueLabel, sourcePath
	}
	return maEvidenceOutside, valueLabel, sourcePath
}

func dynamicCriterionValues(target MATarget, sourcePath string) []any {
	path := strings.TrimPrefix(strings.TrimSpace(sourcePath), "target.")
	switch path {
	case "companyName":
		return stringValues(target.CompanyName)
	case "vatCode":
		return stringValues(target.VATCode)
	case "taxCode":
		return stringValues(target.TaxCode)
	case "province":
		return stringValues(target.Province)
	case "town":
		return stringValues(target.Town)
	case "activityStatus":
		return stringValues(target.ActivityStatus)
	case "turnover":
		return intValues(target.Turnover)
	case "turnoverYear":
		return intValues(target.TurnoverYear)
	case "employees":
		return intValues(target.Employees)
	case "atecoCode":
		return stringValues(target.AtecoCode)
	case "atecoDescription":
		return stringValues(target.AtecoDescription)
	}
	if len(target.VendorPayload) == 0 {
		return nil
	}
	object, err := decodeVendorObject(target.VendorPayload)
	if err != nil {
		return nil
	}
	return vendorValuesAtPath(object, path)
}

func stringValues(value string) []any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return []any{value}
}

func intValues(value *int) []any {
	if value == nil {
		return nil
	}
	return []any{*value}
}

func vendorValuesAtPath(value any, path string) []any {
	parts := strings.Split(strings.TrimSpace(path), ".")
	if len(parts) == 0 || parts[0] == "" {
		return nil
	}
	current := []any{value}
	for _, part := range parts {
		next := make([]any, 0)
		for _, item := range current {
			switch typed := item.(type) {
			case map[string]any:
				if nested, ok := typed[part]; ok {
					next = append(next, nested)
				}
			case []any:
				for _, nested := range typed {
					next = append(next, vendorValuesAtPath(nested, part)...)
				}
			}
		}
		current = next
		if len(current) == 0 {
			return nil
		}
	}
	return flattenDynamicValues(current)
}

func flattenDynamicValues(values []any) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		if list, ok := value.([]any); ok {
			out = append(out, flattenDynamicValues(list)...)
			continue
		}
		out = append(out, value)
	}
	return out
}

func dynamicValueMatches(value any, evaluation MAScoringEvaluation, operator string) (bool, bool) {
	switch operator {
	case "exists":
		return dynamicValueLabel(value) != "", true
	case "contains", "starts_with", "eq", "not_equals":
		actual := strings.ToLower(dynamicValueLabel(value))
		expected := strings.ToLower(dynamicValueLabel(evaluation.Value))
		if actual == "" || expected == "" {
			return false, false
		}
		switch operator {
		case "contains":
			return strings.Contains(actual, expected), true
		case "starts_with":
			return strings.HasPrefix(actual, expected), true
		case "not_equals":
			return actual != expected, true
		default:
			return actual == expected, true
		}
	case "gt", "gte", "lt", "lte", "between", "min", "max":
		actual, ok := vendorNumber(value)
		if !ok {
			return false, false
		}
		tolerance := 0.0
		if evaluation.Tolerance != nil && *evaluation.Tolerance > 0 {
			tolerance = *evaluation.Tolerance
		}
		if operator == "min" || operator == "gte" {
			if evaluation.Min != nil {
				return actual+tolerance >= *evaluation.Min, true
			}
			expected, ok := vendorNumber(evaluation.Value)
			return ok && actual+tolerance >= expected, ok
		}
		if operator == "max" || operator == "lte" {
			if evaluation.Max != nil {
				return actual-tolerance <= *evaluation.Max, true
			}
			expected, ok := vendorNumber(evaluation.Value)
			return ok && actual-tolerance <= expected, ok
		}
		if operator == "gt" || operator == "lt" {
			expected, ok := vendorNumber(evaluation.Value)
			if !ok {
				return false, false
			}
			if operator == "gt" {
				return actual > expected, true
			}
			return actual < expected, true
		}
		if evaluation.Min != nil && actual+tolerance < *evaluation.Min {
			return false, true
		}
		if evaluation.Max != nil && actual-tolerance > *evaluation.Max {
			return false, true
		}
		return evaluation.Min != nil || evaluation.Max != nil, evaluation.Min != nil || evaluation.Max != nil
	default:
		return false, false
	}
}

func dynamicValueLabel(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	default:
		return vendorString(typed)
	}
}

func buildTargetRationale(target MATarget, evidence []MATargetEvidence) string {
	parts := make([]string, 0, 3)
	if target.AtecoDescription != "" {
		parts = append(parts, target.AtecoDescription)
	}
	if target.Turnover != nil {
		parts = append(parts, "fatturato "+strconv.Itoa(*target.Turnover))
	}
	partials := make([]string, 0)
	for _, item := range evidence {
		if item.Status == maEvidencePartial || item.Status == maEvidenceMissing {
			partials = append(partials, strings.ToLower(item.Label))
		}
	}
	if len(partials) > 0 {
		parts = append(parts, "criteri mancanti o parziali: "+strings.Join(partials, ", "))
	}
	if len(parts) == 0 {
		return "Target con dati disponibili limitati."
	}
	return cleanText(strings.Join(parts, ". "), 320)
}

func textIntersects(text string, needles []string) bool {
	source := strings.ToLower(text)
	for _, needle := range needles {
		for _, token := range strings.Fields(strings.ToLower(needle)) {
			token = strings.TrimFunc(token, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
			if len([]rune(token)) < 4 {
				continue
			}
			if strings.Contains(source, token) {
				return true
			}
		}
	}
	return false
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
		"Criteri mancanti",
		"Motivazione",
	})
	for _, target := range targets {
		turnover := ""
		if target.Turnover != nil {
			turnover = strconv.Itoa(*target.Turnover)
		}
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
			strings.Join(target.MissingCriteria, ", "),
			target.Rationale,
		})
	}
	return rows
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
