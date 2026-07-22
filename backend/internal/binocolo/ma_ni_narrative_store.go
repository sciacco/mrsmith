package binocolo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Store for the nota-integrativa NARRATIVE reading (issue #80, Fase N1). Separate from the
// proposals reading (ma_ni_store.go / mig 114): the narrative extracts non-numeric CONTEXT
// facts (attribuzioni | rischi | piani | profilo) that are invisible to the prospetti and
// have NO effect on numbers/valuation. Same database/sql conventions as ma_ni_store.go: methods
// on *SQLStore, $n params, ::uuid casts, columns tracking migration 118 exactly. A narrative run
// is one LLM pass over a processing run; its observations are IMMUTABLE (dedup by fingerprint
// within the run) and there are NO analyst decisions (read-only facts).

// Narrative run status vocabulary (mig 118 CHECK is canonical). The run is created 'running'
// (the regeneration «in corso», observable by the GET), then finalized to 'ready' or 'failed';
// a re-read supersedes the prior non-superseded run.
const (
	maNINarrativeStatusRunning    = "running"
	maNINarrativeStatusReady      = "ready"
	maNINarrativeStatusFailed     = "failed"
	maNINarrativeStatusSuperseded = "superseded"
)

// ---------------------------------------------------------------------------
// ma_ni_narrative_run — one LLM narrative pass over a processing run.
// ---------------------------------------------------------------------------

// maNINarrativeRun mirrors binocolo.ma_ni_narrative_run. prompt_id/model_id are uuid soft-refs
// to the mrsmith registry (nullable, no FK — pattern 105/114), surfaced as strings ("" for NULL).
// Truncated flags a budget-truncated LLM output (telemetry).
type maNINarrativeRun struct {
	ID              string
	FilingID        string
	ProcessingRunID string
	PromptID        string
	ModelID         string
	RequestID       string
	Status          string
	Truncated       bool
	CreatedAt       time.Time
}

const maNINarrativeRunColumns = `id::text, filing_id::text, processing_run_id::text,
	COALESCE(prompt_id::text, ''), COALESCE(model_id::text, ''), COALESCE(request_id, ''),
	status, truncated, created_at`

func scanMANINarrativeRun(scanner interface{ Scan(...any) error }) (maNINarrativeRun, error) {
	var r maNINarrativeRun
	if err := scanner.Scan(
		&r.ID, &r.FilingID, &r.ProcessingRunID,
		&r.PromptID, &r.ModelID, &r.RequestID,
		&r.Status, &r.Truncated, &r.CreatedAt,
	); err != nil {
		return maNINarrativeRun{}, err
	}
	return r, nil
}

// CreateMANINarrativeRun opens a new narrative run for a filing: in one transaction it supersedes
// the filing's prior non-superseded narrative run(s) (running/ready/failed) and inserts the new
// run as 'running' (the observable «in corso» state). model_id/prompt_id are the uuid soft-refs of
// the resolved registry Model/Prompt (contract QA-F1); requestID correlates the run to the trace +
// the per-section llm_call_audit rows. Returns the new run id.
func (s *SQLStore) CreateMANINarrativeRun(ctx context.Context, filingID, processingRunID, promptID, modelID, requestID string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin create ma ni narrative run: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_ni_narrative_run
SET status = 'superseded'
WHERE filing_id = $1::uuid AND status <> 'superseded'
`, filingID); err != nil {
		return "", fmt.Errorf("supersede ma ni narrative runs: %w", err)
	}

	var id string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_ni_narrative_run
  (filing_id, processing_run_id, prompt_id, model_id, request_id, status)
VALUES
  ($1::uuid, $2::uuid, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, NULLIF($5, ''), 'running')
RETURNING id::text
`, filingID, processingRunID, promptID, modelID, requestID).Scan(&id); err != nil {
		return "", fmt.Errorf("insert ma ni narrative run: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit create ma ni narrative run: %w", err)
	}
	return id, nil
}

