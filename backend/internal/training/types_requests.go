package training

// ── Richiesta formativa (#140, §4-Richieste) ──
//
// La richiesta conserva due facce separate: i dati originali espressi dalla
// persona (create-only: nessuna API li modifica dopo la registrazione) e i
// dati accolti da People. Parere TL e decisione People sono fatti immutabili;
// il ritiro e un esito distinto.

// RequestInput sono i dati originali della richiesta, registrati da People
// per conto della persona. Corso a catalogo e titolo libero sono alternativi
// (XOR applicato dal backend: il CHECK db ammette entrambi); il team scelto
// deve essere tra le appartenenze attive della persona.
type RequestInput struct {
	EmployeeID     string   `json:"employeeId"`
	CourseID       string   `json:"courseId,omitempty"`
	FreeTextTitle  string   `json:"freeTextTitle,omitempty"`
	SkillAreaIDs   []string `json:"skillAreaIds,omitempty"`
	Motivation     string   `json:"motivation"`
	SelectedTeamID string   `json:"selectedTeamId"`
	DesiredStart   string   `json:"desiredStart,omitempty"` // YYYY-MM-DD
	DesiredEnd     string   `json:"desiredEnd,omitempty"`   // YYYY-MM-DD
}

// TLOpinionInput registra il parere TL come fatto: chi lo esprime deve essere
// un lead attivo del team scelto sulla richiesta. La motivazione e sempre
// obbligatoria, come per la decisione People.
type TLOpinionInput struct {
	LeadEmployeeID string `json:"leadEmployeeId"`
	Opinion        string `json:"opinion"` // favorable|unfavorable
	Reason         string `json:"reason"`
}

// RequestDecisionInput registra la decisione People. La motivazione e sempre
// obbligatoria: nell'accoglimento con parere sfavorevole e anche la
// motivazione dell'override (D5). Accepted e obbligatorio se la richiesta
// viene accolta, vietato se respinta.
type RequestDecisionInput struct {
	Decision string                `json:"decision"` // accepted|rejected
	Reason   string                `json:"reason"`
	Accepted *RequestAcceptedInput `json:"accepted,omitempty"`
}

// RequestAcceptedInput e la faccia accolta della decisione (D8). Il corso
// deve gia esistere a catalogo (un corso fuori catalogo viene creato prima
// con l'upsert corsi); l'evento indicato viene collegato, altrimenti viene
// creato contestualmente; existingEnrollmentId collega un'iscrizione gia
// presente della persona senza creare doppioni.
type RequestAcceptedInput struct {
	CourseID             string `json:"courseId"`
	EventID              string `json:"eventId,omitempty"`
	VendorID             string `json:"vendorId,omitempty"`
	PeriodStart          string `json:"periodStart,omitempty"` // YYYY-MM-DD
	PeriodEnd            string `json:"periodEnd,omitempty"`   // YYYY-MM-DD
	Notes                string `json:"notes,omitempty"`
	ExistingEnrollmentID string `json:"existingEnrollmentId,omitempty"`
}

// ── Letture ──

type RequestListRow struct {
	ID             string         `json:"id"`
	EmployeeID     string         `json:"employeeId"`
	EmployeeName   string         `json:"employeeName"`
	EmployeeEmail  string         `json:"employeeEmail"`
	CourseID       string         `json:"courseId,omitempty"`
	CourseTitle    string         `json:"courseTitle,omitempty"`
	FreeTextTitle  string         `json:"freeTextTitle,omitempty"`
	SkillAreas     []SkillAreaRef `json:"skillAreas"`
	SelectedTeamID string         `json:"selectedTeamId"`
	SelectedTeam   string         `json:"selectedTeamName"`
	TLOpinion      string         `json:"tlOpinion,omitempty"`
	PeopleDecision string         `json:"peopleDecision,omitempty"`
	Outcome        string         `json:"outcome,omitempty"`
	ClosedAt       string         `json:"closedAt,omitempty"`
	CreatedAt      string         `json:"createdAt"`
}

type RequestListResponse struct {
	Requests []RequestListRow `json:"requests"`
}

