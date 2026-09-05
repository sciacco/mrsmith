package training

// Canonical planning transport types. Pointer fields deliberately preserve JSON null.
type PlanningRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type PlanningDeliveryCounts struct {
	Planned            int `json:"planned"`
	InProgress         int `json:"in_progress"`
	Completed          int `json:"completed"`
	PartiallyCompleted int `json:"partially_completed"`
	NotAttended        int `json:"not_attended"`
	Cancelled          int `json:"cancelled"`
}
type PlanningReminder struct {
	OwnerKind  string  `json:"ownerKind"`
	OwnerID    string  `json:"ownerId"`
	OwnerLabel string  `json:"ownerLabel"`
	CourseID   string  `json:"courseId"`
	Text       string  `json:"text"`
	Date       *string `json:"date"`
	Operative  bool    `json:"operative"`
	Timing     string  `json:"timing"`
}
type PlanningArea struct {
	PlanningRef
	LevelCurrent *int `json:"levelCurrent"`
	LevelTarget  *int `json:"levelTarget"`
}
type PlanningRequestItem struct {
	ID                    string         `json:"id"`
	Employee              PlanningRef    `json:"employee"`
	Team                  PlanningRef    `json:"team"`
	Course                PlanningRef    `json:"course"`
	Priority              *int           `json:"priority"`
	CreatedAt             string         `json:"createdAt"`
	Areas                 []PlanningArea `json:"areas"`
	TLOpinion             *string        `json:"tlOpinion"`
	PeopleDecision        *string        `json:"peopleDecision"`
	Outcome               *string        `json:"outcome"`
	Suspended             bool           `json:"suspended"`
	Operative             bool           `json:"operative"`
	AcceptedCourse        *PlanningRef   `json:"acceptedCourse"`
	AcceptedEventID       *string        `json:"acceptedEventId"`
	ResultingEnrollmentID *string        `json:"resultingEnrollmentId"`
}
type PlanningEventItem struct {
	ID                    string                 `json:"id"`
	Title                 string                 `json:"title"`
	CourseID              string                 `json:"courseId"`
	Origin                string                 `json:"origin"`
	Cancelled             bool                   `json:"cancelled"`
	Operative             bool                   `json:"operative"`
	SessionsCount         int                    `json:"sessionsCount"`
	StartsAt              *string                `json:"startsAt"`
	EndsAt                *string                `json:"endsAt"`
	DueOn                 *string                `json:"dueOn"`
	EnrollmentsCount      int                    `json:"enrollmentsCount"`
	EnrollmentsByStatus   PlanningDeliveryCounts `json:"enrollmentsByStatus"`
	WithoutSessions       bool                   `json:"withoutSessions"`
	UnassignedEnrollments bool                   `json:"unassignedEnrollments"`
	NeedsReconciliation   bool                   `json:"needsReconciliation"`
}
type PlanningEnrollmentItem struct {
	ID             string        `json:"id"`
	Employee       PlanningRef   `json:"employee"`
	Teams          []PlanningRef `json:"teams"`
	Event          PlanningRef   `json:"event"`
	DeliveryStatus string        `json:"deliveryStatus"`
	RequestIDs     []string      `json:"requestIds"`
}
type PlanningEconomic struct {
	DistinctPOCount            int `json:"distinctPOCount"`
	Approved                   int `json:"approved"`
	Pending                    int `json:"pending"`
	Rejected                   int `json:"rejected"`
	CoveredEnrollments         int `json:"coveredEnrollments"`
	ApprovedCoveredEnrollments int `json:"approvedCoveredEnrollments"`
}
type CourseSummary struct {
	ID                     string                 `json:"id"`
	Title                  string                 `json:"title"`
	Tags                   []string               `json:"tags"`
	Areas                  []PlanningRef          `json:"areas"`
	CourseSuspended        bool                   `json:"courseSuspended"`
	SuspensionReason       *string                `json:"suspensionReason"`
	Operative              bool                   `json:"operative"`
	SuspendedWork          bool                   `json:"suspendedWork"`
	History                bool                   `json:"history"`
	Reasons                []string               `json:"reasons"`
	Priority               *int                   `json:"priority"`
	PeopleCount            int                    `json:"peopleCount"`
	EnrolledPeopleCount    int                    `json:"enrolledPeopleCount"`
	RequestsCount          int                    `json:"requestsCount"`
	OperativeRequestsCount int                    `json:"operativeRequestsCount"`
	SuspendedRequestsCount int                    `json:"suspendedRequestsCount"`
	EventsCount            int                    `json:"eventsCount"`
	OperativeEventsCount   int                    `json:"operativeEventsCount"`
	EnrollmentsCount       int                    `json:"enrollmentsCount"`
	EnrollmentsByStatus    PlanningDeliveryCounts `json:"enrollmentsByStatus"`
	Economic               PlanningEconomic       `json:"economic"`
	ReminderCount          int                    `json:"reminderCount"`
	OperativeReminderCount int                    `json:"operativeReminderCount"`
	DueReminderCount       int                    `json:"dueReminderCount"`
	PrimaryReminder        *PlanningReminder      `json:"primaryReminder"`
}
type PlanningListResponse struct {
	Today           string          `json:"today"`
	GeneratedAt     string          `json:"generatedAt"`
	Items           []CourseSummary `json:"items"`
	Total           int             `json:"total"`
	Limit           int             `json:"limit"`
	Offset          int             `json:"offset"`
	DueCoursesTotal int             `json:"dueCoursesTotal"`
}
type PlanningFiltersResponse struct {
	Tags   []string      `json:"tags"`
	People []PlanningRef `json:"people"`
	Teams  []PlanningRef `json:"teams"`
	Areas  []PlanningRef `json:"areas"`
}
type PlanningCourseDetail struct {
	Today           string                `json:"today"`
	Course          CourseSummary         `json:"course"`
	RequestsPreview []PlanningRequestItem `json:"requestsPreview"`
	EventsPreview   []PlanningEventItem   `json:"eventsPreview"`
	EventOptions    []PlanningRef         `json:"eventOptions"`
	TeamOptions     []PlanningRef         `json:"teamOptions"`
}
type PlanningItemsResponse struct {
	Today           string `json:"today"`
	Kind            string `json:"kind"`
	Items           any    `json:"items"`
	Total           int    `json:"total"`
	UnfilteredTotal int    `json:"unfilteredTotal"`
	Limit           int    `json:"limit"`
	Offset          int    `json:"offset"`
}

type PlanningListFilter struct {
	View, Q, Tag, EmployeeID, TeamID, SkillAreaID string
	Limit, Offset                                 int
}
type PlanningItemsFilter struct {
	Kind, Q, TeamID, EventID, Status string
	Limit, Offset                    int
}
type ReminderUpdateInput struct {
	Text *string `json:"text"`
	Date *string `json:"date"`
}
