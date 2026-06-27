package binocolo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type maWorkspaceStore interface {
	ListMASessions(ctx context.Context, visibility string) ([]MASessionSummary, error)
	CreateMASession(ctx context.Context, input maSessionCreate) (MASessionDetail, error)
	GetMASession(ctx context.Context, id string) (MASessionDetail, error)
	GetMASessionState(ctx context.Context, id string) (MASession, error)
	UpdateMASessionLifecycle(ctx context.Context, sessionID, action, subject, email string) (bool, error)
	AddMAStrategyVersion(ctx context.Context, sessionID string, strategy MAStrategySpec, createdByEmail string) (*MAStrategyVersion, error)
	GetMAStrategyVersion(ctx context.Context, sessionID, versionID string) (MAStrategyVersion, error)
	ReplaceMAEstimates(ctx context.Context, sessionID, strategyVersionID, selectedStrategy string, estimates []MAEstimate) error
	EnqueueMAJob(ctx context.Context, input maJobEnqueue) (bool, error)
	SetMASessionEstimateStatus(ctx context.Context, sessionID, strategyVersionID, status string) error
	MarkMASessionExecuting(ctx context.Context, sessionID string) error
	HasRunningMAExecution(ctx context.Context, sessionID string) (bool, error)
	AbandonMASessionExecution(ctx context.Context, sessionID, errorCode string) error
	CreateMAExecutionRun(ctx context.Context, input maExecutionRunCreate) (MAExecutionRun, error)
	CompleteMAExecutionRun(ctx context.Context, runID, status string, resultCount int, errorCode string) error
	ReplaceMATargets(ctx context.Context, sessionID, runID string, targets []MATarget) error
	UpsertMATargetRating(ctx context.Context, sessionID, companyKey string, rating int, subject, email string) error
	UpsertMAWebValidation(ctx context.Context, input maWebValidationUpsert) (MAWebValidation, error)
	StartMATrace(ctx context.Context, input maTraceStart) (string, error)
	LinkMATrace(ctx context.Context, input maTraceLink) error
	CompleteMATrace(ctx context.Context, input maTraceComplete) error
	RecordMATraceEvent(ctx context.Context, input maTraceEventWrite) error
	RecordMAExport(ctx context.Context, sessionID, format string, rowCount int, createdByEmail string) error
	ListMAParameters(ctx context.Context) ([]MAParameter, error)
	UpdateMAParameter(ctx context.Context, key, value, email string) error
	ListMACompanyLegalForms(ctx context.Context) ([]maCompanyLegalForm, error)
	ListMADeepAnalysis(ctx context.Context, companyKeys []string) (map[string]MADeepAnalysis, error)
	EnqueueMADeepAnalysis(ctx context.Context, companyKey, vatCode, taxCode, email string) error
	ListMADeepReadyPayloads(ctx context.Context) ([]maDeepPayloadRow, error)
	UpdateMADeepScorecard(ctx context.Context, companyKey string, scorecard *MADeepScorecard) error
	GetMADeepByVAT(ctx context.Context, vat string) (*maDeepVATRecord, error)
	ListMADeepReadyForBrief(ctx context.Context) ([]maDeepBriefRow, error)
	UpdateMADeepBrief(ctx context.Context, companyKey string, brief *MADeepBrief, modelID, promptID string) error
}

type maCompanyLegalForm struct {
	Code          string
	DescriptionIT string
	DescriptionEN string
}

type maWebValidationUpsert struct {
	SessionID              string
	CompanyKey             string
	TargetID               string
	RunID                  string
	SelectedDomain         string
	DomainConfidence       string
	DomainScore            *int
	WebScore               int
	WebConfidence          string
	WebValidationState     string
	FinalAction            string
	AnalystVerdict         string
	AnalystAction          string
	AnalystConfidence      string
	Summary                json.RawMessage
	KeywordSet             json.RawMessage
	SelectedDomainPayload  json.RawMessage
	DomainResponse         json.RawMessage
	EvidenceRuns           json.RawMessage
	CandidateMatchAnalysis json.RawMessage
	CandidateMatchError    string
	FinalDecision          json.RawMessage
	Subject                string
	Email                  string
}

func (s *SQLStore) ListMASessions(ctx context.Context, visibility string) ([]MASessionSummary, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	where, orderBy := maSessionListClauses(visibility)
	rows, err := s.db.QueryContext(ctx, `
SELECT
  session.id::text,
  session.title,
  session.prompt,
  session.status,
  session.selected_strategy,
  COALESCE((SELECT SUM(estimated_count) FROM binocolo.ma_dry_run_estimate estimate WHERE estimate.session_id = session.id AND estimate.selected), 0) AS estimated_count,
  COALESCE((SELECT SUM(estimated_cost) FROM binocolo.ma_dry_run_estimate estimate WHERE estimate.session_id = session.id AND estimate.selected), 0) AS estimated_cost,
  COALESCE((SELECT COUNT(*) FROM binocolo.ma_target target WHERE target.session_id = session.id), 0) AS result_count,
  session.created_at,
  session.updated_at,
  session.last_executed_at,
  session.archived_at,
  session.archived_by_email,
  session.deleted_at,
  session.deleted_by_email
FROM binocolo.ma_session session
WHERE `+where+`
ORDER BY `+orderBy+`
LIMIT 80
`)
	if err != nil {
		return nil, fmt.Errorf("list ma sessions: %w", err)
	}
	defer rows.Close()

	out := []MASessionSummary{}
	for rows.Next() {
		var item MASessionSummary
		var selected sql.NullString
		var lastRun sql.NullTime
		var archivedAt sql.NullTime
		var archivedByEmail sql.NullString
		var deletedAt sql.NullTime
		var deletedByEmail sql.NullString
		if err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Prompt,
			&item.Status,
			&selected,
			&item.EstimatedCount,
			&item.EstimatedCost,
			&item.ResultCount,
			&item.CreatedAt,
			&item.UpdatedAt,
			&lastRun,
			&archivedAt,
			&archivedByEmail,
			&deletedAt,
			&deletedByEmail,
		); err != nil {
			return nil, fmt.Errorf("scan ma session summary: %w", err)
		}
		item.SelectedStrategy = selected.String
		if lastRun.Valid {
			item.LastRunAt = &lastRun.Time
		}
		if archivedAt.Valid {
			item.ArchivedAt = &archivedAt.Time
		}
		item.ArchivedByEmail = archivedByEmail.String
		if deletedAt.Valid {
			item.DeletedAt = &deletedAt.Time
		}
		item.DeletedByEmail = deletedByEmail.String
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma sessions: %w", err)
	}
	return out, nil
}

func maSessionListClauses(visibility string) (string, string) {
	switch visibility {
	case maSessionVisibilityArchived:
		return "session.archived_at IS NOT NULL AND session.deleted_at IS NULL", "session.archived_at DESC, session.updated_at DESC"
	case maSessionVisibilityDeleted:
		return "session.deleted_at IS NOT NULL AND session.purged_at IS NULL", "session.deleted_at DESC, session.updated_at DESC"
	default:
		return "session.archived_at IS NULL AND session.deleted_at IS NULL", "session.updated_at DESC, session.created_at DESC"
	}
}

