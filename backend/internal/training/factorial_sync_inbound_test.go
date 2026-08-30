package training

import (
	"testing"
	"time"

	"github.com/sciacco/mrsmith/pkg/factorial"
)

func TestComputeTrainingDiff_CourseSeed(t *testing.T) {
	t.Run("nuovo con titolo", func(t *testing.T) {
		training := factorial.TrainingsTraining{ID: ptr("t-1"), Name: ptr("Corso Sicurezza"), Description: ptr("desc")}
		diff := computeTrainingDiff(training, trainingClassPerimeter{}, localTrainingState{}, nil)
		if diff.Course == nil || !diff.Course.Create {
			t.Fatalf("Course = %+v, want Create=true", diff.Course)
		}
		if diff.Course.Title != "Corso Sicurezza" || diff.Course.Description != "desc" || diff.Course.TitleMissing {
			t.Fatalf("Course = %+v, want titolo/descrizione dal remoto senza warning", diff.Course)
		}
		if len(diff.Warnings) != 0 {
			t.Fatalf("Warnings = %+v, want nessuno", diff.Warnings)
		}
	})
	t.Run("nuovo senza titolo: fallback e warning", func(t *testing.T) {
		training := factorial.TrainingsTraining{ID: ptr("t-2")}
		diff := computeTrainingDiff(training, trainingClassPerimeter{}, localTrainingState{}, nil)
		want := "Training Factorial t-2"
		if diff.Course == nil || diff.Course.Title != want || !diff.Course.TitleMissing {
			t.Fatalf("Course = %+v, want Title=%q TitleMissing=true", diff.Course, want)
		}
		if len(diff.Warnings) != 1 || diff.Warnings[0].Kind != "course_title_missing" {
			t.Fatalf("Warnings = %+v, want un solo course_title_missing", diff.Warnings)
		}
	})
	t.Run("gia' attivo: il catalogo locale prevale", func(t *testing.T) {
		training := factorial.TrainingsTraining{ID: ptr("t-3"), Name: ptr("Nuovo Nome")}
		local := localTrainingState{Courses: map[string]localCourse{"t-3": {ID: "c-1", IsActive: true}}}
		diff := computeTrainingDiff(training, trainingClassPerimeter{}, local, nil)
		if diff.Course != nil {
			t.Fatalf("Course = %+v, want nil (corso attivo, nessun aggiornamento)", diff.Course)
		}
	})
	t.Run("seed ancora inattivo e non allineato: si aggiorna", func(t *testing.T) {
		training := factorial.TrainingsTraining{ID: ptr("t-4"), Name: ptr("Titolo Aggiornato")}
		local := localTrainingState{Courses: map[string]localCourse{"t-4": {ID: "c-2", IsActive: false}}}
		diff := computeTrainingDiff(training, trainingClassPerimeter{}, local, nil)
		if diff.Course == nil || diff.Course.Create || diff.Course.CourseID != "c-2" || diff.Course.Title != "Titolo Aggiornato" {
			t.Fatalf("Course = %+v, want update su c-2", diff.Course)
		}
	})
	t.Run("seed inattivo ma gia' allineato: nessuna mutazione", func(t *testing.T) {
		training := factorial.TrainingsTraining{ID: ptr("t-5"), Name: ptr("Corso Sicurezza"), Description: ptr("desc")}
		local := localTrainingState{Courses: map[string]localCourse{"t-5": {ID: "c-3", IsActive: false, Title: "Corso Sicurezza", Description: "desc", ProviderKind: "internal"}}}
		diff := computeTrainingDiff(training, trainingClassPerimeter{}, local, nil)
		if diff.Course != nil {
			t.Fatalf("Course = %+v, want nil (gia' allineato, idempotenza)", diff.Course)
		}
	})
}

