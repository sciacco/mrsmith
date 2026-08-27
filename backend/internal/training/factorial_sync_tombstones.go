package training

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/sciacco/mrsmith/pkg/factorial"
)

// Tombstone dalle delete locali gia' auditate e gestione del missing remoto
// (#141, slice 6/8 = #148). Nessuna nuova tabella: training.audit_log e' il
// tombstone durevole (non si purga mai). Tre parti: selezione dei tombstone
// dalle ultime action delete/remove con riga locale assente (T1), propagazione
// della delete a Factorial (access poi sessioni, 404=successo, T2), e
// risoluzione del missing remoto sui sei tipi Factorial via Get(id) di
// conferma (T4). filterTombstonedGraph (T3) espone il filtro pre-diff-inbound
// che la run (slice 7) applichera' al grafo fetchato.

// actionFactorialMissingReset e' l'action di audit per l'azzeramento di un id
// remoto quando il 404 e' confermato su un oggetto nato localmente e ancora
// attivo (T4): mai una delete, solo lo scollegamento dal remoto defunto.
const actionFactorialMissingReset = "factorial_missing_reset"

// Esiti di decideMissingRemote.
const (
	missingReset         = "reset"
	missingKeepImported  = "keep_imported"
	missingKeepInactive  = "keep_inactive"
	missingKeepAmbiguous = "keep_ambiguous"
)

// auditDeleteRow e' una candidatura di tombstone letta da training.audit_log
// (T1): EntityID e' la riga cancellata (session id, o enrollment id per gli
// access), SessionID e' "" per le sessioni e il session_id di before_state
// per gli access (un'iscrizione ha piu' sessioni, l'audit le disambigua solo
// li'). RemoteID viene da before_state (puo' essere vuoto). RowExists
// conferma, con una query bulk, se la riga locale esiste ancora ora.
type auditDeleteRow struct {
	EntityID  string
	SessionID string
	RemoteID  string
	RowExists bool
}

// missingRow e' un oggetto locale gia' collegato a un id Factorial, da
// verificare quando assente dal grafo fetchato (T4): Kind seleziona il
// present-set e la Get di conferma nell'orchestratore; Reset azzera l'id (con
// audit) quando il 404 e' confermato su un oggetto nato localmente e attivo.
// Ambiguous segnala provenienza non derivabile (nessun audit dedicato, o
// piu' sessioni sulla stessa iscrizione): vince su tutto, conservativa.
type missingRow struct {
	Kind      string
	RemoteID  string
	Active    bool
	Imported  bool
	Ambiguous bool
	Reset     func(context.Context) error
}

// tombstoneResult e' l'esito completo della slice 6: gli insiemi di ID remoti
// tombstoned (esposti per il filtro pre-diff-inbound di slice 7), i conteggi
// di propagazione/missing (pianificati in dry-run, mai eseguiti) e i warning.
type tombstoneResult struct {
	TombstonedSessionIDs map[string]struct{}
	TombstonedAccessIDs  map[string]struct{}
	AccessDestroyed      int
	SessionsDeleted      int
	ResetCount           int
	Warnings             []outboundIssue
}

// --- T1: selezione tombstone -----------------------------------------------

// bulkAuditDeleteRows legge, in due query bulk (nessun N+1), tutte le action
// training_session/delete ed enrollment_session/remove, in ordine cronologico
// crescente: reduceTombstones si affida a quest'ordine per tenere solo
// l'ultima candidatura per entita'. RowExists conferma via EXISTS bulk se la
// riga locale e' stata ricreata dopo quella action.
func (s *SQLStore) bulkAuditDeleteRows(ctx context.Context, q sqlRunner) (sessions, access []auditDeleteRow, err error) {
	sessionRows, err := q.QueryContext(ctx, `
SELECT al.entity_id::text, COALESCE(al.before_state->>'factorial_session_id', ''),
  EXISTS (SELECT 1 FROM training.training_session ts WHERE ts.id = al.entity_id)
FROM training.audit_log al
WHERE al.entity_type = 'training_session' AND al.action = 'delete'
ORDER BY al.occurred_at ASC, al.id ASC`)
	if err != nil {
		return nil, nil, fmt.Errorf("list training_session delete audit: %w", err)
	}
	defer sessionRows.Close()
	for sessionRows.Next() {
		var r auditDeleteRow
		if err := sessionRows.Scan(&r.EntityID, &r.RemoteID, &r.RowExists); err != nil {
			return nil, nil, fmt.Errorf("scan training_session delete audit: %w", err)
		}
		sessions = append(sessions, r)
	}
	if err := sessionRows.Err(); err != nil {
		return nil, nil, err
	}

	accessRows, err := q.QueryContext(ctx, `
SELECT al.entity_id::text, COALESCE(al.before_state->>'session_id', ''),
  COALESCE(al.before_state->>'factorial_access_membership_id', ''),
  EXISTS (
    SELECT 1 FROM training.enrollment_session es
    WHERE es.enrollment_id = al.entity_id AND es.session_id::text = al.before_state->>'session_id'
  )
FROM training.audit_log al
WHERE al.entity_type = 'enrollment_session' AND al.action = 'remove'
ORDER BY al.occurred_at ASC, al.id ASC`)
	if err != nil {
		return nil, nil, fmt.Errorf("list enrollment_session remove audit: %w", err)
	}
	defer accessRows.Close()
	for accessRows.Next() {
		var r auditDeleteRow
		if err := accessRows.Scan(&r.EntityID, &r.SessionID, &r.RemoteID, &r.RowExists); err != nil {
			return nil, nil, fmt.Errorf("scan enrollment_session remove audit: %w", err)
		}
		access = append(access, r)
	}
	return sessions, access, accessRows.Err()
}

