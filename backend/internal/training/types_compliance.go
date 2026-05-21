package training

type ComplianceExpiringRow struct {
	EmployeeID    string `json:"employee_id"`
	EmployeeName  string `json:"employee_name"`
	RuleID        string `json:"rule_id"`
	RuleTitle     string `json:"rule_title"`
	ExpiresInDays int    `json:"expires_in_days"`
	Severity      string `json:"severity"`
}

type ComplianceRuleGap struct {
	EmployeeID    string `json:"employee_id"`
	EmployeeName  string `json:"employee_name"`
	Status        string `json:"status"` // never_covered | expired | expiring_soon
	Detail        string `json:"detail,omitempty"`
}

type ComplianceRule struct {
	ID                 string              `json:"id"`
	Title              string              `json:"title"`
	CadenceLabel       string              `json:"cadence_label,omitempty"`
	PopulationTarget   string              `json:"population_target,omitempty"`
	CoveragePct        float64             `json:"coverage_pct"`
	CoveredCount       int                 `json:"covered_count"`
	TargetCount        int                 `json:"target_count"`
	Gaps               []ComplianceRuleGap `json:"gaps"`
	Severity           string              `json:"severity"` // critical | warning | ok
	SuggestedCourseIDs []string            `json:"suggested_course_ids"`
}

type ComplianceOverviewResponse struct {
	Year               int                    `json:"year"`
	TeamScope          string                 `json:"team_scope"`
	DeadlineDays       int                    `json:"deadline_days"`
	ExpiringDeadlines  []ComplianceExpiringRow `json:"expiring_deadlines"`
	Rules              []ComplianceRule       `json:"rules"`
}