func TestComputeTrainingDiff_ClassWithoutSessions(t *testing.T) {
	training := factorial.TrainingsTraining{ID: ptr("t-1")}
	local := localTrainingState{Courses: map[string]localCourse{"t-1": {ID: "c-1", IsActive: true}}, Events: map[string]localEvent{}}
	sub := trainingClassPerimeter{Classes: []factorial.TrainingsTrainingClass{{ID: ptr("cl-1"), TrainingID: ptr("t-1")}}}
	diff := computeTrainingDiff(training, sub, local, nil)
	if len(diff.Events) != 1 || diff.Events[0].FactorialClassID != "cl-1" {
		t.Fatalf("Events = %+v, want un solo seed su cl-1", diff.Events)
	}
	if len(diff.Sessions) != 0 || len(diff.Enrollments) != 0 {
		t.Fatalf("Sessions/Enrollments non vuoti: %+v / %+v", diff.Sessions, diff.Enrollments)
	}
}

func TestComputeTrainingDiff_SessionThreeWay(t *testing.T) {
	training := factorial.TrainingsTraining{ID: ptr("t-1")}
	courses := map[string]localCourse{"t-1": {IsActive: true}}
	classes := []factorial.TrainingsTrainingClass{{ID: ptr("cl-1"), TrainingID: ptr("t-1")}}
	events := map[string]localEvent{"cl-1": {ID: "ev-1"}}
	sessions := []factorial.TrainingsSession{{ID: ptr("s-1"), TrainingClassID: ptr("cl-1"), StartsAt: &factorial.Time{Time: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)}}}
	remoteState := sessionSyncState{StartsAt: "2026-03-01T00:00:00Z"}
	sub := trainingClassPerimeter{Classes: classes, Sessions: sessions}

	t.Run("propaga remoto", func(t *testing.T) {
		checkpoint := sessionSyncState{StartsAt: "2026-01-01T00:00:00Z"} // == locale: solo il remoto e' cambiato
		local := localTrainingState{Courses: courses, Events: events, Sessions: map[string]localSession{"s-1": {ID: "sess-1", Local: checkpoint, Checkpoint: &checkpoint}}}
		diff := computeTrainingDiff(training, sub, local, nil)
		if len(diff.Sessions) != 1 || diff.Sessions[0].SessionID != "sess-1" || diff.Sessions[0].Remote != remoteState {
			t.Fatalf("Sessions = %+v, want adozione del remoto su sess-1", diff.Sessions)
		}
		if len(diff.Conflicts) != 0 {
			t.Fatalf("Conflicts = %+v, want nessuno", diff.Conflicts)
		}
	})

	t.Run("conflitto", func(t *testing.T) {
		checkpoint := sessionSyncState{StartsAt: "2026-02-01T00:00:00Z"}
		local := localTrainingState{Courses: courses, Events: events, Sessions: map[string]localSession{"s-1": {ID: "sess-1", Local: sessionSyncState{StartsAt: "2026-01-01T00:00:00Z"}, Checkpoint: &checkpoint}}}
		diff := computeTrainingDiff(training, sub, local, nil)
		if len(diff.Sessions) != 0 {
			t.Fatalf("Sessions = %+v, want nessuna scrittura in conflitto", diff.Sessions)
		}
		if len(diff.Conflicts) != 1 || diff.Conflicts[0].Kind != "session_conflict" || diff.Conflicts[0].Ref != "s-1" {
			t.Fatalf("Conflicts = %+v, want un solo session_conflict su s-1", diff.Conflicts)
		}
	})

	t.Run("no checkpoint: provenienza factorial_import propaga il remoto", func(t *testing.T) {
		local := localTrainingState{Courses: courses, Events: events, Sessions: map[string]localSession{"s-1": {ID: "sess-1", Local: sessionSyncState{StartsAt: "2026-01-01T00:00:00Z"}, Provenance: actionFactorialImport}}}
		diff := computeTrainingDiff(training, sub, local, nil)
		if len(diff.Sessions) != 1 || diff.Sessions[0].Remote != remoteState {
			t.Fatalf("Sessions = %+v, want adozione del remoto (provenienza sync)", diff.Sessions)
		}
	})

	t.Run("no checkpoint: gesto locale, nessuna scrittura", func(t *testing.T) {
		local := localTrainingState{Courses: courses, Events: events, Sessions: map[string]localSession{"s-1": {ID: "sess-1", Local: sessionSyncState{StartsAt: "2026-01-01T00:00:00Z"}, Provenance: "create"}}}
		diff := computeTrainingDiff(training, sub, local, nil)
		if len(diff.Sessions) != 0 || len(diff.Conflicts) != 0 || len(diff.Warnings) != 0 {
			t.Fatalf("diff = %+v, want no-op silenzioso (gesto locale)", diff)
		}
	})

	t.Run("no checkpoint: provenienza non dimostrabile", func(t *testing.T) {
		local := localTrainingState{Courses: courses, Events: events, Sessions: map[string]localSession{"s-1": {ID: "sess-1", Local: sessionSyncState{StartsAt: "2026-01-01T00:00:00Z"}}}}
		diff := computeTrainingDiff(training, sub, local, nil)
		if len(diff.Sessions) != 0 {
			t.Fatalf("Sessions = %+v, want nessuna scrittura (nessun vincitore arbitrario)", diff.Sessions)
		}
		if len(diff.Warnings) != 1 || diff.Warnings[0].Kind != "session_provenance_unknown" {
			t.Fatalf("Warnings = %+v, want un solo session_provenance_unknown", diff.Warnings)
		}
	})

	t.Run("no checkpoint ma lati gia' allineati: adotta solo il checkpoint", func(t *testing.T) {
		local := localTrainingState{Courses: courses, Events: events, Sessions: map[string]localSession{"s-1": {ID: "sess-1", Local: remoteState}}}
		diff := computeTrainingDiff(training, sub, local, nil)
		if len(diff.Sessions) != 1 || diff.Sessions[0].Remote != remoteState {
			t.Fatalf("Sessions = %+v, want un adopt per fissare il checkpoint", diff.Sessions)
		}
	})
}

