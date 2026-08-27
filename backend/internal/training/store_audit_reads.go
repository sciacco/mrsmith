package training

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Letture del pannello storia (#164, slice 5 del task 7). Sola lettura su
// training.audit_log: nessuna scrittura, nessuna ricostruzione della storia
// dei figli eliminati (un figlio eliminato semplicemente non compare piu'
// nella tabella viva che alimenta lo scope aggregato, quindi ne resta fuori
// per costruzione).

const auditRowColumns = `
  al.occurred_at::text,
  COALESCE(al.actor_id::text, ''),
  COALESCE(concat(ae.last_name, ' ', ae.first_name), ''),
  al.entity_type,
  al.entity_id::text,
  al.action,
  al.before_state,
  al.after_state`

const auditActorJoin = `LEFT JOIN training.employee ae ON ae.id = al.actor_id`

// AuditHistoryByEntity legge la storia di una singola entita' (regola,
// richiesta, o qualunque altro tipo dell'allow-list validato dall'handler).
func (s *SQLStore) AuditHistoryByEntity(ctx context.Context, entityType, entityID string, limit int) ([]AuditEntry, error) {
	query := fmt.Sprintf(`
SELECT%s
FROM training.audit_log al
%s
WHERE al.entity_type = $1 AND al.entity_id = $2::uuid
ORDER BY al.occurred_at DESC, al.id DESC
LIMIT $3`, auditRowColumns, auditActorJoin)
	rows, err := s.db.QueryContext(ctx, query, entityType, entityID, limit)
	if err != nil {
		return nil, fmt.Errorf("list training audit entries: %w", err)
	}
	defer rows.Close()
	return scanAuditRows(rows)
}

// AuditHistoryByEmployee e' la storia unita della persona (RATIFICATO #164):
// anagrafica, appartenenze, conseguimenti e attestati della persona. Le
// iscrizioni restano fuori: si leggono dal lato evento, come da contratto.
func (s *SQLStore) AuditHistoryByEmployee(ctx context.Context, employeeID string, limit int) ([]AuditEntry, error) {
	const query = `
WITH scope(entity_type, entity_id) AS (
  SELECT 'employee'::text, $1::uuid
  UNION ALL
  SELECT 'team_membership', tm.id FROM training.team_membership tm WHERE tm.employee_id = $1::uuid
  UNION ALL
  SELECT 'certification_award', ca.id FROM training.certification_award ca WHERE ca.employee_id = $1::uuid
  UNION ALL
  SELECT 'document', d.id FROM training.document d
    JOIN training.certification_award ca ON ca.id = d.certification_award_id
    WHERE ca.employee_id = $1::uuid
)
SELECT` + auditRowColumns + `
FROM training.audit_log al
JOIN scope s ON s.entity_type = al.entity_type AND s.entity_id = al.entity_id
` + auditActorJoin + `
ORDER BY al.occurred_at DESC, al.id DESC
LIMIT $2`
	rows, err := s.db.QueryContext(ctx, query, employeeID, limit)
	if err != nil {
		return nil, fmt.Errorf("list training audit entries for employee: %w", err)
	}
	defer rows.Close()
	return scanAuditRows(rows)
}

// AuditHistoryByEvent e' la linea temporale aggregata dell'evento (forma
// scelta in questa slice, #164): evento e figli vivi (sessioni, iscrizioni,
// partecipazioni, spese, documenti delle iscrizioni). Le partecipazioni sono
// audite con l'id dell'iscrizione (store_participations.go): stesso insieme
// di id di "enrollment", nessuna colonna id propria da unire.
func (s *SQLStore) AuditHistoryByEvent(ctx context.Context, eventID string, limit int) ([]AuditEntry, error) {
	const query = `
WITH scope(entity_type, entity_id) AS (
  SELECT 'training_event'::text, $1::uuid
  UNION ALL
  SELECT 'training_session', ts.id FROM training.training_session ts WHERE ts.event_id = $1::uuid
  UNION ALL
  SELECT 'enrollment', en.id FROM training.enrollment en WHERE en.event_id = $1::uuid
  UNION ALL
  SELECT 'enrollment_session', en.id FROM training.enrollment en WHERE en.event_id = $1::uuid
  UNION ALL
  SELECT 'event_expense', ee.id FROM training.event_expense ee WHERE ee.event_id = $1::uuid
  UNION ALL
  SELECT 'document', d.id FROM training.document d
    JOIN training.enrollment en ON en.id = d.enrollment_id
    WHERE en.event_id = $1::uuid
)
SELECT` + auditRowColumns + `
FROM training.audit_log al
JOIN scope s ON s.entity_type = al.entity_type AND s.entity_id = al.entity_id
` + auditActorJoin + `
ORDER BY al.occurred_at DESC, al.id DESC
LIMIT $2`
	rows, err := s.db.QueryContext(ctx, query, eventID, limit)
	if err != nil {
		return nil, fmt.Errorf("list training audit entries for event: %w", err)
	}
	defer rows.Close()
	return scanAuditRows(rows)
}

func scanAuditRows(rows *sql.Rows) ([]AuditEntry, error) {
	result := make([]AuditEntry, 0)
	for rows.Next() {
		var (
			row           AuditEntry
			actorID       string
			actorName     string
			before, after []byte
		)
		if err := rows.Scan(&row.OccurredAt, &actorID, &actorName, &row.EntityType, &row.EntityID, &row.Action, &before, &after); err != nil {
			return nil, fmt.Errorf("scan training audit entry: %w", err)
		}
		row.ActorID = actorID
		// actor_id assente => l'employee della LEFT JOIN e' tutto NULL, e
		// concat(NULL, ' ', NULL) in Postgres da' comunque uno spazio (concat
		// e' null-safe): il trim e' necessario, non solo difensivo.
		if strings.TrimSpace(actorName) == "" {
			row.ActorName = "sistema"
		} else {
			row.ActorName = actorName
		}
		row.Before = json.RawMessage(before)
		row.After = json.RawMessage(after)
		row.ChangedFields, row.ValuesRecorded = changedAuditFields(row.Before, row.After)
		result = append(result, row)
	}
	return result, rows.Err()
}

// auditRawValueField e' il campo sintetico restituito da changedAuditFields
// quando before/after sono JSON validi ma non oggetti (es. l'array scritto
// da pathStepsSnapshot per learning_path/replace_steps, store_paths.go): non
// ci sono chiavi da confrontare per campo, ma il prima/dopo grezzo esiste
// davvero. Il frontend (HistoryPanel.tsx) lo riconosce e mostra l'intero
// before/after invece di indicizzarlo per chiave.
const auditRawValueField = "valori"

// changedAuditFields distilla i nomi dei campi cambiati e segnala se i valori
// sono stati registrati: se after e' il payload compatto scritto da
// auditFields ({"changed_fields": [...]}), before e' sempre NULL (vedi
// store_mutations.go/auditFields) e solo i nomi dei campi sono noti —
// valuesRecorded torna false. Altrimenti confronta le chiavi di before e
// after (snapshot completi scritti da s.audit), incluse le chiavi comparse o
// scomparse, e valuesRecorded torna true. Quando before o after e' un JSON
// valido ma non un oggetto (un array, tipicamente), non ci sono chiavi da
// confrontare: senza questo caso a parte il confronto per chiavi produce
// silenziosamente changedFields=[] con valuesRecorded=true, e la riga
// risulta non espandibile pur avendo un prima/dopo reale in audit_log.
func changedAuditFields(before, after json.RawMessage) ([]string, bool) {
	if len(before) == 0 || string(before) == "null" {
		var wrapper struct {
			ChangedFields []string `json:"changed_fields"`
		}
		if len(after) > 0 && json.Unmarshal(after, &wrapper) == nil && wrapper.ChangedFields != nil {
			return wrapper.ChangedFields, false
		}
	}
	beforeMap, beforeNotObject := decodeJSONObject(before)
	afterMap, afterNotObject := decodeJSONObject(after)
	if beforeNotObject || afterNotObject {
		return []string{auditRawValueField}, true
	}
	seen := make(map[string]bool, len(beforeMap)+len(afterMap))
	fields := make([]string, 0, len(beforeMap)+len(afterMap))
	for key, afterValue := range afterMap {
		beforeValue, existed := beforeMap[key]
		if !existed || !bytes.Equal(bytes.TrimSpace(beforeValue), bytes.TrimSpace(afterValue)) {
			if !seen[key] {
				fields = append(fields, key)
				seen[key] = true
			}
		}
	}
	for key := range beforeMap {
		if _, stillPresent := afterMap[key]; !stillPresent && !seen[key] {
			fields = append(fields, key)
			seen[key] = true
		}
	}
	sort.Strings(fields)
	return fields, true
}

// decodeJSONObject decodifica raw come oggetto JSON. Il secondo valore di
// ritorno distingue "assente/null" (false, nessuna chiave da confrontare ma
// non e' un errore: unmarshal di "null" in una mappa non fallisce) da
// "presente, valido, ma non un oggetto" (true, es. un array): solo quel
// secondo caso segnala a changedAuditFields che il confronto per chiavi non
// e' applicabile.
func decodeJSONObject(raw json.RawMessage) (obj map[string]json.RawMessage, notObject bool) {
	if len(raw) == 0 {
		return nil, false
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, true
	}
	return obj, false
}
