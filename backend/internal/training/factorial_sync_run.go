package training

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/directory"
	"github.com/sciacco/mrsmith/pkg/factorial"
)

// Orchestrazione della run completa di sincronizzazione Factorial (#141,
// slice 7/8 = #149): compone in sequenza fissa i pezzi delle slice 2-6,
// dietro lo stesso advisory lock del directory sync (in sequenza, mai
// annidato). Nessuna nuova logica di sync qui: solo cablaggio, report e log.

// factorialSyncSampleSize: campioni di conflitto max nel log strutturato.
const factorialSyncSampleSize = 20

// FactorialSyncDeps raggruppa le dipendenze esterne di una run, per non far
// crescere la firma di RunFactorialSync (stesso pattern di training.Deps).
type FactorialSyncDeps struct {
	Directory           directory.Provider
	Factorial           *factorial.Client
	TechnicalEmployeeID string
	Logger              *slog.Logger
}

// FactorialSyncRun e' il report di una run, per JobRunner (log strutturato)
// e CLI (stdout + report JSON). Nessun campo porta PII: gli issue sotto
// Tombstone/Inbound/Outbound espongono solo Kind, Ref (id tecnici) ed
// eventuale status HTTP (Err e' json:"-", mai serializzato).
type FactorialSyncRun struct {
	Operation  string `json:"operation"`
	DryRun     bool   `json:"dryRun"`
	Actor      string `json:"actor"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	DurationMS int64  `json:"durationMs"`
	Outcome    string `json:"outcome"`
	Error      string `json:"error,omitempty"`
	// SessionsWithoutClass: sessioni con TrainingClassID nil, scartate a monte del perimetro (gap noto slice 3).
	SessionsWithoutClass int `json:"sessionsWithoutClass"`
	// Tombstone.AccessDestroyed/SessionsDeleted: totali ripropagati ogni run (404=successo), non un delta (#141).
	Tombstone *tombstoneResult    `json:"tombstone"`
	Inbound   *inboundSyncResult  `json:"inbound"`
	Outbound  *outboundSyncResult `json:"outbound"`
}

// RunFactorialSync esegue la sequenza fissa: directory sync (prerequisito) ->
// snapshot attivi -> fetch grafo -> tombstone (delete remote + filtro grafo,
// con verifica missing: T1+T2+T4 slice 6) -> perimetro -> inbound ->
// outbound. Se il directory sync o lo snapshot falliscono, non parte.
func (s *SQLStore) RunFactorialSync(ctx context.Context, deps FactorialSyncDeps, actor string, dryRun bool) (FactorialSyncRun, error) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	started := time.Now()
	report := FactorialSyncRun{
		Operation: "training_factorial_sync", DryRun: dryRun, Actor: actor,
		StartedAt: started.UTC().Format(time.RFC3339),
		Tombstone: &tombstoneResult{}, Inbound: &inboundSyncResult{}, Outbound: &outboundSyncResult{},
	}
	finish := func(err error) (FactorialSyncRun, error) {
		finished := time.Now()
		report.FinishedAt = finished.UTC().Format(time.RFC3339)
		report.DurationMS = finished.Sub(started).Milliseconds()
		level := slog.LevelInfo
		report.Outcome = "ok"
		if err != nil {
			report.Outcome = "failed"
			report.Error = err.Error()
			level = slog.LevelWarn
		}
		args := []any{
			"operation", report.Operation, "dry_run", report.DryRun, "actor", actor,
			"duration", finished.Sub(started), "outcome", report.Outcome,
			"sessions_without_class", report.SessionsWithoutClass,
			"tombstone_access_destroyed", report.Tombstone.AccessDestroyed,
			"tombstone_sessions_deleted", report.Tombstone.SessionsDeleted,
			"tombstone_reset", report.Tombstone.ResetCount,
			"tombstone_warnings", len(report.Tombstone.Warnings),
			"inbound_conflicts", len(report.Inbound.Conflicts),
			"inbound_warnings", len(report.Inbound.Warnings),
			"inbound_rda_exceptions", report.Inbound.RDAExceptions,
			"outbound_conflicts", len(report.Outbound.Conflicts),
			"outbound_warnings", len(report.Outbound.Warnings),
			"outbound_planned", len(report.Outbound.Planned),
			"conflict_samples", factorialSyncConflictSamples(report.Inbound, report.Outbound, factorialSyncSampleSize),
		}
		if err != nil {
			args = append(args, "error", report.Error)
		}
		logger.Log(ctx, level, "training factorial sync run completed", args...)
		return report, err
	}

	if s == nil || s.db == nil {
		return finish(serviceUnavailableError("training_database_not_configured", "database Training non configurato"))
	}
	if deps.Factorial == nil {
		return finish(fmt.Errorf("factorial client not configured"))
	}
	if strings.TrimSpace(deps.TechnicalEmployeeID) == "" {
		return finish(fmt.Errorf("factorial technical employee not configured"))
	}

	if _, err := s.RunDirectorySync(ctx, deps.Directory, actor+":factorial", dryRun); err != nil {
		return finish(fmt.Errorf("prerequisite directory sync: %w", err))
	}

	err := s.withFactorialSyncLock(ctx, func() error {
		snapshot, err := deps.Directory.Snapshot(ctx)
		if err != nil {
			return fmt.Errorf("prerequisite active snapshot: %w", err)
		}
		activeIDs := activeEmployeeIDs(snapshot)

		graph, err := fetchFactorialTrainingGraph(ctx, deps.Factorial)
		if err != nil {
			return fmt.Errorf("fetch factorial training graph: %w", err)
		}

		tombstone, err := s.applyTombstoneSync(ctx, deps.Factorial, graph, dryRun)
		*report.Tombstone = tombstone
		if err != nil {
			return fmt.Errorf("apply factorial tombstone sync: %w", err)
		}
		filtered := filterTombstonedGraph(graph, tombstone.TombstonedSessionIDs, tombstone.TombstonedAccessIDs)
		for _, sess := range filtered.Sessions {
			if sess.TrainingClassID == nil {
				report.SessionsWithoutClass++
			}
		}

		local, err := s.loadLocalSyncState(ctx, s.db)
		if err != nil {
			return fmt.Errorf("load local training sync state: %w", err)
		}
		linkedTrainingIDs := make(map[string]struct{}, len(local.Courses))
		for trainingID := range local.Courses {
			linkedTrainingIDs[trainingID] = struct{}{}
		}
		perimeter := computeFactorialPerimeter(filtered, activeIDs, linkedTrainingIDs)

		inbound, err := s.applyInboundSync(ctx, perimeter, dryRun)
		*report.Inbound = inbound
		if err != nil {
			return fmt.Errorf("apply factorial inbound sync: %w", err)
		}

		outbound, err := s.applyOutboundSync(ctx, deps.Factorial, deps.TechnicalEmployeeID, filtered, activeIDs, dryRun)
		*report.Outbound = outbound
		if err != nil {
			return fmt.Errorf("apply factorial outbound sync: %w", err)
		}
		return nil
	})
	return finish(err)
}

// withFactorialSyncLock: stessa chiave advisory del directory sync, su una
// connessione dedicata, in sequenza mai annidata (gia' rilasciata).
func (s *SQLStore) withFactorialSyncLock(ctx context.Context, fn func() error) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire training factorial sync connection: %w", err)
	}
	defer conn.Close()
	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, directorySyncLockKey).Scan(&locked); err != nil {
		return fmt.Errorf("lock training factorial sync: %w", err)
	}
	if !locked {
		return conflictError("sync_already_running", "sincronizzazione formativa già in corso")
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, directorySyncLockKey)
	}()
	return fn()
}

// factorialSyncConflictSamples: fino a limit campioni "Kind:Ref", solo id tecnici (#141).
func factorialSyncConflictSamples(inbound *inboundSyncResult, outbound *outboundSyncResult, limit int) []string {
	samples := make([]string, 0, limit)
	for _, issue := range inbound.Conflicts {
		if len(samples) >= limit {
			return samples
		}
		samples = append(samples, issue.Kind+":"+issue.Ref)
	}
	for _, issue := range outbound.Conflicts {
		if len(samples) >= limit {
			return samples
		}
		sample := issue.Kind + ":" + issue.Ref
		if apiErr, ok := errors.AsType[*factorial.APIError](issue.Err); ok {
			sample += fmt.Sprintf(" status=%d", apiErr.StatusCode)
		}
		samples = append(samples, sample)
	}
	return samples
}
