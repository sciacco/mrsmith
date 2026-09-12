package training

type NeedInput struct {
	Description   string   `json:"description"`
	Status        string   `json:"status"`
	FinalCourseID string   `json:"finalCourseId,omitempty"`
	SkillAreaIDs  []string `json:"skillAreaIds,omitempty"`
	Notes         string   `json:"notes,omitempty"`
	ReminderText  string   `json:"reminderText,omitempty"`
	ReminderAt    string   `json:"reminderAt,omitempty"`
}

type Need struct {
	ID               string         `json:"id"`
	Description      string         `json:"description"`
	Status           string         `json:"status"`
	FinalCourseID    string         `json:"finalCourseId,omitempty"`
	FinalCourseTitle string         `json:"finalCourseTitle,omitempty"`
	SkillAreas       []SkillAreaRef `json:"skillAreas"`
	Notes            string         `json:"notes,omitempty"`
	ReminderText     string         `json:"reminderText,omitempty"`
	ReminderAt       string         `json:"reminderAt,omitempty"`
	CandidatesCount  int            `json:"candidatesCount"`
	RequestsCount    int            `json:"requestsCount"`
	CreatedAt        string         `json:"createdAt"`
	UpdatedAt        string         `json:"updatedAt"`
}

type NeedCandidateInput struct {
	CourseID       string `json:"courseId,omitempty"`
	NewCourseTitle string `json:"newCourseTitle,omitempty"`
	Notes          string `json:"notes,omitempty"`
	Rank           *int   `json:"rank,omitempty"`
}

type NeedCandidate struct {
	CourseID    string `json:"courseId"`
	CourseTitle string `json:"courseTitle"`
	IsActive    bool   `json:"isActive"`
	Notes       string `json:"notes,omitempty"`
	Rank        *int   `json:"rank,omitempty"`
}

type NeedRequestsInput struct {
	RequestIDs []string `json:"requestIds"`
}

type NeedRequest struct {
	ID                  string `json:"id"`
	Description         string `json:"description"`
	EmployeeID          string `json:"employeeId"`
	EmployeeName        string `json:"employeeName"`
	Outcome             string `json:"outcome,omitempty"`
	SuspendedAt         string `json:"suspendedAt,omitempty"`
	AcceptedCourseID    string `json:"acceptedCourseId,omitempty"`
	AcceptedCourseTitle string `json:"acceptedCourseTitle,omitempty"`
	AcceptedEventID     string `json:"acceptedEventId,omitempty"`
}

type NeedCoverage struct {
	EmployeeID   string `json:"employeeId"`
	EmployeeName string `json:"employeeName"`
	RequestCoverage
}

type NeedDetail struct {
	Need
	Candidates []NeedCandidate `json:"candidates"`
	Requests   []NeedRequest   `json:"requests"`
	Coverage   []NeedCoverage  `json:"coverage"`
}

// Il corso proviene esclusivamente dal definitivo dell'esigenza.
// Nessuna iscrizione individuale nel comando collettivo: viene riusata per persona/evento.
type NeedAcceptInput struct {
	EventID     string `json:"eventId,omitempty"`
	VendorID    string `json:"vendorId,omitempty"`
	PeriodStart string `json:"periodStart,omitempty"`
	PeriodEnd   string `json:"periodEnd,omitempty"`
	Notes       string `json:"notes,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type NeedAcceptResponse struct {
	AcceptedRequestIDs []string      `json:"acceptedRequestIds"`
	ExcludedRequests   []NeedRequest `json:"excludedRequests"`
	EventID            string        `json:"eventId,omitempty"`
}