// reduceTombstones riduce le candidature (in ordine cronologico crescente,
// garantito da bulkAuditDeleteRows) all'ultima per entita' (EntityID+
// SessionID): vince solo se la riga locale e' ancora assente ora
// (RowExists=false), cosi' una ricreazione successiva invalida il tombstone.
// before_state senza l'id remoto scarta la candidatura con un warning invece
// di entrare nell'insieme; localEntity e' il tipo locale di EntityID (gia'
// noto al chiamante), per la persistenza dei finding (#154).
func reduceTombstones(rows []auditDeleteRow, warnKind, localEntity string) (map[string]struct{}, []outboundIssue) {
	type key struct{ entity, session string }
	last := map[key]auditDeleteRow{}
	order := make([]key, 0, len(rows))
	for _, r := range rows {
		k := key{r.EntityID, r.SessionID}
		if _, seen := last[k]; !seen {
			order = append(order, k)
		}
		last[k] = r
	}
	ids := map[string]struct{}{}
	var warnings []outboundIssue
	for _, k := range order {
		r := last[k]
		if r.RowExists {
			continue
		}
		if r.RemoteID == "" {
			warnings = append(warnings, outboundIssue{Kind: warnKind, Ref: r.EntityID, LocalEntity: localEntity, LocalID: r.EntityID})
			continue
		}
		ids[r.RemoteID] = struct{}{}
	}
	return ids, warnings
}

// --- T2: propagazione delete -------------------------------------------------

// applyTombstonePropagation cancella su Factorial cio' che e' gia' stato
// cancellato localmente: prima gli access (BulkDestroy, una chiamata per
// tutto il batch), poi le sessioni (Delete, un id per chiamata: il client non
// offre un bulk per le sessioni). Un 404 vale successo (gia' assente); ogni
// altro errore segue la classificazione (mai su errore nil) e isola il
// singolo oggetto, propagando l'errore originale nel warning. In dry-run:
// solo conteggi, nessuna chiamata.
func applyTombstonePropagation(ctx context.Context, cli *factorial.Client, sessionIDs, accessIDs map[string]struct{}, dryRun bool, result *tombstoneResult) error {
	if len(accessIDs) > 0 {
		ids := make([]string, 0, len(accessIDs))
		for id := range accessIDs {
			ids = append(ids, id)
		}
		sort.Strings(ids) // Ref deterministico nel warning
		if dryRun {
			result.AccessDestroyed = len(ids)
		} else if err := destroyAccessBatch(ctx, cli, ids); err != nil {
			issue, failErr := classifyOutbound("access_bulk_destroy_failed", strings.Join(ids, ","), err)
			if failErr != nil {
				return failErr
			}
			result.Warnings = append(result.Warnings, issue)
		} else {
			result.AccessDestroyed = len(ids)
		}
	}
	if dryRun {
		result.SessionsDeleted = len(sessionIDs)
		return nil
	}
	for id := range sessionIDs {
		_, err := cli.Trainings.Sessions.Delete(ctx, id)
		if err == nil {
			result.SessionsDeleted++
			continue
		}
		if apiErr, ok := errors.AsType[*factorial.APIError](err); ok && apiErr.IsNotFound() {
			result.SessionsDeleted++
			continue
		}
		issue, failErr := classifyOutbound("session_delete_failed", id, err)
		if failErr != nil {
			return failErr
		}
		result.Warnings = append(result.Warnings, issue)
	}
	return nil
}

