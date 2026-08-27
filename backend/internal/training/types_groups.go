package training

// Gruppi locali (#152, slice 1 del task 6): CRUD e vincoli di platea. Un
// gruppo non si puo eliminare quando e platea di una regola attiva o
// collegato a un'area di competenza (migrazione 131).

// GroupMemberRef e un membro del gruppo locale.
type GroupMemberRef struct {
	EmployeeID string `json:"employeeId"`
	Name       string `json:"name"`
	Email      string `json:"email"`
}

type GroupListRow struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Members     []GroupMemberRef `json:"members"`
}

type GroupListResponse struct {
	Groups []GroupListRow `json:"groups"`
}

// GroupInput copre creazione e modifica del gruppo: solo nome e
// descrizione, come da contratto di slice.
type GroupInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// GroupMembersInput e la sostituzione atomica dell'insieme dei membri.
type GroupMembersInput struct {
	EmployeeIDs []string `json:"employeeIds"`
}
