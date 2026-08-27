package training

// Valutazioni di competenza per area (#161, slice 2 del task 7).

type AssessmentInput struct {
	SkillAreaID string `json:"skillAreaId"`
	Level       *int   `json:"level"`
	AssessedOn  string `json:"assessedOn,omitempty"`
	Source      string `json:"source,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

type AssessmentUpdateInput struct {
	Level      *int   `json:"level"`
	AssessedOn string `json:"assessedOn"`
	Source     string `json:"source,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

// PersonAssessmentRef e una valutazione nello storico della scheda persona.
type PersonAssessmentRef struct {
	ID            string `json:"id"`
	SkillAreaID   string `json:"skillAreaId"`
	SkillAreaName string `json:"skillAreaName"`
	Level         int    `json:"level"`
	AssessedOn    string `json:"assessedOn"`
	Source        string `json:"source"`
	Notes         string `json:"notes,omitempty"`
}
