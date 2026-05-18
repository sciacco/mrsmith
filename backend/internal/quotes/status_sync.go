package quotes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/hubspot"
)

const (
	defaultHubSpotStatusSyncInterval  = 5 * time.Minute
	defaultHubSpotStatusSyncBatchSize = 50
	hubSpotStatusSyncLockKey          = int64(44000044)

	hubSpotStatusSyncConfigNamespace = "quotes"
	hubSpotStatusSyncConfigKey       = "hubspot_status_sync"
)

// HubSpotStatusProvider is the HubSpot subset needed by the status sync worker.
type HubSpotStatusProvider interface {
	GetQuoteStatus(ctx context.Context, quoteID int64) (*hubspot.QuoteStatus, error)
}

type HubSpotStatusSyncDeps struct {
	Mistra        *sql.DB
	RuntimeConfig *sql.DB
	HubSpot       HubSpotStatusProvider
	Logger        *slog.Logger
}

type HubSpotStatusSyncWorker struct {
	db        *sql.DB
	configDB  *sql.DB
	hs        HubSpotStatusProvider
	logger    *slog.Logger
	now       func() time.Time
	lockKey   int64
	lockLabel string
}

type HubSpotStatusSyncStats struct {
	Checked     int
	Updated     int
	Skipped     int
	Errors      int
	LockSkipped bool
	Duration    time.Duration
}

type hubSpotStatusSyncConfig struct {
	Enabled   bool
	Interval  time.Duration
	BatchSize int
}

type hubSpotStatusSyncConfigPayload struct {
	Enabled         *bool `json:"enabled"`
	IntervalSeconds int   `json:"interval_seconds"`
	BatchSize       int   `json:"batch_size"`
}

type pendingHubSpotQuote struct {
	ID        int
	HSQuoteID int64
}

func NewHubSpotStatusSyncWorker(deps HubSpotStatusSyncDeps) *HubSpotStatusSyncWorker {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &HubSpotStatusSyncWorker{
		db:        deps.Mistra,
		configDB:  deps.RuntimeConfig,
		hs:        deps.HubSpot,
		logger:    logger.With("component", "quotes", "worker", "hubspot_status_sync"),
		now:       time.Now,
		lockKey:   hubSpotStatusSyncLockKey,
		lockLabel: "quotes_hubspot_status_sync",
	}
}

func (w *HubSpotStatusSyncWorker) Run(ctx context.Context) {
	if w == nil || w.db == nil || w.hs == nil {
		return
	}
	w.logger.Info("hubspot status sync worker started")
	defer w.logger.Info("hubspot status sync worker stopped")

	for {
		cfg := w.loadRuntimeConfig(ctx)
		if cfg.Enabled {
			if _, err := w.processOnceWithConfig(ctx, cfg); err != nil && ctx.Err() == nil {
				w.logger.Warn("hubspot status sync run failed", "error", err)
			}
		} else {
			w.logger.Info("hubspot status sync worker disabled by runtime config")
		}

		timer := time.NewTimer(cfg.Interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
		}
	}
}

func (w *HubSpotStatusSyncWorker) ProcessOnce(ctx context.Context) (HubSpotStatusSyncStats, error) {
	if w == nil || w.db == nil || w.hs == nil {
		return HubSpotStatusSyncStats{}, nil
	}
	cfg := w.loadRuntimeConfig(ctx)
	if !cfg.Enabled {
		w.logger.Info("hubspot status sync worker disabled by runtime config")
		return HubSpotStatusSyncStats{}, nil
	}
	return w.processOnceWithConfig(ctx, cfg)
}

func (w *HubSpotStatusSyncWorker) processOnceWithConfig(ctx context.Context, cfg hubSpotStatusSyncConfig) (HubSpotStatusSyncStats, error) {
	started := w.now()
	stats := HubSpotStatusSyncStats{}

	conn, err := w.db.Conn(ctx)
	if err != nil {
		return stats, fmt.Errorf("open mistra connection: %w", err)
	}
	defer conn.Close()

	locked, err := w.tryAdvisoryLock(ctx, conn)
	if err != nil {
		return stats, err
	}
	if !locked {
		stats.LockSkipped = true
		stats.Duration = w.now().Sub(started)
		w.logger.Info("hubspot status sync skipped because another worker owns the advisory lock")
		return stats, nil
	}
	defer w.releaseAdvisoryLock(conn)

	lastID := 0
	for {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		quotes, err := loadPendingHubSpotQuotes(ctx, conn, lastID, cfg.BatchSize)
		if err != nil {
			return stats, err
		}
		if len(quotes) == 0 {
			break
		}
		for _, quote := range quotes {
			lastID = quote.ID
			if err := ctx.Err(); err != nil {
				return stats, err
			}
			w.syncQuoteStatus(ctx, conn, quote, &stats)
		}
	}

	stats.Duration = w.now().Sub(started)
	w.logger.Info(
		"hubspot status sync completed",
		"checked", stats.Checked,
		"updated", stats.Updated,
		"skipped", stats.Skipped,
		"errors", stats.Errors,
		"duration_ms", stats.Duration.Milliseconds(),
	)
	return stats, nil
}

