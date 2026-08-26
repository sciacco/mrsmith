package training

import (
	"testing"
	"time"

	"github.com/sciacco/mrsmith/pkg/factorial"
)

func TestClassDateEnvelope(t *testing.T) {
	t.Run("min/max tra scheduled e self-paced", func(t *testing.T) {
		sessions := []sessionSyncState{
			{ScheduleType: "scheduled", StartsAt: "2026-09-10T08:00:00Z", EndsAt: "2026-09-10T10:00:00Z"},
			{ScheduleType: "self_paced", DueDate: "2026-09-20"},
			{ScheduleType: "scheduled", StartsAt: "2026-09-05T08:00:00Z", EndsAt: "2026-09-05T10:00:00Z"},
		}
		got := classDateEnvelope(sessions)
		want := dateEnvelope{Start: "2026-09-05", End: "2026-09-20", OK: true}
		if got != want {
			t.Fatalf("classDateEnvelope() = %+v, want %+v", got, want)
		}
	})
	t.Run("sola due date vale come entrambi gli estremi", func(t *testing.T) {
		got := classDateEnvelope([]sessionSyncState{{ScheduleType: "self_paced", DueDate: "2026-09-20"}})
		want := dateEnvelope{Start: "2026-09-20", End: "2026-09-20", OK: true}
		if got != want {
			t.Fatalf("classDateEnvelope() = %+v, want %+v", got, want)
		}
	})
	t.Run("nessuna data utile: non esportabile", func(t *testing.T) {
		got := classDateEnvelope([]sessionSyncState{{}, {ScheduleType: "scheduled"}})
		if got.OK {
			t.Fatalf("classDateEnvelope() = %+v, want OK=false", got)
		}
	})
	t.Run("scheduled senza ends_at: starts_at vale come entrambi gli estremi", func(t *testing.T) {
		got := classDateEnvelope([]sessionSyncState{{ScheduleType: "scheduled", StartsAt: "2026-09-10T08:00:00Z"}})
		want := dateEnvelope{Start: "2026-09-10", End: "2026-09-10", OK: true}
		if got != want {
			t.Fatalf("classDateEnvelope() = %+v, want %+v", got, want)
		}
	})
}

func TestSessionExportReason(t *testing.T) {
	cases := []struct {
		name string
		s    sessionSyncState
		want string
	}{
		{"tipo NULL", sessionSyncState{}, "session_type_missing"},
		{"scheduled multigiorno", sessionSyncState{ScheduleType: "scheduled", StartsAt: "2026-09-10T22:00:00Z", EndsAt: "2026-09-11T06:00:00Z"}, "session_multiday_scheduled"},
		{"scheduled stesso giorno: esportabile", sessionSyncState{ScheduleType: "scheduled", StartsAt: "2026-09-10T08:00:00Z", EndsAt: "2026-09-10T10:00:00Z"}, ""},
		{"self_paced: esportabile", sessionSyncState{ScheduleType: "self_paced", DueDate: "2026-09-20"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sessionExportReason(tc.s); got != tc.want {
				t.Fatalf("sessionExportReason(%+v) = %q, want %q", tc.s, got, tc.want)
			}
		})
	}
}

func TestFirstExportableEnvelope(t *testing.T) {
	t.Run("training senza erogazione esportabile", func(t *testing.T) {
		_, ok := firstExportableEnvelope([]dateEnvelope{{}, {}})
		if ok {
			t.Fatal("firstExportableEnvelope() ok=true, want false: nessun envelope valido")
		}
	})
	t.Run("sceglie l'envelope con inizio piu' antico", func(t *testing.T) {
		envelopes := []dateEnvelope{
			{Start: "2026-10-01", End: "2026-10-02", OK: true},
			{Start: "2026-09-01", End: "2026-09-03", OK: true},
			{},
		}
		got, ok := firstExportableEnvelope(envelopes)
		if !ok {
			t.Fatal("firstExportableEnvelope() ok=false, want true")
		}
		if want := (dateEnvelope{Start: "2026-09-01", End: "2026-09-03", OK: true}); got != want {
			t.Fatalf("firstExportableEnvelope() = %+v, want %+v", got, want)
		}
	})
}

