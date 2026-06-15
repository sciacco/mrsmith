package binocolo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
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
	errMAEstimateTooLarge      = errors.New("estimate too large")
)

type maAIClient interface {
	Chat(context.Context, openrouter.ChatRequest) (openrouter.ChatResponse, error)
}

type maService struct {
	store     maWorkspaceStore
	openapiit *openapiit.Client
	ai        maAIClient
	now       func() time.Time
}

func newMAService(store maWorkspaceStore, openapiitClient *openapiit.Client, ai maAIClient) *maService {
	return &maService{
		store:     store,
		openapiit: openapiitClient,
		ai:        ai,
		now:       func() time.Time { return time.Now().UTC() },
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

func (s *maService) createSession(ctx context.Context, req MACreateSessionRequest, subject, email string) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	prompt := cleanText(req.Prompt, 4000)
	if prompt == "" {
		return MASessionDetail{}, fmt.Errorf("%w: prompt", errMAStrategyInvalid)
	}
	strategy, audit, err := s.draftStrategy(ctx, prompt)
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

func (s *maService) estimateSession(ctx context.Context, sessionID string, req MAEstimateSessionRequest, email string) (MASessionDetail, error) {
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
		strategyVersion, err = s.store.AddMAStrategyVersion(ctx, sessionID, strategy, email)
		if err != nil {
			return MASessionDetail{}, err
		}
	}
	if strategyVersion == nil {
		return MASessionDetail{}, fmt.Errorf("%w: strategy", errMAStrategyInvalid)
	}

	estimates, selected, err := s.runEstimates(ctx, sessionID, strategyVersion.ID, strategyVersion.Strategy)
	if err != nil {
		return MASessionDetail{}, err
	}
	if err := s.store.ReplaceMAEstimates(ctx, sessionID, strategyVersion.ID, selected, estimates); err != nil {
		return MASessionDetail{}, err
	}
	return s.store.GetMASession(ctx, sessionID)
}

func (s *maService) executeSession(ctx context.Context, sessionID string, req MAExecuteSessionRequest, email string) (MASessionDetail, error) {
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
	limit := req.Limit
	if limit <= 0 {
		limit = maDefaultSearchLimit
	}
	if limit > maVendorLimit {
		limit = maVendorLimit
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

	targets, execErr := s.runExecution(ctx, strategyVersion.Strategy, strategyType, limit)
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

func (s *maService) draftStrategy(ctx context.Context, prompt string) (MAStrategySpec, maModelAuditWrite, error) {
	if s.ai == nil {
		return MAStrategySpec{}, maModelAuditWrite{}, errMAOpenRouterUnavailable
	}
	model := maDefaultModel
	if s.store != nil {
		resolved, err := s.store.ResolveMAModel(ctx, maModelScopeStrategy)
		if err != nil {
			return MAStrategySpec{}, maModelAuditWrite{}, err
		}
		if strings.TrimSpace(resolved) != "" {
			model = resolved
		}
	}
	messages := []openrouter.Message{
		{Role: "system", Content: maStrategySystemPrompt},
		{Role: "user", Content: prompt},
	}
	req := openrouter.ChatRequest{
		Model:          model,
		Temperature:    0,
		MaxTokens:      1800,
		ResponseFormat: &openrouter.ResponseFormat{Type: "json_object"},
		Messages:       messages,
	}
	promptRaw, _ := json.Marshal(map[string]any{"messages": messages})
	response, err := s.ai.Chat(ctx, req)
	if err != nil {
		return MAStrategySpec{}, maModelAuditWrite{}, err
	}
	responseRaw := json.RawMessage([]byte(strings.TrimSpace(response.Content)))
	var envelope maStrategyDraftEnvelope
	if err := json.Unmarshal(responseRaw, &envelope); err != nil || strings.TrimSpace(envelope.Strategy.SectorDescription) == "" {
		var direct MAStrategySpec
		if directErr := json.Unmarshal(responseRaw, &direct); directErr != nil {
			if err != nil {
				return MAStrategySpec{}, maModelAuditWrite{}, fmt.Errorf("decode ma strategy: %w", err)
			}
			return MAStrategySpec{}, maModelAuditWrite{}, fmt.Errorf("decode ma strategy: %w", directErr)
		}
		envelope.Strategy = direct
	}
	strategy, err := validateMAStrategy(envelope.Strategy)
	if err != nil {
		return MAStrategySpec{}, maModelAuditWrite{}, err
	}
	usageRaw, _ := json.Marshal(response.Usage)
	return strategy, maModelAuditWrite{
		Scope:    maModelScopeStrategy,
		Model:    model,
		Prompt:   promptRaw,
		Response: responseRaw,
		Usage:    usageRaw,
	}, nil
}

func (s *maService) runEstimates(ctx context.Context, sessionID, strategyVersionID string, strategy MAStrategySpec) ([]MAEstimate, string, error) {
	queries := buildMAEstimateQueries(strategy)
	estimates := make([]MAEstimate, 0, len(queries))
	for _, query := range queries {
		response, err := s.openapiit.Company().SearchITRaw(ctx, query.params)
		if err != nil {
			return nil, "", err
		}
		raw, err := json.Marshal(response)
		if err != nil {
			return nil, "", err
		}
		paramsRaw, err := companySearchParamsJSON(query.params.Values())
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
			EstimatedCount:    envelopeCount(response),
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
			params := baseMASearchParams(strategy, province, &dryRun, 1)
			params.AtecoCode = candidate.Code
			queries = append(queries, maSearchQuery{
				strategyType:     maStrategyTypeATECO,
				atecoCode:        candidate.Code,
				atecoDescription: candidate.Description,
				province:         province,
				params:           params,
			})
		}
		params := baseMASearchParams(strategy, province, &dryRun, 1)
		queries = append(queries, maSearchQuery{
			strategyType: maStrategyTypeExpanded,
			province:     province,
			params:       params,
		})
	}
	return queries
}

func (s *maService) runExecution(ctx context.Context, strategy MAStrategySpec, strategyType string, limit int) ([]MATarget, error) {
	dryRun := 0
	provinces := strategy.Provinces
	if len(provinces) == 0 {
		provinces = []string{""}
	}
	rawTargets := []MATarget{}
	for _, province := range provinces {
		if strategyType == maStrategyTypeATECO {
			for _, candidate := range strategy.AtecoCandidates {
				params := baseMASearchParams(strategy, province, &dryRun, limit)
				params.AtecoCode = candidate.Code
				targets, err := s.executeCompanySearch(ctx, params)
				if err != nil {
					return nil, err
				}
				rawTargets = append(rawTargets, targets...)
			}
			continue
		}
		params := baseMASearchParams(strategy, province, &dryRun, limit)
		targets, err := s.executeCompanySearch(ctx, params)
		if err != nil {
			return nil, err
		}
		rawTargets = append(rawTargets, targets...)
	}
	return dedupeMATargets(rawTargets), nil
}

func (s *maService) executeCompanySearch(ctx context.Context, params openapiit.CompanyITSearchParams) ([]MATarget, error) {
	response, err := s.openapiit.Company().SearchITRaw(ctx, params)
	if err != nil {
		return nil, err
	}
	return parseMATargetsFromVendorData(response.Data)
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
	var payload any
	if err := json.Unmarshal(envelope.Data, &payload); err == nil {
		if value, ok := findNumericMetric(payload, []string{"count", "total", "results"}); ok {
			return int(value)
		}
	}
	return 0
}

func envelopeCost(envelope openapiit.Envelope[openapiit.CompanyDataset]) float64 {
	if envelope.Cost != nil {
		return *envelope.Cost
	}
	var payload any
	if err := json.Unmarshal(envelope.Data, &payload); err == nil {
		if value, ok := findNumericMetric(payload, []string{"cost", "price", "amount", "prezzo"}); ok {
			return value
		}
	}
	return 0
}

func findNumericMetric(value any, hints []string) (float64, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
			for _, hint := range hints {
				if strings.Contains(normalized, hint) {
					if number, ok := vendorNumber(nested); ok {
						return number, true
					}
				}
			}
		}
		for _, nested := range typed {
			if number, ok := findNumericMetric(nested, hints); ok {
				return number, true
			}
		}
	case []any:
		for _, nested := range typed {
			if number, ok := findNumericMetric(nested, hints); ok {
				return number, true
			}
		}
	}
	return 0, false
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

const maStrategySystemPrompt = `Sei un analista M&A per ricerche su aziende italiane.
Trasforma la richiesta dell'utente in una strategia JSON per interrogare Company IT-search.
Rispondi solo con JSON valido nel formato:
{
  "strategy": {
    "title": "titolo breve",
    "sectorDescription": "settore target",
    "territoryLabel": "territorio in parole",
    "provinces": ["MI"],
    "activityStatus": "ATTIVA",
    "turnoverAround": 5000000,
    "turnoverMin": 3500000,
    "turnoverMax": 6500000,
    "employeeMin": null,
    "employeeMax": null,
    "atecoCandidates": [{"code":"6201","description":"...","rationale":"..."}],
    "keywords": ["software"],
    "shareholder": {"requiresEqualSplit": true, "tolerance": 2},
    "shareholderAge": {"required": true, "min": 55, "max": null},
    "rationale": "sintesi della strategia",
    "missingCriteria": []
  }
}
Regole:
- usa activityStatus ATTIVA se non richiesto diversamente;
- se l'utente dice "intorno a" un fatturato, imposta turnoverAround e anche min/max a +/-30%;
- proponi codici ATECO plausibili con razionale, ma non inventare dati aziendali;
- se un criterio non e' derivabile dalla richiesta, lascialo vuoto e aggiungilo a missingCriteria;
- provinces deve contenere sigle italiane di due lettere quando il territorio e' provinciale.`
