package training

import (
	"context"
	"log/slog"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/directory"
	"github.com/sciacco/mrsmith/pkg/factorial"
)

type JobRunner struct {
	store         *SQLStore
	logger        *slog.Logger
	directory     directory.Provider
	syncDirectory bool
	factorial     FactorialSyncDeps
	syncFactorial bool
}

func NewJobRunner(store *SQLStore, logger *slog.Logger) *JobRunner {
	if logger == nil {
		logger = slog.Default()
	}
	return &JobRunner{
		store:  store,
		logger: logger.With("component", "training", "worker", "jobs"),
	}
}

// WithDirectorySync enables the periodic anagrafica reconciliation. It stays
// off unless explicitly enabled: the database is shared across environments.
func (r *JobRunner) WithDirectorySync(provider directory.Provider, enabled bool) *JobRunner {
	if r == nil {
		return r
	}
	r.directory = provider
	r.syncDirectory = enabled && provider != nil
	return r
}

// WithFactorialSync enables the periodic Factorial training reconciliation.
// It stays off unless directory sync is enabled and the Factorial client is
// configured: the run depends on a fresh directory reconciliation. The author
// employee id is runtime configuration (mrsmith.runtime_config) read at
// every run, so it is not a startup prerequisite: a missing row fails the run
// with an explicit error.
func (r *JobRunner) WithFactorialSync(client *factorial.Client, enabled bool) *JobRunner {
	if r == nil {
		return r
	}
	r.factorial = FactorialSyncDeps{Directory: r.directory, Factorial: client, Logger: r.logger}
	ready := r.syncDirectory && client != nil
	r.syncFactorial = enabled && ready
	if enabled && !ready {
		r.logger.Info("training factorial sync disabled: prerequisites not met",
			"directory_sync_enabled", r.syncDirectory,
			"factorial_client_configured", client != nil)
	}
	return r
}

// RunOnce esegue la reconciliation periodica (directory e Factorial): il job
// di notifica scadenze certificazioni alle persone e' stato rimosso (#160,
// slice 1 del task 7).
func (r *JobRunner) RunOnce(ctx context.Context) (JobRunResponse, error) {
	if r == nil || r.store == nil {
		return JobRunResponse{}, serviceUnavailableError("training_database_not_configured", "database Training non configurato")
	}
	// RunFactorialSync esegue il directory sync come proprio prerequisito
	// (stessa chiave di lock, in sequenza): quando e' abilitato, il blocco
	// standalone qui sotto salterebbe duplicando la reconciliation ogni tick.
	if r.syncDirectory && !r.syncFactorial {
		if _, err := r.store.RunDirectorySync(ctx, r.directory, "job", false); err != nil {
			r.logger.Warn("training directory sync job failed", "error", err)
		}
	}
	if r.syncFactorial {
		if _, err := r.store.RunFactorialSync(ctx, r.factorial, "job", false); err != nil {
			r.logger.Warn("training factorial sync job failed", "error", err)
		}
	}
	return JobRunResponse{OK: true}, nil
}

func (r *JobRunner) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := r.RunOnce(ctx); err != nil {
			r.logger.Warn("training jobs failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
