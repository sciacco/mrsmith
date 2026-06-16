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
	maAtecoToolName = "search_ateco_2025"
	maMaxToolRounds = 4
)

type maAIClient interface {
	Chat(context.Context, openrouter.ChatRequest) (openrouter.ChatResponse, error)
}

type maService struct {
	store       maWorkspaceStore
	searchCache companySearchCacheStore
	ateco       atecoStore
	openapiit   *openapiit.Client
	ai          maAIClient
	now         func() time.Time
}

func newMAService(store maWorkspaceStore, searchCache companySearchCacheStore, ateco atecoStore, openapiitClient *openapiit.Client, ai maAIClient) *maService {
	return &maService{
		store:       store,
		searchCache: searchCache,
		ateco:       ateco,
		openapiit:   openapiitClient,
		ai:          ai,
		now:         func() time.Time { return time.Now().UTC() },
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
	strategy, audit, err := s.draftStrategy(ctx, prompt, req.ModelID, req.PromptID)
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
	audit.SessionID = detail.Session.ID
	if detail.Strategy != nil {
		audit.StrategyVersionID = detail.Strategy.ID
	}
	if err := s.store.RecordMAModelAudit(ctx, audit); err != nil {
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
	}
	if strategyVersion == nil {
		return MASessionDetail{}, fmt.Errorf("%w: strategy", errMAStrategyInvalid)
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
	return s.store.GetMASession(ctx, sessionID)
}

func (s *maService) executeSession(ctx context.Context, sessionID string, req MAExecuteSessionRequest, subject, email string) (MASessionDetail, error) {
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
		detail, err = s.store.GetMASession(ctx, sessionID)
		if err != nil {
			return MASessionDetail{}, err
		}
	}
	if strategyVersion == nil {
		return MASessionDetail{}, fmt.Errorf("%w: strategy", errMAStrategyInvalid)
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
		strategyType = chooseSelectedStrategy(estimateTotal(detail.Estimates, maStrategyTypeATECO), len(strategyVersion.Strategy.AtecoCandidates) > 0)
	}
	estimatedCount := estimateTotal(detail.Estimates, strategyType)
	if len(detail.Estimates) == 0 || estimatedCount == 0 {
		return MASessionDetail{}, fmt.Errorf("%w: estimate required", errMAStrategyInvalid)
	}
	if estimatedCount > maVendorLimit {
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

	targets, execErr := s.runExecution(ctx, strategyVersion.Strategy, strategyType, limit, subject, email)
	if execErr != nil {
		_ = s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusFailed, 0, maErrorCode(execErr))
		return MASessionDetail{}, execErr
	}
	for index := range targets {
		targets[index].SessionID = sessionID
		targets[index].RunID = run.ID
		targets[index] = scoreMATarget(targets[index], strategyVersion.Strategy, s.now())
	}
	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].Score == targets[j].Score {
			return targets[i].CompanyName < targets[j].CompanyName
		}
		return targets[i].Score > targets[j].Score
	})
	if err := s.store.ReplaceMATargets(ctx, sessionID, run.ID, targets); err != nil {
		_ = s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusFailed, 0, "store_error")
		return MASessionDetail{}, err
	}
	if err := s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusCompleted, len(targets), ""); err != nil {
		return MASessionDetail{}, err
	}
	return s.store.GetMASession(ctx, sessionID)
}

func (s *maService) exportSession(ctx context.Context, sessionID string, format string, email string) ([]byte, string, string, error) {
	if s.store == nil {
		return nil, "", "", errMAStoreUnavailable
	}
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
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
	return content, filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
}

