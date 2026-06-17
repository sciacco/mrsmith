package binocolo

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sciacco/mrsmith/internal/platform/openapiit"
	"github.com/sciacco/mrsmith/internal/platform/openrouter"
	"github.com/xuri/excelize/v2"
)

var (
	errMAStoreUnavailable      = errors.New("ma store unavailable")
	errMAOpenAPIITUnavailable  = errors.New("openapiit unavailable")
	errMAOpenRouterUnavailable = errors.New("openrouter unavailable")
	errMALLMConfigUnavailable  = errors.New("ma llm config unavailable")
	errMAEstimateTooLarge      = errors.New("estimate too large")
	errMAEstimateOverBudget    = errors.New("estimate over budget")
	errMAVisibilityInvalid     = errors.New("ma session visibility invalid")
	errMASessionArchived       = errors.New("ma session archived")
	errMASessionDeleted        = errors.New("ma session deleted")
)

const (
	maAtecoToolName          = "search_ateco_2025"
	maProvinceRegionToolName = "list_italian_provinces_regions"
	maCompanySurfaceToolName = "probe_company_search_surface"
	maMaxToolRounds          = 4
)

type maSurfaceProbeResult struct {
	Status         string
	EstimatedCount int
	EstimatedCost  float64
	ProbeCount     int
	Params         json.RawMessage
	VendorResponse json.RawMessage
}

type maSurfaceProbeAudit struct {
	Skip     int             `json:"skip"`
	Count    int             `json:"count"`
	Cost     float64         `json:"cost"`
	Params   json.RawMessage `json:"params"`
	Response json.RawMessage `json:"response"`
}

type maAIClient interface {
	Chat(context.Context, openrouter.ChatRequest) (openrouter.ChatResponse, error)
}

type maService struct {
	store         maWorkspaceStore
	searchCache   companySearchCacheStore
	provinceCache provinceCacheStore
	ateco         atecoStore
	openapiit     *openapiit.Client
	ai            maAIClient
	now           func() time.Time
}

func newMAService(store maWorkspaceStore, searchCache companySearchCacheStore, provinceCache provinceCacheStore, ateco atecoStore, openapiitClient *openapiit.Client, ai maAIClient) *maService {
	return &maService{
		store:         store,
		searchCache:   searchCache,
		provinceCache: provinceCache,
		ateco:         ateco,
		openapiit:     openapiitClient,
		ai:            ai,
		now:           func() time.Time { return time.Now().UTC() },
	}
}

func (s *maService) listSessions(ctx context.Context, visibility string) ([]MASessionSummary, error) {
	if s.store == nil {
		return nil, errMAStoreUnavailable
	}
	normalized, err := normalizeMASessionVisibility(visibility)
	if err != nil {
		return nil, err
	}
	return s.store.ListMASessions(ctx, normalized)
}

func (s *maService) getSession(ctx context.Context, id string) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	detail, err := s.store.GetMASession(ctx, id)
	if err != nil {
		return MASessionDetail{}, err
	}
	if detail.Session.DeletedAt != nil {
		return MASessionDetail{}, errMASessionDeleted
	}
	return decorateMACost(detail, s.loadPricing(ctx)), nil
}

func (s *maService) archiveSession(ctx context.Context, id, subject, email string) error {
	return s.updateSessionLifecycle(ctx, id, maSessionLifecycleArchive, subject, email)
}

func (s *maService) restoreSession(ctx context.Context, id, subject, email string) error {
	return s.updateSessionLifecycle(ctx, id, maSessionLifecycleRestore, subject, email)
}

func (s *maService) softDeleteSession(ctx context.Context, id, subject, email string) error {
	return s.updateSessionLifecycle(ctx, id, maSessionLifecycleDelete, subject, email)
}

func (s *maService) updateSessionLifecycle(ctx context.Context, id, action, subject, email string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	updated, err := s.store.UpdateMASessionLifecycle(ctx, id, action, subject, email)
	if err != nil {
		return err
	}
	if !updated {
		return sql.ErrNoRows
	}
	return nil
}

func normalizeMASessionVisibility(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", maSessionVisibilityActive:
		return maSessionVisibilityActive, nil
	case maSessionVisibilityArchived:
		return maSessionVisibilityArchived, nil
	case maSessionVisibilityDeleted:
		return maSessionVisibilityDeleted, nil
	default:
		return "", errMAVisibilityInvalid
	}
}

func ensureMASessionOperational(session MASession) error {
	if session.DeletedAt != nil {
		return errMASessionDeleted
	}
	if session.ArchivedAt != nil {
		return errMASessionArchived
	}
	return nil
}

// maPricing holds the business pricing/budget levers, sourced from the
// ma_parameter table (migration 038) with the compiled constants as fallback.
type maPricing struct {
	CostAdvanced               float64
	CostFull                   float64
	CostDryRun                 float64
	BudgetDefault              float64
	SMEHaircutPct              float64
	EBITDAFallbackPct          float64
	ThesisFitHoldingHaircutPct float64
}

// loadPricing reads the configurable pricing levers; missing/unreadable values
// fall back to the compiled defaults so the feature degrades gracefully.
func (s *maService) loadPricing(ctx context.Context) maPricing {
	pricing := maPricing{
		CostAdvanced:               maCostPerCompanyEUR,
		CostFull:                   maCostPerFullEUR,
		CostDryRun:                 maCostPerDryRunEUR,
		BudgetDefault:              maDefaultBudgetEUR,
		SMEHaircutPct:              maSMEHaircutPctDefault,
		EBITDAFallbackPct:          maEBITDAFallbackPctDefault,
		ThesisFitHoldingHaircutPct: maThesisFitHoldingHaircutPctDefault,
	}
	if s.store == nil {
		return pricing
	}
	params, err := s.store.ListMAParameters(ctx)
	if err != nil {
		return pricing
	}
	values := make(map[string]string, len(params))
	for _, param := range params {
		values[param.Key] = param.Value
	}
	if v, ok := paramFloat(values, "cost_advanced_eur"); ok {
		pricing.CostAdvanced = v
	}
	if v, ok := paramFloat(values, "cost_full_eur"); ok {
		pricing.CostFull = v
	}
	if v, ok := paramFloat(values, "cost_dryrun_eur"); ok {
		pricing.CostDryRun = v
	}
	if v, ok := paramFloat(values, "budget_default_eur"); ok {
		pricing.BudgetDefault = v
	}
	if v, ok := paramFloat(values, "sme_haircut_pct"); ok {
		pricing.SMEHaircutPct = v
	}
	if v, ok := paramFloat(values, "ebitda_fallback_threshold"); ok {
		pricing.EBITDAFallbackPct = v
	}
	if v, ok := paramFloat(values, "thesis_fit_holding_haircut_pct"); ok && v < 100 {
		pricing.ThesisFitHoldingHaircutPct = v
	}
	return pricing
}

func paramFloat(values map[string]string, key string) (float64, bool) {
	raw, ok := values[key]
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// decorateMACost attaches the active enrichment budget and unit price so the UI
// can show projected spend and the cost gate without duplicating the pricing.
func decorateMACost(detail MASessionDetail, pricing maPricing) MASessionDetail {
	detail.CostPerCompanyEUR = pricing.CostAdvanced
	detail.CostFullEUR = pricing.CostFull
	detail.BudgetEUR = pricing.BudgetDefault
	if detail.Strategy != nil {
		detail.BudgetEUR = maStrategyBudget(detail.Strategy.Strategy, pricing.BudgetDefault)
	}
	return detail
}

func (s *maService) listLLMOptions(ctx context.Context) (MALLMOptionsResponse, error) {
	if s.store == nil {
		return MALLMOptionsResponse{}, errMAStoreUnavailable
	}
	return s.store.ListMALLMOptions(ctx)
}

func (s *maService) createSession(ctx context.Context, req MACreateSessionRequest, subject, email string) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	prompt := cleanText(req.Prompt, 4000)
	if prompt == "" {
		return MASessionDetail{}, fmt.Errorf("%w: prompt", errMAStrategyInvalid)
	}
	strategy, audit, err := s.draftStrategy(ctx, prompt, req.ModelID, req.PromptID, subject, email)
	if err != nil {
		return MASessionDetail{}, err
	}
	title := strategy.Title
	if title == "" {
		title = titleFromPrompt(prompt)
	}
	detail, err := s.store.CreateMASession(ctx, maSessionCreate{
		Session: MASession{
			ID:               uuid.NewString(),
			Title:            title,
			Prompt:           prompt,
			CreatedBySubject: subject,
			CreatedByEmail:   email,
		},
		Strategy: strategy,
	})
	if err != nil {
		return MASessionDetail{}, err
	}
	if detail.Strategy != nil {
		if err := s.linkTrace(ctx, maTraceLink{SessionID: detail.Session.ID, StrategyVersionID: detail.Strategy.ID}); err != nil {
			return MASessionDetail{}, err
		}
	} else if err := s.linkTrace(ctx, maTraceLink{SessionID: detail.Session.ID}); err != nil {
		return MASessionDetail{}, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_session_created",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"session_id":          detail.Session.ID,
			"strategy_version_id": nullableTraceString(detail.Session.ActiveStrategyID),
			"title":               detail.Session.Title,
		}),
	}); err != nil {
		return MASessionDetail{}, err
	}
	audit.SessionID = detail.Session.ID
	if detail.Strategy != nil {
		audit.StrategyVersionID = detail.Strategy.ID
	}
	if err := s.store.RecordMAModelAudit(ctx, audit); err != nil {
		return MASessionDetail{}, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_model_audit_recorded",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"scope":               audit.Scope,
			"model_id":            audit.ModelID,
			"prompt_id":           audit.PromptID,
			"session_id":          audit.SessionID,
			"strategy_version_id": audit.StrategyVersionID,
		}),
	}); err != nil {
		return MASessionDetail{}, err
	}
	return decorateMACost(detail, s.loadPricing(ctx)), nil
}