func TestMatchChild(t *testing.T) {
	children := []correlatorChild{
		{ID: "id-1", Name: "Corso Sicurezza [MS:uuid-1]"},
		{ID: "id-2", Name: "Corso Privacy [MS:uuid-2]"},
		{ID: "id-3", Name: "Corso Sicurezza (vecchio) [MS:uuid-1]"},
	}
	t.Run("zero match: create", func(t *testing.T) {
		outcome, adopted := matchChild(children, "[MS:uuid-9]")
		if outcome != matchCreate || adopted != "" {
			t.Fatalf("matchChild() = (%q, %q), want (%q, \"\")", outcome, adopted, matchCreate)
		}
	})
	t.Run("un match: adopt con l'id trovato", func(t *testing.T) {
		outcome, adopted := matchChild(children, "[MS:uuid-2]")
		if outcome != matchAdopt || adopted != "id-2" {
			t.Fatalf("matchChild() = (%q, %q), want (%q, %q)", outcome, adopted, matchAdopt, "id-2")
		}
	})
	t.Run("piu match: conflict", func(t *testing.T) {
		outcome, adopted := matchChild(children, "[MS:uuid-1]")
		if outcome != matchConflict || adopted != "" {
			t.Fatalf("matchChild() = (%q, %q), want (%q, \"\")", outcome, adopted, matchConflict)
		}
	})
	t.Run("training per Code: nessun figlio", func(t *testing.T) {
		outcome, _ := matchChild(nil, "some-uuid")
		if outcome != matchCreate {
			t.Fatalf("matchChild(nil, ...) = %q, want %q", outcome, matchCreate)
		}
	})
}

func TestClassCreateBody(t *testing.T) {
	tech := technicalEmployee{AccessID: "access-1", CompanyID: "company-1"}
	start := factorial.Date{Time: mustParseDate(t, "2026-09-01")}
	end := factorial.Date{Time: mustParseDate(t, "2026-09-20")}
	body := classCreateBody("training-1", "Corso Sicurezza [MS:evt-1]", start, end, tech)

	if body.TrainingID != "training-1" || body.CompanyID != "company-1" || body.AuthorID != "access-1" {
		t.Fatalf("classCreateBody() parent/technical fields = %+v", body)
	}
	if body.Name == nil || *body.Name != "Corso Sicurezza [MS:evt-1]" {
		t.Fatalf("classCreateBody() Name = %v, want correlatore nel nome", body.Name)
	}
	if body.StartDate != start || body.EndDate != end {
		t.Fatalf("classCreateBody() date = (%v, %v), want (%v, %v)", body.StartDate, body.EndDate, start, end)
	}
	if body.Cost != "0" || body.SubsidizedCost != "0" || body.IndirectCost != "0" || body.SalaryCost != "0" {
		t.Fatalf("classCreateBody() placeholder costi = %+v, want tutti \"0\"", body)
	}
	if body.PaymentStatus != factorial.TrainingsTrainingClassesCreateBodyPaymentStatusPending {
		t.Fatalf("classCreateBody() PaymentStatus = %q, want pending", body.PaymentStatus)
	}
}

func TestClassUpdateBody(t *testing.T) {
	start := factorial.Date{Time: mustParseDate(t, "2026-09-01")}
	end := factorial.Date{Time: mustParseDate(t, "2026-09-20")}
	t.Run("preserva costi e payment status del remoto", func(t *testing.T) {
		remote := factorial.TrainingsTrainingClass{
			Cost: ptr("120.00"), SubsidizedCost: ptr("20.00"), IndirectCost: ptr("5.00"), SalaryCost: ptr("30.00"),
			PaymentStatus: ptr(factorial.TrainingsTrainingClassPaymentStatusPaid),
		}
		body, ok := classUpdateBody(remote, start, end)
		if !ok {
			t.Fatal("classUpdateBody() ok=false, want true")
		}
		if body.Cost != "120.00" || body.SubsidizedCost != "20.00" || body.IndirectCost != "5.00" || body.SalaryCost != "30.00" {
			t.Fatalf("classUpdateBody() costi = %+v, want preservati dal remoto", body)
		}
		if body.PaymentStatus != factorial.TrainingsTrainingClassesUpdateBodyPaymentStatusPaid {
			t.Fatalf("classUpdateBody() PaymentStatus = %q, want paid", body.PaymentStatus)
		}
		if *body.StartDate != start || *body.EndDate != end {
			t.Fatalf("classUpdateBody() date = (%v, %v), want (%v, %v)", body.StartDate, body.EndDate, start, end)
		}
	})
	t.Run("costo obbligatorio mancante nel remoto: update saltato", func(t *testing.T) {
		remote := factorial.TrainingsTrainingClass{Cost: ptr("120.00")} // SubsidizedCost/IndirectCost/SalaryCost/PaymentStatus assenti
		if _, ok := classUpdateBody(remote, start, end); ok {
			t.Fatal("classUpdateBody() ok=true, want false: campi obbligatori mancanti nel remoto")
		}
	})
}