func (s *SQLStore) ListMACompanyLegalForms(ctx context.Context) ([]maCompanyLegalForm, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT code, description_it, description_en
FROM binocolo.company_legal_forms
ORDER BY sort_order
`)
	if err != nil {
		return nil, fmt.Errorf("list ma company legal forms: %w", err)
	}
	defer rows.Close()

	out := []maCompanyLegalForm{}
	for rows.Next() {
		var item maCompanyLegalForm
		if err := rows.Scan(&item.Code, &item.DescriptionIT, &item.DescriptionEN); err != nil {
			return nil, fmt.Errorf("scan ma company legal form: %w", err)
		}
		item.Code = strings.ToUpper(strings.TrimSpace(item.Code))
		item.DescriptionIT = cleanText(item.DescriptionIT, 180)
		item.DescriptionEN = cleanText(item.DescriptionEN, 180)
		if item.Code == "" {
			continue
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma company legal forms: %w", err)
	}
	return out, nil
}

func (s *SQLStore) CreateMASession(ctx context.Context, input maSessionCreate) (MASessionDetail, error) {
	if s == nil || s.db == nil {
		return MASessionDetail{}, errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MASessionDetail{}, fmt.Errorf("begin ma session create: %w", err)
	}
	defer tx.Rollback()

	sessionID := input.Session.ID
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	strategyID := uuid.NewString()
	rawStrategy, err := strategyToRaw(input.Strategy)
	if err != nil {
		return MASessionDetail{}, fmt.Errorf("marshal ma strategy: %w", err)
	}

	var session MASession
	var estimated sql.NullTime
	var executed sql.NullTime
	if err := tx.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_session (
  id,
  title,
  prompt,
  status,
  created_by_subject,
  created_by_email
) VALUES (
  $1::uuid,
  $2,
  $3,
  $4,
  $5,
  $6
)
RETURNING id::text, title, prompt, status, COALESCE(selected_strategy, ''),
          COALESCE(created_by_subject, ''), COALESCE(created_by_email, ''), last_estimated_at,
          last_executed_at, created_at, updated_at
`, sessionID,
		input.Session.Title,
		input.Session.Prompt,
		maSessionStatusDraft,
		nullString(input.Session.CreatedBySubject),
		nullString(input.Session.CreatedByEmail),
	).Scan(
		&session.ID,
		&session.Title,
		&session.Prompt,
		&session.Status,
		&session.SelectedStrategy,
		&session.CreatedBySubject,
		&session.CreatedByEmail,
		&estimated,
		&executed,
		&session.CreatedAt,
		&session.UpdatedAt,
	); err != nil {
		return MASessionDetail{}, fmt.Errorf("insert ma session: %w", err)
	}
	if estimated.Valid {
		session.LastEstimatedAt = &estimated.Time
	}
	if executed.Valid {
		session.LastExecutedAt = &executed.Time
	}

	strategy, err := insertMAStrategyVersion(ctx, tx, session.ID, strategyID, 1, rawStrategy, input.Session.CreatedByEmail)
	if err != nil {
		return MASessionDetail{}, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET active_strategy_id = $2::uuid
WHERE id = $1::uuid
`, session.ID, strategyID); err != nil {
		return MASessionDetail{}, fmt.Errorf("activate initial ma strategy: %w", err)
	}
	session.ActiveStrategyID = strategyID
	if err := tx.Commit(); err != nil {
		return MASessionDetail{}, fmt.Errorf("commit ma session create: %w", err)
	}
	return MASessionDetail{Session: session, Strategy: &strategy, Estimates: []MAEstimate{}, Runs: []MAExecutionRun{}, Targets: []MATarget{}}, nil
}

func (s *SQLStore) GetMASession(ctx context.Context, id string) (MASessionDetail, error) {
	if s == nil || s.db == nil {
		return MASessionDetail{}, errors.New("binocolo ma store not configured")
	}
	session, err := s.loadMASession(ctx, s.db, id)
	if err != nil {
		return MASessionDetail{}, err
	}
	strategy, err := s.loadActiveMAStrategy(ctx, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return MASessionDetail{}, err
	}
	estimates, err := s.loadMAEstimates(ctx, id)
	if err != nil {
		return MASessionDetail{}, err
	}
	runs, err := s.loadMARuns(ctx, id)
	if err != nil {
		return MASessionDetail{}, err
	}
	targets, err := s.loadMATargets(ctx, id)
	if err != nil {
		return MASessionDetail{}, err
	}
	detail := MASessionDetail{Session: session, Estimates: estimates, Runs: runs, Targets: targets}
	if strategy.ID != "" {
		detail.Strategy = &strategy
	}
	return detail, nil
}

// GetMASessionState loads only the session row (no strategy/estimates/runs/targets),
// for cheap lifecycle checks such as the rating endpoint.
func (s *SQLStore) GetMASessionState(ctx context.Context, id string) (MASession, error) {
	if s == nil || s.db == nil {
		return MASession{}, errors.New("binocolo ma store not configured")
	}
	return s.loadMASession(ctx, s.db, id)
}

func (s *SQLStore) UpdateMASessionLifecycle(ctx context.Context, sessionID, action, subject, email string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	var query string
	switch action {
	case maSessionLifecycleArchive:
		query = `
UPDATE binocolo.ma_session
SET archived_at = COALESCE(archived_at, now()),
    archived_by_subject = CASE WHEN archived_at IS NULL THEN $2 ELSE archived_by_subject END,
    archived_by_email = CASE WHEN archived_at IS NULL THEN $3 ELSE archived_by_email END,
    deleted_at = NULL,
    deleted_by_subject = NULL,
    deleted_by_email = NULL,
    updated_at = now()
WHERE id = $1::uuid
  AND deleted_at IS NULL
RETURNING id::text
`
	case maSessionLifecycleRestore:
		query = `
UPDATE binocolo.ma_session
SET archived_at = NULL,
    archived_by_subject = NULL,
    archived_by_email = NULL,
    deleted_at = NULL,
    deleted_by_subject = NULL,
    deleted_by_email = NULL,
    updated_at = now()
WHERE id = $1::uuid
  AND (archived_at IS NOT NULL OR deleted_at IS NOT NULL)
RETURNING id::text
`
	case maSessionLifecycleDelete:
		query = `
UPDATE binocolo.ma_session
SET deleted_at = COALESCE(deleted_at, now()),
    deleted_by_subject = CASE WHEN deleted_at IS NULL THEN $2 ELSE deleted_by_subject END,
    deleted_by_email = CASE WHEN deleted_at IS NULL THEN $3 ELSE deleted_by_email END,
    archived_at = NULL,
    archived_by_subject = NULL,
    archived_by_email = NULL,
    updated_at = now()
WHERE id = $1::uuid
RETURNING id::text
`
	case maSessionLifecyclePurge:
		query = `
UPDATE binocolo.ma_session
SET purged_at = COALESCE(purged_at, now()),
    purged_by_subject = CASE WHEN purged_at IS NULL THEN $2 ELSE purged_by_subject END,
    purged_by_email = CASE WHEN purged_at IS NULL THEN $3 ELSE purged_by_email END,
    updated_at = now()
WHERE id = $1::uuid
  AND deleted_at IS NOT NULL
RETURNING id::text
`
	default:
		return false, fmt.Errorf("invalid ma session lifecycle action: %s", action)
	}
	var id string
	args := []any{sessionID}
	actionsWithSubject := map[string]bool{
		maSessionLifecycleArchive: true,
		maSessionLifecycleDelete:  true,
		maSessionLifecyclePurge:   true,
	}
	if actionsWithSubject[action] {
		args = append(args, nullString(subject), nullString(email))
	}
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("update ma session lifecycle: %w", err)
	}
	return true, nil
}

func (s *SQLStore) AddMAStrategyVersion(ctx context.Context, sessionID string, strategySpec MAStrategySpec, createdByEmail string) (*MAStrategyVersion, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin ma strategy version: %w", err)
	}
	defer tx.Rollback()

	var version int
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(MAX(version), 0) + 1
FROM binocolo.ma_strategy_version
WHERE session_id = $1::uuid
`, sessionID).Scan(&version); err != nil {
		return nil, fmt.Errorf("next ma strategy version: %w", err)
	}
	strategyID := uuid.NewString()
	rawStrategy, err := strategyToRaw(strategySpec)
	if err != nil {
		return nil, fmt.Errorf("marshal ma strategy: %w", err)
	}
	strategy, err := insertMAStrategyVersion(ctx, tx, sessionID, strategyID, version, rawStrategy, createdByEmail)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM binocolo.ma_dry_run_estimate
WHERE session_id = $1::uuid
`, sessionID); err != nil {
		return nil, fmt.Errorf("clear ma estimates for new strategy: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET active_strategy_id = $2::uuid,
    selected_strategy = NULL,
    status = $3,
    updated_at = now()
WHERE id = $1::uuid
`, sessionID, strategyID, maSessionStatusDraft); err != nil {
		return nil, fmt.Errorf("activate ma strategy version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit ma strategy version: %w", err)
	}
	return &strategy, nil
}