func (s *maService) estimateSession(ctx context.Context, sessionID string, req MAEstimateSessionRequest, subject, email string) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	if s.openapiit == nil {
		return MASessionDetail{}, errMAOpenAPIITUnavailable
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_estimate_started",
		Status:    maTraceEventStarted,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "strategy_supplied": req.Strategy != nil}),
	}); err != nil {
		return MASessionDetail{}, err
	}
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return MASessionDetail{}, err
	}
	if err := ensureMASessionOperational(detail.Session); err != nil {
		return MASessionDetail{}, err
	}
	strategyVersion := detail.Strategy
	if req.Strategy != nil {
		strategy, err := validateMAStrategy(*req.Strategy)
		if err != nil {
			return MASessionDetail{}, err
		}
		strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategy, nil, false)
		if err != nil {
			return MASessionDetail{}, err
		}
		strategyVersion, err = s.store.AddMAStrategyVersion(ctx, sessionID, strategy, email)
		if err != nil {
			return MASessionDetail{}, err
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_strategy_version_created",
			Status:    maTraceEventSucceeded,
			Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "strategy_version_id": strategyVersion.ID, "version": strategyVersion.Version}),
		}); err != nil {
			return MASessionDetail{}, err
		}
	}
	if strategyVersion == nil {
		return MASessionDetail{}, fmt.Errorf("%w: strategy", errMAStrategyInvalid)
	}
	if err := s.linkTrace(ctx, maTraceLink{SessionID: sessionID, StrategyVersionID: strategyVersion.ID}); err != nil {
		return MASessionDetail{}, err
	}
	strategyVersion.Strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategyVersion.Strategy, nil, false)
	if err != nil {
		return MASessionDetail{}, err
	}
	strategyVersion.Strategy, err = s.expandStrategyAteco(ctx, strategyVersion.Strategy, subject, email)
	if err != nil {
		return MASessionDetail{}, err
	}
	strategyVersion.Strategy, err = s.expandStrategyExpansion(ctx, strategyVersion.Strategy, subject, email)
	if err != nil {
		return MASessionDetail{}, err
	}

	estimates, selected, err := s.runEstimates(ctx, sessionID, strategyVersion.ID, strategyVersion.Strategy, subject, email)
	if err != nil {
		return MASessionDetail{}, err
	}
	if err := s.store.ReplaceMAEstimates(ctx, sessionID, strategyVersion.ID, selected, estimates); err != nil {
		return MASessionDetail{}, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_estimates_saved",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"session_id":          sessionID,
			"strategy_version_id": strategyVersion.ID,
			"selected_strategy":   selected,
			"estimate_count":      len(estimates),
		}),
	}); err != nil {
		return MASessionDetail{}, err
	}
	return s.getSession(ctx, sessionID)
}

func (s *maService) executeSession(ctx context.Context, sessionID string, req MAExecuteSessionRequest, subject, email string) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	if s.openapiit == nil {
		return MASessionDetail{}, errMAOpenAPIITUnavailable
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_execute_started",
		Status:    maTraceEventStarted,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "strategy_supplied": req.Strategy != nil, "requested_strategy_type": req.StrategyType, "limit": req.Limit}),
	}); err != nil {
		return MASessionDetail{}, err
	}
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return MASessionDetail{}, err
	}
	if err := ensureMASessionOperational(detail.Session); err != nil {
		return MASessionDetail{}, err
	}
	strategyVersion := detail.Strategy
	if req.Strategy != nil {
		strategy, err := validateMAStrategy(*req.Strategy)
		if err != nil {
			return MASessionDetail{}, err
		}
		strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategy, nil, false)
		if err != nil {
			return MASessionDetail{}, err
		}
		strategyVersion, err = s.store.AddMAStrategyVersion(ctx, sessionID, strategy, email)
		if err != nil {
			return MASessionDetail{}, err
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_strategy_version_created",
			Status:    maTraceEventSucceeded,
			Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "strategy_version_id": strategyVersion.ID, "version": strategyVersion.Version}),
		}); err != nil {
			return MASessionDetail{}, err
		}
		detail, err = s.store.GetMASession(ctx, sessionID)
		if err != nil {
			return MASessionDetail{}, err
		}
	}
	if strategyVersion == nil {
		return MASessionDetail{}, fmt.Errorf("%w: strategy", errMAStrategyInvalid)
	}
	if err := s.linkTrace(ctx, maTraceLink{SessionID: sessionID, StrategyVersionID: strategyVersion.ID}); err != nil {
		return MASessionDetail{}, err
	}
	strategyVersion.Strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategyVersion.Strategy, nil, false)
	if err != nil {
		return MASessionDetail{}, err
	}
	strategyVersion.Strategy, err = s.expandStrategyAteco(ctx, strategyVersion.Strategy, subject, email)
	if err != nil {
		return MASessionDetail{}, err
	}
	strategyVersion.Strategy, err = s.expandStrategyExpansion(ctx, strategyVersion.Strategy, subject, email)
	if err != nil {
		return MASessionDetail{}, err
	}
	strategyType := normalizeMAStrategyType(req.StrategyType)
	if strategyType == "" {
		strategyType = detail.Session.SelectedStrategy
	}
	if strategyType == "" {
		strategyType = strategyVersion.Strategy.SelectedStrategy
	}
	if strategyType == "" {
		strategyType = chooseSelectedStrategyFromEstimates(detail.Estimates, len(strategyVersion.Strategy.AtecoCandidates) > 0)
	}
	estimatedCount := estimateTotal(detail.Estimates, strategyType)
	if len(detail.Estimates) == 0 || estimatedCount == 0 {
		return MASessionDetail{}, fmt.Errorf("%w: estimate required", errMAStrategyInvalid)
	}
	if estimatesTooBroad(detail.Estimates, strategyType) {
		return MASessionDetail{}, errMAEstimateTooLarge
	}
	limit := normalizeMASearchLimit(req.Limit)
	if limit != strategyVersion.Strategy.SearchLimit {
		return MASessionDetail{}, fmt.Errorf("%w: stale estimate", errMAStrategyInvalid)
	}
	if !estimatesMatchSearchLimit(detail.Estimates, strategyType, limit) {
		return MASessionDetail{}, fmt.Errorf("%w: stale estimate", errMAStrategyInvalid)
	}
	pricing := s.loadPricing(ctx)
	budget := maStrategyBudget(strategyVersion.Strategy, pricing.BudgetDefault)
	projectedCost := maProjectedSpend(estimatedCount, limit, pricing.CostAdvanced)
	if projectedCost > budget && !req.AcknowledgeCost {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_execution_over_budget",
			Status:    maTraceEventInfo,
			Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "projected_cost": projectedCost, "budget": budget, "estimated_count": estimatedCount, "limit": limit}),
		})
		return MASessionDetail{}, errMAEstimateOverBudget
	}

	run, err := s.store.CreateMAExecutionRun(ctx, maExecutionRunCreate{
		SessionID:         sessionID,
		StrategyVersionID: strategyVersion.ID,
		StrategyType:      strategyType,
		EstimatedCount:    estimatedCount,
	})
	if err != nil {
		return MASessionDetail{}, err
	}
	if err := s.linkTrace(ctx, maTraceLink{SessionID: sessionID, StrategyVersionID: strategyVersion.ID, ExecutionRunID: run.ID}); err != nil {
		return MASessionDetail{}, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_execution_run_created",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "strategy_version_id": strategyVersion.ID, "run_id": run.ID, "strategy_type": strategyType, "estimated_count": estimatedCount, "limit": limit}),
	}); err != nil {
		return MASessionDetail{}, err
	}

	targets, execErr := s.runExecution(ctx, strategyVersion.Strategy, strategyType, limit, subject, email)
	if execErr != nil {
		_ = s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusFailed, 0, maErrorCode(execErr))
		return MASessionDetail{}, execErr
	}
	for index := range targets {
		targets[index].SessionID = sessionID
		targets[index].RunID = run.ID
	}
	scoringParams := maScoringParams{ThesisFitHoldingFactor: 1 - pricing.ThesisFitHoldingHaircutPct/100}
	targets = scoreMATargetsV2(targets, strategyVersion.Strategy, scoringParams, s.now())
	missingFinancials := 0
	for _, target := range targets {
		for _, flag := range target.Flags {
			if flag.Code == "bilancio_assente" {
				missingFinancials++
				break
			}
		}
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_scoring_completed",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "run_id": run.ID, "result_count": len(targets), "missing_financials": missingFinancials, "thesis": normalizeMAThesis(strategyVersion.Strategy.Thesis)}),
	})
	if err := s.store.ReplaceMATargets(ctx, sessionID, run.ID, targets); err != nil {
		_ = s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusFailed, 0, "store_error")
		return MASessionDetail{}, err
	}
	if err := s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusCompleted, len(targets), ""); err != nil {
		return MASessionDetail{}, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_execution_completed",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "run_id": run.ID, "result_count": len(targets)}),
	}); err != nil {
		return MASessionDetail{}, err
	}
	return s.getSession(ctx, sessionID)
}

func (s *maService) exportSession(ctx context.Context, sessionID string, format string, email string) ([]byte, string, string, error) {
	if s.store == nil {
		return nil, "", "", errMAStoreUnavailable
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_export_started",
		Status:    maTraceEventStarted,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "format": format}),
	}); err != nil {
		return nil, "", "", err
	}
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return nil, "", "", err
	}
	if detail.Session.DeletedAt != nil {
		return nil, "", "", errMASessionDeleted
	}
	if err := s.linkTrace(ctx, maTraceLink{SessionID: sessionID, StrategyVersionID: detail.Session.ActiveStrategyID}); err != nil {
		return nil, "", "", err
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "xlsx"
	}
	if format != "xlsx" {
		return nil, "", "", fmt.Errorf("%w: export format", errMAStrategyInvalid)
	}
	rows := maExportRows(detail.Targets)
	content, err := buildMAXLSX(rows)
	if err != nil {
		return nil, "", "", err
	}
	if err := s.store.RecordMAExport(ctx, sessionID, "xlsx", len(detail.Targets), email); err != nil {
		return nil, "", "", err
	}
	filename := "target-ma-" + safeFilenamePart(detail.Session.Title) + ".xlsx"
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_export_recorded",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "format": "xlsx", "filename": filename, "row_count": len(detail.Targets)}),
	}); err != nil {
		return nil, "", "", err
	}
	return content, filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
}

// setTargetRating salva il voto preferiti dell'analista su un'azienda della
// sessione (memoria della preferenza + selezione per il deep-dive). Il voto è
// agganciato a company_key, quindi sopravvive al re-execute della sessione.
func (s *maService) setTargetRating(ctx context.Context, sessionID, companyKey string, rating int, subject, email string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	if companyKey == "" {
		return fmt.Errorf("%w: company key", errMAStrategyInvalid)
	}
	if !validMARating(rating) {
		return fmt.Errorf("%w: rating", errMAStrategyInvalid)
	}
	session, err := s.store.GetMASessionState(ctx, sessionID)
	if err != nil {
		return err
	}
	if err := ensureMASessionOperational(session); err != nil {
		return err
	}
	if err := s.store.UpsertMATargetRating(ctx, sessionID, companyKey, rating, subject, email); err != nil {
		return err
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_target_rated",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "company_key": companyKey, "rating": rating}),
	})
	return nil
}

