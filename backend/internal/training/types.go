package training

type Principal struct {
	Subject       string   `json:"subject"`
	Email         string   `json:"email"`
	Name          string   `json:"name"`
	Roles         []string `json:"roles"`
	IsPeopleAdmin bool     `json:"isPeopleAdmin"`
}

type Employee struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Status    string `json:"status"`
}

type MeResponse struct {
	Principal         Principal `json:"principal"`
	Employee          *Employee `json:"employee"`
	OnboardingPending bool      `json:"onboardingPending"`
}

type PlanEnrollment struct {
	ID                string   `json:"id"`
	EmployeeName      string   `json:"employeeName"`
	EmployeeEmail     string   `json:"employeeEmail"`
	TeamCode          string   `json:"teamCode,omitempty"`
	CourseTitle       string   `json:"courseTitle"`
	VendorName        string   `json:"vendorName,omitempty"`
	SkillAreaName     string   `json:"skillAreaName,omitempty"`
	Status            string   `json:"status"`
	Year              int      `json:"year"`
	Priority          *int     `json:"priority,omitempty"`
	LevelAsIs         *int     `json:"levelAsIs,omitempty"`
	LevelToBe         *int     `json:"levelToBe,omitempty"`
	PlannedStart      string   `json:"plannedStart,omitempty"`
	PlannedEnd        string   `json:"plannedEnd,omitempty"`
	HoursPlanned      *int     `json:"hoursPlanned,omitempty"`
	CostPlanned       *float64 `json:"costPlanned,omitempty"`
	Motivation        string   `json:"motivation,omitempty"`
	Objective         string   `json:"objective,omitempty"`
	Notes             string   `json:"notes,omitempty"`
	DocumentID        string   `json:"documentId,omitempty"`
	DocumentFilename  string   `json:"documentFilename,omitempty"`
	DocumentValidated bool     `json:"documentValidated"`
	Mandatory         bool     `json:"mandatory"`
}

type TrainingRequest struct {
	ID            string `json:"id"`
	EmployeeName  string `json:"employeeName"`
	EmployeeEmail string `json:"employeeEmail"`
	CourseID      string `json:"courseId,omitempty"`
	CourseTitle   string `json:"courseTitle,omitempty"`
	FreeTextTitle string `json:"freeTextTitle,omitempty"`
	SkillAreaName string `json:"skillAreaName,omitempty"`
	Motivation    string `json:"motivation"`
	DesiredYear   *int   `json:"desiredYear,omitempty"`
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt"`
}

type CatalogCourse struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	VendorID            string   `json:"vendorId,omitempty"`
	VendorName          string   `json:"vendorName,omitempty"`
	SkillAreaID         string   `json:"skillAreaId,omitempty"`
	SkillAreaName       string   `json:"skillAreaName,omitempty"`
	LeadsToCertID       string   `json:"leadsToCertId,omitempty"`
	CertificationName   string   `json:"certificationName,omitempty"`
	DeliveryMode        string   `json:"deliveryMode"`
	ProviderKind        string   `json:"providerKind"`
	DefaultHours        *int     `json:"defaultHours,omitempty"`
	DefaultCost         *float64 `json:"defaultCost,omitempty"`
	CourseURL           string   `json:"courseUrl,omitempty"`
	Description         string   `json:"description,omitempty"`
	Mandatory           bool     `json:"mandatory"`
	RecurrenceMonths    *int     `json:"recurrenceMonths,omitempty"`
	ComplianceFramework string   `json:"complianceFramework,omitempty"`
	Active              bool     `json:"active"`
}