func (s *SQLStore) ReplaceMAEstimates(ctx context.Context, sessionID, strategyVersionID, selectedStrategy string, estimates []MAEstimate) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ma estimate replace: %w", err)
	}
	defer tx.Rollback()

	// Lock the session row and verify the estimate still targets the active strategy
	// version. If a newer version was activated mid-estimate, abort the whole write
	// (do not clobber the current estimates) and report it superseded so the worker
	// re-runs for the now-active version. FOR UPDATE serializes against the
	// AddMAStrategyVersion that would flip active_strategy_id.
	var activeStrategyID sql.NullString
	if err := tx.QueryRowContext(ctx, `
SELECT active_strategy_id::text
FROM binocolo.ma_session
WHERE id = $1::uuid
FOR UPDATE
`, sessionID).Scan(&activeStrategyID); err != nil {
		return fmt.Errorf("lock ma session for estimate: %w", err)
	}
	if !activeStrategyID.Valid || activeStrategyID.String != strategyVersionID {
		return errMAEstimateSuperseded
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM binocolo.ma_dry_run_estimate
WHERE session_id = $1::uuid
`, sessionID); err != nil {
		return fmt.Errorf("delete ma estimates: %w", err)
	}
	for _, estimate := range estimates {
		id := estimate.ID
		if id == "" {
			id = uuid.NewString()
		}
		params := json.RawMessage(`{}`)
		if len(estimate.Params) > 0 {
			params = estimate.Params
		}
		response := json.RawMessage(`{}`)
		if len(estimate.VendorResponse) > 0 {
			response = estimate.VendorResponse
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO binocolo.ma_dry_run_estimate (
  id,
  session_id,
  strategy_version_id,
  strategy_type,
  ateco_code,
  ateco_description,
  province,
	  estimated_count,
	  estimated_cost,
	  selected,
	  surface_status,
	  execution_limit,
	  probe_count,
	  params,
	  vendor_response
	) VALUES (
	  $1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15::jsonb
	)
`, id,
			sessionID,
			strategyVersionID,
			estimate.StrategyType,
			nullString(estimate.AtecoCode),
			nullString(estimate.AtecoDescription),
			nullString(estimate.Province),
			estimate.EstimatedCount,
			estimate.EstimatedCost,
			estimate.Selected,
			defaultString(estimate.SurfaceStatus, maEstimateSurfaceExact),
			positiveOrDefault(estimate.ExecutionLimit, maDefaultSearchLimit),
			positiveOrDefault(estimate.ProbeCount, 1),
			[]byte(params),
			[]byte(response),
		); err != nil {
			return fmt.Errorf("insert ma estimate: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET status = $3,
    selected_strategy = $4,
    last_estimated_at = now(),
    updated_at = now()
WHERE id = $1::uuid
  AND active_strategy_id = $2::uuid
`, sessionID, strategyVersionID, maSessionStatusEstimated, nullString(selectedStrategy)); err != nil {
		return fmt.Errorf("update ma estimate session: %w", err)
	}
	return tx.Commit()
}

func (s *SQLStore) CreateMAExecutionRun(ctx context.Context, input maExecutionRunCreate) (MAExecutionRun, error) {
	if s == nil || s.db == nil {
		return MAExecutionRun{}, errors.New("binocolo ma store not configured")
	}
	id := input.ID
	if id == "" {
		id = uuid.NewString()
	}
	var run MAExecutionRun
	err := s.db.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_execution_run (
  id,
  session_id,
  strategy_version_id,
  strategy_type,
  status,
  estimated_count
) VALUES (
  $1::uuid, $2::uuid, $3::uuid, $4, $5, $6
)
RETURNING id::text, session_id::text, strategy_version_id::text, strategy_type, status, estimated_count,
          result_count, COALESCE(error_code, ''), started_at, completed_at
`, id,
		input.SessionID,
		input.StrategyVersionID,
		input.StrategyType,
		maRunStatusRunning,
		input.EstimatedCount,
	).Scan(
		&run.ID,
		&run.SessionID,
		&run.StrategyVersionID,
		&run.StrategyType,
		&run.Status,
		&run.EstimatedCount,
		&run.ResultCount,
		&run.ErrorCode,
		&run.StartedAt,
		&run.CompletedAt,
	)
	if err != nil {
		return MAExecutionRun{}, fmt.Errorf("create ma execution run: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET status = $2, updated_at = now()
WHERE id = $1::uuid
`, input.SessionID, maSessionStatusRunning); err != nil {
		return MAExecutionRun{}, fmt.Errorf("mark ma session running: %w", err)
	}
	return run, nil
}

func (s *SQLStore) CompleteMAExecutionRun(ctx context.Context, runID, status string, resultCount int, errorCode string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ma run complete: %w", err)
	}
	defer tx.Rollback()

	var sessionID string
	if err := tx.QueryRowContext(ctx, `
UPDATE binocolo.ma_execution_run
SET status = $2,
    result_count = $3,
    error_code = $4,
    completed_at = now()
WHERE id = $1::uuid
RETURNING session_id::text
`, runID, status, resultCount, nullString(errorCode)).Scan(&sessionID); err != nil {
		return fmt.Errorf("complete ma run: %w", err)
	}
	sessionStatus := maSessionStatusCompleted
	if status == maRunStatusFailed {
		sessionStatus = maSessionStatusFailed
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET status = $2,
    last_executed_at = now(),
    updated_at = now()
WHERE id = $1::uuid
`, sessionID, sessionStatus); err != nil {
		return fmt.Errorf("complete ma session: %w", err)
	}
	return tx.Commit()
}

func (s *SQLStore) ReplaceMATargets(ctx context.Context, sessionID, runID string, targets []MATarget) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ma target replace: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
DELETE FROM binocolo.ma_evidence
WHERE target_id IN (SELECT id FROM binocolo.ma_target WHERE session_id = $1::uuid)
`, sessionID); err != nil {
		return fmt.Errorf("delete ma evidence: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM binocolo.ma_target WHERE session_id = $1::uuid`, sessionID); err != nil {
		return fmt.Errorf("delete ma targets: %w", err)
	}
	for _, target := range targets {
		targetID := target.ID
		if targetID == "" {
			targetID = uuid.NewString()
		}
		missingRaw, err := json.Marshal(target.MissingCriteria)
		if err != nil {
			return fmt.Errorf("marshal ma target missing criteria: %w", err)
		}
		flagsRaw := []byte("[]")
		if len(target.Flags) > 0 {
			raw, err := json.Marshal(target.Flags)
			if err != nil {
				return fmt.Errorf("marshal ma target flags: %w", err)
			}
			flagsRaw = raw
		}
		payload := json.RawMessage(`{}`)
		if len(target.VendorPayload) > 0 {
			payload = target.VendorPayload
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO binocolo.ma_target (
  id,
  session_id,
  run_id,
  vendor_id,
  company_name,
  vat_code,
  tax_code,
  province,
  town,
  activity_status,
  turnover,
  turnover_year,
  employees,
  ateco_code,
  ateco_description,
  score,
  match_state,
  confidence,
  flags,
  rationale,
  missing_criteria,
  vendor_payload
) VALUES (
  $1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10,
  $11, $12, $13, $14, $15, $16, $17, $18, $19::jsonb, $20,
  $21::jsonb, $22::jsonb
)
`, targetID,
			sessionID,
			runID,
			nullString(target.VendorID),
			target.CompanyName,
			nullString(target.VATCode),
			nullString(target.TaxCode),
			nullString(target.Province),
			nullString(target.Town),
			nullString(target.ActivityStatus),
			nullInt(target.Turnover),
			nullInt(target.TurnoverYear),
			nullInt(target.Employees),
			nullString(target.AtecoCode),
			nullString(target.AtecoDescription),
			target.Score,
			target.MatchState,
			nullString(target.Confidence),
			flagsRaw,
			target.Rationale,
			missingRaw,
			[]byte(payload),
		); err != nil {
			return fmt.Errorf("insert ma target: %w", err)
		}
		for _, evidence := range target.Evidence {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO binocolo.ma_evidence (
  id,
  target_id,
  criterion,
  status,
  family,
  label,
  value,
  points,
  weight,
  source_path
) VALUES (
  $1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10
)
`, uuid.NewString(),
				targetID,
				evidence.Criterion,
				evidence.Status,
				nullString(evidence.Family),
				evidence.Label,
				nullString(evidence.Value),
				evidence.Points,
				evidence.Weight,
				nullString(evidence.SourcePath),
			); err != nil {
				return fmt.Errorf("insert ma evidence: %w", err)
			}
		}
	}
	return tx.Commit()
}

func (s *SQLStore) StartMATrace(ctx context.Context, input maTraceStart) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	id := input.ID
	if id == "" {
		id = uuid.NewString()
	}
	request := json.RawMessage(`{}`)
	if len(input.Request) > 0 {
		request = input.Request
	}
	startedAt := input.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	var out string
	if err := s.db.QueryRowContext(ctx, `
	INSERT INTO binocolo.ma_operation_trace (
	  id,
	  request_id,
	  operation,
	  method,
	  path,
	  status,
	  session_id,
	  created_by_subject,
	  created_by_email,
	  request,
	  started_at
	) VALUES (
	  $1::uuid, $2, $3, $4, $5, $6, NULLIF($7, '')::uuid, $8, $9, $10::jsonb, $11
	)
	RETURNING id::text
	`, id,
		input.RequestID,
		input.Operation,
		input.Method,
		input.Path,
		maTraceStatusRunning,
		input.SessionID,
		input.CreatedBySubject,
		input.CreatedByEmail,
		[]byte(request),
		startedAt,
	).Scan(&out); err != nil {
		return "", fmt.Errorf("start ma trace: %w", err)
	}
	return out, nil
}

