package training

import (
	"errors"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/sciacco/mrsmith/pkg/factorial"
)

func ptr[T any](v T) *T { return &v }

func TestAttendanceStatusMapping(t *testing.T) {
	pairs := []struct {
		name   string
		remote factorial.TrainingsSessionAttendanceStatus
		local  string
	}{
		{"pending -> assigned", factorial.TrainingsSessionAttendanceStatusPending, participationAssigned},
		{"inprogress -> in_progress", factorial.TrainingsSessionAttendanceStatusInprogress, participationInProgress},
		{"completed -> completed", factorial.TrainingsSessionAttendanceStatusCompleted, participationCompleted},
		{"missing -> not_attended", factorial.TrainingsSessionAttendanceStatusMissing, participationNotAttended},
	}
	for _, tc := range pairs {
		t.Run(tc.name, func(t *testing.T) {
			gotLocal, err := attendanceLocalFromRemote(tc.remote)
			if err != nil {
				t.Fatalf("attendanceLocalFromRemote(%q) unexpected error: %v", tc.remote, err)
			}
			if gotLocal != tc.local {
				t.Fatalf("attendanceLocalFromRemote(%q) = %q, want %q", tc.remote, gotLocal, tc.local)
			}
			gotRemote, err := attendanceRemoteFromLocal(tc.local)
			if err != nil {
				t.Fatalf("attendanceRemoteFromLocal(%q) unexpected error: %v", tc.local, err)
			}
			if gotRemote != tc.remote {
				t.Fatalf("attendanceRemoteFromLocal(%q) = %q, want %q", tc.local, gotRemote, tc.remote)
			}
		})
	}
	t.Run("valore remoto sconosciuto", func(t *testing.T) {
		if _, err := attendanceLocalFromRemote("bogus"); err == nil {
			t.Fatal("attendanceLocalFromRemote(\"bogus\") should return an error, not panic")
		}
	})
	t.Run("valore locale sconosciuto", func(t *testing.T) {
		if _, err := attendanceRemoteFromLocal("bogus"); err == nil {
			t.Fatal("attendanceRemoteFromLocal(\"bogus\") should return an error, not panic")
		}
	})
}

func TestSessionSyncStateFromLocal(t *testing.T) {
	cest := time.FixedZone("CEST", 2*3600)
	starts := time.Date(2026, 9, 1, 10, 0, 0, 0, cest)
	ends := time.Date(2026, 9, 1, 12, 0, 0, 0, cest)
	due := time.Date(2026, 9, 15, 1, 0, 0, 0, cest) // 2026-09-14T23:00:00Z: la data UTC retrocede

	got := newSessionSyncStateFromLocal(ptr("self_paced"), &starts, &ends, &due)
	want := sessionSyncState{
		ScheduleType: "self_paced",
		StartsAt:     "2026-09-01T08:00:00Z",
		EndsAt:       "2026-09-01T10:00:00Z",
		DueDate:      "2026-09-14",
	}
	if got != want {
		t.Fatalf("newSessionSyncStateFromLocal() = %+v, want %+v", got, want)
	}

	t.Run("tutti i campi null", func(t *testing.T) {
		got := newSessionSyncStateFromLocal(nil, nil, nil, nil)
		if got != (sessionSyncState{}) {
			t.Fatalf("newSessionSyncStateFromLocal(nil...) = %+v, want zero value", got)
		}
	})
}

