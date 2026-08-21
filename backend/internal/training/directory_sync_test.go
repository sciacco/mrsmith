package training

import (
	"testing"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/directory"
)

func directoryActionsByType(actions []directorySyncAction, actionType string) []directorySyncAction {
	found := []directorySyncAction{}
	for _, action := range actions {
		if action.Type == actionType {
			found = append(found, action)
		}
	}
	return found
}

func directorySingleAction(t *testing.T, actions []directorySyncAction, actionType string) directorySyncAction {
	t.Helper()
	found := directoryActionsByType(actions, actionType)
	if len(found) != 1 {
		t.Fatalf("expected exactly one %s action, got %d (%+v)", actionType, len(found), actions)
	}
	return found[0]
}

func TestDiffDirectoryAdoptsPersonByEmail(t *testing.T) {
	snapshot := directory.Snapshot{People: []directory.Person{{
		ExternalID: "f-1",
		FirstName:  "Mario",
		LastName:   "Rossi",
		LoginEmail: "mario.rossi@example.com",
		Active:     true,
	}}}
	local := directoryLocalState{People: []directoryLocalPerson{{
		ID:        "employee-1",
		FirstName: "Mario",
		LastName:  "Rossi",
		Email:     "mario.rossi@example.com",
		Status:    "active",
	}}}

	actions := diffDirectory(snapshot, local)

	adopt := directorySingleAction(t, actions, directoryActionAdoptPerson)
	if adopt.PersonID != "employee-1" || adopt.PersonExternalID != "f-1" {
		t.Fatalf("unexpected adopt action: %+v", adopt)
	}
	if len(directoryActionsByType(actions, directoryActionCreatePerson)) != 0 {
		t.Fatalf("adopted person must not be created: %+v", actions)
	}
	if len(directoryActionsByType(actions, directoryActionUpdatePerson)) != 0 {
		t.Fatalf("unchanged person must not be updated: %+v", actions)
	}
}

func TestDiffDirectoryCreatesUnknownActivePersonAndSkipsTerminated(t *testing.T) {
	snapshot := directory.Snapshot{People: []directory.Person{
		{ExternalID: "f-1", FirstName: "Ada", LastName: "Verdi", LoginEmail: "ada.verdi@example.com", Active: true},
		{ExternalID: "f-2", FirstName: "Luca", LastName: "Neri", LoginEmail: "luca.neri@example.com", Active: false},
	}}

	actions := diffDirectory(snapshot, directoryLocalState{})

	create := directorySingleAction(t, actions, directoryActionCreatePerson)
	if create.Email != "ada.verdi@example.com" || create.Status != "active" {
		t.Fatalf("unexpected create action: %+v", create)
	}
}

func TestDiffDirectoryPropagatesTermination(t *testing.T) {
	terminatedOn := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	snapshot := directory.Snapshot{People: []directory.Person{{
		ExternalID:   "f-1",
		FirstName:    "Mario",
		LastName:     "Rossi",
		LoginEmail:   "mario.rossi@example.com",
		Active:       false,
		TerminatedOn: &terminatedOn,
	}}}
	local := directoryLocalState{People: []directoryLocalPerson{
		{
			ID:         "employee-1",
			ExternalID: "f-1",
			FirstName:  "Mario",
			LastName:   "Rossi",
			Email:      "mario.rossi@example.com",
			Status:     "active",
		},
		{
			ID:         "employee-2",
			ExternalID: "f-9",
			FirstName:  "Ada",
			LastName:   "Verdi",
			Email:      "ada.verdi@example.com",
			Status:     "active",
		},
	}}

	actions := diffDirectory(snapshot, local)

	terminations := directoryActionsByType(actions, directoryActionTerminatePerson)
	if len(terminations) != 2 {
		t.Fatalf("expected two terminations, got %d (%+v)", len(terminations), actions)
	}
	if terminations[0].PersonID != "employee-1" || terminations[0].TerminatedOn == nil || !terminations[0].TerminatedOn.Equal(terminatedOn) {
		t.Fatalf("unexpected source termination: %+v", terminations[0])
	}
	if terminations[1].PersonID != "employee-2" || terminations[1].TerminatedOn != nil {
		t.Fatalf("unexpected absent-from-source termination: %+v", terminations[1])
	}
}

func TestDiffDirectoryLeavesManualPersonUntouched(t *testing.T) {
	local := directoryLocalState{People: []directoryLocalPerson{{
		ID:        "employee-manual",
		FirstName: "Consulente",
		LastName:  "Esterno",
		Email:     "consulente@example.com",
		Status:    "active",
	}}}

	actions := diffDirectory(directory.Snapshot{}, local)

	if len(actions) != 0 {
		t.Fatalf("manual person must not produce actions: %+v", actions)
	}
}