func (s *SQLStore) LinkMATrace(ctx context.Context, input maTraceLink) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if input.ID == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
	UPDATE binocolo.ma_operation_trace
	SET session_id = COALESCE(NULLIF($2, '')::uuid, session_id),
	    strategy_version_id = COALESCE(NULLIF($3, '')::uuid, strategy_version_id),
	    execution_run_id = COALESCE(NULLIF($4, '')::uuid, execution_run_id)
	WHERE id = $1::uuid
	`, input.ID, input.SessionID, input.StrategyVersionID, input.ExecutionRunID)
	if err != nil {
		return fmt.Errorf("link ma trace: %w", err)
	}
	return nil
}

func (s *SQLStore) CompleteMATrace(ctx context.Context, input maTraceComplete) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if input.ID == "" {
		return nil
	}
	completedAt := input.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	status := input.Status
	if status == "" {
		status = maTraceStatusSucceeded
	}
	_, err := s.db.ExecContext(ctx, `
	UPDATE binocolo.ma_operation_trace
	SET status = $2,
	    http_status = $3,
	    error_code = $4,
	    error_message = $5,
	    completed_at = $6,
	    duration_ms = GREATEST(0, FLOOR(EXTRACT(EPOCH FROM ($6 - started_at)) * 1000)::integer)
	WHERE id = $1::uuid
	`, input.ID,
		status,
		nullIntValue(input.HTTPStatus),
		input.ErrorCode,
		input.ErrorMessage,
		completedAt,
	)
	if err != nil {
		return fmt.Errorf("complete ma trace: %w", err)
	}
	return nil
}

func (s *SQLStore) RecordMATraceEvent(ctx context.Context, input maTraceEventWrite) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if input.TraceID == "" {
		return nil
	}
	request := json.RawMessage(`{}`)
	if len(input.Request) > 0 {
		request = input.Request
	}
	response := json.RawMessage(`{}`)
	if len(input.Response) > 0 {
		response = input.Response
	}
	metadata := json.RawMessage(`{}`)
	if len(input.Metadata) > 0 {
		metadata = input.Metadata
	}
	_, err := s.db.ExecContext(ctx, `
	INSERT INTO binocolo.ma_operation_trace_event (
	  trace_id,
	  event_order,
	  event_type,
	  round,
	  tool_name,
	  external_system,
	  status,
	  duration_ms,
	  request,
	  response,
	  metadata,
	  error
	) VALUES (
	  $1::uuid, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11::jsonb, $12
	)
	`, input.TraceID,
		input.EventOrder,
		input.EventType,
		nullInt(input.Round),
		input.ToolName,
		input.ExternalSystem,
		defaultString(input.Status, maTraceEventInfo),
		nullInt(input.DurationMS),
		[]byte(request),
		[]byte(response),
		[]byte(metadata),
		input.Error,
	)
	if err != nil {
		return fmt.Errorf("record ma trace event: %w", err)
	}
	return nil
}

func (s *SQLStore) RecordMAExport(ctx context.Context, sessionID, format string, rowCount int, createdByEmail string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_export (
  id,
  session_id,
  format,
  row_count,
  created_by_email
) VALUES (
  $1::uuid, $2::uuid, $3, $4, $5
)
`, uuid.NewString(), sessionID, strings.ToLower(strings.TrimSpace(format)), rowCount, nullString(createdByEmail))
	if err != nil {
		return fmt.Errorf("record ma export: %w", err)
	}
	return nil
}

func insertMAStrategyVersion(ctx context.Context, tx *sql.Tx, sessionID, strategyID string, version int, rawStrategy json.RawMessage, createdByEmail string) (MAStrategyVersion, error) {
	var strategy MAStrategyVersion
	var raw []byte
	if err := tx.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_strategy_version (
  id,
  session_id,
  version,
  strategy,
  created_by_email
) VALUES (
  $1::uuid, $2::uuid, $3, $4::jsonb, $5
)
RETURNING id::text, session_id::text, version, strategy, COALESCE(created_by_email, ''), created_at
`, strategyID, sessionID, version, []byte(rawStrategy), nullString(createdByEmail)).Scan(
		&strategy.ID,
		&strategy.SessionID,
		&strategy.Version,
		&raw,
		&strategy.CreatedByEmail,
		&strategy.CreatedAt,
	); err != nil {
		return MAStrategyVersion{}, fmt.Errorf("insert ma strategy version: %w", err)
	}
	parsed, err := rawToStrategy(raw)
	if err != nil {
		return MAStrategyVersion{}, fmt.Errorf("decode ma strategy version: %w", err)
	}
	strategy.Strategy = parsed
	return strategy, nil
}

func (s *SQLStore) loadMASession(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string) (MASession, error) {
	var session MASession
	var selected sql.NullString
	var active sql.NullString
	var subject sql.NullString
	var email sql.NullString
	var estimated sql.NullTime
	var executed sql.NullTime
	var archivedAt sql.NullTime
	var archivedBySubject sql.NullString
	var archivedByEmail sql.NullString
	var deletedAt sql.NullTime
	var deletedBySubject sql.NullString
	var deletedByEmail sql.NullString
	err := q.QueryRowContext(ctx, `
SELECT id::text, title, prompt, status, selected_strategy, active_strategy_id::text,
       created_by_subject, created_by_email, last_estimated_at, last_executed_at,
       created_at, updated_at,
       archived_at, archived_by_subject, archived_by_email,
       deleted_at, deleted_by_subject, deleted_by_email
FROM binocolo.ma_session
WHERE id = $1::uuid
`, id).Scan(
		&session.ID,
		&session.Title,
		&session.Prompt,
		&session.Status,
		&selected,
		&active,
		&subject,
		&email,
		&estimated,
		&executed,
		&session.CreatedAt,
		&session.UpdatedAt,
		&archivedAt,
		&archivedBySubject,
		&archivedByEmail,
		&deletedAt,
		&deletedBySubject,
		&deletedByEmail,
	)
	if err != nil {
		return MASession{}, fmt.Errorf("load ma session: %w", err)
	}
	session.SelectedStrategy = selected.String
	session.ActiveStrategyID = active.String
	session.CreatedBySubject = subject.String
	session.CreatedByEmail = email.String
	if estimated.Valid {
		session.LastEstimatedAt = &estimated.Time
	}
	if executed.Valid {
		session.LastExecutedAt = &executed.Time
	}
	if archivedAt.Valid {
		session.ArchivedAt = &archivedAt.Time
	}
	session.ArchivedBySubject = archivedBySubject.String
	session.ArchivedByEmail = archivedByEmail.String
	if deletedAt.Valid {
		session.DeletedAt = &deletedAt.Time
	}
	session.DeletedBySubject = deletedBySubject.String
	session.DeletedByEmail = deletedByEmail.String
	return session, nil
}

func (s *SQLStore) loadActiveMAStrategy(ctx context.Context, sessionID string) (MAStrategyVersion, error) {
	var strategy MAStrategyVersion
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
SELECT strategy.id::text, strategy.session_id::text, strategy.version, strategy.strategy,
       COALESCE(strategy.created_by_email, ''), strategy.created_at
FROM binocolo.ma_strategy_version strategy
JOIN binocolo.ma_session session ON session.active_strategy_id = strategy.id
WHERE session.id = $1::uuid
`, sessionID).Scan(
		&strategy.ID,
		&strategy.SessionID,
		&strategy.Version,
		&raw,
		&strategy.CreatedByEmail,
		&strategy.CreatedAt,
	)
	if err != nil {
		return MAStrategyVersion{}, fmt.Errorf("load active ma strategy: %w", err)
	}
	parsed, err := rawToStrategy(raw)
	if err != nil {
		return MAStrategyVersion{}, fmt.Errorf("decode active ma strategy: %w", err)
	}
	strategy.Strategy = parsed
	return strategy, nil
}

