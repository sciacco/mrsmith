package training

import (
	"strconv"
	"strings"
)

type TrainingImportNormalizationReport struct {
	MatchedRows          int                            `json:"matchedRows"`
	AlreadyEmail         int                            `json:"alreadyEmailRows"`
	SurnameMatchedRows   int                            `json:"surnameMatchedRows"`
	SplitSourceRows      int                            `json:"splitSourceRows"`
	ExpandedRows         int                            `json:"expandedRows"`
	SkippedNonPersonRows int                            `json:"skippedNonPersonRows"`
	SkippedCourseRows    int                            `json:"skippedPlaceholderCourseRows"`
	DeduplicatedRows     int                            `json:"deduplicatedRows"`
	UnmatchedRows        int                            `json:"unmatchedRows"`
	AmbiguousRows        int                            `json:"ambiguousRows"`
	Mappings             []TrainingImportNameMapping    `json:"mappings"`
	NonPersonNames       []TrainingImportNameDiagnostic `json:"nonPersonNames"`
	PlaceholderCourses   []TrainingImportNameDiagnostic `json:"placeholderCourses"`
	UnmatchedNames       []TrainingImportNameDiagnostic `json:"unmatchedNames"`
	AmbiguousNames       []TrainingImportNameDiagnostic `json:"ambiguousNames"`
}

type TrainingImportNameMapping struct {
	Sheet        string `json:"sheet"`
	Row          int    `json:"row"`
	EmployeeName string `json:"employeeName"`
	CourseTitle  string `json:"courseTitle,omitempty"`
	Email        string `json:"email"`
	Strategy     string `json:"strategy"`
}

type TrainingImportNameDiagnostic struct {
	Sheet        string   `json:"sheet"`
	Row          int      `json:"row"`
	EmployeeName string   `json:"employeeName"`
	CourseTitle  string   `json:"courseTitle,omitempty"`
	Candidates   []string `json:"candidates,omitempty"`
}

func ResolveTrainingImportEmployees(response *ImportDryRunResponse, employees []EmployeeImportRow) TrainingImportNormalizationReport {
	matcher := newEmployeeNameMatcher(employees)
	report := TrainingImportNormalizationReport{
		PlaceholderCourses: []TrainingImportNameDiagnostic{},
		NonPersonNames:     []TrainingImportNameDiagnostic{},
		Mappings:           []TrainingImportNameMapping{},
		UnmatchedNames:     []TrainingImportNameDiagnostic{},
		AmbiguousNames:     []TrainingImportNameDiagnostic{},
	}
	handledRows := map[string]struct{}{}

	splitTrainingImportRows(response, &report, handledRows)

	for index := range response.Rows {
		row := &response.Rows[index]
		if row.Status != "candidate" {
			continue
		}
		if isPlaceholderImportCourseTitle(row.CourseTitle) {
			row.Status = "skipped"
			report.SkippedCourseRows++
			report.PlaceholderCourses = append(report.PlaceholderCourses, TrainingImportNameDiagnostic{
				Sheet:        row.Sheet,
				Row:          row.Row,
				EmployeeName: row.EmployeeName,
				CourseTitle:  row.CourseTitle,
			})
			response.Warnings = append(response.Warnings, ImportWarning{
				Sheet:   row.Sheet,
				Row:     row.Row,
				Code:    "placeholder_course",
				Message: "corso riconosciuto come placeholder o nota operativa",
			})
			continue
		}
		if strings.TrimSpace(row.EmployeeEmail) != "" {
			report.AlreadyEmail++
			continue
		}
		match := matcher.match(row.EmployeeName)
		switch match.status {
		case "matched":
			row.EmployeeEmail = match.employee.Email
			report.MatchedRows++
			if match.strategy == "surname" {
				report.SurnameMatchedRows++
			}
			report.Mappings = append(report.Mappings, TrainingImportNameMapping{
				Sheet:        row.Sheet,
				Row:          row.Row,
				EmployeeName: row.EmployeeName,
				CourseTitle:  row.CourseTitle,
				Email:        row.EmployeeEmail,
				Strategy:     match.strategy,
			})
			handledRows[importWarningKey(row.Sheet, row.Row)] = struct{}{}
		case "ambiguous":
			report.AmbiguousRows++
			report.AmbiguousNames = append(report.AmbiguousNames, TrainingImportNameDiagnostic{
				Sheet:        row.Sheet,
				Row:          row.Row,
				EmployeeName: row.EmployeeName,
				CourseTitle:  row.CourseTitle,
				Candidates:   match.candidateEmails(),
			})
		default:
			if isNonPersonImportValue(row.EmployeeName) {
				row.Status = "skipped"
				report.SkippedNonPersonRows++
				report.NonPersonNames = append(report.NonPersonNames, TrainingImportNameDiagnostic{
					Sheet:        row.Sheet,
					Row:          row.Row,
					EmployeeName: row.EmployeeName,
					CourseTitle:  row.CourseTitle,
				})
				handledRows[importWarningKey(row.Sheet, row.Row)] = struct{}{}
				response.Warnings = append(response.Warnings, ImportWarning{
					Sheet:   row.Sheet,
					Row:     row.Row,
					Code:    "non_person_employee_cell",
					Message: "cella dipendente riconosciuta come intestazione, nota o placeholder",
				})
				continue
			}
			report.UnmatchedRows++
			report.UnmatchedNames = append(report.UnmatchedNames, TrainingImportNameDiagnostic{
				Sheet:        row.Sheet,
				Row:          row.Row,
				EmployeeName: row.EmployeeName,
				CourseTitle:  row.CourseTitle,
			})
		}
	}

	if deduplicated := deduplicateResolvedTrainingImportRows(response); deduplicated > 0 {
		report.DeduplicatedRows = deduplicated
	}

	filterEmployeeMatchRequiredWarnings(response)

	RecomputeTrainingImportSummary(response)
	return report
}