type CertificationRow struct {
	AwardID           string `json:"awardId"`
	EmployeeName      string `json:"employeeName"`
	EmployeeEmail     string `json:"employeeEmail,omitempty"`
	CertificationCode string `json:"certificationCode"`
	CertificationName string `json:"certificationName"`
	Outcome           string `json:"outcome"`
	AwardedOn         string `json:"awardedOn"`
	ExpiresOn         string `json:"expiresOn,omitempty"`
	CurrentStatus     string `json:"currentStatus"`
	ValidationSource  string `json:"validationSource"`
	DocumentID        string `json:"documentId,omitempty"`
	DocumentFilename  string `json:"documentFilename,omitempty"`
	DocumentValidated bool   `json:"documentValidated"`
}

type PlanBudgetRow struct {
	Year             int      `json:"year"`
	TeamCode         string   `json:"teamCode,omitempty"`
	EnrollmentsCount int      `json:"enrollmentsCount"`
	CostTotal        *float64 `json:"costTotal,omitempty"`
	HoursTotal       *float64 `json:"hoursTotal,omitempty"`
}

type ExpiringCertificationRow struct {
	EmployeeName      string `json:"employeeName"`
	EmployeeEmail     string `json:"employeeEmail"`
	CertificationCode string `json:"certificationCode"`
	CertificationName string `json:"certificationName"`
	ExpiresOn         string `json:"expiresOn"`
	DaysToExpiry      int    `json:"daysToExpiry"`
}

type ComplianceGapRow struct {
	EmployeeName        string `json:"employeeName"`
	CourseTitle         string `json:"courseTitle"`
	ComplianceFramework string `json:"complianceFramework,omitempty"`
	LastValidAwardedOn  string `json:"lastValidAwardedOn,omitempty"`
	ComplianceStatus    string `json:"complianceStatus"`
}

type CatalogMasterData struct {
	Vendors        []VendorRow        `json:"vendors"`
	Teams          []TeamRow          `json:"teams"`
	SkillAreas     []SkillAreaRow     `json:"skillAreas"`
	Certifications []CatalogCertRow   `json:"certifications"`
	Plans          []TrainingPlanRow  `json:"plans"`
	MandatoryRules []MandatoryRuleRow `json:"mandatoryRules"`
}

type VendorRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Website string `json:"website,omitempty"`
	Notes   string `json:"notes,omitempty"`
	Active  bool   `json:"active"`
}

type TeamRow struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Active      bool   `json:"active"`
}

type SkillAreaRow struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	ParentID    string `json:"parentId,omitempty"`
	ParentLabel string `json:"parentLabel,omitempty"`
	Description string `json:"description,omitempty"`
	Active      bool   `json:"active"`
}

type CatalogCertRow struct {
	ID                    string `json:"id"`
	Code                  string `json:"code"`
	Name                  string `json:"name"`
	IssuerVendorID        string `json:"issuerVendorId,omitempty"`
	IssuerVendorName      string `json:"issuerVendorName,omitempty"`
	SkillAreaID           string `json:"skillAreaId,omitempty"`
	SkillAreaLabel        string `json:"skillAreaLabel,omitempty"`
	TypicalValidityMonths *int   `json:"typicalValidityMonths,omitempty"`
	Description           string `json:"description,omitempty"`
	Active                bool   `json:"active"`
}

type TrainingPlanRow struct {
	ID          string   `json:"id"`
	Year        int      `json:"year"`
	Status      string   `json:"status"`
	BudgetTotal *float64 `json:"budgetTotal,omitempty"`
	Notes       string   `json:"notes,omitempty"`
}

type MandatoryRuleRow struct {
	ID          string `json:"id"`
	CourseID    string `json:"courseId"`
	CourseTitle string `json:"courseTitle"`
	TeamID      string `json:"teamId,omitempty"`
	TeamLabel   string `json:"teamLabel,omitempty"`
	RoleFilter  string `json:"roleFilter,omitempty"`
	Notes       string `json:"notes,omitempty"`
	Active      bool   `json:"active"`
}

type LookupItem struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Active bool   `json:"active"`
}