func validMARating(rating int) bool {
	return rating == 0 || rating == maRatingExcluded || (rating >= 1 && rating <= maRatingMax)
}

func normalizeMACompanyKey(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func (s *maService) listParameters(ctx context.Context) ([]MAParameter, error) {
	if s.store == nil {
		return nil, errMAStoreUnavailable
	}
	return s.store.ListMAParameters(ctx)
}

// updateParameter validates and persists one configurable business parameter.
// Only seeded keys are editable (the UPDATE matches an existing row); the value
// must be a non-negative number. Each change is audited via a trace event.
func (s *maService) updateParameter(ctx context.Context, key, value, subject, email string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: parameter key", errMAStrategyInvalid)
	}
	value = strings.TrimSpace(value)
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return fmt.Errorf("%w: parameter value", errMAStrategyInvalid)
	}
	if err := s.store.UpdateMAParameter(ctx, key, value, email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: unknown parameter", errMAStrategyInvalid)
		}
		return err
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_parameter_updated",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"key": key, "value": value, "by": email, "subject": subject}),
	})
	return nil
}

// deepDive enqueues the rated (>=1 star) companies of a session for IT-full deep
// analysis. Companies already analysed (status=ready, cached globally) are skipped
// and not charged. The projected incremental spend (chargeable x cost_full) gates
// the batch unless the analyst acknowledges going over budget. The async worker
// picks up the queued rows; the returned detail reflects the new statuses.
func (s *maService) deepDive(ctx context.Context, sessionID string, ack bool, email string) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	if s.openapiit == nil {
		return MASessionDetail{}, errMAOpenAPIITUnavailable
	}
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return MASessionDetail{}, err
	}
	if err := ensureMASessionOperational(detail.Session); err != nil {
		return MASessionDetail{}, err
	}
	type candidate struct{ key, vat, tax string }
	candidates := make([]candidate, 0)
	keys := make([]string, 0)
	for _, target := range detail.Targets {
		if target.Rating != nil && *target.Rating >= 1 && target.CompanyKey != "" {
			candidates = append(candidates, candidate{key: target.CompanyKey, vat: target.VATCode, tax: target.TaxCode})
			keys = append(keys, target.CompanyKey)
		}
	}
	if len(candidates) == 0 {
		return MASessionDetail{}, fmt.Errorf("%w: nessun preferito da approfondire", errMAStrategyInvalid)
	}
	existing, err := s.store.ListMADeepAnalysis(ctx, keys)
	if err != nil {
		return MASessionDetail{}, err
	}
	pricing := s.loadPricing(ctx)
	budget := pricing.BudgetDefault
	if detail.Strategy != nil {
		budget = maStrategyBudget(detail.Strategy.Strategy, pricing.BudgetDefault)
	}
	chargeable := 0
	for _, c := range candidates {
		if record, ok := existing[c.key]; ok && record.Status != maDeepStatusFailed {
			// ready/queued/running are not (re)charged nor (re)enqueued.
			continue
		}
		chargeable++
	}
	projected := float64(chargeable) * pricing.CostFull
	if projected > budget && !ack {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_deep_dive_over_budget",
			Status:    maTraceEventInfo,
			Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "chargeable": chargeable, "projected_cost": projected, "budget": budget}),
		})
		return MASessionDetail{}, errMAEstimateOverBudget
	}
	enqueued := 0
	for _, c := range candidates {
		if record, ok := existing[c.key]; ok && record.Status != maDeepStatusFailed {
			// ready/queued/running are not (re)charged nor (re)enqueued.
			continue
		}
		if err := s.store.EnqueueMADeepAnalysis(ctx, c.key, c.vat, c.tax, email); err != nil {
			return MASessionDetail{}, err
		}
		enqueued++
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_deep_dive_started",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "candidates": len(candidates), "enqueued": enqueued, "projected_cost": projected}),
	})
	return s.getSession(ctx, sessionID)
}

func (s *maService) draftStrategy(ctx context.Context, prompt string, modelID string, promptID string, subject, email string) (MAStrategySpec, maModelAuditWrite, error) {
	if s.ai == nil {
		return MAStrategySpec{}, maModelAuditWrite{}, errMAOpenRouterUnavailable
	}
	if s.store == nil {
		return MAStrategySpec{}, maModelAuditWrite{}, errMAStoreUnavailable
	}
	if err := validateOptionalUUID(modelID); err != nil {
		return MAStrategySpec{}, maModelAuditWrite{}, err
	}
	if err := validateOptionalUUID(promptID); err != nil {
		return MAStrategySpec{}, maModelAuditWrite{}, err
	}
	modelConfig, err := s.store.ResolveMAModel(ctx, maModelScopeStrategy, modelID)
	if err != nil {
		return MAStrategySpec{}, maModelAuditWrite{}, llmConfigError(err)
	}
	promptConfig, err := s.store.ResolveMAPrompt(ctx, maModelScopeStrategy, promptID)
	if err != nil {
		return MAStrategySpec{}, maModelAuditWrite{}, llmConfigError(err)
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_strategy_draft_config",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"model_id":     modelConfig.ID,
			"model_scope":  modelConfig.Scope,
			"model":        modelConfig.Model,
			"prompt_id":    promptConfig.ID,
			"prompt_scope": promptConfig.Scope,
			"prompt_name":  promptConfig.Name,
			"prompt":       promptConfig.Prompt,
		}),
	}); err != nil {
		return MAStrategySpec{}, maModelAuditWrite{}, err
	}
	messages := []openrouter.Message{
		{Role: "system", Content: promptConfig.Prompt},
		{Role: "developer", Content: maStrategyToolInstructions()},
		{Role: "user", Content: prompt},
	}
	allowedAteco := map[string]AtecoCode{}
	allowedProvinces := map[string]openapiit.Province{}
	tools := []openrouter.Tool{maAtecoSearchTool(), maProvinceRegionTool(), maCompanySurfaceProbeTool()}
	var response openrouter.ChatResponse
	var responseRaw json.RawMessage
	for round := 0; round <= maMaxToolRounds; round++ {
		req := openrouter.ChatRequest{
			Model:          modelConfig.Model,
			Temperature:    0,
			MaxTokens:      1800,
			ResponseFormat: &openrouter.ResponseFormat{Type: "json_object"},
			Messages:       messages,
			Tools:          tools,
			ToolChoice:     "auto",
		}
		start := time.Now()
		response, err = s.ai.Chat(ctx, req)
		duration := maTraceDuration(start)
		if err != nil {
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType:      "openrouter_chat",
				Round:          maTraceRound(round),
				ExternalSystem: "openrouter",
				Status:         maTraceEventFailed,
				DurationMS:     duration,
				Request:        maTraceJSON(req),
				Error:          err.Error(),
			})
			return MAStrategySpec{}, maModelAuditWrite{}, err
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType:      "openrouter_chat",
			Round:          maTraceRound(round),
			ExternalSystem: "openrouter",
			Status:         maTraceEventSucceeded,
			DurationMS:     duration,
			Request:        maTraceJSON(req),
			Response:       maTraceJSON(response),
			Metadata:       maTraceJSON(map[string]any{"tool_call_count": len(response.ToolCalls), "response_id": response.ID, "response_model": response.Model}),
		}); err != nil {
			return MAStrategySpec{}, maModelAuditWrite{}, err
		}
		if len(response.ToolCalls) == 0 {
			responseRaw = json.RawMessage([]byte(strings.TrimSpace(response.Content)))
			break
		}
		if round == maMaxToolRounds {
			err := fmt.Errorf("%w: ateco tool loop", errMAStrategyInvalid)
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType: "ma_strategy_tool_loop_limit",
				Round:     maTraceRound(round),
				Status:    maTraceEventFailed,
				Metadata:  maTraceJSON(map[string]any{"max_tool_rounds": maMaxToolRounds, "tool_call_count": len(response.ToolCalls)}),
				Error:     err.Error(),
			})
			return MAStrategySpec{}, maModelAuditWrite{}, err
		}
		messages = append(messages, openrouter.Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})
		for _, call := range response.ToolCalls {
			start := time.Now()
			content := s.executeMAStrategyTool(ctx, call, allowedAteco, allowedProvinces, subject, email)
			if err := s.traceEvent(ctx, maTraceEventWrite{
				EventType:  "ma_strategy_tool",
				Round:      maTraceRound(round),
				ToolName:   call.Function.Name,
				Status:     maToolResultStatus(content),
				DurationMS: maTraceDuration(start),
				Request:    maTraceJSON(call),
				Response:   maTraceRawJSON([]byte(content)),
				Metadata: maTraceJSON(map[string]any{
					"allowed_ateco_count":    len(allowedAteco),
					"allowed_province_count": len(allowedProvinces),
				}),
			}); err != nil {
				return MAStrategySpec{}, maModelAuditWrite{}, err
			}
			messages = append(messages, openrouter.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    content,
			})
		}
	}
	strategy, err := decodeMAStrategyResponse(responseRaw)
	if err != nil {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_strategy_decode",
			Status:    maTraceEventFailed,
			Response:  maTraceJSON(responseRaw),
			Error:     err.Error(),
		})
		return MAStrategySpec{}, maModelAuditWrite{}, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_strategy_decode",
		Status:    maTraceEventSucceeded,
		Response:  maTraceJSON(strategy),
	}); err != nil {
		return MAStrategySpec{}, maModelAuditWrite{}, err
	}
	strategy, err = s.canonicalizeMAStrategyProvinces(strategy, allowedProvinces, true)
	if err != nil {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_strategy_province_canonicalization",
			Status:    maTraceEventFailed,
			Metadata:  maTraceJSON(map[string]any{"allowed_province_count": len(allowedProvinces)}),
			Error:     err.Error(),
		})
		return MAStrategySpec{}, maModelAuditWrite{}, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_strategy_province_canonicalization",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"allowed_province_count": len(allowedProvinces), "provinces": strategy.Provinces}),
	}); err != nil {
		return MAStrategySpec{}, maModelAuditWrite{}, err
	}
	strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategy, allowedAteco, true)
	if err != nil {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_strategy_ateco_canonicalization",
			Status:    maTraceEventFailed,
			Metadata:  maTraceJSON(map[string]any{"allowed_ateco_count": len(allowedAteco)}),
			Error:     err.Error(),
		})
		return MAStrategySpec{}, maModelAuditWrite{}, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_strategy_ateco_canonicalization",
		Status:    maTraceEventSucceeded,
		Response:  maTraceJSON(strategy.AtecoCandidates),
		Metadata:  maTraceJSON(map[string]any{"allowed_ateco_count": len(allowedAteco), "candidate_count": len(strategy.AtecoCandidates)}),
	}); err != nil {
		return MAStrategySpec{}, maModelAuditWrite{}, err
	}
	usageRaw, _ := json.Marshal(response.Usage)
	promptRaw, _ := json.Marshal(map[string]any{
		"model_id":  modelConfig.ID,
		"prompt_id": promptConfig.ID,
		"messages":  messages,
		"tools":     tools,
	})
	return strategy, maModelAuditWrite{
		Scope:    maModelScopeStrategy,
		ModelID:  modelConfig.ID,
		PromptID: promptConfig.ID,
		Model:    modelConfig.Model,
		Prompt:   promptRaw,
		Response: responseRaw,
		Usage:    usageRaw,
	}, nil
}

