package binocolo

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

func (s *maService) listSessions(ctx context.Context) ([]MASessionSummary, error) {
	if s.store == nil {
		return nil, errMAStoreUnavailable
	}
	return s.store.ListMASessions(ctx)
}

func (s *maService) getSession(ctx context.Context, id string) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	return s.store.GetMASession(ctx, id)
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
	return detail, nil
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
	return s.store.GetMASession(ctx, sessionID)
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
	targets = scoreMATargetsV2(targets, strategyVersion.Strategy, s.now())
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
	return s.store.GetMASession(ctx, sessionID)
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
	return strategy, nil
}

func (s *maService) runEstimates(ctx context.Context, sessionID, strategyVersionID string, strategy MAStrategySpec, subject, email string) ([]MAEstimate, string, error) {
	queries := buildMAEstimateQueries(strategy)
	estimates := make([]MAEstimate, 0, len(queries))
	for _, query := range queries {
		query.params = baseMASurfaceParams(strategy, query.province, query.params.DryRun)
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
			EstimatedCost:     surface.EstimatedCost,
			Selected:          false,
			SurfaceStatus:     surface.Status,
			ExecutionLimit:    strategy.SearchLimit,
			ProbeCount:        surface.ProbeCount,
			Params:            surface.Params,
			VendorResponse:    surface.VendorResponse,
		})
	}
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
	params           openapiit.CompanyITSearchParams
}

func buildMAEstimateQueries(strategy MAStrategySpec) []maSearchQuery {
	dryRun := 1
	provinces := strategy.Provinces
	if len(provinces) == 0 {
		provinces = []string{""}
	}
	queries := make([]maSearchQuery, 0, len(provinces)*(len(strategy.AtecoCandidates)+1))
	for _, province := range provinces {
		for _, candidate := range strategy.AtecoCandidates {
			searchCode := candidate.SearchCode
			if searchCode == "" {
				searchCode = atecoSearchCode(candidate.Code)
			}
			params := baseMASurfaceParams(strategy, province, &dryRun)
			params.AtecoCode = searchCode
			queries = append(queries, maSearchQuery{
				strategyType:     maStrategyTypeATECO,
				atecoCode:        candidate.Code,
				atecoSearchCode:  searchCode,
				atecoDescription: candidate.Description,
				province:         province,
				params:           params,
			})
		}
		params := baseMASurfaceParams(strategy, province, &dryRun)
		queries = append(queries, maSearchQuery{
			strategyType: maStrategyTypeExpanded,
			province:     province,
			params:       params,
		})
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
	estimatedCost := first.Cost
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

func (s *maService) runExecution(ctx context.Context, strategy MAStrategySpec, strategyType string, limit int, subject, email string) ([]MATarget, error) {
	dryRun := 0
	provinces := strategy.Provinces
	if len(provinces) == 0 {
		provinces = []string{""}
	}
	remaining := limit
	rawTargets := []MATarget{}
	for _, province := range provinces {
		if remaining <= 0 {
			break
		}
		if strategyType == maStrategyTypeATECO {
			for _, candidate := range strategy.AtecoCandidates {
				if remaining <= 0 {
					break
				}
				params := baseMASearchParams(strategy, province, &dryRun, remaining)
				if candidate.SearchCode != "" {
					params.AtecoCode = candidate.SearchCode
				} else {
					params.AtecoCode = atecoSearchCode(candidate.Code)
				}
				if err := s.traceEvent(ctx, maTraceEventWrite{
					EventType: "ma_execution_query",
					Status:    maTraceEventStarted,
					Request:   maTraceJSON(params.Values()),
					Metadata:  maTraceJSON(map[string]any{"strategy_type": strategyType, "province": province, "ateco_code": candidate.Code, "remaining": remaining}),
				}); err != nil {
					return nil, err
				}
				targets, err := s.executeCompanySearch(ctx, params, subject, email)
				if err != nil {
					return nil, err
				}
				rawTargets = append(rawTargets, targets...)
				remaining = limit - len(dedupeMATargets(rawTargets))
			}
			continue
		}
		params := baseMASearchParams(strategy, province, &dryRun, remaining)
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_execution_query",
			Status:    maTraceEventStarted,
			Request:   maTraceJSON(params.Values()),
			Metadata:  maTraceJSON(map[string]any{"strategy_type": strategyType, "province": province, "remaining": remaining}),
		}); err != nil {
			return nil, err
		}
		targets, err := s.executeCompanySearch(ctx, params, subject, email)
		if err != nil {
			return nil, err
		}
		rawTargets = append(rawTargets, targets...)
		remaining = limit - len(dedupeMATargets(rawTargets))
	}
	targets := maFilterByLegalForms(dedupeMATargets(rawTargets), strategy.LegalForms)
	if len(targets) > limit {
		targets = targets[:limit]
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_execution_queries_completed",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"strategy_type": strategyType, "limit": limit, "raw_target_count": len(rawTargets), "deduped_target_count": len(targets)}),
	}); err != nil {
		return nil, err
	}
	return targets, nil
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

func baseMASearchParams(strategy MAStrategySpec, province string, dryRun *int, limit int) openapiit.CompanyITSearchParams {
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
		ActivityStatus: strategy.ActivityStatus,
		MinTurnover:    strategy.TurnoverMin,
		MaxTurnover:    strategy.TurnoverMax,
		MinEmployees:   strategy.EmployeeMin,
		MaxEmployees:   strategy.EmployeeMax,
		Limit:          &limit,
	}
	// A single explicit legal-form constraint filters server-side; multiple forms
	// are enforced by post-filtering the results (see maFilterByLegalForms).
	if len(strategy.LegalForms) == 1 {
		params.LegalFormCode = strategy.LegalForms[0]
	}
	return params
}

func baseMASurfaceParams(strategy MAStrategySpec, province string, dryRun *int) openapiit.CompanyITSearchParams {
	params := openapiit.CompanyITSearchParams{
		DryRun:         dryRun,
		DataEnrichment: "advanced",
		Province:       province,
		ActivityStatus: strategy.ActivityStatus,
		MinTurnover:    strategy.TurnoverMin,
		MaxTurnover:    strategy.TurnoverMax,
		MinEmployees:   strategy.EmployeeMin,
		MaxEmployees:   strategy.EmployeeMax,
	}
	if len(strategy.LegalForms) == 1 {
		params.LegalFormCode = strategy.LegalForms[0]
	}
	return params
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
