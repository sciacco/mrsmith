package training

// ── Richiesta formativa (#140, §4-Richieste; #171) ──
//
// La richiesta conserva due facce separate: i dati originali espressi dalla
// persona (modificabili finche la richiesta non e chiusa; la persona e
// invariata) e i dati accolti da People. Parere TL e decisione People sono
// riscrivibili finche la richiesta non e chiusa (la decisione anche a
// richiesta chiusa da decisione); il ritiro e un esito terminale distinto.

// RequestInput sono i dati originali della richiesta, registrati da People
// per conto della persona. Corso a catalogo e titolo nuovo sono alternativi:
// un titolo e l'embrione di un corso, quindi il backend riusa il corso
// esistente con lo stesso nome oppure lo crea (solo nome) nella stessa
// transazione — ogni richiesta aggancia sempre un corso. Il team scelto e
// obbligatorio solo se la persona ha appartenenze attive; quando presente
// deve essere tra queste (#200).
type RequestInput struct {
	EmployeeID     string                  `json:"employeeId"`
	CourseID       string                  `json:"courseId,omitempty"`
	NewCourseTitle string                  `json:"newCourseTitle,omitempty"`
	SkillAreas     []RequestSkillAreaInput `json:"skillAreas,omitempty"`
	Motivation     string                  `json:"motivation"`
	SelectedTeamID string                  `json:"selectedTeamId,omitempty"`
	DesiredStart   string                  `json:"desiredStart,omitempty"` // YYYY-MM-DD
	DesiredEnd     string                  `json:"desiredEnd,omitempty"`   // YYYY-MM-DD
	// Priorita facoltativa (1 = piu importante, senza unicita) e annotazioni
	// di pianificazione, scritte nel gesto di registrazione.
	Priority     *int   `json:"priority,omitempty"`
	Notes        string `json:"notes,omitempty"`
	ReminderText string `json:"reminderText,omitempty"`
	ReminderAt   string `json:"reminderAt,omitempty"` // YYYY-MM-DD
}

// RequestSkillAreaInput e un'area dell'esigenza con la coppia facoltativa
// livello attuale -> atteso (scala 0-5), scritta una volta alla
// registrazione e mai piu richiesta.
type RequestSkillAreaInput struct {
	ID           string `json:"id"`
	LevelCurrent *int   `json:"levelCurrent,omitempty"`
	LevelTarget  *int   `json:"levelTarget,omitempty"`
}

// RequestAnnotationsInput sostituisce nota, promemoria e priorita di una
// richiesta aperta (il campo vuoto azzera).
type RequestAnnotationsInput struct {
	Notes        string `json:"notes,omitempty"`
	ReminderText string `json:"reminderText,omitempty"`
	ReminderAt   string `json:"reminderAt,omitempty"` // YYYY-MM-DD
	Priority     *int   `json:"priority,omitempty"`
}

// RequestOriginalDataInput sostituisce i dati originali di una richiesta
// aperta (#171): corso a catalogo o titolo nuovo (alternativi; un titolo e
// l'embrione di un corso), aree con livelli, motivazione, team scelto
// (obbligatorio solo se la persona ha appartenenze attive; quando presente
// deve essere tra queste) e date desiderate. La persona e invariata
// (correzione = ritiro + nuova richiesta); priorita, nota e promemoria
// restano sul PUT annotations. Stesse validazioni di RequestInput.
type RequestOriginalDataInput struct {
	CourseID       string                  `json:"courseId,omitempty"`
	NewCourseTitle string                  `json:"newCourseTitle,omitempty"`
	SkillAreas     []RequestSkillAreaInput `json:"skillAreas,omitempty"`
	Motivation     string                  `json:"motivation"`
	SelectedTeamID string                  `json:"selectedTeamId,omitempty"`
	DesiredStart   string                  `json:"desiredStart,omitempty"` // YYYY-MM-DD
	DesiredEnd     string                  `json:"desiredEnd,omitempty"`   // YYYY-MM-DD
}

