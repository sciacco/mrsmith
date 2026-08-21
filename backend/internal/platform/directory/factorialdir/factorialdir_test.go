package factorialdir

import (
	"testing"
	"time"

	"github.com/sciacco/mrsmith/pkg/factorial"
)

func ptr[T any](value T) *T {
	return &value
}

func date(year int, month time.Month, day int) *factorial.Date {
	return &factorial.Date{Time: time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

func TestBuildSnapshotRehireKeepsActiveRecord(t *testing.T) {
	employees := []factorial.EmployeesEmployee{
		{
			ID:           ptr("1"),
			FirstName:    ptr("Mario"),
			LastName:     ptr("Rossi"),
			LoginEmail:   ptr("Mario.Rossi@example.com"),
			Active:       ptr(false),
			TerminatedOn: date(2024, time.June, 30),
		},
		{
			ID:         ptr("2"),
			FirstName:  ptr("Mario"),
			LastName:   ptr("Rossi"),
			LoginEmail: ptr("mario.rossi@example.com"),
			Active:     ptr(true),
		},
	}

	snapshot := buildSnapshot(employees, nil, time.Unix(0, 0))

	if len(snapshot.People) != 1 {
		t.Fatalf("expected one person, got %d (%+v)", len(snapshot.People), snapshot.People)
	}
	person := snapshot.People[0]
	if person.ExternalID != "2" || !person.Active || person.LoginEmail != "mario.rossi@example.com" {
		t.Fatalf("unexpected winner: %+v", person)
	}
	if snapshot.SkippedDuplicate != 1 {
		t.Fatalf("skippedDuplicate = %d, want 1", snapshot.SkippedDuplicate)
	}
}

func TestBuildSnapshotAllTerminatedKeepsMostRecent(t *testing.T) {
	employees := []factorial.EmployeesEmployee{
		{
			ID:           ptr("1"),
			FirstName:    ptr("Ada"),
			LastName:     ptr("Verdi"),
			LoginEmail:   ptr("ada.verdi@example.com"),
			Active:       ptr(false),
			TerminatedOn: date(2022, time.January, 31),
		},
		{
			ID:           ptr("2"),
			FirstName:    ptr("Ada"),
			LastName:     ptr("Verdi"),
			LoginEmail:   ptr("ada.verdi@example.com"),
			Active:       ptr(false),
			TerminatedOn: date(2026, time.February, 28),
		},
	}

	snapshot := buildSnapshot(employees, nil, time.Unix(0, 0))

	if len(snapshot.People) != 1 || snapshot.People[0].ExternalID != "2" {
		t.Fatalf("unexpected people: %+v", snapshot.People)
	}
	if snapshot.People[0].TerminatedOn == nil || snapshot.People[0].TerminatedOn.Year() != 2026 {
		t.Fatalf("unexpected termination: %+v", snapshot.People[0])
	}
}

func TestBuildSnapshotDropsActivePeopleWithoutLoginEmail(t *testing.T) {
	employees := []factorial.EmployeesEmployee{
		{ID: ptr("1"), FirstName: ptr("Senza"), LastName: ptr("Email"), Active: ptr(true)},
		{ID: ptr("2"), FirstName: ptr("Con"), LastName: ptr("Email"), LoginEmail: ptr("con.email@example.com"), Active: ptr(true)},
	}

	snapshot := buildSnapshot(employees, nil, time.Unix(0, 0))

	if len(snapshot.People) != 1 || snapshot.People[0].ExternalID != "2" {
		t.Fatalf("unexpected people: %+v", snapshot.People)
	}
	if snapshot.SkippedNoLoginEmail != 1 {
		t.Fatalf("skippedNoLoginEmail = %d, want 1", snapshot.SkippedNoLoginEmail)
	}
}

func TestBuildSnapshotFiltersMembershipToActivePeople(t *testing.T) {
	employees := []factorial.EmployeesEmployee{
		{ID: ptr("1"), FirstName: ptr("Attiva"), LastName: ptr("Persona"), LoginEmail: ptr("attiva@example.com"), Active: ptr(true)},
		{ID: ptr("2"), FirstName: ptr("Cessata"), LastName: ptr("Persona"), LoginEmail: ptr("cessata@example.com"), Active: ptr(false), TerminatedOn: date(2025, time.December, 31)},
	}
	teams := []factorial.TeamsTeam{{
		ID:          ptr("t-1"),
		Name:        ptr("Cloud"),
		EmployeeIDs: []string{"1", "2", "1"},
		LeadIDs:     []string{"2"},
	}}

	snapshot := buildSnapshot(employees, teams, time.Unix(0, 0))

	if len(snapshot.Teams) != 1 {
		t.Fatalf("expected one team, got %d", len(snapshot.Teams))
	}
	team := snapshot.Teams[0]
	if len(team.MemberExternalIDs) != 1 || team.MemberExternalIDs[0] != "1" {
		t.Fatalf("unexpected members: %+v", team.MemberExternalIDs)
	}
	if len(team.LeadExternalIDs) != 0 {
		t.Fatalf("terminated lead must be dropped: %+v", team.LeadExternalIDs)
	}
	if snapshot.SkippedMembership != 2 {
		t.Fatalf("skippedMembership = %d, want 2", snapshot.SkippedMembership)
	}
}