func decodeMAStrategyResponse(responseRaw json.RawMessage) (MAStrategySpec, error) {
	if len(strings.TrimSpace(string(responseRaw))) == 0 {
		return MAStrategySpec{}, fmt.Errorf("%w: empty strategy response", errMAStrategyInvalid)
	}
	var envelope maStrategyDraftEnvelope
	if err := json.Unmarshal(responseRaw, &envelope); err != nil || strings.TrimSpace(envelope.Strategy.SectorDescription) == "" {
		var direct MAStrategySpec
		if directErr := json.Unmarshal(responseRaw, &direct); directErr != nil {
			if err != nil {
				return MAStrategySpec{}, fmt.Errorf("decode ma strategy: %w", err)
			}
			return MAStrategySpec{}, fmt.Errorf("decode ma strategy: %w", directErr)
		}
		envelope.Strategy = direct
	}
	strategy, err := validateMAStrategy(envelope.Strategy)
	if err != nil {
		return MAStrategySpec{}, err
	}
	return strategy, nil
}

func maStrategyToolInstructions() string {
	return strings.Join([]string{
		"Per compilare atecoCandidates devi usare il tool search_ateco_2025.",
		"Non inventare codici ATECO e non usare codici che non compaiono nei risultati del tool.",
		"Per compilare provinces devi usare il tool list_italian_provinces_regions.",
		"Usa solo sigle provincia restituite dal tool province/regioni; se l'utente indica una regione, espandila nelle province restituite dal tool.",
		"Non inventare appartenenze provincia-regione, macro-aree o sigle non presenti nel tool.",
		"Usa il tool probe_company_search_surface per verificare se una combinazione di provincia, ATECO, fatturato e dipendenti e' praticabile prima di proporla.",
		"Il probe e' solo dry-run e misura la superficie potenziale: non usare searchLimit per restringere questa valutazione.",
		"Se probe_company_search_surface restituisce surfaceStatus=too_broad, restringi territorio, settore o range economici prima della risposta finale.",
		"Se la prima ricerca e' troppo generica, fai piu' chiamate tool mirate con termini italiani come programmazione informatica, consulenza informatica, hosting, elaborazione dati.",
		"Deduci la tesi d'acquisizione (thesis) dalla richiesta: successione, crescita, consolidamento, tuck_in oppure generico se non emerge.",
		"Popola legalForms solo se l'utente richiede esplicitamente una forma societaria; e' un filtro, non un criterio di valutazione.",
		"Non generare scoringCriteria. Per enfatizzare un segnale usa signalWeights solo con id del catalogo (ateco_precision, turnover_proximity, keyword_match, ownership_concentration, company_age, legal_form, turnover_trend, productivity, equity_solidity); non inventare id.",
		"La risposta finale deve restare JSON valido nel formato richiesto dal prompt di sistema.",
	}, "\n")
}

func maAtecoSearchTool() openrouter.Tool {
	return openrouter.Tool{
		Type: "function",
		Function: openrouter.ToolFunction{
			Name:        maAtecoToolName,
			Description: "Cerca codici ATECO 2025 reali nella tabella Binocolo. Restituisce solo codici validi per la selezione ATECO.",
			Parameters: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"query"},
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Termini di ricerca in italiano, per esempio 'programmazione consulenza informatica hosting'.",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Numero massimo di risultati. Default 50, hard cap backend 100.",
						"minimum":     1,
						"maximum":     atecoSearchHardLimit,
					},
				},
			},
		},
	}
}

func maProvinceRegionTool() openrouter.Tool {
	return openrouter.Tool{
		Type: "function",
		Function: openrouter.ToolFunction{
			Name:        maProvinceRegionToolName,
			Description: "Restituisce province italiane reali e rispettive regioni dai dati OpenAPI.it cacheati in Binocolo.",
			Parameters: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"region": map[string]any{
						"type":        "string",
						"description": "Nome regione da filtrare, per esempio Lombardia. Ometti per tutte le regioni.",
					},
					"query": map[string]any{
						"type":        "string",
						"description": "Filtro opzionale su sigla, nome provincia o regione.",
					},
				},
			},
		},
	}
}

func maCompanySurfaceProbeTool() openrouter.Tool {
	return openrouter.Tool{
		Type: "function",
		Function: openrouter.ToolFunction{
			Name:        maCompanySurfaceToolName,
			Description: "Esegue un dry-run Company IT-search senza limit operativo e misura se la superficie e' esatta o troppo ampia.",
			Parameters: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"province": map[string]any{
						"type":        "string",
						"description": "Sigla provincia italiana, per esempio MI. Ometti per ricerca nazionale.",
					},
					"atecoCode": map[string]any{
						"type":        "string",
						"description": "Codice ATECO restituito prima da search_ateco_2025. Ometti per ricerca espansa.",
					},
					"activityStatus": map[string]any{
						"type":        "string",
						"description": "Stato attivita'. Default ATTIVA.",
					},
					"minTurnover": map[string]any{
						"type":        "integer",
						"description": "Fatturato minimo.",
						"minimum":     0,
					},
					"maxTurnover": map[string]any{
						"type":        "integer",
						"description": "Fatturato massimo.",
						"minimum":     0,
					},
					"minEmployees": map[string]any{
						"type":        "integer",
						"description": "Dipendenti minimi.",
						"minimum":     0,
					},
					"maxEmployees": map[string]any{
						"type":        "integer",
						"description": "Dipendenti massimi.",
						"minimum":     0,
					},
				},
			},
		},
	}
}

type maAtecoToolArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

type maProvinceRegionToolArgs struct {
	Region string `json:"region,omitempty"`
	Query  string `json:"query,omitempty"`
}

type maProvinceToolItem struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Region string `json:"region"`
}

type maRegionToolItem struct {
	Name      string   `json:"name"`
	Provinces []string `json:"provinces"`
}

type maCompanySurfaceToolArgs struct {
	Province       string `json:"province,omitempty"`
	AtecoCode      string `json:"atecoCode,omitempty"`
	ActivityStatus string `json:"activityStatus,omitempty"`
	MinTurnover    *int   `json:"minTurnover,omitempty"`
	MaxTurnover    *int   `json:"maxTurnover,omitempty"`
	MinEmployees   *int   `json:"minEmployees,omitempty"`
	MaxEmployees   *int   `json:"maxEmployees,omitempty"`
}

func (s *maService) executeMAStrategyTool(ctx context.Context, call openrouter.ToolCall, allowedAteco map[string]AtecoCode, allowedProvinces map[string]openapiit.Province, subject, email string) string {
	switch call.Function.Name {
	case maAtecoToolName:
		return s.executeMAAtecoTool(ctx, call, allowedAteco)
	case maProvinceRegionToolName:
		return s.executeMAProvinceRegionTool(ctx, call, allowedProvinces)
	case maCompanySurfaceToolName:
		return s.executeMACompanySurfaceTool(ctx, call, allowedAteco, allowedProvinces, subject, email)
	default:
		return maToolErrorJSON("unsupported_tool")
	}
}

func (s *maService) executeMAAtecoTool(ctx context.Context, call openrouter.ToolCall, allowed map[string]AtecoCode) string {
	if s.ateco == nil {
		return maToolErrorJSON("ateco_not_configured")
	}
	var args maAtecoToolArgs
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return maToolErrorJSON("invalid_arguments")
	}
	items, err := s.ateco.SearchAteco(ctx, args.Query, normalizeAtecoSearchLimit(args.Limit))
	if err != nil {
		return maToolErrorJSON("search_failed")
	}
	for _, item := range items {
		rememberAllowedAteco(allowed, item)
	}
	raw, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		return maToolErrorJSON("encode_failed")
	}
	return string(raw)
}

func (s *maService) executeMAProvinceRegionTool(ctx context.Context, call openrouter.ToolCall, allowed map[string]openapiit.Province) string {
	var args maProvinceRegionToolArgs
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return maToolErrorJSON("invalid_arguments")
	}
	envelope, _, err := listProvincesWithCache(ctx, s.provinceCache, s.openapiit, s.now)
	if err != nil {
		return maToolErrorJSON("provinces_unavailable")
	}

	regionFilter := cleanText(args.Region, 80)
	queryFilter := cleanText(args.Query, 80)
	provinces := make([]maProvinceToolItem, 0, len(envelope.Data))
	for _, item := range envelope.Data {
		item.Sigla = strings.ToUpper(strings.TrimSpace(item.Sigla))
		item.Provincia = strings.TrimSpace(item.Provincia)
		item.Regione = strings.TrimSpace(item.Regione)
		if item.Sigla == "" || item.Provincia == "" || item.Regione == "" {
			continue
		}
		if !matchesProvinceToolFilter(item, regionFilter, queryFilter) {
			continue
		}
		rememberAllowedProvince(allowed, item)
		provinces = append(provinces, maProvinceToolItem{
			Code:   item.Sigla,
			Name:   item.Provincia,
			Region: item.Regione,
		})
	}
	sort.SliceStable(provinces, func(i, j int) bool {
		if provinces[i].Region == provinces[j].Region {
			return provinces[i].Code < provinces[j].Code
		}
		return provinces[i].Region < provinces[j].Region
	})

	regionsByName := map[string][]string{}
	for _, item := range provinces {
		regionsByName[item.Region] = append(regionsByName[item.Region], item.Code)
	}
	regionNames := make([]string, 0, len(regionsByName))
	for name := range regionsByName {
		regionNames = append(regionNames, name)
	}
	sort.Strings(regionNames)
	regions := make([]maRegionToolItem, 0, len(regionNames))
	for _, name := range regionNames {
		codes := regionsByName[name]
		sort.Strings(codes)
		regions = append(regions, maRegionToolItem{Name: name, Provinces: codes})
	}

	raw, err := json.Marshal(map[string]any{
		"regions":   regions,
		"provinces": provinces,
	})
	if err != nil {
		return maToolErrorJSON("encode_failed")
	}
	return string(raw)
}