// GetMAStrategyVersion loads a specific strategy version by id (scoped to its
// session). The execute job pins the version the user confirmed, so the worker
// executes exactly that — not whatever is active when it later runs.
func (s *SQLStore) GetMAStrategyVersion(ctx context.Context, sessionID, versionID string) (MAStrategyVersion, error) {
	if s == nil || s.db == nil {
		return MAStrategyVersion{}, errors.New("binocolo ma store not configured")
	}
	var strategy MAStrategyVersion
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
SELECT strategy.id::text, strategy.session_id::text, strategy.version, strategy.strategy,
       COALESCE(strategy.created_by_email, ''), strategy.created_at
FROM binocolo.ma_strategy_version strategy
WHERE strategy.id = $1::uuid AND strategy.session_id = $2::uuid
`, versionID, sessionID).Scan(
		&strategy.ID,
		&strategy.SessionID,
		&strategy.Version,
		&raw,
		&strategy.CreatedByEmail,
		&strategy.CreatedAt,
	)
	if err != nil {
		return MAStrategyVersion{}, fmt.Errorf("get ma strategy version: %w", err)
	}
	parsed, err := rawToStrategy(raw)
	if err != nil {
		return MAStrategyVersion{}, fmt.Errorf("decode ma strategy version: %w", err)
	}
	strategy.Strategy = parsed
	return strategy, nil
}

func (s *SQLStore) loadMAEstimates(ctx context.Context, sessionID string) ([]MAEstimate, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, session_id::text, strategy_version_id::text, strategy_type,
       COALESCE(ateco_code, ''), COALESCE(ateco_description, ''), COALESCE(province, ''),
       estimated_count, estimated_cost, selected,
       COALESCE(surface_status, 'exact'), execution_limit, probe_count,
       params, vendor_response, created_at
FROM binocolo.ma_dry_run_estimate
WHERE session_id = $1::uuid
ORDER BY selected DESC, strategy_type, province, ateco_code
`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load ma estimates: %w", err)
	}
	defer rows.Close()
	out := []MAEstimate{}
	for rows.Next() {
		var item MAEstimate
		if err := rows.Scan(
			&item.ID,
			&item.SessionID,
			&item.StrategyVersionID,
			&item.StrategyType,
			&item.AtecoCode,
			&item.AtecoDescription,
			&item.Province,
			&item.EstimatedCount,
			&item.EstimatedCost,
			&item.Selected,
			&item.SurfaceStatus,
			&item.ExecutionLimit,
			&item.ProbeCount,
			&item.Params,
			&item.VendorResponse,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ma estimate: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma estimates: %w", err)
	}
	return out, nil
}

func (s *SQLStore) loadMARuns(ctx context.Context, sessionID string) ([]MAExecutionRun, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, session_id::text, strategy_version_id::text, strategy_type, status,
       estimated_count, result_count, COALESCE(error_code, ''), started_at, completed_at
FROM binocolo.ma_execution_run
WHERE session_id = $1::uuid
ORDER BY started_at DESC
`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load ma runs: %w", err)
	}
	defer rows.Close()
	out := []MAExecutionRun{}
	for rows.Next() {
		var item MAExecutionRun
		if err := rows.Scan(
			&item.ID,
			&item.SessionID,
			&item.StrategyVersionID,
			&item.StrategyType,
			&item.Status,
			&item.EstimatedCount,
			&item.ResultCount,
			&item.ErrorCode,
			&item.StartedAt,
			&item.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ma run: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma runs: %w", err)
	}
	return out, nil
}

func (s *SQLStore) loadMATargets(ctx context.Context, sessionID string) ([]MATarget, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, session_id::text, run_id::text, COALESCE(vendor_id, ''), company_name,
       COALESCE(vat_code, ''), COALESCE(tax_code, ''), COALESCE(province, ''), COALESCE(town, ''),
       COALESCE(activity_status, ''), turnover, turnover_year, employees, COALESCE(ateco_code, ''),
       COALESCE(ateco_description, ''), score, match_state, COALESCE(confidence, ''), flags,
       rationale, missing_criteria, vendor_payload, created_at
FROM binocolo.ma_target
WHERE session_id = $1::uuid
ORDER BY score DESC, company_name
`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load ma targets: %w", err)
	}
	defer rows.Close()
	targets := []MATarget{}
	for rows.Next() {
		var item MATarget
		var turnover sql.NullInt64
		var turnoverYear sql.NullInt64
		var employees sql.NullInt64
		var missingRaw []byte
		var flagsRaw []byte
		if err := rows.Scan(
			&item.ID,
			&item.SessionID,
			&item.RunID,
			&item.VendorID,
			&item.CompanyName,
			&item.VATCode,
			&item.TaxCode,
			&item.Province,
			&item.Town,
			&item.ActivityStatus,
			&turnover,
			&turnoverYear,
			&employees,
			&item.AtecoCode,
			&item.AtecoDescription,
			&item.Score,
			&item.MatchState,
			&item.Confidence,
			&flagsRaw,
			&item.Rationale,
			&missingRaw,
			&item.VendorPayload,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ma target: %w", err)
		}
		if len(flagsRaw) > 0 {
			_ = json.Unmarshal(flagsRaw, &item.Flags)
		}
		if turnover.Valid {
			value := int(turnover.Int64)
			item.Turnover = &value
		}
		if turnoverYear.Valid {
			value := int(turnoverYear.Int64)
			item.TurnoverYear = &value
		}
		if employees.Valid {
			value := int(employees.Int64)
			item.Employees = &value
		}
		if len(missingRaw) > 0 {
			_ = json.Unmarshal(missingRaw, &item.MissingCriteria)
		}
		item.CompanyKey = maTargetDedupeKey(item)
		targets = append(targets, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma targets: %w", err)
	}
	if len(targets) == 0 {
		return targets, nil
	}
	evidence, err := s.loadMAEvidence(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	ratings, err := s.loadMARatings(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	webValidations, err := s.loadMAWebValidations(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(targets))
	for index := range targets {
		keys = append(keys, targets[index].CompanyKey)
	}
	deep, err := s.ListMADeepAnalysis(ctx, keys)
	if err != nil {
		return nil, err
	}
	for index := range targets {
		targets[index].Evidence = evidence[targets[index].ID]
		if rating, ok := ratings[targets[index].CompanyKey]; ok {
			value := rating
			targets[index].Rating = &value
		}
		if validation, ok := webValidations[targets[index].CompanyKey]; ok {
			record := validation
			targets[index].WebValidation = &record
		}
		if analysis, ok := deep[targets[index].CompanyKey]; ok {
			record := analysis
			targets[index].Deep = &record
		}
	}
	sortMATargetsByRating(targets)
	return targets, nil
}

// sortMATargetsByRating ordina i target con la classifica utente in testa
// (rating DESC), poi per punteggio dello scoring (campo secondario), poi per
// nome. I non valutati (rating nil -> 0) stanno tra le stelle e gli esclusi (-1).
func sortMATargetsByRating(targets []MATarget) {
	sort.SliceStable(targets, func(i, j int) bool {
		ri, rj := maRatingValue(targets[i].Rating), maRatingValue(targets[j].Rating)
		if ri != rj {
			return ri > rj
		}
		if targets[i].Score != targets[j].Score {
			return targets[i].Score > targets[j].Score
		}
		return targets[i].CompanyName < targets[j].CompanyName
	})
}

func maRatingValue(rating *int) int {
	if rating == nil {
		return 0
	}
	return *rating
}

func (s *SQLStore) loadMARatings(ctx context.Context, sessionID string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT company_key, rating
FROM binocolo.ma_target_rating
WHERE session_id = $1::uuid
`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load ma ratings: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var key string
		var rating int
		if err := rows.Scan(&key, &rating); err != nil {
			return nil, fmt.Errorf("scan ma rating: %w", err)
		}
		out[key] = rating
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma ratings: %w", err)
	}
	return out, nil
}

// UpsertMATargetRating salva (o azzera) il voto su un'azienda nella sessione.
// rating == 0 cancella la riga (torna "non valutato"); -1/1..3 fanno upsert.
func (s *SQLStore) UpsertMATargetRating(ctx context.Context, sessionID, companyKey string, rating int, subject, email string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if rating == 0 {
		if _, err := s.db.ExecContext(ctx, `
DELETE FROM binocolo.ma_target_rating
WHERE session_id = $1::uuid AND company_key = $2
`, sessionID, companyKey); err != nil {
			return fmt.Errorf("clear ma target rating: %w", err)
		}
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_target_rating (session_id, company_key, rating, rated_by_subject, rated_by_email, rated_at)
VALUES ($1::uuid, $2, $3, $4, $5, now())
ON CONFLICT (session_id, company_key) DO UPDATE
SET rating = EXCLUDED.rating,
    rated_by_subject = EXCLUDED.rated_by_subject,
    rated_by_email = EXCLUDED.rated_by_email,
    rated_at = now()
`, sessionID, companyKey, rating, nullString(subject), nullString(email)); err != nil {
		return fmt.Errorf("upsert ma target rating: %w", err)
	}
	return nil
}

func (s *SQLStore) loadMAWebValidations(ctx context.Context, sessionID string) (map[string]MAWebValidation, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT
  session_id::text,
  company_key,
  COALESCE(target_id::text, ''),
  COALESCE(run_id::text, ''),
  COALESCE(selected_domain, ''),
  COALESCE(domain_confidence, ''),
  domain_score,
  web_score,
  COALESCE(web_confidence, ''),
  web_validation_state,
  final_action,
  COALESCE(analyst_verdict, ''),
  COALESCE(analyst_action, ''),
  COALESCE(analyst_confidence, ''),
  summary,
  keyword_set,
  selected_domain_payload,
  domain_response,
  evidence_runs,
  candidate_match_analysis,
  COALESCE(candidate_match_error, ''),
  final_decision,
  COALESCE(updated_by_email, ''),
  updated_at
FROM binocolo.ma_target_web_validation
WHERE session_id = $1::uuid
`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load ma web validations: %w", err)
	}
	defer rows.Close()
	out := map[string]MAWebValidation{}
	for rows.Next() {
		item, err := scanMAWebValidation(rows)
		if err != nil {
			return nil, err
		}
		out[item.CompanyKey] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma web validations: %w", err)
	}
	return out, nil
}

func (s *SQLStore) UpsertMAWebValidation(ctx context.Context, input maWebValidationUpsert) (MAWebValidation, error) {
	if s == nil || s.db == nil {
		return MAWebValidation{}, errors.New("binocolo ma store not configured")
	}
	row := s.db.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_target_web_validation (
  session_id,
  company_key,
  target_id,
  run_id,
  selected_domain,
  domain_confidence,
  domain_score,
  web_score,
  web_confidence,
  web_validation_state,
  final_action,
  analyst_verdict,
  analyst_action,
  analyst_confidence,
  summary,
  keyword_set,
  selected_domain_payload,
  domain_response,
  evidence_runs,
  candidate_match_analysis,
  candidate_match_error,
  final_decision,
  created_by_subject,
  created_by_email,
  updated_by_subject,
  updated_by_email
) VALUES (
  $1::uuid, $2, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, $5, $6, $7, $8, $9, $10,
  $11, $12, $13, $14, $15::jsonb, $16::jsonb, $17::jsonb, $18::jsonb, $19::jsonb,
  $20::jsonb, $21, $22::jsonb, $23, $24, $23, $24
)
ON CONFLICT (session_id, company_key) DO UPDATE
SET target_id = EXCLUDED.target_id,
    run_id = EXCLUDED.run_id,
    selected_domain = EXCLUDED.selected_domain,
    domain_confidence = EXCLUDED.domain_confidence,
    domain_score = EXCLUDED.domain_score,
    web_score = EXCLUDED.web_score,
    web_confidence = EXCLUDED.web_confidence,
    web_validation_state = EXCLUDED.web_validation_state,
    final_action = EXCLUDED.final_action,
    analyst_verdict = EXCLUDED.analyst_verdict,
    analyst_action = EXCLUDED.analyst_action,
    analyst_confidence = EXCLUDED.analyst_confidence,
    summary = EXCLUDED.summary,
    keyword_set = EXCLUDED.keyword_set,
    selected_domain_payload = EXCLUDED.selected_domain_payload,
    domain_response = EXCLUDED.domain_response,
    evidence_runs = EXCLUDED.evidence_runs,
    candidate_match_analysis = EXCLUDED.candidate_match_analysis,
    candidate_match_error = EXCLUDED.candidate_match_error,
    final_decision = EXCLUDED.final_decision,
    updated_by_subject = EXCLUDED.updated_by_subject,
    updated_by_email = EXCLUDED.updated_by_email,
    updated_at = now()
RETURNING
  session_id::text,
  company_key,
  COALESCE(target_id::text, ''),
  COALESCE(run_id::text, ''),
  COALESCE(selected_domain, ''),
  COALESCE(domain_confidence, ''),
  domain_score,
  web_score,
  COALESCE(web_confidence, ''),
  web_validation_state,
  final_action,
  COALESCE(analyst_verdict, ''),
  COALESCE(analyst_action, ''),
  COALESCE(analyst_confidence, ''),
  summary,
  keyword_set,
  selected_domain_payload,
  domain_response,
  evidence_runs,
  candidate_match_analysis,
  COALESCE(candidate_match_error, ''),
  final_decision,
  COALESCE(updated_by_email, ''),
  updated_at
`, input.SessionID,
		input.CompanyKey,
		input.TargetID,
		input.RunID,
		nullString(input.SelectedDomain),
		nullString(input.DomainConfidence),
		nullInt(input.DomainScore),
		input.WebScore,
		nullString(input.WebConfidence),
		input.WebValidationState,
		input.FinalAction,
		nullString(input.AnalystVerdict),
		nullString(input.AnalystAction),
		nullString(input.AnalystConfidence),
		[]byte(input.Summary),
		[]byte(input.KeywordSet),
		[]byte(input.SelectedDomainPayload),
		[]byte(input.DomainResponse),
		[]byte(input.EvidenceRuns),
		[]byte(input.CandidateMatchAnalysis),
		nullString(input.CandidateMatchError),
		[]byte(input.FinalDecision),
		nullString(input.Subject),
		nullString(input.Email),
	)
	return scanMAWebValidation(row)
}

