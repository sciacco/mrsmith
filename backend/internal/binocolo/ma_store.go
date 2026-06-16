package binocolo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type maWorkspaceStore interface {
	ListMASessions(ctx context.Context) ([]MASessionSummary, error)
	CreateMASession(ctx context.Context, input maSessionCreate) (MASessionDetail, error)
	GetMASession(ctx context.Context, id string) (MASessionDetail, error)
	AddMAStrategyVersion(ctx context.Context, sessionID string, strategy MAStrategySpec, createdByEmail string) (*MAStrategyVersion, error)
	ReplaceMAEstimates(ctx context.Context, sessionID, strategyVersionID, selectedStrategy string, estimates []MAEstimate) error
	CreateMAExecutionRun(ctx context.Context, input maExecutionRunCreate) (MAExecutionRun, error)
	CompleteMAExecutionRun(ctx context.Context, runID, status string, resultCount int, errorCode string) error
	ReplaceMATargets(ctx context.Context, sessionID, runID string, targets []MATarget) error
	RecordMAModelAudit(ctx context.Context, input maModelAuditWrite) error
	StartMATrace(ctx context.Context, input maTraceStart) (string, error)
	LinkMATrace(ctx context.Context, input maTraceLink) error
	CompleteMATrace(ctx context.Context, input maTraceComplete) error
	RecordMATraceEvent(ctx context.Context, input maTraceEventWrite) error
	ListMALLMOptions(ctx context.Context) (MALLMOptionsResponse, error)
	ResolveMAModel(ctx context.Context, scope string, modelID string) (maLLMModel, error)
	ResolveMAPrompt(ctx context.Context, scope string, promptID string) (maLLMPrompt, error)
	RecordMAExport(ctx context.Context, sessionID, format string, rowCount int, createdByEmail string) error
}