func TestComputeTrainingDiff_SessionDatesInconsistent(t *testing.T) {
	training := factorial.TrainingsTraining{ID: ptr("t-1")}
	local := localTrainingState{Courses: map[string]localCourse{"t-1": {IsActive: true}}, Events: map[string]localEvent{"cl-1": {ID: "ev-1"}}}
	sub := trainingClassPerimeter{
		Classes: []factorial.TrainingsTrainingClass{{ID: ptr("cl-1"), TrainingID: ptr("t-1")}},
		Sessions: []factorial.TrainingsSession{{
			ID: ptr("s-1"), TrainingClassID: ptr("cl-1"),
			StartsAt: &factorial.Time{Time: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)}, EndsAt: &factorial.Time{Time: time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)},
		}},
	}
	diff := computeTrainingDiff(training, sub, local, nil)
	if len(diff.Sessions) != 1 || !diff.Sessions[0].New || diff.Sessions[0].Remote.EndsAt != "" || diff.Sessions[0].Remote.StartsAt == "" {
		t.Fatalf("Sessions = %+v, want seed con ends_at azzerato e starts_at conservato", diff.Sessions)
	}
	if len(diff.Warnings) != 1 || diff.Warnings[0].Kind != "session_ends_before_starts" || diff.Warnings[0].Ref != "s-1" {
		t.Fatalf("Warnings = %+v, want un solo session_ends_before_starts su s-1", diff.Warnings)
	}
}