// RequestOriginalData e la faccia originale della richiesta, cosi come
// espressa dalla persona: nessuna API la modifica.
type RequestOriginalData struct {
	EmployeeID       string         `json:"employeeId"`
	EmployeeName     string         `json:"employeeName"`
	EmployeeEmail    string         `json:"employeeEmail"`
	CourseID         string         `json:"courseId,omitempty"`
	CourseTitle      string         `json:"courseTitle,omitempty"`
	FreeTextTitle    string         `json:"freeTextTitle,omitempty"`
	SkillAreas       []SkillAreaRef `json:"skillAreas"`
	Motivation       string         `json:"motivation"`
	SelectedTeamID   string         `json:"selectedTeamId"`
	SelectedTeamName string         `json:"selectedTeamName"`
	DesiredStart     string         `json:"desiredStart,omitempty"`
	DesiredEnd       string         `json:"desiredEnd,omitempty"`
}

// RequestTLOpinionFacts e il fatto immutabile del parere TL.
type RequestTLOpinionFacts struct {
	Opinion      string `json:"opinion"`
	ByEmployeeID string `json:"byEmployeeId,omitempty"`
	ByName       string `json:"byName,omitempty"`
	At           string `json:"at"`
	Reason       string `json:"reason,omitempty"`
}

// RequestDecisionFacts e il fatto immutabile della decisione People.
type RequestDecisionFacts struct {
	Decision     string `json:"decision"`
	ByEmployeeID string `json:"byEmployeeId,omitempty"`
	ByName       string `json:"byName,omitempty"`
	At           string `json:"at"`
	Reason       string `json:"reason"`
}

// RequestAcceptedData e la faccia accolta: non sovrascrive mai i dati
// originali della richiesta.
type RequestAcceptedData struct {
	CourseID    string `json:"courseId"`
	CourseTitle string `json:"courseTitle"`
	EventID     string `json:"eventId,omitempty"`
	VendorID    string `json:"vendorId,omitempty"`
	VendorName  string `json:"vendorName,omitempty"`
	PeriodStart string `json:"periodStart,omitempty"`
	PeriodEnd   string `json:"periodEnd,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

// RequestCoverageEnrollment e un'iscrizione completata della persona su un
// evento del corso considerato.
type RequestCoverageEnrollment struct {
	EnrollmentID string `json:"enrollmentId"`
	EventID      string `json:"eventId"`
	Origin       string `json:"origin"`
	CompletedOn  string `json:"completedOn"`
}

// RequestCoverageAward e un conseguimento valido della certificazione
// collegata al corso considerato.
type RequestCoverageAward struct {
	AwardID           string `json:"awardId"`
	CertificationID   string `json:"certificationId"`
	CertificationName string `json:"certificationName"`
	AwardedOn         string `json:"awardedOn"`
	ExpiresOn         string `json:"expiresOn,omitempty"`
	ValidationSource  string `json:"validationSource"`
}

// RequestCoverage espone la copertura esistente della persona per il corso
// richiesto o accettato (iscrizioni completed + award validi), cosi People
// vede i doppioni potenziali prima di decidere (D8).
type RequestCoverage struct {
	CourseID             string                      `json:"courseId,omitempty"`
	CourseTitle          string                      `json:"courseTitle,omitempty"`
	CompletedEnrollments []RequestCoverageEnrollment `json:"completedEnrollments"`
	ValidAwards          []RequestCoverageAward      `json:"validAwards"`
}

type RequestDetail struct {
	ID                    string                 `json:"id"`
	Requested             RequestOriginalData    `json:"requested"`
	TLOpinion             *RequestTLOpinionFacts `json:"tlOpinion,omitempty"`
	Decision              *RequestDecisionFacts  `json:"decision,omitempty"`
	Outcome               string                 `json:"outcome,omitempty"`
	ClosedAt              string                 `json:"closedAt,omitempty"`
	Accepted              *RequestAcceptedData   `json:"accepted,omitempty"`
	ResultingEnrollmentID string                 `json:"resultingEnrollmentId,omitempty"`
	ExistingCoverage      RequestCoverage        `json:"existingCoverage"`
	CreatedAt             string                 `json:"createdAt"`
	UpdatedAt             string                 `json:"updatedAt"`
}