func (s *maService) draftStrategy(ctx context.Context, prompt string, modelID string, promptID string) (MAStrategySpec, maModelAuditWrite, error) {
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
	messages := []openrouter.Message{
		{Role: "system", Content: promptConfig.Prompt},
		{Role: "developer", Content: maAtecoToolInstructions()},
		{Role: "user", Content: prompt},
	}
	allowedAteco := map[string]AtecoCode{}
	var response openrouter.ChatResponse
	var responseRaw json.RawMessage
	for round := 0; round <= maMaxToolRounds; round++ {
		req := openrouter.ChatRequest{
			Model:          modelConfig.Model,
			Temperature:    0,
			MaxTokens:      1800,
			ResponseFormat: &openrouter.ResponseFormat{Type: "json_object"},
			Messages:       messages,
			Tools:          []openrouter.Tool{maAtecoSearchTool()},
			ToolChoice:     "auto",
		}
		response, err = s.ai.Chat(ctx, req)
		if err != nil {
			return MAStrategySpec{}, maModelAuditWrite{}, err
		}
		if len(response.ToolCalls) == 0 {
			responseRaw = json.RawMessage([]byte(strings.TrimSpace(response.Content)))
			break
		}
		if round == maMaxToolRounds {
			return MAStrategySpec{}, maModelAuditWrite{}, fmt.Errorf("%w: ateco tool loop", errMAStrategyInvalid)
		}
		messages = append(messages, openrouter.Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})
		for _, call := range response.ToolCalls {
			content := s.executeMAAtecoTool(ctx, call, allowedAteco)
			messages = append(messages, openrouter.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    content,
			})
		}
	}
	strategy, err := decodeMAStrategyResponse(responseRaw)
	if err != nil {
		return MAStrategySpec{}, maModelAuditWrite{}, err
	}
	strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategy, allowedAteco, true)
	if err != nil {
		return MAStrategySpec{}, maModelAuditWrite{}, err
	}
	usageRaw, _ := json.Marshal(response.Usage)
	promptRaw, _ := json.Marshal(map[string]any{
		"model_id":  modelConfig.ID,
		"prompt_id": promptConfig.ID,
		"messages":  messages,
		"tools":     []openrouter.Tool{maAtecoSearchTool()},
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

func maAtecoToolInstructions() string {
	return strings.Join([]string{
		"Per compilare atecoCandidates devi usare il tool search_ateco_2025.",
		"Non inventare codici ATECO e non usare codici che non compaiono nei risultati del tool.",
		"Se la prima ricerca e' troppo generica, fai piu' chiamate tool mirate con termini italiani come programmazione informatica, consulenza informatica, hosting, elaborazione dati.",
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

type maAtecoToolArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

func (s *maService) executeMAAtecoTool(ctx context.Context, call openrouter.ToolCall, allowed map[string]AtecoCode) string {
	if call.Function.Name != maAtecoToolName {
		return maToolErrorJSON("unsupported_tool")
	}
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
	remaining := map[string]int{
		maStrategyTypeATECO:    strategy.SearchLimit,
		maStrategyTypeExpanded: strategy.SearchLimit,
	}
	estimates := make([]MAEstimate, 0, len(queries))
	for _, query := range queries {
		queryLimit := remaining[query.strategyType]
		if queryLimit <= 0 {
			continue
		}
		query.params = baseMASearchParams(strategy, query.province, query.params.DryRun, queryLimit)
		if query.atecoSearchCode != "" {
			query.params.AtecoCode = query.atecoSearchCode
		}
		response, raw, err := s.cachedCompanySearch(ctx, query.params, subject, email)
		if err != nil {
			return nil, "", err
		}
		paramsRaw, err := companySearchParamsJSON(query.params.Values())
		if err != nil {
			return nil, "", err
		}
		estimatedCount := envelopeCount(response)
		if estimatedCount > queryLimit {
			estimatedCount = queryLimit
		}
		remaining[query.strategyType] -= estimatedCount
		estimates = append(estimates, MAEstimate{
			ID:                uuid.NewString(),
			SessionID:         sessionID,
			StrategyVersionID: strategyVersionID,
			StrategyType:      query.strategyType,
			AtecoCode:         query.atecoCode,
			AtecoDescription:  query.atecoDescription,
			Province:          query.province,
			EstimatedCount:    estimatedCount,
			EstimatedCost:     envelopeCost(response),
			Params:            paramsRaw,
			VendorResponse:    raw,
		})
	}
	atecoTotal := estimateTotal(estimates, maStrategyTypeATECO)
	selected := chooseSelectedStrategy(atecoTotal, len(strategy.AtecoCandidates) > 0)
	for index := range estimates {
		estimates[index].Selected = estimates[index].StrategyType == selected
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
			params := baseMASearchParams(strategy, province, &dryRun, strategy.SearchLimit)
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
		params := baseMASearchParams(strategy, province, &dryRun, strategy.SearchLimit)
		queries = append(queries, maSearchQuery{
			strategyType: maStrategyTypeExpanded,
			province:     province,
			params:       params,
		})
	}
	return queries
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
		targets, err := s.executeCompanySearch(ctx, params, subject, email)
		if err != nil {
			return nil, err
		}
		rawTargets = append(rawTargets, targets...)
		remaining = limit - len(dedupeMATargets(rawTargets))
	}
	targets := dedupeMATargets(rawTargets)
	if len(targets) > limit {
		targets = targets[:limit]
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
			return envelope, entry.Response, err
		}
	}
	if s.openapiit == nil {
		return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, errMAOpenAPIITUnavailable
	}

	fetch := func(ctx context.Context) (openapiit.Envelope[openapiit.CompanyDataset], json.RawMessage, error) {
		response, err := s.openapiit.Company().SearchITRaw(ctx, params)
		if err != nil {
			return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
		}
		raw, err := json.Marshal(response)
		if err != nil {
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

func estimatesMatchSearchLimit(estimates []MAEstimate, strategyType string, limit int) bool {
	seen := false
	maxLimit := 0
	for _, estimate := range estimates {
		if estimate.StrategyType != strategyType {
			continue
		}
		seen = true
		value := estimateParamLimit(estimate.Params)
		if value <= 0 {
			return false
		}
		if value > maxLimit {
			maxLimit = value
		}
	}
	return seen && maxLimit == limit
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
