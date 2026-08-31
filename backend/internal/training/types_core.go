package training

// ── Evento ──

// EventInput copre creazione e modifica: alla creazione basta il corso,
// il resto si aggiunge progressivamente. L'update (PUT) e una sostituzione
// completa: il campo vuoto o omesso azzera. I campi sorgente riservati
// (origin, source_rule_id, source_request_id, rule_deadline, factorial_*)
// non sono mai scrivibili dal client.
type EventInput struct {
	CourseID         string   `json:"courseId"`
	VendorID         string   `json:"vendorId,omitempty"`
	AgreedPrice      *float64 `json:"agreedPrice,omitempty"`
	AgreedConditions string   `json:"agreedConditions,omitempty"`
	Notes            string   `json:"notes,omitempty"`
	// Title: alla creazione e sempre ereditato dal corso; in update il campo
	// vuoto conserva il titolo corrente (unico campo che non si azzera: e
	// NOT NULL per contratto).
	Title        string `json:"title,omitempty"`
	ReminderText string `json:"reminderText,omitempty"`
	ReminderAt   string `json:"reminderAt,omitempty"` // YYYY-MM-DD
	// TrainerIDs: in update sostituisce l'elenco dei formatori dell'evento;
	// alla creazione l'evento parte dai formatori designati del corso.
	TrainerIDs []string `json:"trainerIds,omitempty"`
}

// TrainerRef e un formatore interno designato (corso) o assegnato (evento).
type TrainerRef struct {
	EmployeeID string `json:"employeeId"`
	Name       string `json:"name"`
}

type ReasonInput struct {
	Reason string `json:"reason"`
}

// ── Sessione ──