// destroyAccessBatch esegue la BulkDestroy e riduce un 404 a successo (nil):
// gia' assente vale cancellato.
func destroyAccessBatch(ctx context.Context, cli *factorial.Client, ids []string) error {
	_, err := cli.Trainings.SessionAccessMemberships.BulkDestroy(ctx, &factorial.TrainingsSessionAccessMembershipsBulkDestroyBody{IDs: ids})
	if err == nil {
		return nil
	}
	if apiErr, ok := errors.AsType[*factorial.APIError](err); ok && apiErr.IsNotFound() {
		return nil
	}
	return err
}

// --- T3: soppressione del reimport -------------------------------------------

// idSet estrae, da una lista di elementi del grafo Factorial, l'insieme dei
// loro ID non nil.
func idSet[T any](items []T, id func(T) *string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, it := range items {
		if v := id(it); v != nil {
			set[*v] = struct{}{}
		}
	}
	return set
}

// excludeByID filtra una lista del grafo Factorial escludendo gli elementi
// il cui ID e' nell'insieme escluso.
func excludeByID[T any](items []T, excluded map[string]struct{}, id func(T) *string) []T {
	if len(excluded) == 0 {
		return items
	}
	kept := make([]T, 0, len(items))
	for _, it := range items {
		if v := id(it); v != nil {
			if _, ok := excluded[*v]; ok {
				continue
			}
		}
		kept = append(kept, it)
	}
	return kept
}

// filterTombstonedGraph sopprime dal grafo fetchato le sessioni e gli access
// tombstoned: la run (slice 7) la applica prima del diff inbound, cosi' un
// oggetto cancellato localmente non risorge. Le presenze legate a un access
// soppresso escono gia' da sole a valle (trainingSubgraph le raggruppa per
// accessIDs derivati da sub.Access, factorial_sync_inbound.go).
func filterTombstonedGraph(graph factorialTrainingGraph, tombstonedSessionIDs, tombstonedAccessIDs map[string]struct{}) factorialTrainingGraph {
	filtered := graph
	filtered.Sessions = excludeByID(graph.Sessions, tombstonedSessionIDs, func(s factorial.TrainingsSession) *string { return s.ID })
	filtered.SessionAccessMemberships = excludeByID(graph.SessionAccessMemberships, tombstonedAccessIDs, func(a factorial.TrainingsSessionAccessMembership) *string { return a.ID })
	return filtered
}

// --- T4: missing remoto -------------------------------------------------------

// decideMissingRemote decide l'esito del 404 confermato: provenienza
// ambigua -> conservato con warning, sempre per prima (direzione
// conservativa: altrimenti cadrebbe nel ramo reset, distruttivo). Altrimenti:
// nato localmente (audit/origin != factorial_import) e attivo -> reset
// (l'outbound lo ricrea via correlatore); importato, o nato locale ma non
// piu' attivo -> conservato con warning. Mai cancella dati locali.
func decideMissingRemote(active, imported, ambiguous bool) string {
	switch {
	case ambiguous:
		return missingKeepAmbiguous
	case imported:
		return missingKeepImported
	case !active:
		return missingKeepInactive
	default:
		return missingReset
	}
}

// resolveMissingKinds verifica ogni oggetto gia' collegato ma assente dal
// present-set del suo Kind: una Get(id) di conferma, sempre eseguita anche in
// dry-run (e' una lettura che serve al report). Solo il 404 e' conclusivo;
// ogni altro errore segue la classificazione (mai su errore nil), propaga
// l'errore originale nel warning e isola il singolo oggetto. Al 404,
// decideMissingRemote sceglie reset o warning; in dry-run il reset resta un
// conteggio, nessuna scrittura.
func resolveMissingKinds(ctx context.Context, dryRun bool, rows []missingRow, present map[string]map[string]struct{}, getFns map[string]func(context.Context, string) error, result *tombstoneResult) error {
	for _, row := range rows {
		if _, ok := present[row.Kind][row.RemoteID]; ok {
			continue
		}
		err := getFns[row.Kind](ctx, row.RemoteID)
		if err == nil {
			continue // assente dal fetch ma esiste ancora: nessuna azione
		}
		apiErr, ok := errors.AsType[*factorial.APIError](err)
		if !ok || !apiErr.IsNotFound() {
			issue, failErr := classifyOutbound(row.Kind+"_missing_check_failed", row.RemoteID, err)
			if failErr != nil {
				return failErr
			}
			result.Warnings = append(result.Warnings, issue)
			continue
		}
		switch decideMissingRemote(row.Active, row.Imported, row.Ambiguous) {
		case missingReset:
			if dryRun {
				result.ResetCount++
				continue
			}
			if err := row.Reset(ctx); err != nil {
				return err
			}
			result.ResetCount++
		case missingKeepAmbiguous:
			result.Warnings = append(result.Warnings, outboundIssue{Kind: row.Kind + "_missing_remote_provenance_unknown", Ref: row.RemoteID})
		case missingKeepImported:
			result.Warnings = append(result.Warnings, outboundIssue{Kind: row.Kind + "_missing_remote_imported", Ref: row.RemoteID})
		default:
			result.Warnings = append(result.Warnings, outboundIssue{Kind: row.Kind + "_missing_remote_inactive", Ref: row.RemoteID})
		}
	}
	return nil
}

