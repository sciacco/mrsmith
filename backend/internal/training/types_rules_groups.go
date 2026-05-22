package training

type PopulationTarget struct {
	Kind  string `json:"kind"`
	ID    string `json:"id,omitempty"`
	Label string `json:"label,omitempty"`
	Count int    `json:"count,omitempty"`
}

type RuleUsage struct {
	Kind  string `json:"kind"`
	ID    string `json:"id,omitempty"`
	Label string `json:"label"`
	Count int    `json:"count,omitempty"`
}

type MandatoryRule struct {
	ID                  string              `json:"id"`
	Name                string              `json:"name"`
	CourseID            string              `json:"course_id"`
	CourseTitle         string              `json:"course_title"`
	ComplianceFramework string              `json:"compliance_framework,omitempty"`
	CadenceLabel        string              `json:"cadence_label,omitempty"`
	PopulationTarget    PopulationTarget    `json:"population_target"`
	Active              bool                `json:"active"`
	Notes               string              `json:"notes,omitempty"`
	CoveragePct         float64             `json:"coverage_pct"`
	CoveredCount        int                 `json:"covered_count"`
	TargetCount         int                 `json:"target_count"`
	GapCount            int                 `json:"gap_count"`
	Gaps                []ComplianceRuleGap `json:"gaps,omitempty"`
	Severity            string              `json:"severity"`
	UsedBy              []RuleUsage         `json:"used_by,omitempty"`
	CreatedAt           string              `json:"created_at,omitempty"`
	UpdatedAt           string              `json:"updated_at,omitempty"`
}

type MandatoryRuleInputV2 struct {
	Name             string           `json:"name"`
	CourseID         string           `json:"course_id"`
	PopulationTarget PopulationTarget `json:"population_target"`
	Active           *bool            `json:"active,omitempty"`
	Notes            string           `json:"notes,omitempty"`
}

type MandatoryRulesResponse struct {
	Rules []MandatoryRule `json:"rules"`
}

type MandatoryRuleMutationResponse struct {
	Rule     MandatoryRule `json:"rule"`
	Warnings []string      `json:"warnings,omitempty"`
	Impact   RuleImpact    `json:"impact"`
}

type RuleImpact struct {
	TargetCount  int                 `json:"target_count"`
	CoveredCount int                 `json:"covered_count"`
	GapCount     int                 `json:"gap_count"`
	CoveragePct  float64             `json:"coverage_pct"`
	Severity     string              `json:"severity"`
	Gaps         []ComplianceRuleGap `json:"gaps,omitempty"`
}

type GroupMember struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	TeamCode string `json:"team_code,omitempty"`
}

type CustomGroupUsage struct {
	Kind  string `json:"kind"`
	ID    string `json:"id,omitempty"`
	Label string `json:"label"`
	Count int    `json:"count,omitempty"`
}

type CustomGroup struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Active      bool               `json:"active"`
	MemberCount int                `json:"member_count"`
	Members     []GroupMember      `json:"members,omitempty"`
	UsedBy      []CustomGroupUsage `json:"used_by,omitempty"`
	CreatedAt   string             `json:"created_at,omitempty"`
	UpdatedAt   string             `json:"updated_at,omitempty"`
}

type CustomGroupInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Active      *bool    `json:"active,omitempty"`
	MemberIDs   []string `json:"member_ids"`
}

type CustomGroupsResponse struct {
	Groups []CustomGroup `json:"groups"`
}