// GetActiveMANINarrativeRun returns the current (non-superseded) narrative run for a filing —
// whatever its status (running/ready/failed) — or (nil, nil) when none. Application-enforced
// one-current invariant (CreateMANINarrativeRun supersedes prior); ORDER BY created_at DESC
// guards against a transient double-current.
func (s *SQLStore) GetActiveMANINarrativeRun(ctx context.Context, filingID string) (*maNINarrativeRun, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	r, err := scanMANINarrativeRun(s.db.QueryRowContext(ctx, `SELECT `+maNINarrativeRunColumns+`
FROM binocolo.ma_ni_narrative_run
WHERE filing_id = $1::uuid AND status <> 'superseded'
ORDER BY created_at DESC
LIMIT 1`, filingID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active ma ni narrative run: %w", err)
	}
	return &r, nil
}

// SetMANINarrativeRunStatus finalizes a narrative run: running → ready|failed, recording the
// budget-truncation flag. Gated on the source status so a superseded run is never resurrected.
func (s *SQLStore) SetMANINarrativeRunStatus(ctx context.Context, runID, status string, truncated bool) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_ni_narrative_run
SET status = $2, truncated = $3
WHERE id = $1::uuid AND status = 'running'
`, runID, status, truncated); err != nil {
		return fmt.Errorf("set ma ni narrative run status: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ma_ni_observation — immutable narrative observations (dedup by fingerprint per run).
// ---------------------------------------------------------------------------

// maNIObservation mirrors binocolo.ma_ni_observation. page_no surfaces as *int (nil when NULL).
type maNIObservation struct {
	ID          string
	RunID       string
	Tipo        string
	Fingerprint string
	Claim       string
	Quote       string
	PageNo      *int
	CreatedAt   time.Time
}

const maNIObservationColumns = `id::text, run_id::text, COALESCE(tipo, ''), fingerprint,
	COALESCE(claim, ''), COALESCE(quote, ''), page_no, created_at`

func scanMANIObservation(scanner interface{ Scan(...any) error }) (maNIObservation, error) {
	var o maNIObservation
	var pageNo sql.NullInt64
	if err := scanner.Scan(
		&o.ID, &o.RunID, &o.Tipo, &o.Fingerprint,
		&o.Claim, &o.Quote, &pageNo, &o.CreatedAt,
	); err != nil {
		return maNIObservation{}, err
	}
	if pageNo.Valid {
		v := int(pageNo.Int64)
		o.PageNo = &v
	}
	return o, nil
}

// InsertMANIObservations batch-inserts a run's observations, IMMUTABLE and deduplicated by the
// (run_id, fingerprint) unique index: ON CONFLICT DO NOTHING makes a duplicate fact within the
// run a no-op and a retry of the same batch idempotent. Returns the number of rows inserted.
func (s *SQLStore) InsertMANIObservations(ctx context.Context, runID string, observations []maNIObservation) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("binocolo ma store not configured")
	}
	if len(observations) == 0 {
		return 0, nil
	}
	var b strings.Builder
	b.WriteString(`INSERT INTO binocolo.ma_ni_observation
  (run_id, tipo, fingerprint, claim, quote, page_no) VALUES `)
	args := []any{runID}
	for i, o := range observations {
		if i > 0 {
			b.WriteString(", ")
		}
		base := len(args)
		fmt.Fprintf(&b, "($1::uuid, NULLIF($%d, ''), $%d, NULLIF($%d, ''), NULLIF($%d, ''), $%d)",
			base+1, base+2, base+3, base+4, base+5)
		args = append(args, o.Tipo, o.Fingerprint, o.Claim, o.Quote, nullInt(o.PageNo))
	}
	b.WriteString(" ON CONFLICT (run_id, fingerprint) DO NOTHING")
	res, err := s.db.ExecContext(ctx, b.String(), args...)
	if err != nil {
		return 0, fmt.Errorf("insert ma ni observations: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("insert ma ni observations rows: %w", err)
	}
	return int(affected), nil
}

// ListMANIObservationsByRun returns the observations of a narrative run, oldest first.
func (s *SQLStore) ListMANIObservationsByRun(ctx context.Context, runID string) ([]maNIObservation, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+maNIObservationColumns+`
FROM binocolo.ma_ni_observation
WHERE run_id = $1::uuid
ORDER BY created_at, id`, runID)
	if err != nil {
		return nil, fmt.Errorf("list ma ni observations by run: %w", err)
	}
	defer rows.Close()
	out := []maNIObservation{}
	for rows.Next() {
		o, err := scanMANIObservation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan ma ni observation: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma ni observations: %w", err)
	}
	return out, nil
}