// resetFactorialLink azzera una o piu' colonne id/checkpoint remote su una
// riga locale a chiave singola (course/training_event/training_session/
// enrollment), in una tx breve dedicata, con audit. Mai una delete: solo lo
// scollegamento da un remoto defunto.
func (s *SQLStore) resetFactorialLink(ctx context.Context, principal Principal, table, id string, columns []string) error {
	sets := make([]string, len(columns))
	for i, c := range columns {
		sets[i] = c + " = NULL"
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		q := fmt.Sprintf(`UPDATE training.%s SET %s, updated_at = now() WHERE id = $1::uuid`, table, strings.Join(sets, ", "))
		if _, err := tx.ExecContext(ctx, q, id); err != nil {
			return fmt.Errorf("reset %s missing remote link: %w", table, err)
		}
		return s.auditFields(ctx, tx, principal, table, id, actionFactorialMissingReset, columns)
	})
}

// resetEnrollmentSessionLink e' la variante a chiave composita di
// resetFactorialLink, per access/attendance su enrollment_session.
func (s *SQLStore) resetEnrollmentSessionLink(ctx context.Context, principal Principal, enrollmentID, sessionID string, columns []string) error {
	sets := make([]string, len(columns))
	for i, c := range columns {
		sets[i] = c + " = NULL"
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		q := fmt.Sprintf(`UPDATE training.enrollment_session SET %s, updated_at = now() WHERE enrollment_id = $1::uuid AND session_id = $2::uuid`, strings.Join(sets, ", "))
		if _, err := tx.ExecContext(ctx, q, enrollmentID, sessionID); err != nil {
			return fmt.Errorf("reset enrollment_session missing remote link: %w", err)
		}
		return s.auditFields(ctx, tx, principal, "enrollment_session", enrollmentID, actionFactorialMissingReset, columns)
	})
}

// missingResetColumns mappa Kind -> colonne da azzerare per resetFactorialLink
// (Kind coincide col nome tabella per i quattro tipi a chiave singola).
var missingResetColumns = map[string][]string{
	"course":           {"factorial_training_id"},
	"training_event":   {"factorial_class_id"},
	"training_session": {"factorial_session_id", "factorial_synced_state"},
	"enrollment":       {"factorial_training_membership_id"},
}