func (s *maService) executeMACompanySurfaceTool(ctx context.Context, call openrouter.ToolCall, allowed map[string]AtecoCode, allowedProvinces map[string]openapiit.Province, subject, email string) string {
	var args maCompanySurfaceToolArgs
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return maToolErrorJSON("invalid_arguments")
	}
	province, ok := normalizeProvince(args.Province)
	if !ok {
		return maToolErrorJSON("invalid_province")
	}
	if allowedProvinces != nil && province != "" {
		if _, ok := allowedProvinces[province]; !ok {
			return maToolErrorJSON("province_not_allowed")
		}
	}
	activityStatus, ok := normalizeCompanyActivityStatus(args.ActivityStatus)
	if !ok {
		return maToolErrorJSON("invalid_activity_status")
	}
	if activityStatus == "" {
		activityStatus = "ATTIVA"
	}
	if err := validateOptionalRange(args.MinTurnover, args.MaxTurnover, "turnover"); err != nil {
		return maToolErrorJSON("invalid_turnover_range")
	}
	if err := validateOptionalRange(args.MinEmployees, args.MaxEmployees, "employees"); err != nil {
		return maToolErrorJSON("invalid_employees_range")
	}

	dryRun := 1
	params := openapiit.CompanyITSearchParams{
		DryRun:         &dryRun,
		DataEnrichment: "advanced",
		Province:       province,
		ActivityStatus: activityStatus,
		MinTurnover:    args.MinTurnover,
		MaxTurnover:    args.MaxTurnover,
		MinEmployees:   args.MinEmployees,
		MaxEmployees:   args.MaxEmployees,
	}
	if strings.TrimSpace(args.AtecoCode) != "" {
		item, ok := allowed[atecoSearchCode(args.AtecoCode)]
		if !ok || item.CodiceSearch == "" {
			return maToolErrorJSON("ateco_not_allowed")
		}
		params.AtecoCode = item.CodiceSearch
	}
	result, err := s.probeMASearchSurface(ctx, params, subject, email)
	if err != nil {
		return maToolErrorJSON("surface_probe_failed")
	}
	raw, err := json.Marshal(map[string]any{
		"surfaceStatus":  result.Status,
		"estimatedCount": result.EstimatedCount,
		"lowerBound":     false,
		"tooBroad":       result.Status == maEstimateSurfaceTooBroad,
		"estimatedCost":  result.EstimatedCost,
		"probeCount":     result.ProbeCount,
		"params":         result.Params,
	})
	if err != nil {
		return maToolErrorJSON("encode_failed")
	}
	return string(raw)
}

func matchesProvinceToolFilter(item openapiit.Province, regionFilter string, queryFilter string) bool {
	if regionFilter != "" && !textContainsFold(item.Regione, regionFilter) {
		return false
	}
	if queryFilter == "" {
		return true
	}
	return textContainsFold(item.Sigla, queryFilter) ||
		textContainsFold(item.Provincia, queryFilter) ||
		textContainsFold(item.Regione, queryFilter)
}

func textContainsFold(value, query string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	query = strings.ToLower(strings.TrimSpace(query))
	return query == "" || strings.Contains(value, query)
}

func maToolErrorJSON(code string) string {
	raw, _ := json.Marshal(map[string]string{"error": code})
	return string(raw)
}

func rememberAllowedAteco(allowed map[string]AtecoCode, item AtecoCode) {
	if allowed == nil {
		return
	}
	if item.Codice != "" {
		allowed[atecoSearchCode(item.Codice)] = item
	}
	if item.CodiceSearch != "" {
		allowed[atecoSearchCode(item.CodiceSearch)] = item
	}
}

func rememberAllowedProvince(allowed map[string]openapiit.Province, item openapiit.Province) {
	if allowed == nil {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(item.Sigla))
	if code == "" {
		return
	}
	item.Sigla = code
	item.Provincia = strings.TrimSpace(item.Provincia)
	item.Regione = strings.TrimSpace(item.Regione)
	allowed[code] = item
}

func (s *maService) canonicalizeMAStrategyProvinces(strategy MAStrategySpec, allowed map[string]openapiit.Province, requireAllowed bool) (MAStrategySpec, error) {
	if len(strategy.Provinces) == 0 {
		return strategy, nil
	}
	provinces := make([]string, 0, len(strategy.Provinces))
	seen := map[string]struct{}{}
	for _, raw := range strategy.Provinces {
		province, ok := normalizeProvince(raw)
		if !ok {
			return MAStrategySpec{}, fmt.Errorf("%w: province", errMAStrategyInvalid)
		}
		if province == "" {
			continue
		}
		if requireAllowed {
			if _, ok := allowed[province]; !ok {
				return MAStrategySpec{}, fmt.Errorf("%w: province not returned by tool", errMAStrategyInvalid)
			}
		}
		if _, exists := seen[province]; exists {
			continue
		}
		seen[province] = struct{}{}
		provinces = append(provinces, province)
	}
	sort.Strings(provinces)
	strategy.Provinces = provinces
	if strategy.TerritoryLabel == "" && len(provinces) > 0 {
		strategy.TerritoryLabel = strings.Join(provinces, ", ")
	}
	return strategy, nil
}

func (s *maService) canonicalizeMAStrategyAteco(ctx context.Context, strategy MAStrategySpec, allowed map[string]AtecoCode, requireAllowed bool) (MAStrategySpec, error) {
	if len(strategy.AtecoCandidates) == 0 {
		return strategy, nil
	}
	candidates := make([]MAAtecoCandidate, 0, len(strategy.AtecoCandidates))
	seen := map[string]struct{}{}
	for _, candidate := range strategy.AtecoCandidates {
		code := normalizeAtecoCode(candidate.Code)
		if code == "" {
			continue
		}
		var item AtecoCode
		var ok bool
		if requireAllowed {
			item, ok = allowed[atecoSearchCode(code)]
			if !ok {
				return MAStrategySpec{}, fmt.Errorf("%w: ateco candidate not returned by tool", errMAStrategyInvalid)
			}
		} else {
			if s.ateco == nil {
				return MAStrategySpec{}, errAtecoStoreUnavailable
			}
			resolved, err := s.ateco.ResolveAtecoCode(ctx, code)
			if err != nil {
				return MAStrategySpec{}, err
			}
			item = resolved
		}
		if item.CodiceSearch == "" {
			return MAStrategySpec{}, errAtecoCodeNotFound
		}
		if _, exists := seen[item.CodiceSearch]; exists {
			continue
		}
		seen[item.CodiceSearch] = struct{}{}
		candidates = append(candidates, MAAtecoCandidate{
			Code:        item.Codice,
			Description: item.Titolo,
			Rationale:   cleanText(candidate.Rationale, 240),
			SearchCode:  item.CodiceSearch,
		})
	}
	strategy.AtecoCandidates = candidates
	// Capture the sector perimeter (2-digit divisions) from the canonical candidates
	// NOW, before expandStrategyAteco prunes empty leaves — otherwise a division the
	// analyst intended (e.g. 63 for hosting) would vanish from the gate if none of
	// its leaf codes happened to be populated.
	strategy.SectorDivisions = atecoDivisions(candidates)
	return strategy, nil
}

// expandStrategyAteco replaces each analyst/LLM-selected ATECO sector node with
// the exact codes in its subtree that are actually populated at OpenAPI.it.
// OpenAPI.it matches the ATECO code exactly (no prefix, no descent) and companies
// are tagged at heterogeneous levels per branch, so a selected node (e.g. 62.20)
// is frequently empty while a descendant (62.20.1) holds the companies.
// Discovery is a dry-run probe (count only, €0.01) per subtree code, applying the
// economic filters but neither province nor legal form for maximum cache reuse.
// It is deterministic and cache-backed, so estimate and execution derive the same
// populated set without persisting it onto the stored strategy.
func (s *maService) expandStrategyAteco(ctx context.Context, strategy MAStrategySpec, subject, email string) (MAStrategySpec, error) {
	if s.ateco == nil || len(strategy.AtecoCandidates) == 0 {
		return strategy, nil
	}
	type subtreeCode struct {
		code, searchCode, titolo string
	}
	seen := map[string]struct{}{}
	ordered := make([]subtreeCode, 0, len(strategy.AtecoCandidates)*4)
	for _, candidate := range strategy.AtecoCandidates {
		descendants, err := s.ateco.SubtreeAtecoCodes(ctx, candidate.Code)
		if err != nil {
			return MAStrategySpec{}, err
		}
		for _, descendant := range descendants {
			if descendant.CodiceSearch == "" {
				continue
			}
			if _, exists := seen[descendant.CodiceSearch]; exists {
				continue
			}
			seen[descendant.CodiceSearch] = struct{}{}
			ordered = append(ordered, subtreeCode{code: descendant.Codice, searchCode: descendant.CodiceSearch, titolo: descendant.Titolo})
		}
	}
	if len(ordered) == 0 {
		return strategy, nil
	}
	if len(ordered) > maAtecoSubtreeProbeCap {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_ateco_discovery_skipped",
			Status:    maTraceEventInfo,
			Metadata:  maTraceJSON(map[string]any{"subtree_codes": len(ordered), "cap": maAtecoSubtreeProbeCap}),
		})
		return strategy, nil
	}
	populated := make([]MAAtecoCandidate, 0, len(ordered))
	for _, item := range ordered {
		params := openapiit.CompanyITSearchParams{
			DataEnrichment: "advanced",
			ActivityStatus: strategy.ActivityStatus,
			MinTurnover:    strategy.TurnoverMin,
			MaxTurnover:    strategy.TurnoverMax,
			MinEmployees:   strategy.EmployeeMin,
			MaxEmployees:   strategy.EmployeeMax,
			AtecoCode:      item.searchCode,
		}
		surface, err := s.probeMASearchSurface(ctx, params, subject, email)
		if err != nil {
			return MAStrategySpec{}, err
		}
		if surface.EstimatedCount <= 0 {
			continue
		}
		populated = append(populated, MAAtecoCandidate{
			Code:        item.code,
			Description: item.titolo,
			SearchCode:  item.searchCode,
		})
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_ateco_discovery",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"selected_nodes":  len(strategy.AtecoCandidates),
			"subtree_codes":   len(ordered),
			"populated_codes": len(populated),
		}),
	})
	if len(populated) == 0 {
		// None of the subtree codes returned companies under the economic
		// filters; keep the original selection so the estimate still runs (and
		// reports zero) rather than silently emptying the strategy.
		return strategy, nil
	}
	strategy.AtecoCandidates = populated
	return strategy, nil
}