type LookupResponse struct {
	Employees      []LookupItem `json:"employees"`
	Teams          []LookupItem `json:"teams"`
	Vendors        []LookupItem `json:"vendors"`
	SkillAreas     []LookupItem `json:"skillAreas"`
	Courses        []LookupItem `json:"courses"`
	Certifications []LookupItem `json:"certifications"`
	Plans          []LookupItem `json:"plans"`
}

type WorkspaceResponse struct {
	Me                      MeResponse                 `json:"me"`
	Plan                    []PlanEnrollment           `json:"plan"`
	Requests                []TrainingRequest          `json:"requests"`
	Catalog                 []CatalogCourse            `json:"catalog"`
	Certifications          []CertificationRow         `json:"certifications"`
	PlanBudget              []PlanBudgetRow            `json:"planBudget"`
	ExpiringCertifications  []ExpiringCertificationRow `json:"expiringCertifications"`
	MandatoryComplianceGaps []ComplianceGapRow         `json:"mandatoryComplianceGaps"`
	MasterData              *CatalogMasterData         `json:"masterData,omitempty"`
}

type OKResponse struct {
	OK bool `json:"ok"`
}

type ActionResponse struct {
	OK     bool   `json:"ok"`
	ID     string `json:"id,omitempty"`
	Status string `json:"status,omitempty"`
}

type EnrollmentInput struct {
	EmployeeID     string   `json:"employeeId"`
	CourseID       string   `json:"courseId"`
	TrainingPlanID string   `json:"trainingPlanId"`
	Priority       *int     `json:"priority,omitempty"`
	LevelAsIs      *int     `json:"levelAsIs,omitempty"`
	LevelToBe      *int     `json:"levelToBe,omitempty"`
	PlannedStart   string   `json:"plannedStart,omitempty"`
	PlannedEnd     string   `json:"plannedEnd,omitempty"`
	HoursPlanned   *int     `json:"hoursPlanned,omitempty"`
	CostPlanned    *float64 `json:"costPlanned,omitempty"`
	Motivation     string   `json:"motivation,omitempty"`
	Objective      string   `json:"objective,omitempty"`
	Notes          string   `json:"notes,omitempty"`
}

type EnrollmentTransitionInput struct {
	Transition  string `json:"transition"`
	Reason      string `json:"reason,omitempty"`
	ActualStart string `json:"actualStart,omitempty"`
	ActualEnd   string `json:"actualEnd,omitempty"`
}

type PersonNextDeadline struct {
	Type  string `json:"type"`
	Date  string `json:"date"`
	Label string `json:"label"`
}

type PersonSummary struct {
	ID                     string              `json:"id"`
	Name                   string              `json:"name"`
	Email                  string              `json:"email"`
	TeamCode               string              `json:"team_code"`
	ComplianceStatus       string              `json:"compliance_status"`
	ActiveEnrollmentsCount int                 `json:"active_enrollments_count"`
	NextDeadline           *PersonNextDeadline `json:"next_deadline,omitempty"`
	PriorityScore          float64             `json:"priority_score"`
	GapsOpen               int                 `json:"gaps_open"`
	ExpiringCertsCount     int                 `json:"expiring_certs_count"`
	HistoricalEnrollments  int                 `json:"historical_enrollments"`
}

type PeopleDirectoryFilters struct {
	Year   int
	Team   string
	Filter string
	Search string
}

type BulkAssignPlanParams struct {
	Year         int      `json:"year"`
	PlannedStart string   `json:"planned_start,omitempty"`
	PlannedEnd   string   `json:"planned_end,omitempty"`
	HoursPlanned *int     `json:"hours_planned,omitempty"`
	CostPlanned  *float64 `json:"cost_planned,omitempty"`
	Mandatory    bool     `json:"mandatory"`
}

type BulkAssignInput struct {
	EmployeeIDs []string             `json:"employee_ids"`
	CourseID    string               `json:"course_id"`
	PlanParams  BulkAssignPlanParams `json:"plan_params"`
}