// bulkMissingLinked legge, in una query bulk (UNION ALL, nessun N+1), i
// quattro tipi a chiave singola (Trainings, TrainingClasses, Sessions,
// TrainingMemberships): id remoto, "ancora attivo", provenienza.
// training_event espone gia' origin; course/training_session la derivano
// dalla prima action di audit per l'entita' (nessun campo strutturale).
// enrollment usa la prima action che ha toccato factorial_training_membership_id
// nello specifico (origin e' dell'iscrizione, non del legame): se assente, ambigua.
func (s *SQLStore) bulkMissingLinked(ctx context.Context, q sqlRunner, principal Principal) ([]missingRow, error) {
	rows, err := q.QueryContext(ctx, `
SELECT 'course', c.id::text, c.factorial_training_id, c.is_active, COALESCE(fa.action, '') = 'factorial_import', false
FROM training.course c
LEFT JOIN LATERAL (
  SELECT action FROM training.audit_log WHERE entity_type = 'course' AND entity_id = c.id ORDER BY occurred_at ASC, id ASC LIMIT 1
) fa ON true
WHERE c.factorial_training_id IS NOT NULL
UNION ALL
SELECT 'training_event', ev.id::text, ev.factorial_class_id, ev.cancelled_at IS NULL, ev.origin = 'factorial_import', false
FROM training.training_event ev
WHERE ev.factorial_class_id IS NOT NULL
UNION ALL
SELECT 'training_session', s.id::text, s.factorial_session_id, tev.cancelled_at IS NULL, COALESCE(fa2.action, '') = 'factorial_import', false
FROM training.training_session s
JOIN training.training_event tev ON tev.id = s.event_id
LEFT JOIN LATERAL (
  SELECT action FROM training.audit_log WHERE entity_type = 'training_session' AND entity_id = s.id ORDER BY occurred_at ASC, id ASC LIMIT 1
) fa2 ON true
WHERE s.factorial_session_id IS NOT NULL
UNION ALL
SELECT 'enrollment', en.id::text, en.factorial_training_membership_id, en.delivery_status <> 'cancelled',
  COALESCE(fa3.action, '') = 'factorial_import', fa3.action IS NULL
FROM training.enrollment en
LEFT JOIN LATERAL (
  SELECT action FROM training.audit_log
  WHERE entity_type = 'enrollment' AND entity_id = en.id AND (after_state->'changed_fields') ? 'factorial_training_membership_id'
  ORDER BY occurred_at ASC, id ASC LIMIT 1
) fa3 ON true
WHERE en.factorial_training_membership_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("list missing-check linked entities: %w", err)
	}
	defer rows.Close()
	var out []missingRow
	for rows.Next() {
		var kind, id, remoteID string
		var active, imported, ambiguous bool
		if err := rows.Scan(&kind, &id, &remoteID, &active, &imported, &ambiguous); err != nil {
			return nil, fmt.Errorf("scan missing-check linked entity: %w", err)
		}
		columns := missingResetColumns[kind]
		out = append(out, missingRow{Kind: kind, RemoteID: remoteID, Active: active, Imported: imported, Ambiguous: ambiguous,
			Reset: func(ctx context.Context) error { return s.resetFactorialLink(ctx, principal, kind, id, columns) }})
	}
	return out, rows.Err()
}

// bulkMissingEnrollmentSessions legge, in una query bulk, gli
// enrollment_session gia' collegati (SessionAccessMemberships,
// SessionAttendances): stessa riga, due id remoti. Provenienza = prima
// action di audit per l'iscrizione (entity_id = enrollment_id): con piu'
// sessioni sulla stessa iscrizione non si sa a quale si riferisca ->
// Ambiguous (conta via PK di enrollment_session, nessun N+1).
func (s *SQLStore) bulkMissingEnrollmentSessions(ctx context.Context, q sqlRunner, principal Principal) ([]missingRow, error) {
	rows, err := q.QueryContext(ctx, `
SELECT es.enrollment_id::text, es.session_id::text, COALESCE(es.factorial_access_membership_id, ''), COALESCE(es.factorial_attendance_id, ''),
  en.delivery_status <> 'cancelled', COALESCE(fa.action, '') = 'factorial_import',
  (SELECT count(*) FROM training.enrollment_session x WHERE x.enrollment_id = es.enrollment_id) > 1
FROM training.enrollment_session es
JOIN training.enrollment en ON en.id = es.enrollment_id
LEFT JOIN LATERAL (
  SELECT action FROM training.audit_log WHERE entity_type = 'enrollment_session' AND entity_id = es.enrollment_id ORDER BY occurred_at ASC, id ASC LIMIT 1
) fa ON true
WHERE es.factorial_access_membership_id IS NOT NULL OR es.factorial_attendance_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("list missing-check enrollment sessions: %w", err)
	}
	defer rows.Close()
	var out []missingRow
	for rows.Next() {
		var enrollmentID, sessionID, accessID, attendanceID string
		var active, imported, ambiguous bool
		if err := rows.Scan(&enrollmentID, &sessionID, &accessID, &attendanceID, &active, &imported, &ambiguous); err != nil {
			return nil, fmt.Errorf("scan missing-check enrollment session: %w", err)
		}
		if accessID != "" {
			out = append(out, missingRow{Kind: "access", RemoteID: accessID, Active: active, Imported: imported, Ambiguous: ambiguous,
				Reset: func(ctx context.Context) error {
					return s.resetEnrollmentSessionLink(ctx, principal, enrollmentID, sessionID, []string{"factorial_access_membership_id"})
				}})
		}
		if attendanceID != "" {
			out = append(out, missingRow{Kind: "attendance", RemoteID: attendanceID, Active: active, Imported: imported, Ambiguous: ambiguous,
				Reset: func(ctx context.Context) error {
					return s.resetEnrollmentSessionLink(ctx, principal, enrollmentID, sessionID, []string{"factorial_attendance_id", "factorial_synced_status"})
				}})
		}
	}
	return out, rows.Err()
}

