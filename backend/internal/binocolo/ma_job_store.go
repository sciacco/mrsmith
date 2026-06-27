package binocolo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// maJob is a queued async unit of session work (estimate today, execute next).
// State lives in binocolo.ma_job so a process restart resumes naturally — the
// first worker tick picks up any queued/running rows. See migration 049.
type maJob struct {
	ID                string
	JobType           string
	SessionID         string
	StrategyVersionID string
	Status            string
	Attempts          int
	Payload           json.RawMessage
	TraceID           string
	CreatedBySubject  string
	CreatedByEmail    string
}

// maJobEnqueue is the input to enqueue a new job.
type maJobEnqueue struct {
	JobType           string
	SessionID         string
	StrategyVersionID string
	Status            string
	Subject           string
	Email             string
	Payload           json.RawMessage
}

// EnqueueMAJob inserts a queued job. The partial unique index
// ma_job_inflight_idx makes a second in-flight job for the same (session,
// job_type) a no-op (ON CONFLICT DO NOTHING), so a double-submit while a job is
// queued/running does not launch duplicate work. Returns true only when a row
// was actually created.
func (s *SQLStore) EnqueueMAJob(ctx context.Context, input maJobEnqueue) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	payload := json.RawMessage(`{}`)
	if len(input.Payload) > 0 {
		payload = input.Payload
	}
	status := input.Status
	if status == "" {
		status = maJobStatusQueued
	}
	conflictPredicate := "status IN ('queued', 'running')"
	if status == maJobStatusPending {
		conflictPredicate = "status IN ('queued', 'running', 'pending', 'processing')"
	}
	result, err := s.db.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO binocolo.ma_job (id, job_type, session_id, strategy_version_id, status, payload, created_by_subject, created_by_email)
VALUES ($1::uuid, $2, $3::uuid, $4::uuid, $5, $6::jsonb, $7, $8)
ON CONFLICT (session_id, job_type) WHERE %s
DO NOTHING
`, conflictPredicate), uuid.NewString(), input.JobType, input.SessionID, input.StrategyVersionID, status, []byte(payload), nullString(input.Subject), nullString(input.Email))
	if err != nil {
		return false, fmt.Errorf("enqueue ma job: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("enqueue ma job rows: %w", err)
	}
	return affected == 1, nil
}

// ListMAJobs returns pending jobs of the types this binary can process. The
// job-type filter is part of the rollout contract: during progressive deploys,
// older workers must ignore job types introduced by newer versions instead of
// failing them as unknown.
func (s *SQLStore) ListMAJobs(ctx context.Context, limit int, workerID string, jobTypes []string) ([]maJob, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	if limit <= 0 {
		limit = 16
	}
	if len(jobTypes) == 0 {
		return []maJob{}, nil
	}
	args := []any{limit, workerID}
	placeholders := make([]string, 0, len(jobTypes))
	for _, jobType := range jobTypes {
		if jobType == "" {
			continue
		}
		args = append(args, jobType)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}
	if len(placeholders) == 0 {
		return []maJob{}, nil
	}
	query := fmt.Sprintf(`
SELECT id::text, job_type, session_id::text, strategy_version_id::text, status, attempts,
       payload, COALESCE(trace_id::text, ''), COALESCE(created_by_subject, ''), COALESCE(created_by_email, '')
FROM binocolo.ma_job
WHERE status IN ('queued', 'running', 'pending', 'processing')
  AND (lease_until IS NULL OR lease_until < now() OR locked_by = $2)
  AND job_type IN (%s)