func TestDiffDirectoryAdoptsTeamByName(t *testing.T) {
	snapshot := directory.Snapshot{
		People: []directory.Person{{
			ExternalID: "f-1",
			FirstName:  "Mario",
			LastName:   "Rossi",
			LoginEmail: "mario.rossi@example.com",
			Active:     true,
		}},
		Teams: []directory.Team{
			{ExternalID: "t-1", Name: "Cloud Operations", MemberExternalIDs: []string{"f-1"}},
			{ExternalID: "t-2", Name: "Security"},
		},
	}
	local := directoryLocalState{
		People: []directoryLocalPerson{{
			ID:         "employee-1",
			ExternalID: "f-1",
			FirstName:  "Mario",
			LastName:   "Rossi",
			Email:      "mario.rossi@example.com",
			Status:     "active",
		}},
		Teams: []directoryLocalTeam{{ID: "team-cloud", Code: "CLOUD", Name: "cloud operations"}},
	}

	actions := diffDirectory(snapshot, local)

	adopt := directorySingleAction(t, actions, directoryActionAdoptTeam)
	if adopt.TeamID != "team-cloud" || adopt.TeamExternalID != "t-1" {
		t.Fatalf("unexpected team adoption: %+v", adopt)
	}
	rename := directorySingleAction(t, actions, directoryActionRenameTeam)
	if rename.TeamName != "Cloud Operations" {
		t.Fatalf("unexpected rename: %+v", rename)
	}
	create := directorySingleAction(t, actions, directoryActionCreateTeam)
	if create.TeamExternalID != "t-2" || create.TeamCode != "SECURITY" {
		t.Fatalf("unexpected team creation: %+v", create)
	}
	open := directorySingleAction(t, actions, directoryActionOpenMembership)
	if open.PersonID != "employee-1" || open.TeamID != "team-cloud" || open.Lead {
		t.Fatalf("unexpected membership: %+v", open)
	}
}

func TestDiffDirectoryMovesLead(t *testing.T) {
	snapshot := directory.Snapshot{
		People: []directory.Person{{
			ExternalID: "f-1",
			FirstName:  "Mario",
			LastName:   "Rossi",
			LoginEmail: "mario.rossi@example.com",
			Active:     true,
		}},
		Teams: []directory.Team{
			{ExternalID: "t-1", Name: "Cloud", MemberExternalIDs: []string{"f-1"}},
			{ExternalID: "t-2", Name: "Security", MemberExternalIDs: []string{"f-1"}, LeadExternalIDs: []string{"f-1"}},
		},
	}
	local := directoryLocalState{
		People: []directoryLocalPerson{{
			ID:         "employee-1",
			ExternalID: "f-1",
			FirstName:  "Mario",
			LastName:   "Rossi",
			Email:      "mario.rossi@example.com",
			Status:     "active",
			Memberships: []directoryLocalMembership{
				{ID: "membership-cloud", TeamID: "team-cloud", Lead: true},
				{ID: "membership-sec", TeamID: "team-sec"},
			},
		}},
		Teams: []directoryLocalTeam{
			{ID: "team-cloud", ExternalID: "t-1", Code: "CLOUD", Name: "Cloud"},
			{ID: "team-sec", ExternalID: "t-2", Code: "SEC", Name: "Security"},
		},
	}

	actions := diffDirectory(snapshot, local)

	unset := directorySingleAction(t, actions, directoryActionUnsetLead)
	if unset.MembershipID != "membership-cloud" || unset.Lead {
		t.Fatalf("unexpected unset lead: %+v", unset)
	}
	set := directorySingleAction(t, actions, directoryActionSetLead)
	if set.MembershipID != "membership-sec" || !set.Lead {
		t.Fatalf("unexpected set lead: %+v", set)
	}
	if len(directoryActionsByType(actions, directoryActionCloseMembership)) != 0 {
		t.Fatalf("memberships still desired must stay open: %+v", actions)
	}
}

func TestDiffDirectoryClosesMembershipDroppedBySource(t *testing.T) {
	snapshot := directory.Snapshot{
		People: []directory.Person{{
			ExternalID: "f-1",
			FirstName:  "Mario",
			LastName:   "Rossi",
			LoginEmail: "mario.rossi@example.com",
			Active:     true,
		}},
		Teams: []directory.Team{{ExternalID: "t-1", Name: "Cloud"}},
	}
	local := directoryLocalState{
		People: []directoryLocalPerson{{
			ID:         "employee-1",
			ExternalID: "f-1",
			FirstName:  "Mario",
			LastName:   "Rossi",
			Email:      "mario.rossi@example.com",
			Status:     "active",
			Memberships: []directoryLocalMembership{
				{ID: "membership-cloud", TeamID: "team-cloud"},
				{ID: "membership-manual", TeamID: "team-manual"},
			},
		}},
		Teams: []directoryLocalTeam{
			{ID: "team-cloud", ExternalID: "t-1", Code: "CLOUD", Name: "Cloud"},
			{ID: "team-manual", Code: "MANUAL", Name: "Progetto interno"},
		},
	}

	actions := diffDirectory(snapshot, local)

	closed := directorySingleAction(t, actions, directoryActionCloseMembership)
	if closed.MembershipID != "membership-cloud" {
		t.Fatalf("only directory-managed memberships may be closed: %+v", actions)
	}
}