func TestSessionCreateBody(t *testing.T) {
	local := sessionSyncState{ScheduleType: "self_paced", DueDate: "2026-09-20"}
	body, err := sessionCreateBody("training-1", "class-1", "Corso Sicurezza [MS:sess-1]", local)
	if err != nil {
		t.Fatalf("sessionCreateBody() unexpected error: %v", err)
	}
	if body.Name != "Corso Sicurezza [MS:sess-1]" {
		t.Fatalf("sessionCreateBody() Name = %q, want correlatore nel nome", body.Name)
	}
	if body.TrainingClassID == nil || *body.TrainingClassID != "class-1" {
		t.Fatalf("sessionCreateBody() TrainingClassID = %v, want class-1", body.TrainingClassID)
	}
	if len(body.Reminders) != 0 {
		t.Fatalf("sessionCreateBody() Reminders = %v, want []", body.Reminders)
	}
	if body.SendCalendarInvites == nil || *body.SendCalendarInvites {
		t.Fatalf("sessionCreateBody() SendCalendarInvites = %v, want false", body.SendCalendarInvites)
	}
	if body.Schedule == nil || *body.Schedule != factorial.TrainingsSessionsCreateBodyScheduleSelfpaced {
		t.Fatalf("sessionCreateBody() Schedule = %v, want selfpaced (self_paced locale mappato)", body.Schedule)
	}
	if body.DueDate == nil || body.DueDate.String() != "2026-09-20" {
		t.Fatalf("sessionCreateBody() DueDate = %v, want 2026-09-20", body.DueDate)
	}
}

func TestSessionUpdateBody(t *testing.T) {
	local := sessionSyncState{ScheduleType: "scheduled", StartsAt: "2026-09-10T08:00:00Z", EndsAt: "2026-09-10T10:00:00Z"}
	body, err := sessionUpdateBody("Nome remoto originale", local)
	if err != nil {
		t.Fatalf("sessionUpdateBody() unexpected error: %v", err)
	}
	if body.Name != "Nome remoto originale" {
		t.Fatalf("sessionUpdateBody() Name = %q, want il nome remoto preservato", body.Name)
	}
	if body.Schedule == nil || *body.Schedule != factorial.TrainingsSessionsUpdateBodyScheduleScheduled {
		t.Fatalf("sessionUpdateBody() Schedule = %v, want scheduled", body.Schedule)
	}
	if body.StartsAt == nil || body.EndsAt == nil {
		t.Fatal("sessionUpdateBody() StartsAt/EndsAt = nil, want valorizzati dal locale")
	}
}

func TestTrainingCreateBody(t *testing.T) {
	tech := technicalEmployee{AccessID: "access-1", CompanyID: "company-1"}
	t.Run("mapping completo", func(t *testing.T) {
		body := trainingCreateBody("course-uuid", "Corso Sicurezza", "una descrizione", "external", 2026, tech)
		if body.Code == nil || *body.Code != "course-uuid" {
			t.Fatalf("trainingCreateBody() Code = %v, want course-uuid (correlatore)", body.Code)
		}
		if !body.External {
			t.Fatal("trainingCreateBody() External = false, want true per provider_kind external")
		}
		if body.Year != 2026 {
			t.Fatalf("trainingCreateBody() Year = %d, want 2026", body.Year)
		}
		if body.CompanyID == nil || *body.CompanyID != "company-1" || body.AuthorID == nil || *body.AuthorID != "access-1" {
			t.Fatalf("trainingCreateBody() company/author = %+v, want dall'employee tecnico", body)
		}
		if len(body.Attachments) != 0 {
			t.Fatalf("trainingCreateBody() Attachments = %v, want []", body.Attachments)
		}
		if body.Description != "una descrizione" {
			t.Fatalf("trainingCreateBody() Description = %q, want %q", body.Description, "una descrizione")
		}
	})
	t.Run("provider_kind internal: External false", func(t *testing.T) {
		body := trainingCreateBody("course-uuid", "Corso", "", "internal", 2026, tech)
		if body.External {
			t.Fatal("trainingCreateBody() External = true, want false per provider_kind internal")
		}
	})
	t.Run("descrizione vuota: fallback al titolo", func(t *testing.T) {
		body := trainingCreateBody("course-uuid", "Corso Sicurezza", "  ", "external", 2026, tech)
		if body.Description != "Corso Sicurezza" {
			t.Fatalf("trainingCreateBody() Description = %q, want il titolo come fallback", body.Description)
		}
	})
}

func TestTrainingUpdateBody(t *testing.T) {
	body := trainingUpdateBody("Corso Sicurezza", "", "internal", 2019)
	if body.Year != 2019 {
		t.Fatalf("trainingUpdateBody() Year = %d, want 2019 (preservato dal remoto)", body.Year)
	}
	if body.Description != "Corso Sicurezza" {
		t.Fatalf("trainingUpdateBody() Description = %q, want fallback al titolo", body.Description)
	}
	if body.External {
		t.Fatal("trainingUpdateBody() External = true, want false per provider_kind internal")
	}
}

func TestAccessBulkCreateBody(t *testing.T) {
	body := accessBulkCreateBody("session-1", []string{"emp-1", "emp-2"})
	if body.Notify {
		t.Fatal("accessBulkCreateBody() Notify = true, want false")
	}
	if body.SessionID != "session-1" || len(body.EmployeeIDs) != 2 {
		t.Fatalf("accessBulkCreateBody() = %+v, want session/employee mappati", body)
	}
}

func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := factorialDateFromState(s)
	if err != nil {
		t.Fatalf("mustParseDate(%q): %v", s, err)
	}
	return parsed.Time
}
