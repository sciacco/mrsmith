package training

// Tipi delle code operative derivate (#140, §4-Code). Le code sono in sola
// lettura: query sui fatti del dominio, nessuna tabella di to-do e nessuna
// scadenza automatica.

// RequestAwaitingDecisionRow e una richiesta aperta senza decisione People
// (coda 2): con o senza team scelto e con o senza parere TL, esposto quando
// presente (#200).
type RequestAwaitingDecisionRow struct {
	RequestID        string `json:"requestId"`
	EmployeeID       string `json:"employeeId"`
	EmployeeName     string `json:"employeeName"`
	SelectedTeamID   string `json:"selectedTeamId,omitempty"`
	SelectedTeamName string `json:"selectedTeamName,omitempty"`
	CourseID         string `json:"courseId,omitempty"`
	CourseTitle      string `json:"courseTitle,omitempty"`
	TLOpinion        string `json:"tlOpinion,omitempty"`
	TLOpinionByID    string `json:"tlOpinionById,omitempty"`
	TLOpinionByName  string `json:"tlOpinionByName,omitempty"`
	TLOpinionAt      string `json:"tlOpinionAt,omitempty"`
	TLOpinionReason  string `json:"tlOpinionReason,omitempty"`
	AgeDays          int    `json:"ageDays"`
	CreatedAt        string `json:"createdAt"`
}

type RequestsAwaitingDecisionResponse struct {
	Requests []RequestAwaitingDecisionRow `json:"requests"`
}

// SeatRuleInTrainingRow e un'iscrizione collegata alla regola a posizioni e
// non conclusa (planned/in_progress): mostra chi si sta formando; la
// copertura arriva solo con il fatto previsto dalla natura del bisogno.
type SeatRuleInTrainingRow struct {
	EnrollmentID   string `json:"enrollmentId"`
	EmployeeID     string `json:"employeeId"`
	EmployeeName   string `json:"employeeName"`
	EventID        string `json:"eventId"`
	DeliveryStatus string `json:"deliveryStatus"`
}

// SeatRuleCoverageRow e lo stato di una regola attiva a posizioni (coda 3):
// posizioni richieste, coperte secondo la natura del bisogno, mancanti e
// persone in formazione.
type SeatRuleCoverageRow struct {
	RuleID          string                  `json:"ruleId"`
	RuleName        string                  `json:"ruleName"`
	CourseID        string                  `json:"courseId"`
	CourseTitle     string                  `json:"courseTitle"`
	Need            string                  `json:"need"` // attendance|certification
	CertificationID string                  `json:"certificationId,omitempty"`
	Deadline        string                  `json:"deadline"`
	SeatCount       int                     `json:"seatCount"`
	Covered         int                     `json:"covered"`
	Missing         int                     `json:"missing"`
	InTraining      []SeatRuleInTrainingRow `json:"inTraining"`
}

type SeatRuleCoverageResponse struct {
	Rules []SeatRuleCoverageRow `json:"rules"`
}

// ExpiringPersonRow e una persona con la copertura in scadenza o mancante
// entro l'orizzonte (coda 4): regola, persona, scadenza rilevante e giorni
// (negativi = scadenza gia superata).
type ExpiringPersonRow struct {
	RuleID       string `json:"ruleId"`
	RuleName     string `json:"ruleName"`
	CourseID     string `json:"courseId"`
	CourseTitle  string `json:"courseTitle"`
	Need         string `json:"need"` // attendance|certification
	EmployeeID   string `json:"employeeId"`
	EmployeeName string `json:"employeeName"`
	Deadline     string `json:"deadline"`
	DaysUntil    int    `json:"daysUntil"`
	// Reason qualifica la riga: uncovered|never_completed|personal_deadline
	// (frequenza), award_expiring|award_expired|never_awarded (certificazione).
	Reason string `json:"reason"`
}

// ExpiringSeatRuleRow e una regola a posizioni con bisogno di certificazione
// il cui conteggio valido scenderebbe sotto le posizioni richieste entro
// l'orizzonte (coda 4): valide oggi, valide a orizzonte e soglia.
type ExpiringSeatRuleRow struct {
	RuleID          string `json:"ruleId"`
	RuleName        string `json:"ruleName"`
	CourseID        string `json:"courseId"`
	CourseTitle     string `json:"courseTitle"`
	CertificationID string `json:"certificationId"`
	SeatCount       int    `json:"seatCount"`
	ValidToday      int    `json:"validToday"`
	ValidAtHorizon  int    `json:"validAtHorizon"`
}

type ExpiringCoverageResponse struct {
	WithinDays int                   `json:"withinDays"`
	People     []ExpiringPersonRow   `json:"people"`
	SeatRules  []ExpiringSeatRuleRow `json:"seatRules"`
}