func TestComputeTrainingDiff_AttendanceRDAException(t *testing.T) {
	training := factorial.TrainingsTraining{ID: ptr("t-1")}
	sub := trainingClassPerimeter{
		Classes:  []factorial.TrainingsTrainingClass{{ID: ptr("cl-1"), TrainingID: ptr("t-1")}},
		Sessions: []factorial.TrainingsSession{{ID: ptr("s-1"), TrainingClassID: ptr("cl-1")}},
		Access:   []factorial.TrainingsSessionAccessMembership{{ID: ptr("a-1"), SessionID: ptr("s-1"), EmployeeID: ptr("emp-1")}},
		Attendances: []factorial.TrainingsSessionAttendance{
			{ID: ptr("att-1"), SessionAccessMembershipID: ptr("a-1"), Status: ptr(factorial.TrainingsSessionAttendanceStatusCompleted)},
		},
	}
	local := localTrainingState{
		Courses: map[string]localCourse{"t-1": {IsActive: true}}, Events: map[string]localEvent{"cl-1": {ID: "ev-1"}},
		Employees: map[string]string{"emp-1": "emp-local-1"}, Enrollments: map[[2]string]localEnrollment{{"emp-local-1", "ev-1"}: {ID: "enr-1"}},
	}
	// Nessun gate al livello diff: un "completed" gia' avvenuto si adotta
	// comunque (il bypass del gate e la segnalazione della discrepanza sono
	// responsabilita' dell'applicazione, non del diff puro).
	diff := computeTrainingDiff(training, sub, local, nil)
	if len(diff.Attendances) != 1 || diff.Attendances[0].FactorialAccessMembershipID != "a-1" || diff.Attendances[0].NewStatus != participationCompleted {
		t.Fatalf("Attendances = %+v, want adozione diretta di 'completed'", diff.Attendances)
	}
	if len(diff.Enrollments) != 0 {
		t.Fatalf("Enrollments = %+v, want zero (iscrizione gia' esistente)", diff.Enrollments)
	}
}

func TestComputeTrainingDiff_CancelledNotReopened(t *testing.T) {
	training := factorial.TrainingsTraining{ID: ptr("t-1")}

	t.Run("evento cancellato: conflitto, sessioni escluse", func(t *testing.T) {
		local := localTrainingState{Courses: map[string]localCourse{"t-1": {IsActive: true}}, Events: map[string]localEvent{"cl-1": {ID: "ev-1", Cancelled: true}}}
		sub := trainingClassPerimeter{
			Classes:  []factorial.TrainingsTrainingClass{{ID: ptr("cl-1"), TrainingID: ptr("t-1")}},
			Sessions: []factorial.TrainingsSession{{ID: ptr("s-1"), TrainingClassID: ptr("cl-1")}},
		}
		diff := computeTrainingDiff(training, sub, local, nil)
		if len(diff.Conflicts) != 1 || diff.Conflicts[0].Kind != "event_reopen_blocked" || diff.Conflicts[0].Ref != "cl-1" {
			t.Fatalf("Conflicts = %+v, want un solo event_reopen_blocked su cl-1", diff.Conflicts)
		}
		if len(diff.Events) != 0 || len(diff.Sessions) != 0 {
			t.Fatalf("Events/Sessions = %+v / %+v, want entrambi vuoti", diff.Events, diff.Sessions)
		}
	})

	t.Run("iscrizione cancellata: conflitto, nessuna nuova assegnazione", func(t *testing.T) {
		local := localTrainingState{
			Courses: map[string]localCourse{"t-1": {IsActive: true}}, Events: map[string]localEvent{"cl-1": {ID: "ev-1"}},
			Employees: map[string]string{"emp-1": "emp-local-1"}, Enrollments: map[[2]string]localEnrollment{{"emp-local-1", "ev-1"}: {ID: "enr-1", Cancelled: true}},
		}
		sub := trainingClassPerimeter{
			Classes:  []factorial.TrainingsTrainingClass{{ID: ptr("cl-1"), TrainingID: ptr("t-1")}},
			Sessions: []factorial.TrainingsSession{{ID: ptr("s-1"), TrainingClassID: ptr("cl-1")}},
			Access:   []factorial.TrainingsSessionAccessMembership{{ID: ptr("a-1"), SessionID: ptr("s-1"), EmployeeID: ptr("emp-1")}},
		}
		diff := computeTrainingDiff(training, sub, local, nil)
		if len(diff.Conflicts) != 1 || diff.Conflicts[0].Kind != "enrollment_reopen_blocked" || diff.Conflicts[0].Ref != "enr-1" {
			t.Fatalf("Conflicts = %+v, want un solo enrollment_reopen_blocked su enr-1", diff.Conflicts)
		}
		if len(diff.Assigns) != 0 {
			t.Fatalf("Assigns = %+v, want vuoto (iscrizione annullata)", diff.Assigns)
		}
	})
}

