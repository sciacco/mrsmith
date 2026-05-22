package training

// PlanningSuggestion is a row in the /pianificazione queue.
// id is a stable signature so dismiss persistence survives recomputation.
type PlanningSuggestion struct {
	ID                   string   `json:"id"`
	Severity             string   `json:"severity"` // critical | warning | info
	Origin               string   `json:"origin"`   // compliance | expiring | skill_gap | employee_request
	Title                string   `json:"title"`
	Description          string   `json:"description,omitempty"`
	AffectedCount        int      `json:"affected_count"`
	AffectedEmployeeIDs  []string `json:"affected_employee_ids"`
	SuggestedCourseID    string   `json:"suggested_course_id,omitempty"`
	SuggestedCourseName  string   `json:"suggested_course_name,omitempty"`
	SuggestedCourseHrs   *int     `json:"suggested_course_hours,omitempty"`
	SuggestedCourseCost  *float64 `json:"suggested_course_cost,omitempty"`
	AlternativeCourseIDs []string `json:"alternative_course_ids,omitempty"`
	EstimatedCost        float64  `json:"estimated_cost"`
	Dismissed            bool     `json:"dismissed"`
	RuleID               string   `json:"rule_id,omitempty"`
	SourceCustomGroupID  string   `json:"source_custom_group_id,omitempty"`
}

type PlanningSummary struct {
	PlanID             string  `json:"plan_id"`
	Year               int     `json:"year"`
	Status             string  `json:"status"`
	BudgetTotal        float64 `json:"budget_total"`
	BudgetSpent        float64 `json:"budget_spent"`
	BudgetResidual     float64 `json:"budget_residual"`
	BudgetPct          float64 `json:"budget_pct"`
	CalendarAlignment  string  `json:"calendar_alignment"`
	EnrollmentsPlanned int     `json:"enrollments_planned"`
	HasPrevYearPlan    bool    `json:"has_prev_year_plan"`
	Notes              string  `json:"notes,omitempty"`
}

type PlanningResponse struct {
	Year        int                  `json:"year"`
	TeamScope   string               `json:"team_scope"`
	Plan        *PlanningSummary     `json:"plan,omitempty"`
	Suggestions []PlanningSuggestion `json:"suggestions"`
}

type CreatePlanInput struct {
	Year          int      `json:"year"`
	BudgetTotal   *float64 `json:"budget_total,omitempty"`
	DuplicateFrom *int     `json:"duplicate_from,omitempty"`
}

type UpdatePlanInput struct {
	BudgetTotal *float64 `json:"budget_total,omitempty"`
	Notes       *string  `json:"notes,omitempty"`
}

type UpdatePlanResponse struct {
	OK       bool     `json:"ok"`
	PlanID   string   `json:"plan_id"`
	Warnings []string `json:"warnings,omitempty"`
}

type TrainingPlanListRow struct {
	ID          string  `json:"id"`
	Year        int     `json:"year"`
	Status      string  `json:"status"`
	BudgetTotal float64 `json:"budget_total"`
	CreatedAt   string  `json:"created_at"`
}

type TrainingPlansResponse struct {
	Plans []TrainingPlanListRow `json:"plans"`
}

type TransitionPlanInput struct {
	Target string `json:"target"` // open | closed | reopened | frozen
}

type TransitionPlanResponse struct {
	OK                      bool   `json:"ok"`
	PlanID                  string `json:"plan_id"`
	Status                  string `json:"status"`
	ExpiredEnrollmentsCount int    `json:"expired_enrollments_count,omitempty"`
}

type BulkPlanFromSuggestionInput struct {
	SuggestionID        *string              `json:"suggestion_id"`
	EmployeeIDs         []string             `json:"employee_ids"`
	CourseID            string               `json:"course_id"`
	PlanParams          BulkAssignPlanParams `json:"plan_params"`
	MandatoryRuleID     string               `json:"mandatory_rule_id,omitempty"`
	SourceCustomGroupID string               `json:"source_custom_group_id,omitempty"`
}

type BulkReviewEmployeeRequestsInput struct {
	RequestIDs []string `json:"request_ids"`
	Target     string   `json:"target"` // approved | rejected
	Motivation string   `json:"motivation,omitempty"`
	CourseID   string   `json:"course_id,omitempty"`
}

type BulkReviewEmployeeRequestsResponse struct {
	Succeeded int                 `json:"succeeded"`
	Failed    int                 `json:"failed"`
	Failures  []BulkAssignFailure `json:"failures,omitempty"`
}

type DismissSuggestionInput struct {
	PlanID string `json:"plan_id"`
}

type PlanAuditActor struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type PlanAuditEvent struct {
	ID        int64          `json:"id"`
	PlanID    string         `json:"plan_id"`
	EventType string         `json:"event_type"`
	Actor     PlanAuditActor `json:"actor"`
	Payload   map[string]any `json:"payload"`
	CreatedAt string         `json:"created_at"`
}

type PlanAuditResponse struct {
	Events     []PlanAuditEvent `json:"events"`
	NextCursor string           `json:"next_cursor,omitempty"`
}
