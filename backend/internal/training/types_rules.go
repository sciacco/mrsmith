package training

// Tipi delle regole formative (#140, §4-Regole). Gli input coprono creazione
// e modifica; il PUT e una sostituzione completa (idioma del pacchetto).
// Lo stato attivo non fa parte dell'input: si governa con i gesti
// activate/deactivate.

// RuleInput descrive una regola formativa: corso di riferimento, platea XOR
// posizioni, obbligatorieta, scadenza e ricorrenza (mesi + ancora in coppia).
type RuleInput struct {
	Name             string               `json:"name"`
	CourseID         string               `json:"courseId"`
	Population       *RulePopulationInput `json:"population,omitempty"` // XOR SeatCount
	SeatCount        *int                 `json:"seatCount,omitempty"`
	IsMandatory      bool                 `json:"isMandatory"`
	Deadline         string               `json:"deadline"`                   // YYYY-MM-DD, obbligatoria
	RecurrenceMonths *int                 `json:"recurrenceMonths,omitempty"` // in coppia con RecurrenceAnchor
	RecurrenceAnchor string               `json:"recurrenceAnchor,omitempty"` // calendar|completion
	Notes            string               `json:"notes,omitempty"`
}

// RulePopulationInput descrive la platea della regola.
type RulePopulationInput struct {
	Kind      string   `json:"kind"`                // all|team|skill_area|custom_group|people
	ID        string   `json:"id,omitempty"`        // team|skill_area|custom_group
	PersonIDs []string `json:"personIds,omitempty"` // people
}

// RuleListRow e la riga della lista regole con i campi calcolati: natura del
// bisogno, kind platea (o posizioni), dimensione platea risolta, coperti,
// prossima tornata e tornata in corso.
type RuleListRow struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	CourseID             string `json:"courseId"`
	CourseTitle          string `json:"courseTitle"`
	Need                 string `json:"need"`                     // attendance|certification
	PopulationKind       string `json:"populationKind,omitempty"` // vuoto per le regole a posizioni
	SeatCount            *int   `json:"seatCount,omitempty"`      // solo regole a posizioni
	PopulationSize       *int   `json:"populationSize,omitempty"` // solo regole a platea
	CoveredCount         int    `json:"coveredCount"`
	IsMandatory          bool   `json:"isMandatory"`
	Deadline             string `json:"deadline"`
	RecurrenceMonths     *int   `json:"recurrenceMonths,omitempty"`
	RecurrenceAnchor     string `json:"recurrenceAnchor,omitempty"`
	NextRoundDeadline    string `json:"nextRoundDeadline,omitempty"`
	CurrentRoundEventID  string `json:"currentRoundEventId,omitempty"`
	CurrentRoundDeadline string `json:"currentRoundDeadline,omitempty"`
	IsActive             bool   `json:"isActive"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
}

type RuleListResponse struct {
	Rules []RuleListRow `json:"rules"`
}

// RulePopulationMember e una persona della platea risolta, con la copertura
// calcolata secondo la natura del bisogno.
type RulePopulationMember struct {
	EmployeeID string `json:"employeeId"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Covered    bool   `json:"covered"`
}

// RulePopulationDetail e la platea risolta della regola.
type RulePopulationDetail struct {
	Kind         string                 `json:"kind"`
	TargetID     string                 `json:"targetId,omitempty"`
	PersonIDs    []string               `json:"personIds,omitempty"` // solo kind people: righe registrate
	Size         int                    `json:"size"`
	CoveredCount int                    `json:"coveredCount"`
	Members      []RulePopulationMember `json:"members"`
}

// RuleSeatsDetail e il conteggio delle regole a posizioni: richieste e
// coperte secondo la natura del bisogno.
type RuleSeatsDetail struct {
	Requested int `json:"requested"`
	Covered   int `json:"covered"`
}

// RuleRoundRow e una tornata della regola: un evento con origin='rule'.
type RuleRoundRow struct {
	EventID          string `json:"eventId"`
	RuleDeadline     string `json:"ruleDeadline,omitempty"`
	Cancelled        bool   `json:"cancelled"`
	CancelledAt      string `json:"cancelledAt,omitempty"`
	EnrollmentsCount int    `json:"enrollmentsCount"` // iscrizioni non annullate
	CreatedAt        string `json:"createdAt"`
}

// RuleDetail e la regola completa: platea risolta con copertura per persona
// (o posizioni richieste/coperte) e tornate.
type RuleDetail struct {
	ID                   string                `json:"id"`
	Name                 string                `json:"name"`
	CourseID             string                `json:"courseId"`
	CourseTitle          string                `json:"courseTitle"`
	Need                 string                `json:"need"`
	CertificationID      string                `json:"certificationId,omitempty"`
	IsMandatory          bool                  `json:"isMandatory"`
	Deadline             string                `json:"deadline"`
	RecurrenceMonths     *int                  `json:"recurrenceMonths,omitempty"`
	RecurrenceAnchor     string                `json:"recurrenceAnchor,omitempty"`
	IsActive             bool                  `json:"isActive"`
	Notes                string                `json:"notes,omitempty"`
	Population           *RulePopulationDetail `json:"population,omitempty"`
	Seats                *RuleSeatsDetail      `json:"seats,omitempty"`
	NextRoundDeadline    string                `json:"nextRoundDeadline,omitempty"`
	CurrentRoundEventID  string                `json:"currentRoundEventId,omitempty"`
	CurrentRoundDeadline string                `json:"currentRoundDeadline,omitempty"`
	Rounds               []RuleRoundRow        `json:"rounds"`
	CreatedAt            string                `json:"createdAt"`
	UpdatedAt            string                `json:"updatedAt"`
}

// RuleEventResponse e la risposta del gesto "crea evento da regola":
// l'evento della nuova tornata e il conteggio delle iscrizioni generate.
type RuleEventResponse struct {
	OK                 bool   `json:"ok"`
	ID                 string `json:"id"`
	EnrollmentsCreated int    `json:"enrollmentsCreated"`
}

// FeedAddedRow e una persona aggiunta dal gesto "alimenta platea".
type FeedAddedRow struct {
	EnrollmentID string `json:"enrollmentId"`
	EmployeeID   string `json:"employeeId"`
	EmployeeName string `json:"employeeName"`
}

// FeedEventResponse e la risposta del gesto "alimenta platea": l'elenco
// delle persone aggiunte all'evento della tornata.
type FeedEventResponse struct {
	OK      bool           `json:"ok"`
	EventID string         `json:"eventId"`
	Added   []FeedAddedRow `json:"added"`
}