type employeeNameMatcher struct {
	byName    map[string][]EmployeeImportRow
	bySurname map[string][]EmployeeImportRow
}

type employeeNameMatch struct {
	status    string
	strategy  string
	employee  EmployeeImportRow
	employees []EmployeeImportRow
}

func newEmployeeNameMatcher(employees []EmployeeImportRow) employeeNameMatcher {
	matcher := employeeNameMatcher{
		byName:    map[string][]EmployeeImportRow{},
		bySurname: map[string][]EmployeeImportRow{},
	}
	seen := map[string]map[string]struct{}{}
	seenSurname := map[string]map[string]struct{}{}
	for _, employee := range employees {
		if employee.Status != "candidate" || employee.Email == "" {
			continue
		}
		for _, key := range employeeNameKeys(employee.FirstName, employee.LastName) {
			if key == "" {
				continue
			}
			if seen[key] == nil {
				seen[key] = map[string]struct{}{}
			}
			if _, ok := seen[key][employee.Email]; ok {
				continue
			}
			seen[key][employee.Email] = struct{}{}
			matcher.byName[key] = append(matcher.byName[key], employee)
		}
		surname := normalizeImportPersonName(employee.LastName)
		if surname == "" {
			continue
		}
		if seenSurname[surname] == nil {
			seenSurname[surname] = map[string]struct{}{}
		}
		if _, ok := seenSurname[surname][employee.Email]; ok {
			continue
		}
		seenSurname[surname][employee.Email] = struct{}{}
		matcher.bySurname[surname] = append(matcher.bySurname[surname], employee)
	}
	return matcher
}

func (m employeeNameMatcher) match(value string) employeeNameMatch {
	key := normalizeImportPersonName(value)
	if key == "" {
		return employeeNameMatch{status: "unmatched"}
	}
	matches := m.byName[key]
	switch len(matches) {
	case 0:
		if len(strings.Fields(key)) == 1 {
			surnameMatches := m.bySurname[key]
			switch len(surnameMatches) {
			case 1:
				return employeeNameMatch{status: "matched", strategy: "surname", employee: surnameMatches[0]}
			case 0:
				return employeeNameMatch{status: "unmatched"}
			default:
				return employeeNameMatch{status: "ambiguous", employees: surnameMatches}
			}
		}
		return employeeNameMatch{status: "unmatched"}
	case 1:
		return employeeNameMatch{status: "matched", strategy: "exact", employee: matches[0]}
	default:
		return employeeNameMatch{status: "ambiguous", employees: matches}
	}
}

func splitTrainingImportRows(response *ImportDryRunResponse, report *TrainingImportNormalizationReport, handledRows map[string]struct{}) {
	nextRows := make([]ImportRow, 0, len(response.Rows))
	for _, row := range response.Rows {
		if row.Status != "candidate" || strings.TrimSpace(row.EmployeeEmail) != "" {
			nextRows = append(nextRows, row)
			continue
		}
		names := splitImportEmployeeNames(row.EmployeeName)
		if len(names) <= 1 {
			nextRows = append(nextRows, row)
			continue
		}
		report.SplitSourceRows++
		handledRows[importWarningKey(row.Sheet, row.Row)] = struct{}{}
		for _, name := range names {
			splitRow := row
			splitRow.EmployeeName = name
			if isNonPersonImportValue(name) {
				splitRow.Status = "skipped"
				report.SkippedNonPersonRows++
				report.NonPersonNames = append(report.NonPersonNames, TrainingImportNameDiagnostic{
					Sheet:        splitRow.Sheet,
					Row:          splitRow.Row,
					EmployeeName: splitRow.EmployeeName,
					CourseTitle:  splitRow.CourseTitle,
				})
				response.Warnings = append(response.Warnings, ImportWarning{
					Sheet:   splitRow.Sheet,
					Row:     splitRow.Row,
					Code:    "non_person_employee_cell",
					Message: "cella dipendente riconosciuta come intestazione, nota o placeholder",
				})
			}
			nextRows = append(nextRows, splitRow)
		}
		report.ExpandedRows += len(names) - 1
		response.Warnings = append(response.Warnings, ImportWarning{
			Sheet:   row.Sheet,
			Row:     row.Row,
			Code:    "employee_cell_split",
			Message: "cella dipendente multi-persona esplosa in righe distinte",
		})
	}
	response.Rows = nextRows
}

