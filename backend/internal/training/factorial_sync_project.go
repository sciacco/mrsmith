package training

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/pkg/factorial"
)

// Costruttori, mapping, decisione 3-way, correlatori e classificazione
// errori: le funzioni pure che operano sui tipi di factorial_sync_types.go.

// newSessionSyncStateFromLocal proietta le colonne locali di
// training.training_session (schedule_type, starts_at, ends_at, due_at). I
// puntatori nil rappresentano NULL.
func newSessionSyncStateFromLocal(scheduleType *string, startsAt, endsAt, dueAt *time.Time) sessionSyncState {
	return sessionSyncState{
		ScheduleType: deref(scheduleType),
		StartsAt:     formatInstantUTC(startsAt),
		EndsAt:       formatInstantUTC(endsAt),
		DueDate:      formatDateUTC(dueAt),
	}
}

// newSessionSyncStateFromRemote proietta una factorial.TrainingsSession.
// "selfpaced" viene normalizzato a "self_paced".
func newSessionSyncStateFromRemote(remote factorial.TrainingsSession) sessionSyncState {
	state := sessionSyncState{ScheduleType: normalizeRemoteSchedule(remote.Schedule)}
	if remote.StartsAt != nil {
		state.StartsAt = formatInstantUTC(&remote.StartsAt.Time)
	}
	if remote.EndsAt != nil {
		state.EndsAt = formatInstantUTC(&remote.EndsAt.Time)
	}
	if remote.DueDate != nil {
		state.DueDate = formatDateUTC(&remote.DueDate.Time)
	}
	return state
}

// formatInstantUTC normalizza un istante in UTC e lo formatta RFC3339; nil o
// zero-value tornano "" (null).
func formatInstantUTC(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// formatDateUTC riduce un istante alla sola data (UTC) in formato
// YYYY-MM-DD; nil o zero-value tornano "" (null).
func formatDateUTC(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func normalizeRemoteSchedule(schedule *factorial.TrainingsSessionSchedule) string {
	if schedule == nil {
		return ""
	}
	if *schedule == factorial.TrainingsSessionScheduleSelfpaced {
		return "self_paced"
	}
	return string(*schedule)
}

// attendanceLocalFromRemote mappa lo stato Factorial (SessionAttendance) al
// vocabolario locale (participation_status). Sconosciuto -> errore, mai panic.
func attendanceLocalFromRemote(remote factorial.TrainingsSessionAttendanceStatus) (string, error) {
	switch remote {
	case factorial.TrainingsSessionAttendanceStatusPending:
		return participationAssigned, nil
	case factorial.TrainingsSessionAttendanceStatusInprogress:
		return participationInProgress, nil
	case factorial.TrainingsSessionAttendanceStatusCompleted:
		return participationCompleted, nil
	case factorial.TrainingsSessionAttendanceStatusMissing:
		return participationNotAttended, nil
	default:
		return "", fmt.Errorf("training: unknown remote attendance status %q", remote)
	}
}

// attendanceRemoteFromLocal e l'inverso di attendanceLocalFromRemote.
func attendanceRemoteFromLocal(local string) (factorial.TrainingsSessionAttendanceStatus, error) {
	switch local {
	case participationAssigned:
		return factorial.TrainingsSessionAttendanceStatusPending, nil
	case participationInProgress:
		return factorial.TrainingsSessionAttendanceStatusInprogress, nil
	case participationCompleted:
		return factorial.TrainingsSessionAttendanceStatusCompleted, nil
	case participationNotAttended:
		return factorial.TrainingsSessionAttendanceStatusMissing, nil
	default:
		return "", fmt.Errorf("training: unknown local participation status %q", local)
	}
}

// threeWayDecision confronta locale, remoto e l'ultimo checkpoint noto su un
// valore proiettato confrontabile e restituisce quale lato (se alcuno) va
// propagato. checkpoint nil = nessun checkpoint precedente.
func threeWayDecision[T comparable](local, remote T, checkpoint *T) syncDecision {
	if checkpoint == nil {
		return syncNoCheckpoint
	}
	switch {
	case local == remote:
		return syncInSync
	case local == *checkpoint:
		return syncPropagateRemote
	case remote == *checkpoint:
		return syncPropagateLocal
	default:
		return syncConflict
	}
}

// factorialNameMaxLen limita la lunghezza (in rune) dei nomi Factorial di
// classi/sessioni create dal reconciler. Non documentato nello spec
// dell'issue: valore ipotesi, da riverificare prima della create live (slice 5+).
const factorialNameMaxLen = 255

// factorialClassName costruisce il nome Factorial di una classe creata da un
// evento locale: titolo + spazio + token [MS:<uuid>].
func factorialClassName(title, eventID string) string {
	return buildCorrelatedName(title, msToken(eventID))
}

// factorialSessionName costruisce il nome Factorial di una sessione creata
// da un evento locale: titolo del corso + spazio + token [MS:<uuid>].
func factorialSessionName(courseTitle, sessionID string) string {
	return buildCorrelatedName(courseTitle, msToken(sessionID))
}

// msToken e il correlatore embeddabile nei nomi Factorial: [MS:<uuid>] con
// l'UUID locale di evento o sessione. Formato fissato, mai troncato.
func msToken(localID string) string {
	return "[MS:" + localID + "]"
}

// buildCorrelatedName compone titolo+spazio+token, troncando solo il titolo
// (mai il token, mai a meta di un rune) se il risultato supera
// factorialNameMaxLen rune.
func buildCorrelatedName(title, token string) string {
	name := title + " " + token
	if len([]rune(name)) <= factorialNameMaxLen {
		return name
	}
	titleRunes := []rune(title)
	keep := factorialNameMaxLen - len([]rune(token)) - 1 // spazio separatore
	if keep < 0 {
		keep = 0
	}
	if keep > len(titleRunes) {
		keep = len(titleRunes)
	}
	return string(titleRunes[:keep]) + " " + token
}

// matchCorrelator restituisce gli indici di candidates che contengono token
// per sottostringa esatta, case-sensitive (0/1/>1 = crea/adotta/conflitto,
// semantica applicata dai chiamanti in slice 5).
func matchCorrelator(candidates []string, token string) []int {
	var matches []int
	for i, candidate := range candidates {
		if strings.Contains(candidate, token) {
			matches = append(matches, i)
		}
	}
	return matches
}

// Codici di stato HTTP usati dalla classificazione. Duplicati qui invece di
// importare net/http, vietato nei file nuovi di questa slice.
const (
	statusBadRequest          = 400
	statusUnprocessableEntity = 422
)

// classifySyncError classifica un errore Factorial: *factorial.APIError
// 400/422 isola il ramo; 401/403/429/5xx, trasporto o paginazione fanno
// fallire la run. Il 404 non passa da qui: i chiamanti lo gestiscono
// esplicitamente (missing/delete, slice 6).
func classifySyncError(err error) error {
	if apiErr, ok := errors.AsType[*factorial.APIError](err); ok {
		switch apiErr.StatusCode {
		case statusBadRequest, statusUnprocessableEntity:
			return errIsolateBranch
		}
	}
	return errFailRun
}