// SessionInput copre creazione e modifica. L'update (PUT) e una sostituzione
// completa: il campo vuoto o omesso azzera, capienza compresa (omettere
// maxCapacity rimuove il limite).
type SessionInput struct {
	// ScheduleType e obbligatorio per People (scheduled|self_paced); il NULL
	// nello schema resta riservato all'import Factorial.
	ScheduleType string `json:"scheduleType"`
	StartsAt     string `json:"startsAt,omitempty"` // RFC 3339
	EndsAt       string `json:"endsAt,omitempty"`   // RFC 3339
	DueAt        string `json:"dueAt,omitempty"`    // RFC 3339
	MaxCapacity  *int   `json:"maxCapacity,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

// ── Iscrizione ──

type EnrollmentCreateInput struct {
	EmployeeID string `json:"employeeId"`
	Objective  string `json:"objective,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

// EnrollmentFactsInput e l'update unico dei fatti senza effetti di stato:
// obiettivo, note, date effettive, ore e esito. Sostituzione completa; il
// campo vuoto azzera.
type EnrollmentFactsInput struct {
	Objective       string `json:"objective,omitempty"`
	Notes           string `json:"notes,omitempty"`
	ActualStart     string `json:"actualStart,omitempty"` // YYYY-MM-DD
	ActualEnd       string `json:"actualEnd,omitempty"`   // YYYY-MM-DD
	HoursActual     *int   `json:"hoursActual,omitempty"`
	LearningOutcome string `json:"learningOutcome,omitempty"`
}

// ── Partecipazione ──

type ParticipationInput struct {
	ParticipationStatus string `json:"participationStatus"`
}

// ── Letture event-centric ──

type EventFlags struct {
	Cancelled             bool `json:"cancelled"`
	WithoutSessions       bool `json:"withoutSessions"`
	UnassignedEnrollments bool `json:"unassignedEnrollments"`
	InProgress            bool `json:"inProgress"`
	NeedsReconciliation   bool `json:"needsReconciliation"`
	Concluded             bool `json:"concluded"`
}

type EventListRow struct {
	ID                        string     `json:"id"`
	CourseID                  string     `json:"courseId"`
	CourseTitle               string     `json:"courseTitle"`
	Title                     string     `json:"title"`
	ReminderText              string     `json:"reminderText,omitempty"`
	ReminderAt                string     `json:"reminderAt,omitempty"`
	VendorID                  string     `json:"vendorId,omitempty"`
	VendorName                string     `json:"vendorName,omitempty"`
	AgreedPrice               *float64   `json:"agreedPrice,omitempty"`
	Origin                    string     `json:"origin"`
	CancelledAt               string     `json:"cancelledAt,omitempty"`
	SessionsCount             int        `json:"sessionsCount"`
	EnrollmentsCount          int        `json:"enrollmentsCount"`
	CancelledEnrollmentsCount int        `json:"cancelledEnrollmentsCount"`
	Flags                     EventFlags `json:"flags"`
	CreatedAt                 string     `json:"createdAt"`
	UpdatedAt                 string     `json:"updatedAt"`
}

type SessionDetail struct {
	ID                 string   `json:"id"`
	ScheduleType       string   `json:"scheduleType,omitempty"`
	StartsAt           string   `json:"startsAt,omitempty"`
	EndsAt             string   `json:"endsAt,omitempty"`
	DueAt              string   `json:"dueAt,omitempty"`
	MaxCapacity        *int     `json:"maxCapacity,omitempty"`
	Occupancy          int      `json:"occupancy"`
	Notes              string   `json:"notes,omitempty"`
	FactorialSessionID string   `json:"factorialSessionId,omitempty"`
	Topic              string   `json:"topic,omitempty"`
	Modality           string   `json:"modality,omitempty"`
	DurationHours      *float64 `json:"durationHours,omitempty"`
	Location           string   `json:"location,omitempty"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
}

type EnrollmentDetail struct {
	ID                 string `json:"id"`
	EmployeeID         string `json:"employeeId"`
	EmployeeName       string `json:"employeeName"`
	EmployeeEmail      string `json:"employeeEmail"`
	DeliveryStatus     string `json:"deliveryStatus"`
	LearningOutcome    string `json:"learningOutcome,omitempty"`
	Origin             string `json:"origin"`
	Objective          string `json:"objective,omitempty"`
	Notes              string `json:"notes,omitempty"`
	ActualStart        string `json:"actualStart,omitempty"`
	ActualEnd          string `json:"actualEnd,omitempty"`
	HoursActual        *int   `json:"hoursActual,omitempty"`
	CancellationReason string `json:"cancellationReason,omitempty"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

type ParticipationRow struct {
	EnrollmentID        string   `json:"enrollmentId"`
	SessionID           string   `json:"sessionId"`
	ParticipationStatus string   `json:"participationStatus"`
	CompletedHours      *float64 `json:"completedHours,omitempty"`
	AssignedAt          string   `json:"assignedAt"`
	UpdatedAt           string   `json:"updatedAt"`
}

type EventDetail struct {
	ID                 string             `json:"id"`
	CourseID           string             `json:"courseId"`
	CourseTitle        string             `json:"courseTitle"`
	Title              string             `json:"title"`
	ReminderText       string             `json:"reminderText,omitempty"`
	ReminderAt         string             `json:"reminderAt,omitempty"`
	Trainers           []TrainerRef       `json:"trainers"`
	VendorID           string             `json:"vendorId,omitempty"`
	VendorName         string             `json:"vendorName,omitempty"`
	AgreedPrice        *float64           `json:"agreedPrice,omitempty"`
	AgreedConditions   string             `json:"agreedConditions,omitempty"`
	Origin             string             `json:"origin"`
	SourceRuleID       string             `json:"sourceRuleId,omitempty"`
	SourceRequestID    string             `json:"sourceRequestId,omitempty"`
	RuleDeadline       string             `json:"ruleDeadline,omitempty"`
	FactorialClassID   string             `json:"factorialClassId,omitempty"`
	CancelledAt        string             `json:"cancelledAt,omitempty"`
	CancellationReason string             `json:"cancellationReason,omitempty"`
	Notes              string             `json:"notes,omitempty"`
	Flags              EventFlags         `json:"flags"`
	Sessions           []SessionDetail    `json:"sessions"`
	Enrollments        []EnrollmentDetail `json:"enrollments"`
	Participations     []ParticipationRow `json:"participations"`
	Expenses           []EventExpense     `json:"expenses"`
	CreatedAt          string             `json:"createdAt"`
	UpdatedAt          string             `json:"updatedAt"`
}

type EventListResponse struct {
	Events []EventListRow `json:"events"`
}
