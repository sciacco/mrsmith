package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Persistenza e letture delle run del sync Factorial (#154, task 6.3 di
// #151; parent #141): la migrazione 133 introduce factorial_sync_run e
// factorial_sync_finding. Emendamento ratificato alla #141: qui si traccia
// tutto, dati personali compresi; la sanitizzazione (campioni <=20, soli id
// tecnici) resta solo nei log strutturati (factorial_sync_run.go).

// factorialSyncFindingRow e' una riga preparata per l'INSERT dei finding,
// prima di conoscere l'id della run appena creata.
type factorialSyncFindingRow struct {
	phase, severity, kind, ref, localEntity, localID, employeeID string
	detail                                                       []byte
}

// factorialSyncCounters riassume in un'unica mappa i contatori aggregati di
// tombstone/inbound/outbound, persistiti come counters jsonb sulla run
// (stessi numeri del log strutturato di RunFactorialSync.finish).
func factorialSyncCounters(report FactorialSyncRun) map[string]int {
	counters := map[string]int{}
	if report.Tombstone != nil {
		counters["tombstoneAccessDestroyed"] = report.Tombstone.AccessDestroyed
		counters["tombstoneSessionsDeleted"] = report.Tombstone.SessionsDeleted
		counters["tombstoneReset"] = report.Tombstone.ResetCount
		counters["tombstoneWarnings"] = len(report.Tombstone.Warnings)
	}
	if report.Inbound != nil {
		counters["inboundConflicts"] = len(report.Inbound.Conflicts)
		counters["inboundWarnings"] = len(report.Inbound.Warnings)
		counters["inboundRDAExceptions"] = report.Inbound.RDAExceptions
	}
	if report.Outbound != nil {
		counters["outboundConflicts"] = len(report.Outbound.Conflicts)
		counters["outboundWarnings"] = len(report.Outbound.Warnings)
		counters["outboundPlanned"] = len(report.Outbound.Planned)
		counters["outboundWrites"] = len(report.Outbound.Writes)
	}
	return counters
}

// factorialSyncFindingRows appiattisce Tombstone/Inbound/Outbound del
// report nelle righe da persistere: il ramo tombstone produce solo warning
// (nessun Conflicts li', applyTombstoneSync non ne genera), inbound/outbound
// distinguono Conflicts (severity=conflict) da Warnings (severity=warning,
// eccezioni RDA comprese: gia' dentro Inbound.Warnings). Planned resta solo
// nel report in memoria (dry-run): e' un piano, non un esito, e la tabella
// non ha una severity per quello.
func factorialSyncFindingRows(report FactorialSyncRun) ([]factorialSyncFindingRow, error) {
	var rows []factorialSyncFindingRow
	appendOutbound := func(phase, severity string, issues []outboundIssue) error {
		for _, issue := range issues {
			row := factorialSyncFindingRow{
				phase: phase, severity: severity, kind: issue.Kind, ref: issue.Ref,
				localEntity: issue.LocalEntity, localID: issue.LocalID,
			}
			if issue.Err != nil {
				detail, err := json.Marshal(map[string]string{"error": issue.Err.Error()})
				if err != nil {
					return fmt.Errorf("marshal factorial sync finding detail: %w", err)
				}
				row.detail = detail
			}
			rows = append(rows, row)
		}
		return nil
	}
	appendInbound := func(phase, severity string, issues []inboundIssue) {
		for _, issue := range issues {
			rows = append(rows, factorialSyncFindingRow{
				phase: phase, severity: severity, kind: issue.Kind, ref: issue.Ref,
				localEntity: issue.LocalEntity, localID: issue.LocalID, employeeID: issue.EmployeeID,
			})
		}
	}
	if report.Tombstone != nil {
		if err := appendOutbound("tombstone", "warning", report.Tombstone.Warnings); err != nil {
			return nil, err
		}
	}
	if report.Inbound != nil {
		appendInbound("inbound", "conflict", report.Inbound.Conflicts)
		appendInbound("inbound", "warning", report.Inbound.Warnings)
	}
	if report.Outbound != nil {
		if err := appendOutbound("outbound", "conflict", report.Outbound.Conflicts); err != nil {
			return nil, err
		}
		if err := appendOutbound("outbound", "warning", report.Outbound.Warnings); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

// persistFactorialSyncRun salva la run e i suoi finding in una transazione
// dedicata, separata dalle transazioni di dominio della run stessa (T1..T4,
// inbound, outbound): si scrive a fine run (nel finish di RunFactorialSync),
// anche su run fallita e anche in dry-run (report.DryRun la marca). db nil =
// training non configurato: nessun errore, nulla da persistere (coerente col
// primo controllo di RunFactorialSync, che ha gia' fallito la run per lo
// stesso motivo).
func (s *SQLStore) persistFactorialSyncRun(ctx context.Context, report FactorialSyncRun) error {
	if s == nil || s.db == nil {
		return nil
	}
	ctx = context.WithoutCancel(ctx)
	counters, err := json.Marshal(factorialSyncCounters(report))
	if err != nil {
		return fmt.Errorf("marshal factorial sync run counters: %w", err)
	}
	findingRows, err := factorialSyncFindingRows(report)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var runID string
		if err := tx.QueryRowContext(ctx, `
INSERT INTO training.factorial_sync_run
  (started_at, finished_at, duration_ms, actor, dry_run, outcome, error, sessions_without_class, counters)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
RETURNING id::text`,
			report.StartedAt, report.FinishedAt, report.DurationMS, report.Actor, report.DryRun,
			report.Outcome, nullableText(report.Error), report.SessionsWithoutClass, counters,
		).Scan(&runID); err != nil {
			return fmt.Errorf("insert factorial sync run: %w", err)
		}
		for _, row := range findingRows {
			detail := row.detail
			if len(detail) == 0 {
				detail = []byte("{}")
			}
			if _, err := tx.ExecContext(ctx, `
INSERT INTO training.factorial_sync_finding
  (run_id, phase, severity, kind, ref, local_entity, local_id, employee_id, detail)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7::uuid, $8::uuid, $9::jsonb)`,
				runID, row.phase, row.severity, row.kind, row.ref,
				nullableText(row.localEntity), nullableUUID(row.localID), nullableUUID(row.employeeID), detail,
			); err != nil {
				return fmt.Errorf("insert factorial sync finding: %w", err)
			}
		}
		return nil
	})
}

// --- Letture -----------------------------------------------------------------

// ListFactorialSyncRuns ritorna le ultime run, piu' recente prima.
func (s *SQLStore) ListFactorialSyncRuns(ctx context.Context, limit int) ([]FactorialSyncRunRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("training database not configured")
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT r.id::text, r.started_at, r.finished_at, r.duration_ms, r.actor, r.dry_run, r.outcome,
  COALESCE(r.error, ''), r.sessions_without_class, r.counters::text,
  (SELECT count(*) FROM training.factorial_sync_finding f WHERE f.run_id = r.id)
FROM training.factorial_sync_run r
ORDER BY r.started_at DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list factorial sync runs: %w", err)
	}
	defer rows.Close()
	runs := make([]FactorialSyncRunRecord, 0)
	for rows.Next() {
		run, err := scanFactorialSyncRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// GetFactorialSyncRun ritorna una run con tutti i suoi finding, ordinati per
// fase/severita'/kind, con i riferimenti locali risolti (EmployeeName).
func (s *SQLStore) GetFactorialSyncRun(ctx context.Context, id string) (FactorialSyncRunDetail, error) {
	if s == nil || s.db == nil {
		return FactorialSyncRunDetail{}, errors.New("training database not configured")
	}
	run, err := scanFactorialSyncRun(s.db.QueryRowContext(ctx, `
SELECT r.id::text, r.started_at, r.finished_at, r.duration_ms, r.actor, r.dry_run, r.outcome,
  COALESCE(r.error, ''), r.sessions_without_class, r.counters::text,
  (SELECT count(*) FROM training.factorial_sync_finding f WHERE f.run_id = r.id)
FROM training.factorial_sync_run r
WHERE r.id = $1::uuid`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return FactorialSyncRunDetail{}, notFoundError("factorial_sync_run_not_found", "esecuzione non trovata")
		}
		return FactorialSyncRunDetail{}, err
	}

	findingRows, err := s.db.QueryContext(ctx, `
SELECT f.id::text, f.phase, f.severity, f.kind, f.ref, COALESCE(f.local_entity, ''),
  COALESCE(f.local_id::text, ''),
  COALESCE(ts.event_id::text, en.event_id::text, ''),
  COALESCE(f.employee_id::text, ''),
  TRIM(COALESCE(concat(e.last_name, ' ', e.first_name), '')), COALESCE(f.detail::text, '{}')
FROM training.factorial_sync_finding f
LEFT JOIN training.employee e ON e.id = f.employee_id
LEFT JOIN training.training_session ts
  ON f.local_entity = 'training_session' AND ts.id = f.local_id
LEFT JOIN training.enrollment en
  ON f.local_entity = 'enrollment' AND en.id = f.local_id
WHERE f.run_id = $1::uuid
ORDER BY f.phase, f.severity, f.kind`, id)
	if err != nil {
		return FactorialSyncRunDetail{}, fmt.Errorf("list factorial sync findings: %w", err)
	}
	defer findingRows.Close()
	findings := make([]FactorialSyncFindingRecord, 0)
	for findingRows.Next() {
		var f FactorialSyncFindingRecord
		var detail string
		if err := findingRows.Scan(&f.ID, &f.Phase, &f.Severity, &f.Kind, &f.Ref, &f.LocalEntity,
			&f.LocalID, &f.LocalEventID, &f.EmployeeID, &f.EmployeeName, &detail); err != nil {
			return FactorialSyncRunDetail{}, fmt.Errorf("scan factorial sync finding: %w", err)
		}
		if detail != "" && detail != "{}" {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(detail), &parsed); err != nil {
				return FactorialSyncRunDetail{}, fmt.Errorf("decode factorial sync finding detail: %w", err)
			}
			f.Detail = parsed
		}
		findings = append(findings, f)
	}
	if err := findingRows.Err(); err != nil {
		return FactorialSyncRunDetail{}, err
	}
	return FactorialSyncRunDetail{FactorialSyncRunRecord: run, Findings: findings}, nil
}

func scanFactorialSyncRun(scanner directoryRunScanner) (FactorialSyncRunRecord, error) {
	var (
		run        FactorialSyncRunRecord
		startedAt  time.Time
		finishedAt sql.NullTime
		counters   string
	)
	if err := scanner.Scan(
		&run.ID, &startedAt, &finishedAt, &run.DurationMS, &run.Actor, &run.DryRun, &run.Outcome,
		&run.Error, &run.SessionsWithoutClass, &counters, &run.FindingCount,
	); err != nil {
		return FactorialSyncRunRecord{}, err
	}
	run.StartedAt = startedAt.UTC().Format(time.RFC3339)
	if finishedAt.Valid {
		run.FinishedAt = finishedAt.Time.UTC().Format(time.RFC3339)
	}
	if counters != "" {
		parsed := map[string]int{}
		if err := json.Unmarshal([]byte(counters), &parsed); err != nil {
			return FactorialSyncRunRecord{}, fmt.Errorf("decode factorial sync run counters: %w", err)
		}
		run.Counters = parsed
	}
	return run, nil
}