type BulkAssignFailure struct {
	EmployeeID string `json:"employee_id"`
	Code       string `json:"code,omitempty"`
	Message    string `json:"message,omitempty"`
}

type BulkAssignResponse struct {
	Created  int                 `json:"created"`
	Failed   int                 `json:"failed"`
	Failures []BulkAssignFailure `json:"failures,omitempty"`
}

type PersonIdentityMin struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	TeamCode string `json:"team_code"`
}

type PersonComplianceMandatoryRule struct {
	CourseID            string `json:"course_id"`
	CourseTitle         string `json:"course_title"`
	ComplianceFramework string `json:"compliance_framework,omitempty"`
	Status              string `json:"status"`
	LastValidAwardedOn  string `json:"last_valid_awarded_on,omitempty"`
}

type PersonComplianceSection struct {
	MandatoryRules []PersonComplianceMandatoryRule `json:"mandatory_rules"`
	CoveragePct    float64                         `json:"coverage_pct"`
	OpenGaps       []PersonComplianceMandatoryRule `json:"open_gaps"`
	ExpiringCerts  []ExpiringCertificationRow      `json:"expiring_certs"`
}

type PersonHistoryYearRow struct {
	Year           int     `json:"year"`
	CompletedCount int     `json:"completed_count"`
	FailedCount    int     `json:"failed_count"`
	HoursTotal     float64 `json:"hours_total"`
	CostTotal      float64 `json:"cost_total"`
}

type PersonSkillEvidence struct {
	CoursesCompleted []string `json:"courses_completed"`
	Certs            []string `json:"certs"`
}

type PersonSkillArea struct {
	SkillAreaID  string              `json:"skill_area_id"`
	Name         string              `json:"name"`
	DerivedLevel string              `json:"derived_level"`
	Evidence     PersonSkillEvidence `json:"evidence"`
}

type PersonGap struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type PersonSuggestion struct {
	Gap                PersonGap       `json:"gap"`
	RecommendedCourses []CatalogCourse `json:"recommended_courses"`
}

type OverviewException struct {
	ID           string `json:"id"`
	Severity     string `json:"severity"`
	Title        string `json:"title"`
	DrilldownURL string `json:"drilldown_url"`
}

type OverviewTrend struct {
	VsPreviousYear string  `json:"vs_previous_year,omitempty"`
	VsTarget       *string `json:"vs_target,omitempty"`
}

type OverviewFamily struct {
	Value             string              `json:"value"`
	Trend             OverviewTrend       `json:"trend"`
	Exceptions        []OverviewException `json:"exceptions"`
	SpentPct          *float64            `json:"spent_pct,omitempty"`
	CalendarAlignment string              `json:"calendar_alignment,omitempty"`
	CoursesPerPerson  *float64            `json:"courses_per_person,omitempty"`
}

type OverviewResponse struct {
	Year       int            `json:"year"`
	TeamScope  string         `json:"team_scope"`
	Esecuzione OverviewFamily `json:"esecuzione"`
	Compliance OverviewFamily `json:"compliance"`
	Budget     OverviewFamily `json:"budget"`
	Engagement OverviewFamily `json:"engagement"`
}

type PersonProfile struct {
	IdentityMin             PersonIdentityMin       `json:"identity_min"`
	Compliance              PersonComplianceSection `json:"compliance"`
	EnrollmentsCurrentYear  []PlanEnrollment        `json:"enrollments_current_year"`
	Certifications          []CertificationRow      `json:"certifications"`
	HistoryByYear           []PersonHistoryYearRow  `json:"history_by_year"`
	SkillAreas              []PersonSkillArea       `json:"skill_areas"`
	Suggestions             []PersonSuggestion      `json:"suggestions"`
}