// excludedSearchCodes returns the dot-stripped search codes of the excluded ATECO
// entries. A descendant/target whose search code has any of these as a prefix is
// inside an excluded subtree (e.g. excluding 63.10.21 elaborazione dati contabili
// removes that branch while hosting 63.10.10 stays).
func excludedSearchCodes(excluded []string) []string {
	out := make([]string, 0, len(excluded))
	for _, ex := range excluded {
		if code := atecoSearchCode(ex); code != "" {
			out = append(out, code)
		}
	}
	return out
}

func isExcludedSearchCode(searchCode string, excluded []string) bool {
	for _, ex := range excluded {
		if strings.HasPrefix(searchCode, ex) {
			return true
		}
	}
	return false
}

// expandStrategyExpansion computes the "expanded" search's ATECO code set: the
// populated subtree of the sector divisions, minus any excluded subtree. It
// replaces the old expanded behaviour — drop the ATECO filter entirely, which
// retrieved the whole provincial economy — with a sector-bounded net that still
// catches mis-tagged siblings inside the IT divisions (a real MSP tagged 62.09
// rather than 62.20). Like expandStrategyAteco it probes dry-run (count only,
// €0.01, cache-backed) applying the economic filters but neither province nor
// legal form, and respects the subtree probe cap. The result lives only on the
// transient ExpandedAtecoCandidates field; an empty result leaves the legacy
// province-only fallback in place (sector-less strategies).
func (s *maService) expandStrategyExpansion(ctx context.Context, strategy MAStrategySpec, subject, email string) (MAStrategySpec, error) {
	if s.ateco == nil || len(strategy.SectorDivisions) == 0 {
		return strategy, nil
	}
	excluded := excludedSearchCodes(strategy.ExcludedAteco)
	type subtreeCode struct{ code, searchCode, titolo string }
	seen := map[string]struct{}{}
	ordered := make([]subtreeCode, 0, 32)
	for _, division := range strategy.SectorDivisions {
		descendants, err := s.ateco.SubtreeAtecoCodes(ctx, division)
		if err != nil {
			return MAStrategySpec{}, err
		}
		for _, descendant := range descendants {
			if descendant.CodiceSearch == "" {
				continue
			}
			if _, exists := seen[descendant.CodiceSearch]; exists {
				continue
			}
			if isExcludedSearchCode(descendant.CodiceSearch, excluded) {
				continue
			}
			seen[descendant.CodiceSearch] = struct{}{}
			ordered = append(ordered, subtreeCode{code: descendant.Codice, searchCode: descendant.CodiceSearch, titolo: descendant.Titolo})
		}
	}
	if len(ordered) == 0 {
		return strategy, nil
	}
	if len(ordered) > maAtecoSubtreeProbeCap {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_expansion_discovery_skipped",
			Status:    maTraceEventInfo,
			Metadata:  maTraceJSON(map[string]any{"divisions": strategy.SectorDivisions, "subtree_codes": len(ordered), "cap": maAtecoSubtreeProbeCap}),
		})
		return strategy, nil
	}
	populated := make([]MAAtecoCandidate, 0, len(ordered))
	for _, item := range ordered {
		params := openapiit.CompanyITSearchParams{
			DataEnrichment: "advanced",
			ActivityStatus: strategy.ActivityStatus,
			MinTurnover:    strategy.TurnoverMin,
			MaxTurnover:    strategy.TurnoverMax,
			MinEmployees:   strategy.EmployeeMin,
			MaxEmployees:   strategy.EmployeeMax,
			AtecoCode:      item.searchCode,
		}
		surface, err := s.probeMASearchSurface(ctx, params, subject, email)
		if err != nil {
			return MAStrategySpec{}, err
		}
		if surface.EstimatedCount <= 0 {
			continue
		}
		populated = append(populated, MAAtecoCandidate{
			Code:        item.code,
			Description: item.titolo,
			SearchCode:  item.searchCode,
		})
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_expansion_discovery",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"divisions":       strategy.SectorDivisions,
			"excluded":        strategy.ExcludedAteco,
			"subtree_codes":   len(ordered),
			"populated_codes": len(populated),
		}),
	})
	strategy.ExpandedAtecoCandidates = populated
	return strategy, nil
}

func (s *maService) runEstimates(ctx context.Context, sessionID, strategyVersionID string, strategy MAStrategySpec, subject, email string) ([]MAEstimate, string, error) {
	queries := buildMAEstimateQueries(strategy)
	estimates := make([]MAEstimate, 0, len(queries))
	pricing := s.loadPricing(ctx)
	for _, query := range queries {
		query.params = baseMASurfaceParams(strategy, query.province, query.legalForm, query.params.DryRun)
		if query.atecoSearchCode != "" {
			query.params.AtecoCode = query.atecoSearchCode
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_estimate_query",
			Status:    maTraceEventStarted,
			Request:   maTraceJSON(query.params.Values()),
			Metadata: maTraceJSON(map[string]any{
				"session_id":          sessionID,
				"strategy_version_id": strategyVersionID,
				"strategy_type":       query.strategyType,
				"province":            query.province,
				"ateco_code":          query.atecoCode,
				"legal_form":          query.legalForm,
			}),
		}); err != nil {
			return nil, "", err
		}
		surface, err := s.probeMASearchSurface(ctx, query.params, subject, email)
		if err != nil {
			return nil, "", err
		}
		estimates = append(estimates, MAEstimate{
			ID:                uuid.NewString(),
			SessionID:         sessionID,
			StrategyVersionID: strategyVersionID,
			StrategyType:      query.strategyType,
			AtecoCode:         query.atecoCode,
			AtecoDescription:  query.atecoDescription,
			Province:          query.province,
			EstimatedCount:    surface.EstimatedCount,
			EstimatedCost:     float64(surface.EstimatedCount) * pricing.CostAdvanced,
			Selected:          false,
			SurfaceStatus:     surface.Status,
			ExecutionLimit:    strategy.SearchLimit,
			ProbeCount:        surface.ProbeCount,
			Params:            surface.Params,
			VendorResponse:    surface.VendorResponse,
		})
	}
	estimates = aggregateExpandedEstimates(estimates, strategy, pricing.CostAdvanced)
	selected := chooseSelectedStrategyFromEstimates(estimates, len(strategy.AtecoCandidates) > 0)
	for index := range estimates {
		estimates[index].Selected = selected != "" && estimates[index].StrategyType == selected
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_estimates_completed",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"session_id":          sessionID,
			"strategy_version_id": strategyVersionID,
			"estimate_count":      len(estimates),
			"selected_strategy":   selected,
		}),
	}); err != nil {
		return nil, "", err
	}
	return estimates, selected, nil
}

type maSearchQuery struct {
	strategyType     string
	atecoCode        string
	atecoSearchCode  string
	atecoDescription string
	province         string
	legalForm        string
	params           openapiit.CompanyITSearchParams
}

// maStrategyLegalForms returns the legal-form axis of the search cartesian: the
// explicit perimeter, or a single empty value meaning "any form" (no server-side
// legal-form filter). Because legalFormCode is single-valued at OpenAPI.it, each
// form is a separate query rather than a post-enrichment filter.
func maStrategyLegalForms(strategy MAStrategySpec) []string {
	if len(strategy.LegalForms) == 0 {
		return []string{""}
	}
	return strategy.LegalForms
}

// aggregateExpandedEstimates collapses the per-code expanded estimates into one
// row per province: the sector-bounded net is presented as a single "expanded"
// surface, not a wall of division-code chips. ATECO estimates pass through
// unchanged. Count/probe are summed; cost is recomputed from the sum; the surface
// is too_broad if any part was or the total exceeds the vendor limit.
func aggregateExpandedEstimates(estimates []MAEstimate, strategy MAStrategySpec, costAdvanced float64) []MAEstimate {
	out := make([]MAEstimate, 0, len(estimates))
	byProvince := map[string]int{}
	label := expandedSectorLabel(strategy)
	for _, est := range estimates {
		if est.StrategyType != maStrategyTypeExpanded {
			out = append(out, est)
			continue
		}
		if idx, ok := byProvince[est.Province]; ok {
			out[idx].EstimatedCount += est.EstimatedCount
			out[idx].ProbeCount += est.ProbeCount
			if est.SurfaceStatus == maEstimateSurfaceTooBroad {
				out[idx].SurfaceStatus = maEstimateSurfaceTooBroad
			}
			continue
		}
		merged := est
		merged.AtecoCode = ""
		merged.AtecoDescription = label
		byProvince[est.Province] = len(out)
		out = append(out, merged)
	}
	for i := range out {
		if out[i].StrategyType != maStrategyTypeExpanded {
			continue
		}
		out[i].EstimatedCost = float64(out[i].EstimatedCount) * costAdvanced
		if out[i].EstimatedCount > maVendorLimit {
			out[i].SurfaceStatus = maEstimateSurfaceTooBroad
		}
	}
	return out
}

// expandedSectorLabel describes the expanded net's perimeter for the estimate UI.
func expandedSectorLabel(strategy MAStrategySpec) string {
	if len(strategy.SectorDivisions) == 0 {
		return "Ricerca espansa"
	}
	return "Divisioni ATECO " + strings.Join(strategy.SectorDivisions, ", ")
}

func buildMAEstimateQueries(strategy MAStrategySpec) []maSearchQuery {
	dryRun := 1
	provinces := strategy.Provinces
	if len(provinces) == 0 {
		provinces = []string{""}
	}
	forms := maStrategyLegalForms(strategy)
	queries := make([]maSearchQuery, 0, len(provinces)*(len(strategy.AtecoCandidates)+1)*len(forms))
	for _, province := range provinces {
		for _, form := range forms {
			for _, candidate := range strategy.AtecoCandidates {
				searchCode := candidate.SearchCode
				if searchCode == "" {
					searchCode = atecoSearchCode(candidate.Code)
				}
				params := baseMASurfaceParams(strategy, province, form, &dryRun)
				params.AtecoCode = searchCode
				queries = append(queries, maSearchQuery{
					strategyType:     maStrategyTypeATECO,
					atecoCode:        candidate.Code,
					atecoSearchCode:  searchCode,
					atecoDescription: candidate.Description,
					province:         province,
					legalForm:        form,
					params:           params,
				})
			}
			if len(strategy.ExpandedAtecoCandidates) > 0 {
				// Sector-bounded expanded net: one dry-run per populated division
				// code instead of a single ATECO-less query over the whole province.
				for _, candidate := range strategy.ExpandedAtecoCandidates {
					searchCode := candidate.SearchCode
					if searchCode == "" {
						searchCode = atecoSearchCode(candidate.Code)
					}
					params := baseMASurfaceParams(strategy, province, form, &dryRun)
					params.AtecoCode = searchCode
					queries = append(queries, maSearchQuery{
						strategyType:     maStrategyTypeExpanded,
						atecoCode:        candidate.Code,
						atecoSearchCode:  searchCode,
						atecoDescription: candidate.Description,
						province:         province,
						legalForm:        form,
						params:           params,
					})
				}
			} else {
				// Legacy fallback for sector-less strategies (no divisions to widen):
				// the province-only surface, kept so such searches still run.
				params := baseMASurfaceParams(strategy, province, form, &dryRun)
				queries = append(queries, maSearchQuery{
					strategyType: maStrategyTypeExpanded,
					province:     province,
					legalForm:    form,
					params:       params,
				})
			}
		}
	}
	return queries
}

