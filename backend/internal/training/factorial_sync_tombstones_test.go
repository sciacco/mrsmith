package training

import (
	"testing"

	"github.com/sciacco/mrsmith/pkg/factorial"
)

func TestReduceTombstones(t *testing.T) {
	t.Run("vince solo l'ultima action", func(t *testing.T) {
		rows := []auditDeleteRow{
			{EntityID: "s1", RemoteID: "fs-old"},
			{EntityID: "s1", RemoteID: "fs-new"},
		}
		ids, warnings := reduceTombstones(rows, "warn", "training_session")
		if _, ok := ids["fs-new"]; !ok {
			t.Fatal("fs-new (ultima action) assente dal tombstone")
		}
		if _, ok := ids["fs-old"]; ok {
			t.Fatal("fs-old (action superata) presente nel tombstone")
		}
		if len(warnings) != 0 {
			t.Fatalf("warnings = %v, want nessuno", warnings)
		}
	})

	t.Run("riga locale ricreata: tombstone non valido", func(t *testing.T) {
		rows := []auditDeleteRow{
			{EntityID: "s1", RemoteID: "fs-1", RowExists: false},
			{EntityID: "s1", RemoteID: "fs-1", RowExists: true}, // ricreata dopo la delete
		}
		ids, warnings := reduceTombstones(rows, "warn", "training_session")
		if len(ids) != 0 {
			t.Fatalf("ids = %v, want vuoto (riga ricreata invalida il tombstone)", ids)
		}
		if len(warnings) != 0 {
			t.Fatalf("warnings = %v, want nessuno (ricreata: scarto silenzioso)", warnings)
		}
	})

	t.Run("before_state senza id remoto: scarto con warning", func(t *testing.T) {
		rows := []auditDeleteRow{{EntityID: "s1", RemoteID: ""}}
		ids, warnings := reduceTombstones(rows, "session_tombstone_missing_remote_id", "training_session")
		if len(ids) != 0 {
			t.Fatalf("ids = %v, want vuoto", ids)
		}
		if len(warnings) != 1 || warnings[0].Kind != "session_tombstone_missing_remote_id" || warnings[0].Ref != "s1" {
			t.Fatalf("warnings = %+v, want un warning su s1", warnings)
		}
	})

	t.Run("access: la chiave e' (iscrizione, sessione), non solo iscrizione", func(t *testing.T) {
		rows := []auditDeleteRow{
			{EntityID: "en1", SessionID: "sess-a", RemoteID: "acc-a"},
			{EntityID: "en1", SessionID: "sess-b", RemoteID: "acc-b"},
		}
		ids, _ := reduceTombstones(rows, "warn", "enrollment_session")
		if _, ok := ids["acc-a"]; !ok {
			t.Fatal("acc-a assente: due sessioni della stessa iscrizione devono restare distinte")
		}
		if _, ok := ids["acc-b"]; !ok {
			t.Fatal("acc-b assente: due sessioni della stessa iscrizione devono restare distinte")
		}
	})
}

func TestFilterTombstonedGraph(t *testing.T) {
	graph := factorialTrainingGraph{
		Sessions: []factorial.TrainingsSession{
			{ID: ptr("sess-keep")},
			{ID: ptr("sess-tombstoned")},
		},
		SessionAccessMemberships: []factorial.TrainingsSessionAccessMembership{
			{ID: ptr("acc-keep")},
			{ID: ptr("acc-tombstoned")},
		},
	}

	filtered := filterTombstonedGraph(graph,
		map[string]struct{}{"sess-tombstoned": {}},
		map[string]struct{}{"acc-tombstoned": {}})

	if len(filtered.Sessions) != 1 || deref(filtered.Sessions[0].ID) != "sess-keep" {
		t.Fatalf("Sessions = %+v, want solo sess-keep", filtered.Sessions)
	}
	if len(filtered.SessionAccessMemberships) != 1 || deref(filtered.SessionAccessMemberships[0].ID) != "acc-keep" {
		t.Fatalf("SessionAccessMemberships = %+v, want solo acc-keep", filtered.SessionAccessMemberships)
	}
}

func TestDecideMissingRemote(t *testing.T) {
	cases := []struct {
		name                        string
		active, imported, ambiguous bool
		want                        string
	}{
		{"nato localmente e ancora attivo: azzeramento", true, false, false, missingReset},
		{"importato: conservato con warning anche se attivo", true, true, false, missingKeepImported},
		{"nato localmente ma non piu' attivo: conservato con warning", false, false, false, missingKeepInactive},
		{"provenienza ambigua: conservato con warning, vince su attivo e non-importato", true, false, true, missingKeepAmbiguous},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := decideMissingRemote(tc.active, tc.imported, tc.ambiguous); got != tc.want {
				t.Fatalf("decideMissingRemote(%v, %v, %v) = %q, want %q", tc.active, tc.imported, tc.ambiguous, got, tc.want)
			}
		})
	}
}
