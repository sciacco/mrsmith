package training

// Letture di dominio (#152, slice 1 del task 6): catalogo, persone e
// anagrafiche non ancora esposte dal backend. Le liste gestionali includono
// gli elementi inattivi (flag active); le lookups per i form restano
// invariate e a soli attivi (store.go).

type CourseListRow struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	SkillAreaID         string   `json:"skillAreaId,omitempty"`
	SkillAreaName       string   `json:"skillAreaName,omitempty"`
	VendorID            string   `json:"vendorId,omitempty"`
	VendorName          string   `json:"vendorName,omitempty"`
	DeliveryMode        string   `json:"deliveryMode"`
	ProviderKind        string   `json:"providerKind"`
	DefaultHours        *int     `json:"defaultHours,omitempty"`
	DefaultCost         *float64 `json:"defaultCost,omitempty"`
	LeadsToCertID       string   `json:"leadsToCertId,omitempty"`
	LeadsToCertName     string   `json:"leadsToCertName,omitempty"`
	ComplianceRelated   bool     `json:"complianceRelated"`
	ComplianceFramework string   `json:"complianceFramework,omitempty"`
	Active              bool     `json:"active"`
	FactorialTrainingID string   `json:"factorialTrainingId,omitempty"`
	UpdatedAt           string   `json:"updatedAt"`
}

type CourseListResponse struct {
	Courses []CourseListRow `json:"courses"`
}

type CourseRuleRef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"isActive"`
}

type CourseEventRef struct {
	ID               string `json:"id"`
	CreatedAt        string `json:"createdAt"`
	CancelledAt      string `json:"cancelledAt,omitempty"`
	EnrollmentsCount int    `json:"enrollmentsCount"`
	SessionsCount    int    `json:"sessionsCount"`
}

type CourseDetail struct {
	CourseListRow
	Description string           `json:"description,omitempty"`
	CourseURL   string           `json:"courseUrl,omitempty"`
	Rules       []CourseRuleRef  `json:"rules"`
	Events      []CourseEventRef `json:"events"`
}

type PersonTeamRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

type PersonGroupRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PersonListRow struct {
	ID              string           `json:"id"`
	FirstName       string           `json:"firstName"`
	LastName        string           `json:"lastName"`
	Email           string           `json:"email"`
	Status          string           `json:"status"`
	DirectoryExempt bool             `json:"directoryExempt"`
	Teams           []PersonTeamRef  `json:"teams"`
	Groups          []PersonGroupRef `json:"groups"`
}

type PersonListResponse struct {
	People []PersonListRow `json:"people"`
}

// PersonEnrollmentRef e un'iscrizione della persona su un evento, con lo
// stato di cancellazione dell'evento (cross-evento: nessun cancelled_at
// proprio dell'iscrizione).
type PersonEnrollmentRef struct {
	EnrollmentID    string `json:"enrollmentId"`
	EventID         string `json:"eventId"`
	CourseTitle     string `json:"courseTitle"`
	DeliveryStatus  string `json:"deliveryStatus"`
	LearningOutcome string `json:"learningOutcome,omitempty"`
	ActualStart     string `json:"actualStart,omitempty"`
	ActualEnd       string `json:"actualEnd,omitempty"`
	CancelledAt     string `json:"cancelledAt,omitempty"`
	CreatedAt       string `json:"createdAt"`
}

// PersonRequestRef e una richiesta formativa della persona; Outcome e nil
// quando la richiesta e ancora aperta.
type PersonRequestRef struct {
	ID            string  `json:"id"`
	CourseTitle   string  `json:"courseTitle,omitempty"`
	FreeTextTitle string  `json:"freeTextTitle,omitempty"`
	Outcome       *string `json:"outcome"`
	CreatedAt     string  `json:"createdAt"`
}

// PersonRuleCoverageRef e la copertura della persona su una regola attiva a
// platea che la include (nucleo condiviso di store_coverage.go).
type PersonRuleCoverageRef struct {
	RuleID   string `json:"ruleId"`
	RuleName string `json:"ruleName"`
	Need     string `json:"need"`
	Covered  bool   `json:"covered"`
	Deadline string `json:"deadline"`
}

type PersonDetail struct {
	PersonListRow
	Enrollments  []PersonEnrollmentRef   `json:"enrollments"`
	Requests     []PersonRequestRef      `json:"requests"`
	RuleCoverage []PersonRuleCoverageRef `json:"ruleCoverage"`
}

type TeamLeadRef struct {
	EmployeeID string `json:"employeeId"`
	Name       string `json:"name"`
}

type TeamListRow struct {
	ID            string        `json:"id"`
	Code          string        `json:"code"`
	Name          string        `json:"name"`
	Active        bool          `json:"active"`
	ManagedBySync bool          `json:"managedBySync"`
	Leads         []TeamLeadRef `json:"leads"`
	ActiveMembers int           `json:"activeMembers"`
}

type TeamListResponse struct {
	Teams []TeamListRow `json:"teams"`
}

type VendorListRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Website string `json:"website,omitempty"`
	Notes   string `json:"notes,omitempty"`
	Active  bool   `json:"active"`
}

type VendorListResponse struct {
	Vendors []VendorListRow `json:"vendors"`
}

type SkillAreaListRow struct {
	ID              string `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Active          bool   `json:"active"`
	CustomGroupID   string `json:"customGroupId,omitempty"`
	CustomGroupName string `json:"customGroupName,omitempty"`
	ParentID        string `json:"parentId,omitempty"`
}

type SkillAreaListResponse struct {
	SkillAreas []SkillAreaListRow `json:"skillAreas"`
}

type CertificationCatalogRow struct {
	ID                    string `json:"id"`
	Code                  string `json:"code"`
	Name                  string `json:"name"`
	Description           string `json:"description,omitempty"`
	Active                bool   `json:"active"`
	IssuerVendorID        string `json:"issuerVendorId,omitempty"`
	IssuerVendorName      string `json:"issuerVendorName,omitempty"`
	SkillAreaID           string `json:"skillAreaId,omitempty"`
	SkillAreaName         string `json:"skillAreaName,omitempty"`
	TypicalValidityMonths *int   `json:"typicalValidityMonths,omitempty"`
}

type CertificationCatalogResponse struct {
	Certifications []CertificationCatalogRow `json:"certifications"`
}