// TLOpinionInput registra il parere TL come fatto: chi lo esprime deve essere
// un lead attivo del team scelto sulla richiesta. Il parere e riscrivibile
// finche la richiesta non e chiusa: vale l'ultimo, la storia resta
// nell'audit. La motivazione e facoltativa.
type TLOpinionInput struct {
	LeadEmployeeID string `json:"leadEmployeeId"`
	Opinion        string `json:"opinion"` // favorable|unfavorable
	Reason         string `json:"reason,omitempty"`
}

// RequestDecisionInput registra la decisione People: la prima decisione e
// ammessa su richiesta aperta, senza prerequisito di parere TL (#200); il
// parere resta un fatto consultivo facoltativo. La riscrittura e ammessa
// anche a richiesta chiusa da decisione. La motivazione e facoltativa:
// nell'accoglimento con parere sfavorevole e anche la motivazione
// dell'override (D5). Accepted e obbligatorio se la richiesta viene
// accolta, vietato se respinta.
type RequestDecisionInput struct {
	Decision string                `json:"decision"` // accepted|rejected
	Reason   string                `json:"reason,omitempty"`
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
	SkillAreas     []SkillAreaRef `json:"skillAreas"`
	SelectedTeamID string         `json:"selectedTeamId,omitempty"`
	SelectedTeam   string         `json:"selectedTeamName,omitempty"`
	Priority       *int           `json:"priority,omitempty"`
	ReminderText   string         `json:"reminderText,omitempty"`
	ReminderAt     string         `json:"reminderAt,omitempty"`
	SuspendedAt    string         `json:"suspendedAt,omitempty"`
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
// espressa dalla persona: modificabile finche la richiesta non e chiusa
// (PUT /requests/{id}/original; la persona e invariata).
type RequestOriginalData struct {
	EmployeeID       string           `json:"employeeId"`
	EmployeeName     string           `json:"employeeName"`
	EmployeeEmail    string           `json:"employeeEmail"`
	CourseID         string           `json:"courseId,omitempty"`
	CourseTitle      string           `json:"courseTitle,omitempty"`
	SkillAreas       []RequestAreaRef `json:"skillAreas"`
	Motivation       string           `json:"motivation"`
	SelectedTeamID   string           `json:"selectedTeamId,omitempty"`
	SelectedTeamName string           `json:"selectedTeamName,omitempty"`
	DesiredStart     string           `json:"desiredStart,omitempty"`
	DesiredEnd       string           `json:"desiredEnd,omitempty"`
}

// RequestAreaRef e un'area dell'esigenza con la coppia facoltativa di
// livelli attuale -> atteso.
type RequestAreaRef struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	LevelCurrent *int   `json:"levelCurrent,omitempty"`
	LevelTarget  *int   `json:"levelTarget,omitempty"`
}

// RequestTLOpinionFacts e il parere TL: vale l'ultimo, la storia resta
// nell'audit (riscrivibile finche la richiesta non e chiusa).
type RequestTLOpinionFacts struct {
	Opinion      string `json:"opinion"`
	ByEmployeeID string `json:"byEmployeeId,omitempty"`
	ByName       string `json:"byName,omitempty"`
	At           string `json:"at"`
	Reason       string `json:"reason,omitempty"`
}

// RequestDecisionFacts e la decisione People: riscrivibile anche a
// richiesta chiusa da decisione, la storia resta nell'audit.
type RequestDecisionFacts struct {
	Decision     string `json:"decision"`
	ByEmployeeID string `json:"byEmployeeId,omitempty"`
	ByName       string `json:"byName,omitempty"`
	At           string `json:"at"`
	Reason       string `json:"reason,omitempty"`
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
	Priority              *int                   `json:"priority,omitempty"`
	Notes                 string                 `json:"notes,omitempty"`
	ReminderText          string                 `json:"reminderText,omitempty"`
	ReminderAt            string                 `json:"reminderAt,omitempty"`
	SuspendedAt           string                 `json:"suspendedAt,omitempty"`
	SuspendedByName       string                 `json:"suspendedByName,omitempty"`
	SuspensionReason      string                 `json:"suspensionReason,omitempty"`
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
