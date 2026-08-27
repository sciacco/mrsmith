package training

// Tipi degli endpoint massivi del workspace (#153, slice 2 del task 6):
// assegnazioni iscrizioni->sessioni, presenze per sessione, iscrizioni
// multiple. Riusano gli stati e le regole del nucleo esistente.

// BulkAssignmentsInput e il corpo di POST .../bulk-assignments.
// SessionIDs vale per all_to_all e distribute (default: tutte le sessioni
// dell'evento); SessionID e obbligatorio solo per fill_session.
type BulkAssignmentsInput struct {
	Mode       string   `json:"mode"`
	SessionIDs []string `json:"sessionIds,omitempty"`
	SessionID  string   `json:"sessionId,omitempty"`
}

// BulkAssignmentsResponse riporta gli assegnamenti creati (le coppie gia
// esistenti saltate non contano) e la distribuzione per sessione.
type BulkAssignmentsResponse struct {
	OK         bool           `json:"ok"`
	Assigned   int            `json:"assigned"`
	PerSession map[string]int `json:"perSession"`
}

// BulkParticipationInput e il corpo di POST .../bulk-participation. Senza
// EnrollmentIDs agisce su tutte le relazioni della sessione appartenenti a
// iscrizioni non annullate.
type BulkParticipationInput struct {
	ParticipationStatus string   `json:"participationStatus"`
	EnrollmentIDs       []string `json:"enrollmentIds,omitempty"`
}

type BulkParticipationResponse struct {
	OK      bool `json:"ok"`
	Updated int  `json:"updated"`
}

// BulkEnrollInput e il corpo di POST .../enrollments/bulk.
type BulkEnrollInput struct {
	EmployeeIDs []string `json:"employeeIds"`
	Objective   string   `json:"objective,omitempty"`
	Notes       string   `json:"notes,omitempty"`
}

type BulkEnrollCreatedRow struct {
	EnrollmentID string `json:"enrollmentId"`
	EmployeeID   string `json:"employeeId"`
}

// BulkEnrollSkippedRow riporta la stessa semantica di salto del gesto
// "alimenta platea" (#140): reason vale already_enrolled o inactive.
type BulkEnrollSkippedRow struct {
	EmployeeID string `json:"employeeId"`
	Reason     string `json:"reason"`
}

type BulkEnrollResponse struct {
	OK      bool                   `json:"ok"`
	Created []BulkEnrollCreatedRow `json:"created"`
	Skipped []BulkEnrollSkippedRow `json:"skipped"`
}