func (s *SQLStore) ListMASessions(ctx context.Context) ([]MASessionSummary, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
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
  session.last_executed_at
FROM binocolo.ma_session session
ORDER BY session.updated_at DESC, session.created_at DESC
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
		); err != nil {
			return nil, fmt.Errorf("scan ma session summary: %w", err)
		}
		item.SelectedStrategy = selected.String
		if lastRun.Valid {
			item.LastRunAt = &lastRun.Time
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma sessions: %w", err)
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

func (s *SQLStore) RecordMAModelAudit(ctx context.Context, input maModelAuditWrite) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	prompt := json.RawMessage(`{}`)
	if len(input.Prompt) > 0 {
		prompt = input.Prompt
	}
	response := json.RawMessage(`{}`)
	if len(input.Response) > 0 {
		response = input.Response
	}
	usage := json.RawMessage(`{}`)
	if len(input.Usage) > 0 {
		usage = input.Usage
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_model_audit (
  id,
  session_id,
  strategy_version_id,
  scope,
  model_id,
  prompt_id,
  model,
  prompt,
  response,
  usage
) VALUES (
  $1::uuid, $2::uuid, NULLIF($3, '')::uuid, $4, NULLIF($5, '')::uuid, NULLIF($6, '')::uuid, $7, $8::jsonb, $9::jsonb, $10::jsonb
)
`, uuid.NewString(),
		input.SessionID,
		input.StrategyVersionID,
		input.Scope,
		input.ModelID,
		input.PromptID,
		input.Model,
		[]byte(prompt),
		[]byte(response),
		[]byte(usage),
	)
	if err != nil {
		return fmt.Errorf("record ma model audit: %w", err)
	}
	return nil
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

func (s *SQLStore) ListMALLMOptions(ctx context.Context) (MALLMOptionsResponse, error) {
	if s == nil || s.db == nil {
		return MALLMOptionsResponse{}, errors.New("binocolo ma store not configured")
	}
	models, err := s.listMALLMModels(ctx)
	if err != nil {
		return MALLMOptionsResponse{}, err
	}
	prompts, err := s.listMALLMPrompts(ctx)
	if err != nil {
		return MALLMOptionsResponse{}, err
	}
	return MALLMOptionsResponse{Models: models, Prompts: prompts}, nil
}

func (s *SQLStore) listMALLMModels(ctx context.Context) ([]MALLMModelOption, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, scope, name, model, is_default
FROM binocolo.llm_model
WHERE scope IN ($1, $2, $3)
ORDER BY scope, is_default DESC, name
`, "default", maModelScopeStrategy, maModelScopeSectorClassification)
	if err != nil {
		return nil, fmt.Errorf("list ma llm models: %w", err)
	}
	defer rows.Close()
	out := []MALLMModelOption{}
	for rows.Next() {
		var item MALLMModelOption
		if err := rows.Scan(&item.ID, &item.Scope, &item.Name, &item.Model, &item.IsDefault); err != nil {
			return nil, fmt.Errorf("scan ma llm model: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma llm models: %w", err)
	}
	return out, nil
}

func (s *SQLStore) listMALLMPrompts(ctx context.Context) ([]MALLMPromptOption, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, scope, name, is_default
FROM binocolo.llm_prompt
WHERE scope IN ($1, $2)
ORDER BY scope, is_default DESC, name
`, "default", maModelScopeStrategy)
	if err != nil {
		return nil, fmt.Errorf("list ma llm prompts: %w", err)
	}
	defer rows.Close()
	out := []MALLMPromptOption{}
	for rows.Next() {
		var item MALLMPromptOption
		if err := rows.Scan(&item.ID, &item.Scope, &item.Name, &item.IsDefault); err != nil {
			return nil, fmt.Errorf("scan ma llm prompt: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma llm prompts: %w", err)
	}
	return out, nil
}

func (s *SQLStore) ResolveMAModel(ctx context.Context, scope string, modelID string) (maLLMModel, error) {
	if s == nil || s.db == nil {
		return maLLMModel{}, errors.New("binocolo ma store not configured")
	}
	scope = normalizeLLMScope(scope, maModelScopeStrategy)
	modelID = strings.TrimSpace(modelID)
	if modelID != "" {
		return s.loadMALLMModel(ctx, `
SELECT id::text, scope, name, model, is_default
FROM binocolo.llm_model
WHERE id = $1::uuid
  AND scope IN ($2, 'default')
`, modelID, scope)
	}
	model, err := s.loadMALLMModel(ctx, `
SELECT id::text, scope, name, model, is_default
FROM binocolo.llm_model
WHERE scope = $1
  AND is_default
`, scope)
	if errors.Is(err, sql.ErrNoRows) && scope != "default" {
		model, err = s.loadMALLMModel(ctx, `
SELECT id::text, scope, name, model, is_default
FROM binocolo.llm_model
WHERE scope = 'default'
  AND is_default
`)
	}
	return model, err
}

func (s *SQLStore) ResolveMAPrompt(ctx context.Context, scope string, promptID string) (maLLMPrompt, error) {
	if s == nil || s.db == nil {
		return maLLMPrompt{}, errors.New("binocolo ma store not configured")
	}
	scope = normalizeLLMScope(scope, maModelScopeStrategy)
	promptID = strings.TrimSpace(promptID)
	if promptID != "" {
		return s.loadMALLMPrompt(ctx, `
SELECT id::text, scope, name, prompt, is_default
FROM binocolo.llm_prompt
WHERE id = $1::uuid
  AND scope IN ($2, 'default')
`, promptID, scope)
	}
	prompt, err := s.loadMALLMPrompt(ctx, `
SELECT id::text, scope, name, prompt, is_default
FROM binocolo.llm_prompt
WHERE scope = $1
  AND is_default
`, scope)
	if errors.Is(err, sql.ErrNoRows) && scope != "default" {
		prompt, err = s.loadMALLMPrompt(ctx, `
SELECT id::text, scope, name, prompt, is_default
FROM binocolo.llm_prompt
WHERE scope = 'default'
  AND is_default
`)
	}
	return prompt, err
}

func (s *SQLStore) loadMALLMModel(ctx context.Context, query string, args ...any) (maLLMModel, error) {
	var item maLLMModel
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&item.ID, &item.Scope, &item.Name, &item.Model, &item.IsDefault)
	if err != nil {
		return maLLMModel{}, err
	}
	item.Scope = strings.TrimSpace(item.Scope)
	item.Name = strings.TrimSpace(item.Name)
	item.Model = strings.TrimSpace(item.Model)
	if item.Model == "" {
		return maLLMModel{}, sql.ErrNoRows
	}
	return item, nil
}

func (s *SQLStore) loadMALLMPrompt(ctx context.Context, query string, args ...any) (maLLMPrompt, error) {
	var item maLLMPrompt
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&item.ID, &item.Scope, &item.Name, &item.Prompt, &item.IsDefault)
	if err != nil {
		return maLLMPrompt{}, err
	}
	item.Scope = strings.TrimSpace(item.Scope)
	item.Name = strings.TrimSpace(item.Name)
	item.Prompt = strings.TrimSpace(item.Prompt)
	if item.Prompt == "" {
		return maLLMPrompt{}, sql.ErrNoRows
	}
	return item, nil
}

func normalizeLLMScope(scope string, fallback string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return fallback
	}
	return scope
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
	err := q.QueryRowContext(ctx, `
SELECT id::text, title, prompt, status, selected_strategy, active_strategy_id::text,
       created_by_subject, created_by_email, last_estimated_at, last_executed_at,
       created_at, updated_at
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
	for index := range targets {
		targets[index].Evidence = evidence[targets[index].ID]
	}
	return targets, nil
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