func TestComputeTrainingDiff_Idempotent(t *testing.T) {
	t.Run("tutte le entita' allineate", func(t *testing.T) {
		training := factorial.TrainingsTraining{ID: ptr("t-1")}
		remote := sessionSyncState{StartsAt: "2026-01-01T00:00:00Z"}
		sub := trainingClassPerimeter{
			Classes:  []factorial.TrainingsTrainingClass{{ID: ptr("cl-1"), TrainingID: ptr("t-1")}},
			Sessions: []factorial.TrainingsSession{{ID: ptr("s-1"), TrainingClassID: ptr("cl-1"), StartsAt: &factorial.Time{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}},
			Access:   []factorial.TrainingsSessionAccessMembership{{ID: ptr("a-1"), SessionID: ptr("s-1"), EmployeeID: ptr("emp-1")}},
			Attendances: []factorial.TrainingsSessionAttendance{
				{ID: ptr("att-1"), SessionAccessMembershipID: ptr("a-1"), Status: ptr(factorial.TrainingsSessionAttendanceStatusPending)},
			},
		}
		local := localTrainingState{
			Courses: map[string]localCourse{"t-1": {ID: "c-1", IsActive: true}}, Events: map[string]localEvent{"cl-1": {ID: "ev-1"}},
			Sessions:  map[string]localSession{"s-1": {ID: "sess-1", Local: remote, Checkpoint: &remote}},
			Employees: map[string]string{"emp-1": "emp-local-1"}, Enrollments: map[[2]string]localEnrollment{{"emp-local-1", "ev-1"}: {ID: "enr-1"}},
			Assigns: map[[2]string]localAssign{{"enr-1", "sess-1"}: {
				Status: participationAssigned, Checkpoint: ptr(participationAssigned),
				FactorialAccessMembershipID: "a-1", FactorialAttendanceID: "att-1"}},
		}
		diff := computeTrainingDiff(training, sub, local, nil)
		if diff.Course != nil {
			t.Fatalf("Course = %+v, want nil", diff.Course)
		}
		if len(diff.Events) != 0 || len(diff.Sessions) != 0 || len(diff.Enrollments) != 0 || len(diff.Attendances) != 0 {
			t.Fatalf("diff con mutazioni: %+v", diff)
		}
		if len(diff.Assigns) != 1 || diff.Assigns[0].NeedsWrite {
			t.Fatalf("Assigns = %+v, want un item presente ma NeedsWrite=false", diff.Assigns)
		}
		if len(diff.Conflicts) != 0 || len(diff.Warnings) != 0 {
			t.Fatalf("Conflicts/Warnings non vuoti: %+v / %+v", diff.Conflicts, diff.Warnings)
		}
	})

	t.Run("corso inattivo ma gia' allineato", func(t *testing.T) {
		training := factorial.TrainingsTraining{ID: ptr("t-2"), Name: ptr("Corso Sicurezza"), Description: ptr("desc")}
		local := localTrainingState{Courses: map[string]localCourse{"t-2": {ID: "c-2", IsActive: false, Title: "Corso Sicurezza", Description: "desc", ProviderKind: "internal"}}}
		diff := computeTrainingDiff(training, trainingClassPerimeter{}, local, nil)
		if diff.Course != nil {
			t.Fatalf("Course = %+v, want nil (gia' allineato, idempotenza)", diff.Course)
		}
	})
}