func (w *HubSpotStatusSyncWorker) syncQuoteStatus(ctx context.Context, conn *sql.Conn, quote pendingHubSpotQuote, stats *HubSpotStatusSyncStats) {
	stats.Checked++

	remote, err := w.hs.GetQuoteStatus(ctx, quote.HSQuoteID)
	if err != nil {
		stats.Errors++
		w.logger.Warn(
			"hubspot quote status lookup failed",
			"quote_id", quote.ID,
			"hs_quote_id", quote.HSQuoteID,
			"error", err,
		)
		return
	}

	remoteStatus, ok := mapHubSpotQuoteStatus(remoteQuoteState(remote))
	if !ok {
		stats.Skipped++
		w.logger.Debug(
			"hubspot quote status skipped",
			"quote_id", quote.ID,
			"hs_quote_id", quote.HSQuoteID,
			"remote_hs_status", remoteQuoteState(remote),
		)
		return
	}

	result, err := conn.ExecContext(ctx, `
		UPDATE quotes.quote
		SET status = $1
		WHERE id = $2 AND status = 'PENDING_APPROVAL'
	`, remoteStatus, quote.ID)
	if err != nil {
		stats.Errors++
		w.logger.Warn(
			"local quote status update failed",
			"quote_id", quote.ID,
			"hs_quote_id", quote.HSQuoteID,
			"remote_hs_status", remoteStatus,
			"error", err,
		)
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		stats.Skipped++
		w.logger.Debug(
			"local quote status update skipped because quote is no longer pending approval",
			"quote_id", quote.ID,
			"hs_quote_id", quote.HSQuoteID,
			"remote_hs_status", remoteStatus,
		)
		return
	}

	stats.Updated++
	w.logger.Info(
		"local quote status synchronized from hubspot",
		"quote_id", quote.ID,
		"hs_quote_id", quote.HSQuoteID,
		"previous_status", "PENDING_APPROVAL",
		"new_status", remoteStatus,
	)
}

func (w *HubSpotStatusSyncWorker) loadRuntimeConfig(ctx context.Context) hubSpotStatusSyncConfig {
	cfg := defaultHubSpotStatusSyncConfig()
	if w == nil || w.configDB == nil {
		return cfg
	}

	var raw []byte
	err := w.configDB.QueryRowContext(ctx, `
		SELECT value
		FROM mrsmith.runtime_config
		WHERE namespace = $1 AND key = $2
	`, hubSpotStatusSyncConfigNamespace, hubSpotStatusSyncConfigKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return cfg
	}
	if err != nil {
		w.logger.Warn("hubspot status sync runtime config read failed; using defaults", "error", err)
		return cfg
	}

	var payload hubSpotStatusSyncConfigPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		w.logger.Warn("hubspot status sync runtime config parse failed; using defaults", "error", err)
		return cfg
	}
	if payload.Enabled != nil {
		cfg.Enabled = *payload.Enabled
	}
	if payload.IntervalSeconds > 0 {
		cfg.Interval = time.Duration(payload.IntervalSeconds) * time.Second
	}
	if payload.BatchSize > 0 {
		cfg.BatchSize = payload.BatchSize
	}
	return cfg
}

func defaultHubSpotStatusSyncConfig() hubSpotStatusSyncConfig {
	return hubSpotStatusSyncConfig{
		Enabled:   true,
		Interval:  defaultHubSpotStatusSyncInterval,
		BatchSize: defaultHubSpotStatusSyncBatchSize,
	}
}

func loadPendingHubSpotQuotes(ctx context.Context, conn *sql.Conn, afterID int, batchSize int) ([]pendingHubSpotQuote, error) {
	if batchSize <= 0 {
		batchSize = defaultHubSpotStatusSyncBatchSize
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, hs_quote_id
		FROM quotes.quote
		WHERE status = 'PENDING_APPROVAL'
		  AND hs_quote_id IS NOT NULL
		  AND id > $1
		ORDER BY id
		LIMIT $2
	`, afterID, batchSize)
	if err != nil {
		return nil, fmt.Errorf("load pending hubspot quotes: %w", err)
	}
	defer rows.Close()

	quotes := []pendingHubSpotQuote{}
	for rows.Next() {
		var quote pendingHubSpotQuote
		if err := rows.Scan(&quote.ID, &quote.HSQuoteID); err != nil {
			return nil, fmt.Errorf("scan pending hubspot quote: %w", err)
		}
		quotes = append(quotes, quote)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending hubspot quotes: %w", err)
	}
	return quotes, nil
}

func (w *HubSpotStatusSyncWorker) tryAdvisoryLock(ctx context.Context, conn *sql.Conn) (bool, error) {
	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, w.lockKey).Scan(&locked); err != nil {
		return false, fmt.Errorf("acquire %s advisory lock: %w", w.lockLabel, err)
	}
	return locked, nil
}

func (w *HubSpotStatusSyncWorker) releaseAdvisoryLock(conn *sql.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var unlocked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_advisory_unlock($1)`, w.lockKey).Scan(&unlocked); err != nil {
		w.logger.Warn("release hubspot status sync advisory lock failed", "error", err)
		return
	}
	if !unlocked {
		w.logger.Warn("hubspot status sync advisory lock was not held during release")
	}
}

func mapHubSpotQuoteStatus(raw string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "APPROVED":
		return "APPROVED", true
	case "APPROVAL_NOT_NEEDED":
		return "APPROVAL_NOT_NEEDED", true
	case "REJECTED":
		return "REJECTED", true
	default:
		return "", false
	}
}