func deduplicateResolvedTrainingImportRows(response *ImportDryRunResponse) int {
	seen := map[string]int{}
	rows := make([]ImportRow, 0, len(response.Rows))
	deduplicated := 0
	for _, row := range response.Rows {
		if row.Status != "candidate" {
			rows = append(rows, row)
			continue
		}
		key := importDedupKey(row)
		if existingIndex, ok := seen[key]; ok {
			deduplicated++
			response.Warnings = append(response.Warnings, ImportWarning{
				Sheet:   row.Sheet,
				Row:     row.Row,
				Code:    "duplicate_candidate_normalized",
				Message: "riga duplicata dopo normalizzazione dipendente/corso/anno",
			})
			if isBudgetImportSheet(row.Sheet) && !isBudgetImportSheet(rows[existingIndex].Sheet) {
				rows[existingIndex] = row
			}
			continue
		}
		seen[key] = len(rows)
		rows = append(rows, row)
	}
	response.Rows = rows
	return deduplicated
}

func splitImportEmployeeNames(value string) []string {
	cleaned := strings.ReplaceAll(value, "\r\n", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\r", "\n")
	lines := strings.Split(cleaned, "\n")
	result := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, part := range splitImportEmployeeNameLine(line) {
			if part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

func splitImportEmployeeNameLine(value string) []string {
	if !strings.Contains(value, " e ") {
		return []string{strings.TrimSpace(value)}
	}
	parts := strings.Split(value, " e ")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		result = append(result, strings.TrimSpace(part))
	}
	return result
}

func (m employeeNameMatch) candidateEmails() []string {
	result := make([]string, 0, len(m.employees))
	for _, employee := range m.employees {
		result = append(result, employee.Email)
	}
	return result
}

func employeeNameKeys(firstName, lastName string) []string {
	first := normalizeImportPersonName(firstName)
	last := normalizeImportPersonName(lastName)
	return []string{
		strings.TrimSpace(first + " " + last),
		strings.TrimSpace(last + " " + first),
	}
}

func normalizeImportPersonName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		"\t", " ",
		"\n", " ",
		"\r", " ",
		",", " ",
		".", " ",
		";", " ",
		":", " ",
		"(", " ",
		")", " ",
	)
	value = replacer.Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func isNonPersonImportValue(value string) bool {
	normalized := normalizeImportPersonName(value)
	if normalized == "" {
		return true
	}
	if normalized == "?" || normalized == "/" || normalized == "all" {
		return true
	}
	if strings.HasPrefix(normalized, "definire ") {
		return true
	}
	if strings.Contains(normalized, " partecipanti") {
		return true
	}
	for _, token := range strings.Fields(normalized) {
		if token == "nse" {
			return true
		}
	}
	if importValueHasDigit(normalized) {
		return true
	}
	if len(strings.Fields(normalized)) <= 2 && importValueIsUppercaseLabel(value) {
		return true
	}
	return false
}

func isPlaceholderImportCourseTitle(value string) bool {
	normalized := normalizeImportPersonName(value)
	switch normalized {
	case "", "/", "da individuare", "da definire", "esame":
		return true
	}
	return strings.Contains(normalized, "scegliere quale corso")
}

func filterEmployeeMatchRequiredWarnings(response *ImportDryRunResponse) {
	unresolvedRows := map[string]struct{}{}
	for _, row := range response.Rows {
		if row.Status == "candidate" && strings.TrimSpace(row.EmployeeEmail) == "" {
			unresolvedRows[importWarningKey(row.Sheet, row.Row)] = struct{}{}
		}
	}
	filtered := response.Warnings[:0]
	for _, warning := range response.Warnings {
		if warning.Code == "employee_match_required" {
			if _, ok := unresolvedRows[importWarningKey(warning.Sheet, warning.Row)]; !ok {
				continue
			}
		}
		filtered = append(filtered, warning)
	}
	response.Warnings = filtered
}

func importValueHasDigit(value string) bool {
	for _, r := range value {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func importValueIsUppercaseLabel(value string) bool {
	hasLetter := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			hasLetter = true
		}
	}
	return hasLetter
}

func importWarningKey(sheet string, row int) string {
	return sheet + "\x00" + strconv.Itoa(row)
}