ORDER BY updated_at
LIMIT $1
`, strings.Join(placeholders, ", "))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ma jobs: %w", err)
	}
	defer rows.Close()
	out := []maJob{}
	for rows.Next() {
		var job maJob
		var payload []byte
		if err := rows.Scan(&job.ID, &job.JobType, &job.SessionID, &job.StrategyVersionID, &job.Status, &job.Attempts, &payload, &job.TraceID, &job.CreatedBySubject, &job.CreatedByEmail); err != nil {
			return nil, fmt.Errorf("scan ma job: %w", err)
		}
		if len(payload) > 0 {
			job.Payload = json.RawMessage(payload)
		}
		out = append(out, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma jobs: %w", err)
	}
	return out, nil
}

// AcquireMAJobLease grants the calling worker exclusive ownership of a job row
// for leaseSeconds (measured on the DB clock, immune to cross-machine skew).
// Returns true only for the worker that wins the atomic update; a crashed
// worker's lease simply expires and the row becomes reclaimable. Same mechanism
// as AcquireMADeepLease — correct with multiple replicas on one DB.
func (s *SQLStore) AcquireMAJobLease(ctx context.Context, jobID, workerID string, leaseSeconds int) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_job
SET lease_until = now() + ($3::int * interval '1 second'), locked_by = $2
WHERE id = $1::uuid AND status IN ('queued', 'running', 'pending', 'processing')
  AND (lease_until IS NULL OR lease_until < now() OR locked_by = $2)
`, jobID, workerID, leaseSeconds)
	if err != nil {
		return false, fmt.Errorf("acquire ma job lease: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("acquire ma job lease rows: %w", err)
	}
	return affected == 1, nil
}

// ClaimMAJobQueued atomically transitions a row from queued to running. Returns
// true only for the worker/replica that won the claim (rows affected == 1).
func (s *SQLStore) ClaimMAJobQueued(ctx context.Context, jobID string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_job
SET status = CASE WHEN status = 'pending' THEN 'processing' ELSE 'running' END,
    error_code = NULL,
    updated_at = now()
WHERE id = $1::uuid AND status IN ('queued', 'pending')
`, jobID)
	if err != nil {
		return false, fmt.Errorf("claim ma job queued: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim ma job queued rows: %w", err)
	}
	return affected == 1, nil
}

// SetMAJobTrace records the operation-trace id the worker created for this job,
// for correlation between the job row and ma_operation_trace.
func (s *SQLStore) SetMAJobTrace(ctx context.Context, jobID, traceID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_job
SET trace_id = $2::uuid, updated_at = now()
WHERE id = $1::uuid
`, jobID, nullString(traceID))
	if err != nil {
		return fmt.Errorf("set ma job trace: %w", err)
	}
	return nil
}

// CompleteMAJob marks a job ready and releases its lease, dropping it from the
// pending set.
func (s *SQLStore) CompleteMAJob(ctx context.Context, jobID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_job
SET status = 'ready', error_code = NULL, lease_until = NULL, updated_at = now()
WHERE id = $1::uuid
`, jobID)
	if err != nil {
		return fmt.Errorf("complete ma job: %w", err)
	}
	return nil
}

// FailMAJob marks a job failed (terminal) and releases its lease.
func (s *SQLStore) FailMAJob(ctx context.Context, jobID, errorCode string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_job
SET status = 'failed', error_code = $2, lease_until = NULL, updated_at = now()
WHERE id = $1::uuid
`, jobID, nullString(errorCode))
	if err != nil {
		return fmt.Errorf("fail ma job: %w", err)
	}
	return nil
}

// BumpMAJobAttempt increments and returns the job's attempt counter, keeping the
// row in the pending set (updated_at bumped) so the next tick retries it.
func (s *SQLStore) BumpMAJobAttempt(ctx context.Context, jobID string) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("binocolo ma store not configured")
	}
	var attempts int
	if err := s.db.QueryRowContext(ctx, `
UPDATE binocolo.ma_job
SET attempts = attempts + 1, updated_at = now()
WHERE id = $1::uuid
RETURNING attempts
`, jobID).Scan(&attempts); err != nil {
		return 0, fmt.Errorf("bump ma job attempt: %w", err)
	}
	return attempts, nil
}