func TestSessionSyncStateFromRemote(t *testing.T) {
	cest := time.FixedZone("CEST", 2*3600)
	starts := factorial.Time{Time: time.Date(2026, 9, 1, 10, 0, 0, 0, cest)}
	ends := factorial.Time{Time: time.Date(2026, 9, 1, 12, 0, 0, 0, cest)}
	due := factorial.Date{Time: time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)}
	selfpaced := factorial.TrainingsSessionScheduleSelfpaced

	got := newSessionSyncStateFromRemote(factorial.TrainingsSession{
		Schedule: &selfpaced,
		StartsAt: &starts,
		EndsAt:   &ends,
		DueDate:  &due,
	})
	want := sessionSyncState{
		ScheduleType: "self_paced",
		StartsAt:     "2026-09-01T08:00:00Z",
		EndsAt:       "2026-09-01T10:00:00Z",
		DueDate:      "2026-09-16",
	}
	if got != want {
		t.Fatalf("newSessionSyncStateFromRemote() = %+v, want %+v", got, want)
	}

	t.Run("scheduled passa invariato", func(t *testing.T) {
		scheduled := factorial.TrainingsSessionScheduleScheduled
		got := newSessionSyncStateFromRemote(factorial.TrainingsSession{Schedule: &scheduled})
		if got.ScheduleType != "scheduled" {
			t.Fatalf("ScheduleType = %q, want %q", got.ScheduleType, "scheduled")
		}
	})

	t.Run("tutti i campi assenti", func(t *testing.T) {
		got := newSessionSyncStateFromRemote(factorial.TrainingsSession{})
		if got != (sessionSyncState{}) {
			t.Fatalf("newSessionSyncStateFromRemote({}) = %+v, want zero value", got)
		}
	})
}

func TestSessionSyncStateJSON(t *testing.T) {
	full := sessionSyncState{
		ScheduleType: "scheduled",
		StartsAt:     "2026-09-01T08:00:00Z",
		EndsAt:       "2026-09-01T10:00:00Z",
		DueDate:      "2026-09-16",
	}
	raw, err := full.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() unexpected error: %v", err)
	}
	wantJSON := `{"schedule_type":"scheduled","starts_at":"2026-09-01T08:00:00Z","ends_at":"2026-09-01T10:00:00Z","due_date":"2026-09-16"}`
	if string(raw) != wantJSON {
		t.Fatalf("MarshalJSON() = %s, want %s", raw, wantJSON)
	}
	var roundTrip sessionSyncState
	if err := roundTrip.UnmarshalJSON(raw); err != nil {
		t.Fatalf("UnmarshalJSON() unexpected error: %v", err)
	}
	if roundTrip != full {
		t.Fatalf("round-trip = %+v, want %+v", roundTrip, full)
	}

	t.Run("valore nullo serializza a null, non stringa vuota", func(t *testing.T) {
		raw, err := sessionSyncState{}.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() unexpected error: %v", err)
		}
		wantJSON := `{"schedule_type":null,"starts_at":null,"ends_at":null,"due_date":null}`
		if string(raw) != wantJSON {
			t.Fatalf("MarshalJSON() = %s, want %s", raw, wantJSON)
		}
		var parsed sessionSyncState
		if err := parsed.UnmarshalJSON(raw); err != nil {
			t.Fatalf("UnmarshalJSON() unexpected error: %v", err)
		}
		if parsed != (sessionSyncState{}) {
			t.Fatalf("UnmarshalJSON(all null) = %+v, want zero value", parsed)
		}
	})
}

func TestThreeWayDecision(t *testing.T) {
	cases := []struct {
		name       string
		local      string
		remote     string
		checkpoint *string
		want       syncDecision
	}{
		{"nessun checkpoint", "A", "B", nil, syncNoCheckpoint},
		{"nessun checkpoint anche se lati uguali", "A", "A", nil, syncNoCheckpoint},
		{"lati uguali e checkpoint allineato", "A", "A", ptr("A"), syncInSync},
		{"lati uguali ma checkpoint disallineato", "A", "A", ptr("B"), syncInSync},
		{"solo il locale differisce", "A", "B", ptr("B"), syncPropagateLocal},
		{"solo il remoto differisce", "A", "B", ptr("A"), syncPropagateRemote},
		{"entrambi divergono in modo diverso", "A", "B", ptr("C"), syncConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := threeWayDecision(tc.local, tc.remote, tc.checkpoint); got != tc.want {
				t.Fatalf("threeWayDecision(%q, %q, %v) = %v, want %v", tc.local, tc.remote, tc.checkpoint, got, tc.want)
			}
		})
	}

	t.Run("generica su sessionSyncState", func(t *testing.T) {
		local := sessionSyncState{ScheduleType: "scheduled"}
		remote := sessionSyncState{ScheduleType: "self_paced"}
		checkpoint := sessionSyncState{ScheduleType: "scheduled"}
		if got := threeWayDecision(local, remote, &checkpoint); got != syncPropagateRemote {
			t.Fatalf("threeWayDecision(sessionSyncState) = %v, want %v", got, syncPropagateRemote)
		}
	})
}

