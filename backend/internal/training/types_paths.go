package training

// Percorsi formativi e progresso calcolato (#161, slice 2 del task 7): CRUD
// dei percorsi/passi, assegnazione persona-percorso, progresso letto dai
// fatti in lettura (nessuna persistenza, decisioni #159 punto 8).

type PathInput struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	SkillAreaID string `json:"skillAreaId,omitempty"`
	Description string `json:"description,omitempty"`
	Active      *bool  `json:"active,omitempty"`
}

type PathListRow struct {
	ID             string `json:"id"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	SkillAreaID    string `json:"skillAreaId,omitempty"`
	SkillAreaName  string `json:"skillAreaName,omitempty"`
	Description    string `json:"description,omitempty"`
	Active         bool   `json:"active"`
	StepsCount     int    `json:"stepsCount"`
	AssigneesCount int    `json:"assigneesCount"`
}

type PathListResponse struct {
	Paths []PathListRow `json:"paths"`
}

// PathStepRef e la definizione di un passo: corso XOR certificazione (il
// CHECK di 012 richiede almeno uno, la slice impone esattamente uno).
type PathStepRef struct {
	StepID            string `json:"stepId"`
	StepOrder         int    `json:"stepOrder"`
	CourseID          string `json:"courseId,omitempty"`
	CourseTitle       string `json:"courseTitle,omitempty"`
	CertificationID   string `json:"certificationId,omitempty"`
	CertificationName string `json:"certificationName,omitempty"`
	IsRequired        bool   `json:"isRequired"`
	Notes             string `json:"notes,omitempty"`
}

type PathStepInput struct {
	StepOrder       int    `json:"stepOrder"`
	CourseID        string `json:"courseId,omitempty"`
	CertificationID string `json:"certificationId,omitempty"`
	IsRequired      *bool  `json:"isRequired,omitempty"`
	Notes           string `json:"notes,omitempty"`
}

type PathStepsInput struct {
	Steps []PathStepInput `json:"steps"`
}

// PathStepProgressRef e lo stato di copertura di un passo per una persona
// (#161, decisioni #159 punto 8): il riferimento e l'iscrizione/evento piu
// recente che copre per i corsi, il conseguimento per le certificazioni.
type PathStepProgressRef struct {
	StepID            string `json:"stepId"`
	StepOrder         int    `json:"stepOrder"`
	CourseID          string `json:"courseId,omitempty"`
	CourseTitle       string `json:"courseTitle,omitempty"`
	CertificationID   string `json:"certificationId,omitempty"`
	CertificationName string `json:"certificationName,omitempty"`
	IsRequired        bool   `json:"isRequired"`
	Covered           bool   `json:"covered"`
	EnrollmentID      string `json:"enrollmentId,omitempty"`
	EventID           string `json:"eventId,omitempty"`
	AwardID           string `json:"awardId,omitempty"`
}

// PathProgress e il progresso calcolato di un'assegnazione sui passi del
// suo percorso (#161, decisioni #159 punto 8): condiviso da PathAssigneeRef
// (vista dal percorso) e PersonPathRef (vista dalla scheda persona).
type PathProgress struct {
	Steps              []PathStepProgressRef `json:"steps"`
	RequiredTotal      int                   `json:"requiredTotal"`
	RequiredCovered    int                   `json:"requiredCovered"`
	AllRequiredCovered bool                  `json:"allRequiredCovered"`
}

// PathAssigneeRef e un'assegnazione persona-percorso vista dal dettaglio
// percorso, con il progresso calcolato.
type PathAssigneeRef struct {
	EmployeeID       string `json:"employeeId"`
	EmployeeName     string `json:"employeeName"`
	StartedOn        string `json:"startedOn"`
	TargetCompletion string `json:"targetCompletion,omitempty"`
	CompletedOn      string `json:"completedOn,omitempty"`
	Notes            string `json:"notes,omitempty"`
	PathProgress
}

type PathDetail struct {
	PathListRow
	Steps     []PathStepRef     `json:"steps"`
	Assignees []PathAssigneeRef `json:"assignees"`
}

// PersonPathRef e un'assegnazione percorso vista dalla scheda persona,
// stessa forma di progresso di PathAssigneeRef.
type PersonPathRef struct {
	PathID           string `json:"pathId"`
	PathName         string `json:"pathName"`
	StartedOn        string `json:"startedOn"`
	TargetCompletion string `json:"targetCompletion,omitempty"`
	CompletedOn      string `json:"completedOn,omitempty"`
	Notes            string `json:"notes,omitempty"`
	PathProgress
}

type PathAssignmentInput struct {
	PathID           string `json:"pathId"`
	StartedOn        string `json:"startedOn,omitempty"`
	TargetCompletion string `json:"targetCompletion,omitempty"`
	Notes            string `json:"notes,omitempty"`
}

// PathAssignmentUpdateInput e una sostituzione completa dei fatti
// dell'assegnazione (stesso stile PUT di AwardUpdateInput/EnrollmentFacts):
// CompletedOn vuoto azzera la conclusione registrata.
type PathAssignmentUpdateInput struct {
	StartedOn        string `json:"startedOn"`
	TargetCompletion string `json:"targetCompletion,omitempty"`
	CompletedOn      string `json:"completedOn,omitempty"`
	Notes            string `json:"notes,omitempty"`
}