type maWebValidationScanner interface {
	Scan(dest ...any) error
}

func scanMAWebValidation(row maWebValidationScanner) (MAWebValidation, error) {
	var item MAWebValidation
	var domainScore sql.NullInt64
	var summaryRaw, keywordSetRaw, selectedDomainRaw, domainResponseRaw, evidenceRunsRaw, analysisRaw, finalDecisionRaw []byte
	if err := row.Scan(
		&item.SessionID,
		&item.CompanyKey,
		&item.TargetID,
		&item.RunID,
		&item.SelectedDomain,
		&item.DomainConfidence,
		&domainScore,
		&item.WebScore,
		&item.WebConfidence,
		&item.WebValidationState,
		&item.FinalAction,
		&item.AnalystVerdict,
		&item.AnalystAction,
		&item.AnalystConfidence,
		&summaryRaw,
		&keywordSetRaw,
		&selectedDomainRaw,
		&domainResponseRaw,
		&evidenceRunsRaw,
		&analysisRaw,
		&item.CandidateMatchError,
		&finalDecisionRaw,
		&item.UpdatedByEmail,
		&item.UpdatedAt,
	); err != nil {
		return MAWebValidation{}, fmt.Errorf("scan ma web validation: %w", err)
	}
	if domainScore.Valid {
		value := int(domainScore.Int64)
		item.DomainScore = &value
	}
	_ = json.Unmarshal(summaryRaw, &item.Summary)
	_ = json.Unmarshal(keywordSetRaw, &item.KeywordSet)
	if len(selectedDomainRaw) > 0 && string(selectedDomainRaw) != "null" && string(selectedDomainRaw) != "{}" {
		var candidate DomainResolutionCandidate
		if err := json.Unmarshal(selectedDomainRaw, &candidate); err == nil && candidate.Domain != "" {
			item.SelectedDomainPayload = &candidate
		}
	}
	_ = json.Unmarshal(domainResponseRaw, &item.DomainResponse)
	_ = json.Unmarshal(evidenceRunsRaw, &item.EvidenceRuns)
	if len(analysisRaw) > 0 && string(analysisRaw) != "null" && string(analysisRaw) != "{}" {
		var analysis CandidateMatchAnalysisResponse
		if err := json.Unmarshal(analysisRaw, &analysis); err == nil && analysis.Verdict != "" {
			item.CandidateMatchAnalysis = &analysis
		}
	}
	_ = json.Unmarshal(finalDecisionRaw, &item.FinalDecision)
	return item, nil
}

func (s *SQLStore) loadMAEvidence(ctx context.Context, sessionID string) (map[string][]MATargetEvidence, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT evidence.target_id::text, evidence.criterion, evidence.status,
       COALESCE(evidence.family, ''), evidence.label, COALESCE(evidence.value, ''),
       COALESCE(evidence.points, 0)::float8, COALESCE(evidence.weight, 0)::float8,
       COALESCE(evidence.source_path, '')
FROM binocolo.ma_evidence evidence
JOIN binocolo.ma_target target ON target.id = evidence.target_id
WHERE target.session_id = $1::uuid
ORDER BY evidence.created_at, evidence.id
`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load ma evidence: %w", err)
	}
	defer rows.Close()
	out := map[string][]MATargetEvidence{}
	for rows.Next() {
		var targetID string
		var item MATargetEvidence
		if err := rows.Scan(&targetID, &item.Criterion, &item.Status, &item.Family, &item.Label, &item.Value, &item.Points, &item.Weight, &item.SourcePath); err != nil {
			return nil, fmt.Errorf("scan ma evidence: %w", err)
		}
		out[targetID] = append(out[targetID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma evidence: %w", err)
	}
	return out, nil
}

func (s *SQLStore) ListMAParameters(ctx context.Context) ([]MAParameter, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT key, value, value_type, label, COALESCE(description, ''), COALESCE(updated_by_email, ''), updated_at
FROM binocolo.ma_parameter
ORDER BY key
`)
	if err != nil {
		return nil, fmt.Errorf("list ma parameters: %w", err)
	}
	defer rows.Close()
	out := []MAParameter{}
	for rows.Next() {
		var item MAParameter
		var updatedAt time.Time
		if err := rows.Scan(&item.Key, &item.Value, &item.ValueType, &item.Label, &item.Description, &item.UpdatedByEmail, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan ma parameter: %w", err)
		}
		ts := updatedAt
		item.UpdatedAt = &ts
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma parameters: %w", err)
	}
	return out, nil
}