func TestDiffDirectoryUpdatesLoginEmailMatchedByExternalID(t *testing.T) {
	snapshot := directory.Snapshot{People: []directory.Person{{
		ExternalID: "f-1",
		FirstName:  "Ada",
		LastName:   "Verdi",
		LoginEmail: "ada.verdi@example.com",
		Active:     true,
	}}}
	local := directoryLocalState{People: []directoryLocalPerson{
		{
			ID:         "employee-1",
			ExternalID: "f-1",
			FirstName:  "Ada",
			LastName:   "Bianchi",
			Email:      "ada.bianchi@example.com",
			Status:     "active",
		},
		{
			ID:        "employee-omonima",
			FirstName: "Ada",
			LastName:  "Verdi",
			Email:     "ada.verdi@example.com",
			Status:    "active",
		},
	}}

	actions := diffDirectory(snapshot, local)

	update := directorySingleAction(t, actions, directoryActionUpdatePerson)
	if update.PersonID != "employee-1" {
		t.Fatalf("external id match must win over email match: %+v", update)
	}
	if update.Email != "ada.verdi@example.com" || update.LastName != "Verdi" {
		t.Fatalf("unexpected update payload: %+v", update)
	}
	if len(directoryActionsByType(actions, directoryActionAdoptPerson)) != 0 {
		t.Fatalf("record already matched by external id must not be adopted: %+v", actions)
	}
	if len(directoryActionsByType(actions, directoryActionTerminatePerson)) != 0 {
		t.Fatalf("manual homonym must stay untouched: %+v", actions)
	}
}

func TestDiffDirectoryIgnoresExemptPersonWithTerminatedSource(t *testing.T) {
	terminatedOn := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	snapshot := directory.Snapshot{
		People: []directory.Person{{
			ExternalID:   "f-1",
			FirstName:    "Andrea",
			LastName:     "Maldotti",
			LoginEmail:   "andrea.maldotti@example.com",
			Active:       false,
			TerminatedOn: &terminatedOn,
		}},
		Teams: []directory.Team{{ExternalID: "t-1", Name: "MS"}},
	}
	local := directoryLocalState{
		People: []directoryLocalPerson{{
			ID:          "employee-1",
			ExternalID:  "f-1",
			Exempt:      true,
			FirstName:   "Andrea",
			LastName:    "Maldotti",
			Email:       "andrea.maldotti@example.com",
			Status:      "active",
			Memberships: []directoryLocalMembership{{ID: "m-1", TeamID: "team-1"}},
		}},
		Teams: []directoryLocalTeam{{ID: "team-1", ExternalID: "t-1", Code: "MS", Name: "MS"}},
	}

	actions := diffDirectory(snapshot, local)

	if len(actions) != 0 {
		t.Fatalf("exempt person must produce no actions, got %+v", actions)
	}
}

func TestDiffDirectoryIgnoresExemptPersonWithActiveSource(t *testing.T) {
	snapshot := directory.Snapshot{
		People: []directory.Person{{
			ExternalID: "f-1",
			FirstName:  "Maria",
			LastName:   "Verdi",
			LoginEmail: "maria.verdi@example.com",
			Active:     true,
		}},
		Teams: []directory.Team{{
			ExternalID:        "t-1",
			Name:              "MS",
			MemberExternalIDs: []string{"f-1"},
		}},
	}
	local := directoryLocalState{
		People: []directoryLocalPerson{{
			ID:        "employee-1",
			Exempt:    true,
			FirstName: "Maria",
			LastName:  "Verdi Bianchi",
			Email:     "maria.verdi@example.com",
			Status:    "active",
		}},
		Teams: []directoryLocalTeam{{ID: "team-1", ExternalID: "t-1", Code: "MS", Name: "MS"}},
	}

	actions := diffDirectory(snapshot, local)

	if len(directoryActionsByType(actions, directoryActionCreatePerson)) != 0 {
		t.Fatalf("exempt person matched by email must suppress create_person: %+v", actions)
	}
	if len(actions) != 0 {
		t.Fatalf("exempt person must produce no actions, got %+v", actions)
	}
}
