package training

import (
	"testing"

	"github.com/sciacco/mrsmith/internal/platform/directory"
	"github.com/sciacco/mrsmith/pkg/factorial"
)

func TestActiveEmployeeIDs(t *testing.T) {
	snapshot := directory.Snapshot{
		People: []directory.Person{
			{ExternalID: "emp-active", Active: true},
			{ExternalID: "emp-terminated", Active: false},
			{ExternalID: "emp-active", Active: true}, // duplicato: l'insieme resta deduplicato
		},
	}
	got := activeEmployeeIDs(snapshot)
	if _, ok := got["emp-active"]; !ok {
		t.Fatal(`"emp-active" attivo assente dall'insieme`)
	}
	if _, ok := got["emp-terminated"]; ok {
		t.Fatal(`"emp-terminated" cessato presente nell'insieme`)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (duplicato deduplicato)", len(got))
	}
}

func TestSelectTrainingIDs(t *testing.T) {
	activeIDs := map[string]struct{}{"emp-active": {}}
	linkedTrainingIDs := map[string]struct{}{"t-linked": {}}
	graph := factorialTrainingGraph{
		Trainings: []factorial.TrainingsTraining{
			{ID: ptr("t-new-no-active")},
			{ID: ptr("t-new-active")},
			{ID: ptr("t-linked")},
		},
		TrainingMemberships: []factorial.TrainingsTrainingMembership{
			{TrainingID: ptr("t-new-active"), EmployeeID: ptr("emp-active")},
			{TrainingID: ptr("t-new-no-active"), EmployeeID: ptr("emp-terminated")},
		},
	}

	got := selectTrainingIDs(graph, activeIDs, linkedTrainingIDs)

	cases := []struct {
		name string
		id   string
		want bool
	}{
		{"nuovo senza attivi: fuori", "t-new-no-active", false},
		{"nuovo con un attivo: dentro", "t-new-active", true},
		{"gia' collegato senza attivi: dentro", "t-linked", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := got[tc.id]
			if ok != tc.want {
				t.Fatalf("selectTrainingIDs()[%q] presente = %v, want %v", tc.id, ok, tc.want)
			}
		})
	}
}

func TestSelectTrainingClasses(t *testing.T) {
	activeIDs := map[string]struct{}{"emp-active": {}}

	t.Run("classe senza sessioni: tenuta", func(t *testing.T) {
		graph := factorialTrainingGraph{
			TrainingClasses: []factorial.TrainingsTrainingClass{
				{ID: ptr("c-no-sessions"), TrainingID: ptr("t-1")},
			},
		}
		got := selectTrainingClasses("t-1", graph, activeIDs)
		if len(got.Classes) != 1 {
			t.Fatalf("Classes = %d, want 1 (tenuta)", len(got.Classes))
		}
	})

	t.Run("classe con roster vuoto: tenuta", func(t *testing.T) {
		graph := factorialTrainingGraph{
			TrainingClasses: []factorial.TrainingsTrainingClass{
				{ID: ptr("c-empty-roster"), TrainingID: ptr("t-1")},
			},
			Sessions: []factorial.TrainingsSession{
				{ID: ptr("s-1"), TrainingID: ptr("t-1"), TrainingClassID: ptr("c-empty-roster")},
			},
		}
		got := selectTrainingClasses("t-1", graph, activeIDs)
		if len(got.Classes) != 1 {
			t.Fatalf("Classes = %d, want 1 (tenuta)", len(got.Classes))
		}
		if len(got.Sessions) != 1 {
			t.Fatalf("Sessions = %d, want 1", len(got.Sessions))
		}
	})

	t.Run("tutte le assegnazioni inattive: scartata", func(t *testing.T) {
		graph := factorialTrainingGraph{
			TrainingClasses: []factorial.TrainingsTrainingClass{
				{ID: ptr("c-all-inactive"), TrainingID: ptr("t-1")},
			},
			Sessions: []factorial.TrainingsSession{
				{ID: ptr("s-1"), TrainingID: ptr("t-1"), TrainingClassID: ptr("c-all-inactive")},
			},
			SessionAccessMemberships: []factorial.TrainingsSessionAccessMembership{
				{ID: ptr("a-1"), SessionID: ptr("s-1"), EmployeeID: ptr("emp-terminated")},
			},
		}
		got := selectTrainingClasses("t-1", graph, activeIDs)
		if len(got.Classes) != 0 {
			t.Fatalf("Classes = %d, want 0 (scartata)", len(got.Classes))
		}
	})

	t.Run("mista: tenuta con filtro su membership/access/attendance", func(t *testing.T) {
		graph := factorialTrainingGraph{
			TrainingClasses: []factorial.TrainingsTrainingClass{
				{ID: ptr("c-mixed"), TrainingID: ptr("t-1")},
			},
			Sessions: []factorial.TrainingsSession{
				{ID: ptr("s-1"), TrainingID: ptr("t-1"), TrainingClassID: ptr("c-mixed")},
			},
			TrainingMemberships: []factorial.TrainingsTrainingMembership{
				{ID: ptr("m-active"), TrainingID: ptr("t-1"), EmployeeID: ptr("emp-active")},
				{ID: ptr("m-inactive"), TrainingID: ptr("t-1"), EmployeeID: ptr("emp-terminated")},
			},
			SessionAccessMemberships: []factorial.TrainingsSessionAccessMembership{
				{ID: ptr("a-active"), SessionID: ptr("s-1"), EmployeeID: ptr("emp-active")},
				{ID: ptr("a-inactive"), SessionID: ptr("s-1"), EmployeeID: ptr("emp-terminated")},
			},
			SessionAttendances: []factorial.TrainingsSessionAttendance{
				{ID: ptr("att-active"), SessionAccessMembershipID: ptr("a-active"), EmployeeID: ptr("emp-active")},
				{ID: ptr("att-inactive"), SessionAccessMembershipID: ptr("a-inactive"), EmployeeID: ptr("emp-terminated")},
			},
		}
		got := selectTrainingClasses("t-1", graph, activeIDs)
		if len(got.Classes) != 1 {
			t.Fatalf("Classes = %d, want 1 (tenuta)", len(got.Classes))
		}
		if len(got.TrainingMemberships) != 1 || deref(got.TrainingMemberships[0].ID) != "m-active" {
			t.Fatalf("TrainingMemberships = %+v, want solo m-active", got.TrainingMemberships)
		}
		if len(got.Access) != 1 || deref(got.Access[0].ID) != "a-active" {
			t.Fatalf("Access = %+v, want solo a-active", got.Access)
		}
		if len(got.Attendances) != 1 || deref(got.Attendances[0].ID) != "att-active" {
			t.Fatalf("Attendances = %+v, want solo att-active", got.Attendances)
		}
	})
}