func (s *maService) probeMASearchSurface(ctx context.Context, params openapiit.CompanyITSearchParams, subject, email string) (maSurfaceProbeResult, error) {
	dryRun := 1
	params.DryRun = &dryRun
	params.Limit = nil
	params.Skip = nil

	first, err := s.runMASurfaceProbe(ctx, params, subject, email)
	if err != nil {
		return maSurfaceProbeResult{}, err
	}
	probes := []maSurfaceProbeAudit{first}
	status := maEstimateSurfaceExact
	estimatedCount := first.Count
	// Projected advanced-enrichment cost of fetching these companies — NOT the
	// dry-run's own spend (first.Cost, ~€0.01), which is tracked in the probe audit.
	estimatedCost := float64(estimatedCount) * maCostPerCompanyEUR
	if first.Count > maVendorLimit {
		status = maEstimateSurfaceTooBroad
	}

	paramsRaw, err := companySearchParamsJSON(params.Values())
	if err != nil {
		return maSurfaceProbeResult{}, err
	}
	responseRaw, err := json.Marshal(map[string]any{"probes": probes})
	if err != nil {
		return maSurfaceProbeResult{}, fmt.Errorf("marshal ma surface probe response: %w", err)
	}
	return maSurfaceProbeResult{
		Status:         status,
		EstimatedCount: estimatedCount,
		EstimatedCost:  estimatedCost,
		ProbeCount:     len(probes),
		Params:         paramsRaw,
		VendorResponse: responseRaw,
	}, nil
}

func (s *maService) runMASurfaceProbe(ctx context.Context, params openapiit.CompanyITSearchParams, subject, email string) (maSurfaceProbeAudit, error) {
	params.Limit = nil
	response, raw, err := s.cachedCompanySearch(ctx, params, subject, email)
	if err != nil {
		return maSurfaceProbeAudit{}, err
	}
	paramsRaw, err := companySearchParamsJSON(params.Values())
	if err != nil {
		return maSurfaceProbeAudit{}, err
	}
	skip := 0
	if params.Skip != nil {
		skip = *params.Skip
	}
	return maSurfaceProbeAudit{
		Skip:     skip,
		Count:    envelopeCount(response),
		Cost:     envelopeCost(response),
		Params:   paramsRaw,
		Response: raw,
	}, nil
}

// maExecutionCombo is one cell of the (province × ateco code × legal form)
// search cartesian. A company matches exactly one combo (one registered
// province, one exact ateco code, one legal form), so the union of combo results
// is distinct and the per-combo dry-run count feeds a fair budget allocation.
type maExecutionCombo struct {
	province   string
	atecoCode  string
	searchCode string
	legalForm  string
	count      int
}

func (s *maService) runExecution(ctx context.Context, strategy MAStrategySpec, strategyType string, limit int, subject, email string) ([]MATarget, error) {
	provinces := strategy.Provinces
	if len(provinces) == 0 {
		provinces = []string{""}
	}
	forms := maStrategyLegalForms(strategy)

	combos := []maExecutionCombo{}
	for _, province := range provinces {
		for _, form := range forms {
			if strategyType == maStrategyTypeATECO {
				for _, candidate := range strategy.AtecoCandidates {
					searchCode := candidate.SearchCode
					if searchCode == "" {
						searchCode = atecoSearchCode(candidate.Code)
					}
					combos = append(combos, maExecutionCombo{province: province, atecoCode: candidate.Code, searchCode: searchCode, legalForm: form})
				}
			} else if len(strategy.ExpandedAtecoCandidates) > 0 {
				// Sector-bounded expanded net: enrich only companies under the
				// populated division codes (minus exclusions), never the whole
				// province. Dedup downstream collapses cross-code overlaps.
				for _, candidate := range strategy.ExpandedAtecoCandidates {
					searchCode := candidate.SearchCode
					if searchCode == "" {
						searchCode = atecoSearchCode(candidate.Code)
					}
					combos = append(combos, maExecutionCombo{province: province, atecoCode: candidate.Code, searchCode: searchCode, legalForm: form})
				}
			} else {
				// Legacy province-only fallback (sector-less strategies).
				combos = append(combos, maExecutionCombo{province: province, legalForm: form})
			}
		}
	}

	// Count per combo (dry-run, count only). These hit the cache populated by the
	// estimate phase, so the allocation costs effectively nothing.
	for index := range combos {
		surfaceParams := baseMASurfaceParams(strategy, combos[index].province, combos[index].legalForm, nil)
		if combos[index].searchCode != "" {
			surfaceParams.AtecoCode = combos[index].searchCode
		}
		surface, err := s.probeMASearchSurface(ctx, surfaceParams, subject, email)
		if err != nil {
			return nil, err
		}
		combos[index].count = surface.EstimatedCount
	}

	counts := make([]int, len(combos))
	for index, combo := range combos {
		counts[index] = combo.count
	}
	allocation := allocateExecutionLimit(counts, limit)

	dryRun := 0
	rawTargets := []MATarget{}
	for index, combo := range combos {
		slot := allocation[index]
		if slot <= 0 {
			continue
		}
		params := baseMASearchParams(strategy, combo.province, combo.legalForm, &dryRun, slot)
		if combo.searchCode != "" {
			params.AtecoCode = combo.searchCode
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_execution_query",
			Status:    maTraceEventStarted,
			Request:   maTraceJSON(params.Values()),
			Metadata:  maTraceJSON(map[string]any{"strategy_type": strategyType, "province": combo.province, "ateco_code": combo.atecoCode, "legal_form": combo.legalForm, "allocated": slot, "available": combo.count}),
		}); err != nil {
			return nil, err
		}
		targets, err := s.executeCompanySearch(ctx, params, subject, email)
		if err != nil {
			return nil, err
		}
		rawTargets = append(rawTargets, targets...)
	}
	targets := dedupeMATargets(rawTargets)
	if len(targets) > limit {
		targets = targets[:limit]
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_execution_queries_completed",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"strategy_type": strategyType, "limit": limit, "combo_count": len(combos), "raw_target_count": len(rawTargets), "deduped_target_count": len(targets)}),
	}); err != nil {
		return nil, err
	}
	return targets, nil
}

// allocateExecutionLimit distributes a fetch limit across search combos using a
// floor (equal guaranteed minimum per non-empty combo) plus proportional shares
// of the remainder (largest-remainder method). When the combos together hold no
// more than the limit, every company is fetched. The result is aligned with the
// input counts and sums to min(limit, totalAvailable). Deterministic: ties break
// by lowest index.
func allocateExecutionLimit(counts []int, limit int) []int {
	slots := make([]int, len(counts))
	if limit <= 0 {
		return slots
	}
	total, nonEmpty := 0, 0
	for _, count := range counts {
		if count > 0 {
			total += count
			nonEmpty++
		}
	}
	if total == 0 {
		return slots
	}
	if total <= limit {
		for index, count := range counts {
			if count > 0 {
				slots[index] = count
			}
		}
		return slots
	}
	// Stage 1 — floor: a small guaranteed minimum per non-empty combo so a named
	// province/form is never fully starved by a larger one. Only when affordable.
	floor := 0
	if limit >= nonEmpty {
		floor = 1
	}
	used := 0
	for index, count := range counts {
		if count <= 0 {
			continue
		}
		give := floor
		if give > count {
			give = count
		}
		slots[index] = give
		used += give
	}
	// Stage 2 — proportional remainder over residual demand (largest-remainder).
	remaining := limit - used
	residualTotal := 0
	for index, count := range counts {
		if count > slots[index] {
			residualTotal += count - slots[index]
		}
	}
	remainder := make([]float64, len(counts))
	if residualTotal > 0 {
		for index, count := range counts {
			capacity := count - slots[index]
			if capacity <= 0 {
				continue
			}
			ideal := float64(remaining) * float64(capacity) / float64(residualTotal)
			give := int(ideal)
			if give > capacity {
				give = capacity
			}
			slots[index] += give
			used += give
			remainder[index] = ideal - float64(int(ideal))
		}
	}
	// Distribute any leftover one unit at a time by largest remainder; on ties
	// prefer the least-filled combo so units spread rather than pile on index 0.
	for used < limit {
		best := -1
		for index, count := range counts {
			if count <= slots[index] {
				continue
			}
			if best == -1 {
				best = index
				continue
			}
			if remainder[index] > remainder[best] ||
				(remainder[index] == remainder[best] && slots[index] < slots[best]) {
				best = index
			}
		}
		if best == -1 {
			break
		}
		slots[best]++
		used++
	}
	return slots
}

func (s *maService) executeCompanySearch(ctx context.Context, params openapiit.CompanyITSearchParams, subject, email string) ([]MATarget, error) {
	response, _, err := s.cachedCompanySearch(ctx, params, subject, email)
	if err != nil {
		return nil, err
	}
	return parseMATargetsFromVendorData(response.Data)
}