type BulkEnrollmentTransitionInput struct {
	EnrollmentIDs []string `json:"enrollment_ids"`
	TargetState   string   `json:"target_state"`
	Motivation    string   `json:"motivation,omitempty"`
}

type BulkEnrollmentTransitionFailure struct {
	EnrollmentID string `json:"enrollment_id"`
	Code         string `json:"code,omitempty"`
	Message      string `json:"message,omitempty"`
}

type BulkEnrollmentTransitionResponse struct {
	Succeeded int                               `json:"succeeded"`
	Failed    int                               `json:"failed"`
	Failures  []BulkEnrollmentTransitionFailure `json:"failures,omitempty"`
}

type TrainingRequestInput struct {
	CourseID      string `json:"courseId,omitempty"`
	FreeTextTitle string `json:"freeTextTitle,omitempty"`
	SkillAreaID   string `json:"skillAreaId,omitempty"`
	Motivation    string `json:"motivation"`
	DesiredYear   *int   `json:"desiredYear,omitempty"`
}

type TrainingRequestTransitionInput struct {
	Transition     string `json:"transition"`
	Reason         string `json:"reason,omitempty"`
	TrainingPlanID string `json:"trainingPlanId,omitempty"`
}

type VendorInput struct {
	Name    string `json:"name"`
	Website string `json:"website,omitempty"`
	Notes   string `json:"notes,omitempty"`
	Active  *bool  `json:"active,omitempty"`
}

type TeamInput struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Active      *bool  `json:"active,omitempty"`
}

type SkillAreaInput struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	ParentID    string `json:"parentId,omitempty"`
	Description string `json:"description,omitempty"`
	Active      *bool  `json:"active,omitempty"`
}

type CertificationInput struct {
	Code                  string `json:"code"`
	Name                  string `json:"name"`
	IssuerVendorID        string `json:"issuerVendorId,omitempty"`
	SkillAreaID           string `json:"skillAreaId,omitempty"`
	TypicalValidityMonths *int   `json:"typicalValidityMonths,omitempty"`
	Description           string `json:"description,omitempty"`
	Active                *bool  `json:"active,omitempty"`
}

type CourseInput struct {
	Title               string   `json:"title"`
	VendorID            string   `json:"vendorId,omitempty"`
	SkillAreaID         string   `json:"skillAreaId,omitempty"`
	LeadsToCertID       string   `json:"leadsToCertId,omitempty"`
	DeliveryMode        string   `json:"deliveryMode,omitempty"`
	ProviderKind        string   `json:"providerKind,omitempty"`
	DefaultHours        *int     `json:"defaultHours,omitempty"`
	DefaultCost         *float64 `json:"defaultCost,omitempty"`
	CourseURL           string   `json:"courseUrl,omitempty"`
	Description         string   `json:"description,omitempty"`
	Mandatory           bool     `json:"mandatory"`
	RecurrenceMonths    *int     `json:"recurrenceMonths,omitempty"`
	ComplianceFramework string   `json:"complianceFramework,omitempty"`
	Active              *bool    `json:"active,omitempty"`
}

type TrainingPlanInput struct {
	Year        int      `json:"year"`
	Status      string   `json:"status,omitempty"`
	BudgetTotal *float64 `json:"budgetTotal,omitempty"`
	Notes       string   `json:"notes,omitempty"`
}

