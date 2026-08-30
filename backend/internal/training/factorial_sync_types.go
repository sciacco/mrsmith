package training

import (
	"encoding/json"
	"errors"
)

// Proiezioni pure del reconciler Factorial (#141, slice 2/8 = #144): la
// proiezione canonica di sessione con il suo checkpoint jsonb, la decisione
// 3-way e la classificazione degli errori. Zero IO: nessun accesso a
// database, rete o client. Le slice successive (store, applicazione) usano
// questi tipi senza duplicarli.

// sessionSyncState e la proiezione canonica dello stato sincronizzabile di
// una sessione (training.training_session): schedule_type, starts_at,
// ends_at (istanti normalizzati in UTC) e due_date (YYYY-MM-DD). Tutti i
// campi sono nullable; "" rappresenta null. E' il valore confrontato dalla
// decisione 3-way e il contenuto di training_session.factorial_synced_state.
type sessionSyncState struct {
	ScheduleType string
	StartsAt     string
	EndsAt       string
	DueDate      string
}

// sessionSyncStateJSON e la forma su jsonb di sessionSyncState: stesse
// chiavi in ordine fisso, null per gli attributi assenti al posto di "".
type sessionSyncStateJSON struct {
	ScheduleType *string `json:"schedule_type"`
	StartsAt     *string `json:"starts_at"`
	EndsAt       *string `json:"ends_at"`
	DueDate      *string `json:"due_date"`
}

// MarshalJSON implementa json.Marshaler con chiavi fisse e ordinate
// (schedule_type, starts_at, ends_at, due_date); "" diventa null.
func (p sessionSyncState) MarshalJSON() ([]byte, error) {
	return json.Marshal(sessionSyncStateJSON{
		ScheduleType: emptyToNil(p.ScheduleType),
		StartsAt:     emptyToNil(p.StartsAt),
		EndsAt:       emptyToNil(p.EndsAt),
		DueDate:      emptyToNil(p.DueDate),
	})
}

// UnmarshalJSON e il parse inverso di MarshalJSON: null torna "".
func (p *sessionSyncState) UnmarshalJSON(data []byte) error {
	var raw sessionSyncStateJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.ScheduleType = deref(raw.ScheduleType)
	p.StartsAt = deref(raw.StartsAt)
	p.EndsAt = deref(raw.EndsAt)
	p.DueDate = deref(raw.DueDate)
	return nil
}

// emptyToNil converte "" in nil cosi il campo JSON serializza come null
// invece che come stringa vuota.
func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// sessionOpState e' la proiezione dei dati operativi di sessione importati
// da Factorial (argomento, modalita', durata in ore, luogo). Resta FUORI dal
// checkpoint 3-way (che copre solo schedule e date): finche' l'integrazione
// e' attiva questi campi sono di proprieta' del remoto — l'inbound li
// riallinea quando differiscono, senza generare conflitti, e l'outbound non
// li esporta mai. DurationHours e' in forma canonica (vedi canonicalHours)
// per confronti stabili tra remoto e colonna numeric locale.
type sessionOpState struct {
	Topic         string
	Modality      string
	DurationHours string
	Location      string
}

// syncDecision e l'esito della decisione 3-way tra locale, remoto e
// checkpoint per un singolo attributo sincronizzato.
type syncDecision int

const (
	// syncInSync: locale e remoto coincidono; si aggiorna solo il checkpoint.
	syncInSync syncDecision = iota
	// syncPropagateLocal: solo il locale differisce; si propaga al remoto.
	syncPropagateLocal
	// syncPropagateRemote: solo il remoto differisce; si propaga al locale.
	syncPropagateRemote
	// syncConflict: locale e remoto divergono tra loro e dal checkpoint.
	syncConflict
	// syncNoCheckpoint: nessun checkpoint; decide il chiamante (slice 4).
	syncNoCheckpoint
)

// errIsolateBranch e errFailRun sono i due esiti di classifySyncError:
// isolare e contare il singolo oggetto, oppure abortire l'intera run.
var (
	errIsolateBranch = errors.New("training: sync error isolates this branch")
	errFailRun       = errors.New("training: sync error fails the run")
)