func (s *maService) cachedCompanySearch(ctx context.Context, params openapiit.CompanyITSearchParams, subject, email string) (openapiit.Envelope[openapiit.CompanyDataset], json.RawMessage, error) {
	cacheKey, paramsJSON, err := companySearchCacheKey(params)
	if err != nil {
		return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
	}
	now := s.now()
	if s.searchCache != nil {
		entry, err := s.searchCache.GetValidCompanySearch(ctx, cacheKey, now)
		if err != nil {
			return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
		}
		if entry != nil {
			envelope, err := decodeCompanySearchEnvelope(entry.Response)
			status := maTraceEventSucceeded
			errMessage := ""
			if err != nil {
				status = maTraceEventFailed
				errMessage = err.Error()
			}
			if traceErr := s.traceEvent(ctx, maTraceEventWrite{
				EventType:      "company_search_cache_hit",
				ExternalSystem: "binocolo_cache",
				Status:         status,
				Request:        maTraceJSON(paramsJSON),
				Response:       maTraceJSON(entry.Response),
				Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey, "count": envelopeCount(envelope), "cost": envelopeCost(envelope)}),
				Error:          errMessage,
			}); traceErr != nil {
				return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, traceErr
			}
			return envelope, entry.Response, err
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType:      "company_search_cache_miss",
			ExternalSystem: "binocolo_cache",
			Status:         maTraceEventInfo,
			Request:        maTraceJSON(paramsJSON),
			Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey}),
		}); err != nil {
			return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
		}
	} else if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType:      "company_search_cache_disabled",
		ExternalSystem: "binocolo_cache",
		Status:         maTraceEventInfo,
		Request:        maTraceJSON(paramsJSON),
		Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey}),
	}); err != nil {
		return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
	}
	if s.openapiit == nil {
		return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, errMAOpenAPIITUnavailable
	}

	fetch := func(ctx context.Context) (openapiit.Envelope[openapiit.CompanyDataset], json.RawMessage, error) {
		start := time.Now()
		response, err := s.openapiit.Company().SearchITRaw(ctx, params)
		if err != nil {
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType:      "company_search_upstream",
				ExternalSystem: "openapiit",
				Status:         maTraceEventFailed,
				DurationMS:     maTraceDuration(start),
				Request:        maTraceJSON(paramsJSON),
				Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey}),
				Error:          err.Error(),
			})
			return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
		}
		raw, err := json.Marshal(response)
		if err != nil {
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType:      "company_search_upstream",
				ExternalSystem: "openapiit",
				Status:         maTraceEventFailed,
				DurationMS:     maTraceDuration(start),
				Request:        maTraceJSON(paramsJSON),
				Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey}),
				Error:          err.Error(),
			})
			return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType:      "company_search_upstream",
			ExternalSystem: "openapiit",
			Status:         maTraceEventSucceeded,
			DurationMS:     maTraceDuration(start),
			Request:        maTraceJSON(paramsJSON),
			Response:       maTraceJSON(raw),
			Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey, "count": envelopeCount(response), "cost": envelopeCost(response)}),
		}); err != nil {
			return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
		}
		return response, raw, nil
	}

	if s.searchCache == nil {
		response, raw, err := fetch(ctx)
		return response, raw, err
	}

	var response openapiit.Envelope[openapiit.CompanyDataset]
	var raw json.RawMessage
	var upstreamErr error
	err = s.searchCache.WithCompanySearchCacheLock(ctx, cacheKey, func(ctx context.Context) error {
		entry, err := s.searchCache.GetValidCompanySearch(ctx, cacheKey, s.now())
		if err != nil {
			return err
		}
		if entry != nil {
			response, err = decodeCompanySearchEnvelope(entry.Response)
			raw = entry.Response
			status := maTraceEventSucceeded
			errMessage := ""
			if err != nil {
				status = maTraceEventFailed
				errMessage = err.Error()
			}
			if traceErr := s.traceEvent(ctx, maTraceEventWrite{
				EventType:      "company_search_cache_hit_after_lock",
				ExternalSystem: "binocolo_cache",
				Status:         status,
				Request:        maTraceJSON(paramsJSON),
				Response:       maTraceJSON(entry.Response),
				Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey, "count": envelopeCount(response), "cost": envelopeCost(response)}),
				Error:          errMessage,
			}); traceErr != nil {
				return traceErr
			}
			return err
		}
		response, raw, upstreamErr = fetch(ctx)
		if upstreamErr != nil {
			return nil
		}
		fetchedAt := s.now()
		dryRun := params.DryRun != nil && *params.DryRun == 1
		if err := s.searchCache.UpsertCompanySearch(ctx, companySearchCacheWrite{
			CacheKey:           cacheKey,
			Params:             paramsJSON,
			Response:           raw,
			DryRun:             dryRun,
			DataEnrichment:     params.DataEnrichment,
			FetchedAt:          fetchedAt,
			ExpiresAt:          fetchedAt.Add(companySearchCacheTTL),
			RefreshedBySubject: subject,
			RefreshedByEmail:   email,
		}); err != nil {
			return err
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType:      "company_search_cache_write",
			ExternalSystem: "binocolo_cache",
			Status:         maTraceEventSucceeded,
			Request:        maTraceJSON(paramsJSON),
			Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey}),
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
	}
	if upstreamErr != nil {
		return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, upstreamErr
	}
	return response, raw, nil
}

func decodeCompanySearchEnvelope(raw json.RawMessage) (openapiit.Envelope[openapiit.CompanyDataset], error) {
	var response openapiit.Envelope[openapiit.CompanyDataset]
	if err := json.Unmarshal(raw, &response); err != nil {
		return response, fmt.Errorf("decode cached company search: %w", err)
	}
	return response, nil
}

// baseMASearchParams builds an execution search for one cell of the
// (province × ateco code × legal form) iteration. Province, atecoCode and
// legalFormCode are single-valued at OpenAPI.it, so each is one axis of the
// cartesian; the caller iterates and passes a single value per call. An empty
// legalForm means "any form" (no server-side legal-form filter).
func baseMASearchParams(strategy MAStrategySpec, province, legalForm string, dryRun *int, limit int) openapiit.CompanyITSearchParams {
	if limit <= 0 {
		limit = maDefaultSearchLimit
	}
	if limit > maVendorLimit {
		limit = maVendorLimit
	}
	params := openapiit.CompanyITSearchParams{
		DryRun:         dryRun,
		DataEnrichment: "advanced",
		Province:       province,
		LegalFormCode:  legalForm,
		ActivityStatus: strategy.ActivityStatus,
		MinTurnover:    strategy.TurnoverMin,
		MaxTurnover:    strategy.TurnoverMax,
		MinEmployees:   strategy.EmployeeMin,
		MaxEmployees:   strategy.EmployeeMax,
		Limit:          &limit,
	}
	return params
}

func baseMASurfaceParams(strategy MAStrategySpec, province, legalForm string, dryRun *int) openapiit.CompanyITSearchParams {
	return openapiit.CompanyITSearchParams{
		DryRun:         dryRun,
		DataEnrichment: "advanced",
		Province:       province,
		LegalFormCode:  legalForm,
		ActivityStatus: strategy.ActivityStatus,
		MinTurnover:    strategy.TurnoverMin,
		MaxTurnover:    strategy.TurnoverMax,
		MinEmployees:   strategy.EmployeeMin,
		MaxEmployees:   strategy.EmployeeMax,
	}
}

func envelopeCount(envelope openapiit.Envelope[openapiit.CompanyDataset]) int {
	if envelope.Count != nil {
		return *envelope.Count
	}
	return 0
}

func envelopeCost(envelope openapiit.Envelope[openapiit.CompanyDataset]) float64 {
	if envelope.Cost != nil {
		return *envelope.Cost
	}
	return 0
}

func maStrategyBudget(strategy MAStrategySpec, defaultBudget float64) float64 {
	if strategy.MaxBudgetEUR != nil && *strategy.MaxBudgetEUR > 0 {
		return *strategy.MaxBudgetEUR
	}
	return defaultBudget
}

// maProjectedSpend is the advanced-enrichment cost of a run: at most `limit`
// companies out of `available` are fetched, each priced at maCostPerCompanyEUR.
func maProjectedSpend(available, limit int, costPerCompany float64) float64 {
	fetched := available
	if limit > 0 && fetched > limit {
		fetched = limit
	}
	if fetched < 0 {
		fetched = 0
	}
	return float64(fetched) * costPerCompany
}

func estimateTotal(estimates []MAEstimate, strategyType string) int {
	total := 0
	for _, estimate := range estimates {
		if estimate.StrategyType == strategyType {
			total += estimate.EstimatedCount
		}
	}
	return total
}

func estimatesForStrategy(estimates []MAEstimate, strategyType string) int {
	total := 0
	for _, estimate := range estimates {
		if estimate.StrategyType == strategyType {
			total++
		}
	}
	return total
}

func estimatesTooBroad(estimates []MAEstimate, strategyType string) bool {
	for _, estimate := range estimates {
		if estimate.StrategyType == strategyType && estimate.SurfaceStatus == maEstimateSurfaceTooBroad {
			return true
		}
	}
	return false
}

func estimatesMatchSearchLimit(estimates []MAEstimate, strategyType string, limit int) bool {
	seen := false
	for _, estimate := range estimates {
		if estimate.StrategyType != strategyType {
			continue
		}
		seen = true
		value := estimateExecutionLimit(estimate)
		if value <= 0 {
			return false
		}
		if value != limit {
			return false
		}
	}
	return seen
}

func estimateExecutionLimit(estimate MAEstimate) int {
	if estimate.ExecutionLimit > 0 {
		return estimate.ExecutionLimit
	}
	return estimateParamLimit(estimate.Params)
}

func estimateParamLimit(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var params map[string]string
	if err := json.Unmarshal(raw, &params); err != nil {
		return 0
	}
	value, ok := params["limit"]
	if !ok {
		return 0
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}

func buildMAXLSX(rows [][]any) ([]byte, error) {
	file := excelize.NewFile()
	defer file.Close()
	sheet := file.GetSheetName(0)
	if sheet == "" {
		sheet = "Sheet1"
	}
	_ = file.SetSheetName(sheet, "Target")
	sheet = "Target"
	for rowIndex, row := range rows {
		for colIndex, value := range row {
			cell, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
			if err != nil {
				return nil, err
			}
			if err := file.SetCellValue(sheet, cell, value); err != nil {
				return nil, err
			}
		}
	}
	if len(rows) > 0 {
		lastCol, _ := excelize.ColumnNumberToName(len(rows[0]))
		_ = file.AutoFilter(sheet, "A1:"+lastCol+"1", nil)
	}
	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func titleFromPrompt(prompt string) string {
	title := cleanText(prompt, 80)
	if title == "" {
		return "Nuova ricerca"
	}
	return title
}

func safeFilenamePart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "ricerca"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
		if b.Len() >= 48 {
			break
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "ricerca"
	}
	return out
}

func maErrorCode(err error) string {
	var apiErr *openapiit.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusPaymentRequired {
			return "openapiit_credit_required"
		}
		return "openapiit_upstream_error"
	}
	if errors.Is(err, errMAEstimateTooLarge) {
		return "estimate_too_large"
	}
	return "execution_error"
}

func validateOptionalUUID(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if _, err := uuid.Parse(value); err != nil {
		return fmt.Errorf("%w: llm selection", errMAStrategyInvalid)
	}
	return nil
}

func llmConfigError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return errMALLMConfigUnavailable
	}
	return err
}