type MandatoryRuleInput struct {
	CourseID   string `json:"courseId"`
	TeamID     string `json:"teamId,omitempty"`
	RoleFilter string `json:"roleFilter,omitempty"`
	Active     *bool  `json:"active,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

type AwardInput struct {
	EmployeeID            string `json:"employeeId"`
	CertificationID       string `json:"certificationId"`
	EnrollmentID          string `json:"enrollmentId,omitempty"`
	Outcome               string `json:"outcome"`
	AwardedOn             string `json:"awardedOn"`
	ExpiresOn             string `json:"expiresOn,omitempty"`
	ValidationSource      string `json:"validationSource,omitempty"`
	ExternalCredentialID  string `json:"externalCredentialId,omitempty"`
	ExternalCredentialURL string `json:"externalCredentialUrl,omitempty"`
	Notes                 string `json:"notes,omitempty"`
	Reason                string `json:"reason,omitempty"`
}

type AwardUpdateInput struct {
	Outcome          string  `json:"outcome"`
	AwardedOn        string  `json:"awardedOn"`
	ExpiresOn        string  `json:"expiresOn,omitempty"`
	ValidationSource string  `json:"validationSource,omitempty"`
	Notes            *string `json:"notes,omitempty"`
}

type DocumentMetadata struct {
	ID                   string `json:"id"`
	EnrollmentID         string `json:"enrollmentId,omitempty"`
	CertificationAwardID string `json:"certificationAwardId,omitempty"`
	Filename             string `json:"filename"`
	SHA256               string `json:"sha256"`
	MIME                 string `json:"mime"`
	SizeBytes            int64  `json:"sizeBytes"`
	UploadedAt           string `json:"uploadedAt"`
	Validated            bool   `json:"validated"`
}

type ImportDryRunResponse struct {
	OK       bool            `json:"ok"`
	DryRun   bool            `json:"dryRun"`
	FileName string          `json:"fileName"`
	Sheets   []ImportSheet   `json:"sheets"`
	Summary  ImportSummary   `json:"summary"`
	Warnings []ImportWarning `json:"warnings"`
	Rows     []ImportRow     `json:"rows"`
}

type ImportSheet struct {
	Name string `json:"name"`
	Rows int    `json:"rows"`
}

type ImportSummary struct {
	ParsedRows           int `json:"parsedRows"`
	CandidateRows        int `json:"candidateRows"`
	SkippedRows          int `json:"skippedRows"`
	AmbiguousRows        int `json:"ambiguousRows"`
	CreatedEnrollments   int `json:"createdEnrollments,omitempty"`
	UpdatedEnrollments   int `json:"updatedEnrollments,omitempty"`
	UnchangedEnrollments int `json:"unchangedEnrollments,omitempty"`
}

type ImportWarning struct {
	Sheet   string `json:"sheet"`
	Row     int    `json:"row"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ImportRow struct {
	Sheet            string   `json:"sheet"`
	Row              int      `json:"row"`
	EmployeeName     string   `json:"employeeName,omitempty"`
	EmployeeEmail    string   `json:"employeeEmail,omitempty"`
	CourseTitle      string   `json:"courseTitle,omitempty"`
	TeamName         string   `json:"teamName,omitempty"`
	SkillAreaName    string   `json:"skillAreaName,omitempty"`
	VendorName       string   `json:"vendorName,omitempty"`
	CourseURL        string   `json:"courseUrl,omitempty"`
	Priority         *int     `json:"priority,omitempty"`
	LevelAsIs        *int     `json:"levelAsIs,omitempty"`
	LevelToBe        *int     `json:"levelToBe,omitempty"`
	PlannedStart     string   `json:"plannedStart,omitempty"`
	PlannedEnd       string   `json:"plannedEnd,omitempty"`
	HoursPlanned     *int     `json:"hoursPlanned,omitempty"`
	CostPlanned      *float64 `json:"costPlanned,omitempty"`
	Motivation       string   `json:"motivation,omitempty"`
	Objective        string   `json:"objective,omitempty"`
	Notes            string   `json:"notes,omitempty"`
	EnrollmentStatus string   `json:"enrollmentStatus,omitempty"`
	Year             int      `json:"year,omitempty"`
	Status           string   `json:"status"`
}

type JobRunResponse struct {
	OK                         bool `json:"ok"`
	ExpiredEnrollments         int  `json:"expiredEnrollments"`
	ComplianceNotifications    int  `json:"complianceNotifications"`
	CertificationNotifications int  `json:"certificationNotifications"`
}