// QueueMemberRef e un membro della platea citato da una coda.
type QueueMemberRef struct {
	EmployeeID string `json:"employeeId"`
	Name       string `json:"name"`
}

// UnfedPopulationRow e una regola a platea con tornata in corso e membri
// attuali della platea non coperti e senza alcuna iscrizione, anche
// annullata, sull'evento della tornata (coda 5): i candidati che il gesto di
// alimentazione aggiungerebbe davvero.
type UnfedPopulationRow struct {
	RuleID        string           `json:"ruleId"`
	RuleName      string           `json:"ruleName"`
	CourseID      string           `json:"courseId"`
	CourseTitle   string           `json:"courseTitle"`
	EventID       string           `json:"eventId"`
	RoundDeadline string           `json:"roundDeadline"`
	Members       []QueueMemberRef `json:"members"`
}

type UnfedPopulationResponse struct {
	Rules []UnfedPopulationRow `json:"rules"`
}

// RoundWithoutEventRow e una regola attiva (ancora di calendario o senza
// ricorrenza) la cui prossima tornata cade entro l'orizzonte e non ha ancora
// un evento non annullato con quella scadenza (coda 6).
type RoundWithoutEventRow struct {
	RuleID            string `json:"ruleId"`
	RuleName          string `json:"ruleName"`
	CourseID          string `json:"courseId"`
	CourseTitle       string `json:"courseTitle"`
	Need              string `json:"need"`
	RecurrenceMonths  *int   `json:"recurrenceMonths,omitempty"`
	RecurrenceAnchor  string `json:"recurrenceAnchor,omitempty"`
	NextRoundDeadline string `json:"nextRoundDeadline"`
	DaysUntil         int    `json:"daysUntil"`
	FirstRound        bool   `json:"firstRound"` // nessuna tornata non annullata registrata
}

type RoundsWithoutEventResponse struct {
	WithinDays int                    `json:"withinDays"`
	Rules      []RoundWithoutEventRow `json:"rules"`
}

// UnapprovedEventExpenseRow is a local event expense whose current Arak PO is
// not economically approved. Economic fields are hydrated at request time.
type UnapprovedEventExpenseRow struct {
	EventID         string             `json:"eventId"`
	CourseID        string             `json:"courseId"`
	CourseTitle     string             `json:"courseTitle"`
	ExpenseID       string             `json:"expenseId"`
	POID            int64              `json:"poId"`
	POCode          string             `json:"poCode"`
	RawState        string             `json:"rawState"`
	EconomicState   EconomicState      `json:"economicState"`
	TotalPrice      string             `json:"totalPrice"`
	Currency        string             `json:"currency"`
	Budget          EventExpenseBudget `json:"budget"`
	EnrollmentCount int                `json:"enrollmentCount"`
}

type UnapprovedEventExpensesResponse struct {
	Expenses []UnapprovedEventExpenseRow `json:"expenses"`
}

type unapprovedEventExpenseLocal struct {
	Expense         eventExpenseLocal
	CourseID        string
	CourseTitle     string
	EnrollmentCount int
}

// StaleEnrollmentRow e un'iscrizione ferma (coda 7, #152): pianificata
// (delivery_status planned), su un evento non annullato, piu vecchia della
// soglia e senza un'assegnazione a una sessione con data futura.
// LastSessionDate e nil quando non esiste alcuna sessione trascorsa.
type StaleEnrollmentRow struct {
	EnrollmentID    string  `json:"enrollmentId"`
	EmployeeID      string  `json:"employeeId"`
	EmployeeName    string  `json:"employeeName"`
	EventID         string  `json:"eventId"`
	CourseTitle     string  `json:"courseTitle"`
	AgeDays         int     `json:"ageDays"`
	CreatedAt       string  `json:"createdAt"`
	LastSessionDate *string `json:"lastSessionDate"`
}

type StaleEnrollmentsResponse struct {
	OlderThanDays int                  `json:"olderThanDays"`
	Enrollments   []StaleEnrollmentRow `json:"enrollments"`
}

// ExpiringCertificationQueueRow e un conseguimento passed_exam di una
// persona attiva la cui scadenza cade entro l'orizzonte (coda 8, #160,
// slice 1 del task 7): wrapper su training.v_expiring_certifications con
// l'identificativo del conseguimento.
type ExpiringCertificationQueueRow struct {
	AwardID           string `json:"awardId"`
	EmployeeID        string `json:"employeeId"`
	EmployeeName      string `json:"employeeName"`
	EmployeeEmail     string `json:"employeeEmail"`
	CertificationCode string `json:"certificationCode"`
	CertificationName string `json:"certificationName"`
	ExpiresOn         string `json:"expiresOn"`
	DaysToExpiry      int    `json:"daysToExpiry"`
}

type ExpiringCertificationsResponse struct {
	WithinDays     int                             `json:"withinDays"`
	Certifications []ExpiringCertificationQueueRow `json:"certifications"`
}