// --- Orchestrazione ------------------------------------------------------------

// applyTombstoneSync e' l'entry point della slice 6 (T1+T2+T4): seleziona i
// tombstone dalle ultime action di audit delete/remove con riga locale
// assente, propaga la delete su Factorial (access poi sessioni, 404=successo)
// e risolve il missing remoto sui sei tipi Factorial confermando via Get(id).
// graph e' il grafo fetchato per intero (non il perimetro attivi, stesso
// principio di applyOutboundSync): l'assenza dal fetch e' cio' che conta per
// "missing remoto", non l'esclusione per employee inattivo. Le chiamate
// Factorial restano fuori da ogni transazione DB; dryRun sospende solo le
// scritture (BulkDestroy/Delete/reset), mai le letture di conferma.
func (s *SQLStore) applyTombstoneSync(ctx context.Context, cli *factorial.Client, graph factorialTrainingGraph, dryRun bool) (tombstoneResult, error) {
	if s == nil || s.db == nil {
		return tombstoneResult{}, errors.New("training database not configured")
	}
	if cli == nil {
		return tombstoneResult{}, errors.New("factorial client not configured")
	}
	principal := Principal{IsPeopleAdmin: true}

	sessionRows, accessRows, err := s.bulkAuditDeleteRows(ctx, s.db)
	if err != nil {
		return tombstoneResult{}, err
	}
	sessionIDs, sessionWarnings := reduceTombstones(sessionRows, "session_tombstone_missing_remote_id", "training_session")
	accessIDs, accessWarnings := reduceTombstones(accessRows, "access_tombstone_missing_remote_id", "enrollment_session")
	result := tombstoneResult{TombstonedSessionIDs: sessionIDs, TombstonedAccessIDs: accessIDs}
	result.Warnings = append(result.Warnings, sessionWarnings...)
	result.Warnings = append(result.Warnings, accessWarnings...)

	if err := applyTombstonePropagation(ctx, cli, sessionIDs, accessIDs, dryRun, &result); err != nil {
		return result, err
	}

	linked, err := s.bulkMissingLinked(ctx, s.db, principal)
	if err != nil {
		return result, err
	}
	enrollmentSessions, err := s.bulkMissingEnrollmentSessions(ctx, s.db, principal)
	if err != nil {
		return result, err
	}

	present := map[string]map[string]struct{}{
		"course":           idSet(graph.Trainings, func(t factorial.TrainingsTraining) *string { return t.ID }),
		"training_event":   idSet(graph.TrainingClasses, func(c factorial.TrainingsTrainingClass) *string { return c.ID }),
		"training_session": idSet(graph.Sessions, func(sess factorial.TrainingsSession) *string { return sess.ID }),
		"enrollment":       idSet(graph.TrainingMemberships, func(m factorial.TrainingsTrainingMembership) *string { return m.ID }),
		"access":           idSet(graph.SessionAccessMemberships, func(a factorial.TrainingsSessionAccessMembership) *string { return a.ID }),
		"attendance":       idSet(graph.SessionAttendances, func(a factorial.TrainingsSessionAttendance) *string { return a.ID }),
	}
	getFns := map[string]func(context.Context, string) error{
		"course": func(ctx context.Context, id string) error { _, err := cli.Trainings.Trainings.Get(ctx, id); return err },
		"training_event": func(ctx context.Context, id string) error {
			_, err := cli.Trainings.TrainingClasses.Get(ctx, id)
			return err
		},
		"training_session": func(ctx context.Context, id string) error { _, err := cli.Trainings.Sessions.Get(ctx, id); return err },
		"enrollment": func(ctx context.Context, id string) error {
			_, err := cli.Trainings.TrainingMemberships.Get(ctx, id)
			return err
		},
		"access": func(ctx context.Context, id string) error {
			_, err := cli.Trainings.SessionAccessMemberships.Get(ctx, id)
			return err
		},
		"attendance": func(ctx context.Context, id string) error {
			_, err := cli.Trainings.SessionAttendances.Get(ctx, id)
			return err
		},
	}
	rows := append(linked, enrollmentSessions...)
	if err := resolveMissingKinds(ctx, dryRun, rows, present, getFns, &result); err != nil {
		return result, err
	}
	return result, nil
}
