package training

// I due prospetti di report ratificati (#163, slice 4 del task 7): consuntivo
// economico per anno di competenza RDA ed erogato per periodo sulle date
// proprie del fatto. Nessuna tabella propria: entrambi sono proiezioni sui
// fatti gia persistiti altrove (expense/PO vivo, iscrizioni/sessioni).

// EconomicReportRow e una voce di spesa con il suo PO vivo (#163 punto 1).
// L'attribuzione all'anno di competenza segue budgetYear del PO, mai le date
// dell'evento (decisione #159). Un errore nella risoluzione del PO non
// blocca il prospetto: la riga resta con i soli campi locali e poError
// valorizzato, i campi vivi restano vuoti.
type EconomicReportRow struct {
	ExpenseID          string        `json:"expenseId"`
	EventID            string        `json:"eventId"`
	CourseTitle        string        `json:"courseTitle"`
	EventCancelled     bool          `json:"eventCancelled"`
	CreatedAt          string        `json:"createdAt"`
	CoveredEnrollments int           `json:"coveredEnrollments"`
	POID               int64         `json:"poId"`
	POCode             string        `json:"poCode,omitempty"`
	Amount             string        `json:"amount,omitempty"`
	Currency           string        `json:"currency,omitempty"`
	EconomicState      EconomicState `json:"economicState,omitempty"`
	BudgetName         string        `json:"budgetName,omitempty"`
	BudgetYear         int           `json:"budgetYear,omitempty"`
	POError            string        `json:"poError,omitempty"`
}

type EconomicReportResponse struct {
	Rows []EconomicReportRow `json:"rows"`
}

// economicReportLocal e la riga locale prima della risoluzione del PO vivo.
type economicReportLocal struct {
	ExpenseID          string
	EventID            string
	POID               int64
	CourseTitle        string
	EventCancelled     bool
	CreatedAt          string
	CoveredEnrollments int
}

// DeliveredReportRow e un'iscrizione conclusa (completed/partially_completed,
// evento non annullato) con data e ore di riferimento del fatto (#163 punto
// 2). Teams sono le appartenenze attive correnti della persona, non quelle
// alla data del fatto: nel raggruppamento per team la persona conta in
// ciascuno dei suoi team.
type DeliveredReportRow struct {
	EnrollmentID    string          `json:"enrollmentId"`
	EmployeeID      string          `json:"employeeId"`
	EmployeeName    string          `json:"employeeName"`
	Teams           []PersonTeamRef `json:"teams"`
	CourseID        string          `json:"courseId"`
	CourseTitle     string          `json:"courseTitle"`
	SkillAreaNames  []string        `json:"skillAreaNames"`
	EventID         string          `json:"eventId"`
	DeliveryStatus  string          `json:"deliveryStatus"`
	LearningOutcome string          `json:"learningOutcome,omitempty"`
	ReferenceDate   string          `json:"referenceDate"`
	Hours           *int            `json:"hours,omitempty"`
}

type DeliveredReportResponse struct {
	From string               `json:"from"`
	To   string               `json:"to"`
	Rows []DeliveredReportRow `json:"rows"`
}