func TestFactorialCorrelatorNames(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	t.Run("classe: titolo breve", func(t *testing.T) {
		got := factorialClassName("Corso Sicurezza", id)
		want := "Corso Sicurezza [MS:" + id + "]"
		if got != want {
			t.Fatalf("factorialClassName() = %q, want %q", got, want)
		}
	})
	t.Run("sessione: titolo breve", func(t *testing.T) {
		got := factorialSessionName("Corso Sicurezza", id)
		want := "Corso Sicurezza [MS:" + id + "]"
		if got != want {
			t.Fatalf("factorialSessionName() = %q, want %q", got, want)
		}
	})
	t.Run("titolo lungo si tronca, il token mai", func(t *testing.T) {
		longTitle := ""
		for i := 0; i < 300; i++ {
			longTitle += "à" // multi-byte: verifica anche il taglio a limite di rune
		}
		got := factorialClassName(longTitle, id)
		token := "[MS:" + id + "]"
		if !utf8.ValidString(got) {
			t.Fatalf("factorialClassName() produced invalid UTF-8: %q", got)
		}
		if got == longTitle+" "+token {
			t.Fatal("expected truncation, title was not shortened")
		}
		if got[len(got)-len(token):] != token {
			t.Fatalf("factorialClassName() = %q, token was altered, want suffix %q", got, token)
		}
		if runeLen := len([]rune(got)); runeLen > factorialNameMaxLen {
			t.Fatalf("factorialClassName() length = %d runes, want <= %d", runeLen, factorialNameMaxLen)
		}
	})
}

func TestMatchCorrelator(t *testing.T) {
	candidates := []string{
		"Corso Sicurezza [MS:uuid-1]",
		"Corso Privacy [MS:uuid-2]",
		"Corso Sicurezza (vecchio) [MS:uuid-1]",
	}
	cases := []struct {
		name  string
		token string
		want  []int
	}{
		{"zero match", "[MS:uuid-9]", nil},
		{"un match", "[MS:uuid-2]", []int{1}},
		{"piu match", "[MS:uuid-1]", []int{0, 2}},
		{"case-sensitive: nessun match su case diverso", "[MS:UUID-2]", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := matchCorrelator(candidates, tc.token)
			if len(got) != len(tc.want) {
				t.Fatalf("matchCorrelator() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("matchCorrelator() = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestClassifySyncError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"400 isola il ramo", &factorial.APIError{StatusCode: 400}, errIsolateBranch},
		{"422 isola il ramo", &factorial.APIError{StatusCode: 422}, errIsolateBranch},
		{"401 fa fallire la run", &factorial.APIError{StatusCode: 401}, errFailRun},
		{"403 fa fallire la run", &factorial.APIError{StatusCode: 403}, errFailRun},
		{"429 fa fallire la run", &factorial.APIError{StatusCode: 429}, errFailRun},
		{"500 fa fallire la run", &factorial.APIError{StatusCode: 500}, errFailRun},
		{"503 fa fallire la run", &factorial.APIError{StatusCode: 503}, errFailRun},
		{"404 non gestito qui: fa fallire la run", &factorial.APIError{StatusCode: 404}, errFailRun},
		{"errore di trasporto non-API", errors.New("connection reset by peer"), errFailRun},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifySyncError(tc.err); got != tc.want {
				t.Fatalf("classifySyncError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