// UpdateMAParameter updates one existing parameter. It returns sql.ErrNoRows when
// the key does not exist, so callers can reject unknown keys (no implicit insert).
func (s *SQLStore) UpdateMAParameter(ctx context.Context, key, value, email string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_parameter
SET value = $2, updated_by_email = $3, updated_at = now()
WHERE key = $1
`, key, value, nullString(email))
	if err != nil {
		return fmt.Errorf("update ma parameter: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update ma parameter rows: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// --- Deep analysis (veryshort IT-full) -------------------------------------

type maDeepJob struct {
	CompanyKey      string
	VATCode         string
	TaxCode         string
	Status          string
	VendorRequestID string
	Attempts        int
}

func (s *SQLStore) ListMADeepAnalysis(ctx context.Context, companyKeys []string) (map[string]MADeepAnalysis, error) {
	out := map[string]MADeepAnalysis{}
	if s == nil || s.db == nil || len(companyKeys) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(companyKeys))
	args := make([]any, len(companyKeys))
	for i, key := range companyKeys {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = key
	}
	query := `
SELECT company_key, status, scorecard, valuation, brief, cost_eur, COALESCE(error_code, ''), updated_at
FROM binocolo.ma_deep_analysis
WHERE company_key IN (` + strings.Join(placeholders, ", ") + `)`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ma deep analysis: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item MADeepAnalysis
		var scorecardRaw, valuationRaw, briefRaw []byte
		var updatedAt time.Time
		if err := rows.Scan(&item.CompanyKey, &item.Status, &scorecardRaw, &valuationRaw, &briefRaw, &item.CostEUR, &item.ErrorCode, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan ma deep analysis: %w", err)
		}
		if len(scorecardRaw) > 0 {
			var scorecard MADeepScorecard
			if json.Unmarshal(scorecardRaw, &scorecard) == nil {
				item.Scorecard = &scorecard
			}
		}
		if len(valuationRaw) > 0 {
			var valuation MADeepValuation
			if json.Unmarshal(valuationRaw, &valuation) == nil {
				item.Valuation = &valuation
			}
		}
		if len(briefRaw) > 0 {
			var brief MADeepBrief
			if json.Unmarshal(briefRaw, &brief) == nil {
				item.Brief = &brief
			}
		}
		ts := updatedAt
		item.UpdatedAt = &ts
		out[item.CompanyKey] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma deep analysis: %w", err)
	}
	return out, nil
}

// EnqueueMADeepAnalysis queues a company for IT-full. New companies are inserted
// queued; previously failed ones are re-queued; ready/running/queued rows are left
// untouched (the ON CONFLICT WHERE only matches failed) so paid analyses are reused.
func (s *SQLStore) EnqueueMADeepAnalysis(ctx context.Context, companyKey, vatCode, taxCode, email string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_deep_analysis (company_key, vat_code, tax_code, status, refreshed_by_email)
VALUES ($1, $2, $3, 'queued', $4)
ON CONFLICT (company_key) DO UPDATE
SET status = 'queued', attempts = 0, error_code = NULL, vendor_request_id = NULL, requested_at = NULL,
    lease_until = NULL, locked_by = NULL,
    vat_code = EXCLUDED.vat_code, tax_code = EXCLUDED.tax_code,
    refreshed_by_email = EXCLUDED.refreshed_by_email, updated_at = now()
WHERE binocolo.ma_deep_analysis.status = 'failed'
`, companyKey, nullString(vatCode), nullString(taxCode), nullString(email))
	if err != nil {
		return fmt.Errorf("enqueue ma deep analysis: %w", err)
	}
	return nil
}

func (s *SQLStore) ListMADeepJobs(ctx context.Context, limit int, workerID string) ([]maDeepJob, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	if limit <= 0 {
		limit = 16
	}
	// Only surface rows that are free or already owned by this worker, so workers do
	// not even fetch rows another worker is actively processing.
	rows, err := s.db.QueryContext(ctx, `
SELECT company_key, COALESCE(vat_code, ''), COALESCE(tax_code, ''), status, COALESCE(vendor_request_id, ''), attempts
FROM binocolo.ma_deep_analysis
WHERE status IN ('queued', 'running')
  AND (lease_until IS NULL OR lease_until < now() OR locked_by = $2)
ORDER BY updated_at
LIMIT $1
`, limit, workerID)
	if err != nil {
		return nil, fmt.Errorf("list ma deep jobs: %w", err)
	}
	defer rows.Close()
	out := []maDeepJob{}
	for rows.Next() {
		var job maDeepJob
		if err := rows.Scan(&job.CompanyKey, &job.VATCode, &job.TaxCode, &job.Status, &job.VendorRequestID, &job.Attempts); err != nil {
			return nil, fmt.Errorf("scan ma deep job: %w", err)
		}
		out = append(out, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma deep jobs: %w", err)
	}
	return out, nil
}

// ClaimMADeepQueued atomically transitions a row from queued to running. It returns
// true only for the worker/replica that won the claim (rows affected == 1), so with
// multiple replicas the IT-full POST — and its €0.30 charge — happens exactly once.
// Resets the poll attempt budget for the running phase.
func (s *SQLStore) ClaimMADeepQueued(ctx context.Context, companyKey string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET status = 'running', attempts = 0, error_code = NULL, requested_at = now(), updated_at = now()
WHERE company_key = $1 AND status = 'queued'
`, companyKey)
	if err != nil {
		return false, fmt.Errorf("claim ma deep queued: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim ma deep queued rows: %w", err)
	}
	return affected == 1, nil
}

// AcquireMADeepLease grants the calling worker exclusive ownership of a row for
// leaseSeconds (measured on the DB clock, so it is immune to clock skew across worker
// machines). Returns true only for the worker that wins the atomic update; others skip.
// A worker renews its own lease (locked_by match); a crashed worker's lease simply
// expires and the row becomes reclaimable. This is what makes the worker correct with
// multiple instances on one DB (shared staging, k8s replicas, rolling-deploy overlap).
func (s *SQLStore) AcquireMADeepLease(ctx context.Context, companyKey, workerID string, leaseSeconds int) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET lease_until = now() + ($3::int * interval '1 second'), locked_by = $2
WHERE company_key = $1 AND status IN ('queued', 'running')
  AND (lease_until IS NULL OR lease_until < now() OR locked_by = $2)
`, companyKey, workerID, leaseSeconds)
	if err != nil {
		return false, fmt.Errorf("acquire ma deep lease: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("acquire ma deep lease rows: %w", err)
	}
	return affected == 1, nil
}

func (s *SQLStore) SetMADeepVendorRequest(ctx context.Context, companyKey, vendorRequestID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET vendor_request_id = $2, updated_at = now()
WHERE company_key = $1
`, companyKey, vendorRequestID)
	if err != nil {
		return fmt.Errorf("set ma deep vendor request: %w", err)
	}
	return nil
}

func (s *SQLStore) SaveMADeepReady(ctx context.Context, companyKey string, result maDeepResult) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	marshalJSON := func(value any, label string) ([]byte, error) {
		if value == nil {
			return []byte("null"), nil
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("marshal ma deep %s: %w", label, err)
		}
		return raw, nil
	}
	scorecardRaw, err := marshalJSON(result.Scorecard, "scorecard")
	if err != nil {
		return err
	}
	valuationRaw, err := marshalJSON(result.Valuation, "valuation")
	if err != nil {
		return err
	}
	briefRaw, err := marshalJSON(result.Brief, "brief")
	if err != nil {
		return err
	}
	payloadRaw := json.RawMessage("null")
	if len(result.Payload) > 0 {
		payloadRaw = result.Payload
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET status = 'ready', itfull_payload = $2::jsonb, scorecard = $3::jsonb, valuation = $4::jsonb, brief = $5::jsonb,
    model_id = $6::uuid, prompt_id = $7::uuid, cost_eur = $8, error_code = NULL, updated_at = now()
WHERE company_key = $1
`, companyKey, []byte(payloadRaw), scorecardRaw, valuationRaw, briefRaw, nullString(result.ModelID), nullString(result.PromptID), result.CostEUR)
	if err != nil {
		return fmt.Errorf("save ma deep ready: %w", err)
	}
	return nil
}

func (s *SQLStore) MarkMADeepFailed(ctx context.Context, companyKey, errorCode string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET status = 'failed', error_code = $2, updated_at = now()
WHERE company_key = $1
`, companyKey, nullString(errorCode))
	if err != nil {
		return fmt.Errorf("mark ma deep failed: %w", err)
	}
	return nil
}

func (s *SQLStore) BumpMADeepAttempt(ctx context.Context, companyKey string) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("binocolo ma store not configured")
	}
	var attempts int
	if err := s.db.QueryRowContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET attempts = attempts + 1, updated_at = now()
WHERE company_key = $1
RETURNING attempts
`, companyKey).Scan(&attempts); err != nil {
		return 0, fmt.Errorf("bump ma deep attempt: %w", err)
	}
	return attempts, nil
}