// HasRunningMAExecution reports whether the session already has an execution run
// in flight. The execute worker checks this before creating a run: if a prior
// attempt created one but never finished (a worker crashed mid-fetch), it must NOT
// create a second run and re-charge the paid company fetch. Charge-idempotency
// here does not depend on lease timing — a reclaimer always sees the orphan run.
func (s *SQLStore) HasRunningMAExecution(ctx context.Context, sessionID string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM binocolo.ma_execution_run
  WHERE session_id = $1::uuid AND status = 'running'
)`, sessionID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check running ma execution: %w", err)
	}
	return exists, nil
}

// AbandonMASessionExecution fails any in-flight execution run for the session and
// moves the session out of 'running' into 'failed'. Used when the execute worker
// finds a run already in flight (a crashed prior attempt) and refuses to re-charge.
// If the original worker is in fact still alive and finishes, its CompleteMAExecutionRun
// overwrites this — so the worst case is a transient 'failed' that self-heals, never
// a double charge.
func (s *SQLStore) AbandonMASessionExecution(ctx context.Context, sessionID, errorCode string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin abandon ma execution: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_execution_run
SET status = 'failed', error_code = $2, completed_at = now()
WHERE session_id = $1::uuid AND status = 'running'
`, sessionID, nullString(errorCode)); err != nil {
		return fmt.Errorf("abandon ma execution run: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET status = 'failed', updated_at = now()
WHERE id = $1::uuid AND status = 'running'
`, sessionID); err != nil {
		return fmt.Errorf("abandon ma session: %w", err)
	}
	return tx.Commit()
}

// MarkMASessionExecuting flips the session into the in-flight 'running' state at
// execute enqueue, so the UI shows progress and polls immediately (the worker
// creates the execution run a tick later). Idempotent; skips a no-op write when
// already running.
func (s *SQLStore) MarkMASessionExecuting(ctx context.Context, sessionID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET status = 'running', updated_at = now()
WHERE id = $1::uuid AND status <> 'running'
`, sessionID)
	if err != nil {
		return fmt.Errorf("mark ma session executing: %w", err)
	}
	return nil
}

// MarkMASessionExecuteFailed moves a session out of 'running' into 'failed' when
// the execute job gives up before an execution run was even created (a run, once
// created, is completed-failed instead, which sets the session itself). Gated on
// status='running' so it never stomps a session the user has moved on from.
func (s *SQLStore) MarkMASessionExecuteFailed(ctx context.Context, sessionID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET status = 'failed', updated_at = now()
WHERE id = $1::uuid AND status = 'running'
`, sessionID)
	if err != nil {
		return fmt.Errorf("mark ma session execute failed: %w", err)
	}
	return nil
}

// MarkMASessionEstimateFailed moves a session out of the in-flight 'estimating'
// state into 'failed' when the estimate job gives up. Gated on status='estimating'
// (not on version) so it never stomps a session the user has already moved on from.
func (s *SQLStore) MarkMASessionEstimateFailed(ctx context.Context, sessionID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET status = 'failed', updated_at = now()
WHERE id = $1::uuid AND status = 'estimating'
`, sessionID)
	if err != nil {
		return fmt.Errorf("mark ma session estimate failed: %w", err)
	}
	return nil
}

// SetMASessionEstimateStatus moves the session to status only while the given
// strategy version is still the active one. If a newer version has since been
// activated (a fresh estimate superseded this job), the update is a no-op — the
// stale job must not clobber the current session state. Used for the 'estimating'
// flip at enqueue and the 'failed' flip on terminal worker failure.
func (s *SQLStore) SetMASessionEstimateStatus(ctx context.Context, sessionID, strategyVersionID, status string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET status = $3, updated_at = now()
WHERE id = $1::uuid AND active_strategy_id = $2::uuid
`, sessionID, strategyVersionID, status)
	if err != nil {
		return fmt.Errorf("set ma session estimate status: %w", err)
	}
	return nil
}