type maDeepPayloadRow struct {
	CompanyKey string
	Payload    json.RawMessage
}

// ListMADeepReadyPayloads returns the cached IT-full payload of every ready deep
// analysis, so the scorecard can be rebuilt deterministically without a vendor call.
func (s *SQLStore) ListMADeepReadyPayloads(ctx context.Context) ([]maDeepPayloadRow, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT company_key, itfull_payload
FROM binocolo.ma_deep_analysis
WHERE status = 'ready' AND itfull_payload IS NOT NULL AND itfull_payload <> 'null'::jsonb
ORDER BY updated_at
`)
	if err != nil {
		return nil, fmt.Errorf("list ma deep ready payloads: %w", err)
	}
	defer rows.Close()
	out := []maDeepPayloadRow{}
	for rows.Next() {
		var row maDeepPayloadRow
		var payload []byte
		if err := rows.Scan(&row.CompanyKey, &payload); err != nil {
			return nil, fmt.Errorf("scan ma deep ready payload: %w", err)
		}
		row.Payload = json.RawMessage(payload)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma deep ready payloads: %w", err)
	}
	return out, nil
}

// UpdateMADeepScorecard overwrites only the scorecard column; valuation and brief
// are untouched. Used by the deterministic recompute after an engine calibration
// (the scorecard fixes do not change the valuation inputs).
func (s *SQLStore) UpdateMADeepScorecard(ctx context.Context, companyKey string, scorecard *MADeepScorecard) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	raw := json.RawMessage("null")
	if scorecard != nil {
		b, err := json.Marshal(scorecard)
		if err != nil {
			return fmt.Errorf("marshal ma deep scorecard: %w", err)
		}
		raw = b
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET scorecard = $2::jsonb, updated_at = now()
WHERE company_key = $1
`, companyKey, []byte(raw)); err != nil {
		return fmt.Errorf("update ma deep scorecard: %w", err)
	}
	return nil
}

// maDeepVATRecord is a single cached deep analysis resolved by partita IVA, carrying
// the raw IT-full payload (facts layer) alongside the derived scorecard/valuation/brief.
type maDeepVATRecord struct {
	CompanyKey string
	Status     string
	Scorecard  *MADeepScorecard
	Valuation  *MADeepValuation
	Brief      *MADeepBrief
	Payload    json.RawMessage
	CostEUR    float64
	ErrorCode  string
	UpdatedAt  time.Time
}

// GetMADeepByVAT resolves a deep analysis by vat_code regardless of how its row was
// keyed (the funnel keys by vendor id, the standalone lookup by VAT). A ready row
// wins over an in-flight/failed one, then most recent. Returns (nil, nil) when absent.
func (s *SQLStore) GetMADeepByVAT(ctx context.Context, vat string) (*maDeepVATRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	var rec maDeepVATRecord
	var scorecardRaw, valuationRaw, briefRaw, payloadRaw []byte
	err := s.db.QueryRowContext(ctx, `
SELECT company_key, status, scorecard, valuation, brief, itfull_payload, cost_eur, COALESCE(error_code, ''), updated_at
FROM binocolo.ma_deep_analysis
WHERE vat_code = $1
ORDER BY (status = 'ready') DESC, updated_at DESC
LIMIT 1
`, vat).Scan(&rec.CompanyKey, &rec.Status, &scorecardRaw, &valuationRaw, &briefRaw, &payloadRaw, &rec.CostEUR, &rec.ErrorCode, &rec.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ma deep by vat: %w", err)
	}
	if len(scorecardRaw) > 0 {
		var v MADeepScorecard
		if json.Unmarshal(scorecardRaw, &v) == nil {
			rec.Scorecard = &v
		}
	}
	if len(valuationRaw) > 0 {
		var v MADeepValuation
		if json.Unmarshal(valuationRaw, &v) == nil {
			rec.Valuation = &v
		}
	}
	if len(briefRaw) > 0 {
		var v MADeepBrief
		if json.Unmarshal(briefRaw, &v) == nil {
			rec.Brief = &v
		}
	}
	if len(payloadRaw) > 0 && string(payloadRaw) != "null" {
		rec.Payload = json.RawMessage(payloadRaw)
	}
	return &rec, nil
}

type maDeepBriefRow struct {
	CompanyKey string
	Payload    json.RawMessage
	Valuation  *MADeepValuation
}

// ListMADeepReadyForBrief returns ready rows with their cached payload + valuation, for
// LLM brief regeneration (the scorecard is rebuilt from the payload). No vendor call.
func (s *SQLStore) ListMADeepReadyForBrief(ctx context.Context) ([]maDeepBriefRow, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT company_key, itfull_payload, valuation
FROM binocolo.ma_deep_analysis
WHERE status = 'ready' AND itfull_payload IS NOT NULL AND itfull_payload <> 'null'::jsonb
ORDER BY updated_at
`)
	if err != nil {
		return nil, fmt.Errorf("list ma deep ready for brief: %w", err)
	}
	defer rows.Close()
	out := []maDeepBriefRow{}
	for rows.Next() {
		var row maDeepBriefRow
		var payload, valuationRaw []byte
		if err := rows.Scan(&row.CompanyKey, &payload, &valuationRaw); err != nil {
			return nil, fmt.Errorf("scan ma deep brief row: %w", err)
		}
		row.Payload = json.RawMessage(payload)
		if len(valuationRaw) > 0 {
			var v MADeepValuation
			if json.Unmarshal(valuationRaw, &v) == nil {
				row.Valuation = &v
			}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma deep brief rows: %w", err)
	}
	return out, nil
}

// UpdateMADeepBrief overwrites the brief column and its model/prompt provenance, leaving
// scorecard/valuation/payload untouched. Used by brief regeneration.
func (s *SQLStore) UpdateMADeepBrief(ctx context.Context, companyKey string, brief *MADeepBrief, modelID, promptID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	raw := json.RawMessage("null")
	if brief != nil {
		b, err := json.Marshal(brief)
		if err != nil {
			return fmt.Errorf("marshal ma deep brief: %w", err)
		}
		raw = b
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET brief = $2::jsonb, model_id = $3::uuid, prompt_id = $4::uuid, updated_at = now()
WHERE company_key = $1
`, companyKey, []byte(raw), nullString(modelID), nullString(promptID)); err != nil {
		return fmt.Errorf("update ma deep brief: %w", err)
	}
	return nil
}

type sectorMultiple struct {
	Prefix     string
	Industry   string
	EVEbitda   *float64
	EVSales    *float64
	NFirms     int
	Source     string
	SourceDate string
}

// ResolveSectorMultiple finds the Damodaran sector multiple for an ATECO code by
// longest-prefix match (4 -> 3 -> 2 digits), falling back to the TOTAL market row.
func (s *SQLStore) ResolveSectorMultiple(ctx context.Context, ateco string) (*sectorMultiple, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	code := strings.ReplaceAll(normalizeAtecoCode(ateco), ".", "")
	prefixes := make([]string, 0, 4)
	for _, n := range []int{4, 3, 2} {
		if len(code) >= n {
			prefixes = append(prefixes, code[:n])
		}
	}
	prefixes = append(prefixes, "TOTAL")
	for _, prefix := range prefixes {
		multiple, err := s.loadSectorMultiple(ctx, prefix)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}
		return multiple, nil
	}
	return nil, nil
}

func (s *SQLStore) loadSectorMultiple(ctx context.Context, prefix string) (*sectorMultiple, error) {
	var multiple sectorMultiple
	var evEbitda, evSales sql.NullFloat64
	var nFirms sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
SELECT ateco_prefix, damodaran_industry, ev_ebitda, ev_sales, n_firms, source, source_date
FROM binocolo.sector_valuation_multiple
WHERE ateco_prefix = $1
`, prefix).Scan(&multiple.Prefix, &multiple.Industry, &evEbitda, &evSales, &nFirms, &multiple.Source, &multiple.SourceDate)
	if err != nil {
		return nil, err
	}
	if evEbitda.Valid {
		v := evEbitda.Float64
		multiple.EVEbitda = &v
	}
	if evSales.Valid {
		v := evSales.Float64
		multiple.EVSales = &v
	}
	if nFirms.Valid {
		multiple.NFirms = int(nFirms.Int64)
	}
	return &multiple, nil
}

func nullInt(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func nullIntValue(value int) sql.NullInt64 {
	if value <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(value), Valid: true}
}
