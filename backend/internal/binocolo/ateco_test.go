package binocolo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/openrouter"
)

func TestAtecoSearchTokensExpandITWithoutBroadServizi(t *testing.T) {
	tokens := atecoSearchTokens("servizi IT")
	if slices.Contains(tokens, "servizi") {
		t.Fatalf("tokens = %#v, broad token servizi should be filtered", tokens)
	}
	for _, want := range []string{"informatica", "software", "programmazione", "hosting", "elaborazione"} {
		if !slices.Contains(tokens, want) {
			t.Fatalf("tokens = %#v, want %q", tokens, want)
		}
	}
}

func TestCanonicalizeMAStrategyAtecoUsesTableCodeAndSearchCode(t *testing.T) {
	ateco := newFakeAtecoStore([]AtecoCode{{
		Codice:       "62.10.00",
		CodiceSearch: "621000",
		Titolo:       "Attività di programmazione informatica",
	}})
	service := &maService{ateco: ateco}
	strategy, err := service.canonicalizeMAStrategyAteco(context.Background(), MAStrategySpec{
		SectorDescription: "servizi IT",
		AtecoCandidates: []MAAtecoCandidate{
			{Code: "621000", Description: "testo LLM", Rationale: "coerente"},
		},
	}, nil, false)
	if err != nil {
		t.Fatalf("canonicalize returned error: %v", err)
	}
	if len(strategy.AtecoCandidates) != 1 {
		t.Fatalf("candidates = %#v, want one", strategy.AtecoCandidates)
	}
	got := strategy.AtecoCandidates[0]
	if got.Code != "62.10.00" {
		t.Fatalf("code = %q, want canonical dotted code", got.Code)
	}
	if got.SearchCode != "621000" {
		t.Fatalf("search code = %q, want 621000", got.SearchCode)
	}
	if got.Description != "Attività di programmazione informatica" {
		t.Fatalf("description = %q, want table title", got.Description)
	}
}

func TestDraftStrategyUsesAtecoToolWhitelist(t *testing.T) {
	ateco := newFakeAtecoStore([]AtecoCode{{
		Codice:       "62.10.00",
		CodiceSearch: "621000",
		Titolo:       "Attività di programmazione informatica",
	}})
	ai := &fakeMAAI{responses: []openrouter.ChatResponse{
		{
			ToolCalls: []openrouter.ToolCall{
				{
					ID:   "call_ateco",
					Type: "function",
					Function: openrouter.ToolCallFunction{
						Name:      maAtecoToolName,
						Arguments: `{"query":"servizi IT","limit":500}`,
					},
				},
			},
		},
		{
			Content: `{"strategy":{"sectorDescription":"servizi IT","provinces":[],"activityStatus":"ATTIVA","searchLimit":100,"atecoCandidates":[{"code":"62.10.00","description":"LLM","rationale":"trovato via tool"}],"keywords":[],"rationale":"ok","missingCriteria":[]}}`,
		},
	}}
	service := newMAService(&fakeMAWorkspaceStore{}, nil, ateco, nil, ai)

	strategy, _, err := service.draftStrategy(context.Background(), "trova aziende servizi IT", "", "")
	if err != nil {
		t.Fatalf("draftStrategy returned error: %v", err)
	}
	if len(ai.requests) != 2 {
		t.Fatalf("ai requests = %d, want 2", len(ai.requests))
	}
	if len(ai.requests[0].Tools) != 1 || ai.requests[0].Tools[0].Function.Name != maAtecoToolName {
		t.Fatalf("first request tools = %#v, want ateco tool", ai.requests[0].Tools)
	}
	if ateco.lastLimit != atecoSearchHardLimit {
		t.Fatalf("tool limit = %d, want hard cap %d", ateco.lastLimit, atecoSearchHardLimit)
	}
	if len(strategy.AtecoCandidates) != 1 || strategy.AtecoCandidates[0].SearchCode != "621000" {
		t.Fatalf("strategy candidates = %#v, want canonicalized tool result", strategy.AtecoCandidates)
	}
	if strategy.AtecoCandidates[0].Description != "Attività di programmazione informatica" {
		t.Fatalf("candidate description = %q, want table title", strategy.AtecoCandidates[0].Description)
	}
}

func TestDraftStrategyRejectsAtecoOutsideToolWhitelist(t *testing.T) {
	ateco := newFakeAtecoStore([]AtecoCode{{
		Codice:       "62.10.00",
		CodiceSearch: "621000",
		Titolo:       "Attività di programmazione informatica",
	}})
	ai := &fakeMAAI{responses: []openrouter.ChatResponse{
		{
			ToolCalls: []openrouter.ToolCall{
				{
					ID:   "call_ateco",
					Type: "function",
					Function: openrouter.ToolCallFunction{
						Name:      maAtecoToolName,
						Arguments: `{"query":"servizi IT"}`,
					},
				},
			},
		},
		{
			Content: `{"strategy":{"sectorDescription":"servizi IT","provinces":[],"activityStatus":"ATTIVA","searchLimit":100,"atecoCandidates":[{"code":"63.10.10","description":"inventato","rationale":"non restituito"}],"keywords":[],"rationale":"ok","missingCriteria":[]}}`,
		},
	}}
	service := newMAService(&fakeMAWorkspaceStore{}, nil, ateco, nil, ai)

	_, _, err := service.draftStrategy(context.Background(), "trova aziende servizi IT", "", "")
	if !errors.Is(err, errMAStrategyInvalid) {
		t.Fatalf("err = %v, want errMAStrategyInvalid", err)
	}
}

func TestMAEstimatesSendDotlessAtecoToOpenAPIIT(t *testing.T) {
	upstreamAteco := []string{}
	client := newCompanySearchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		upstreamAteco = append(upstreamAteco, r.URL.Query().Get("atecoCode"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":    []map[string]any{},
			"success": true,
			"message": "ok",
			"error":   nil,
			"count":   7,
			"cost":    0.1,
		})
	})
	service := &maService{
		openapiit: client,
		now:       func() time.Time { return time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC) },
	}

	estimates, _, err := service.runEstimates(context.Background(), "session", "version", MAStrategySpec{
		SectorDescription: "servizi IT",
		ActivityStatus:    "ATTIVA",
		SearchLimit:       10,
		AtecoCandidates: []MAAtecoCandidate{
			{Code: "62.10.00", SearchCode: "621000", Description: "Attività di programmazione informatica"},
		},
	}, "", "")
	if err != nil {
		t.Fatalf("runEstimates returned error: %v", err)
	}
	if !slices.Contains(upstreamAteco, "621000") {
		t.Fatalf("upstream atecoCode values = %#v, want one 621000", upstreamAteco)
	}
	if len(estimates) == 0 || estimates[0].AtecoCode != "62.10.00" {
		t.Fatalf("estimates = %#v, want canonical dotted ateco code", estimates)
	}
	var params map[string]string
	if err := json.Unmarshal(estimates[0].Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["atecoCode"] != "621000" {
		t.Fatalf("estimate params atecoCode = %q, want 621000", params["atecoCode"])
	}
}

func TestCompanySearchNormalizesAtecoBeforeCacheAndUpstream(t *testing.T) {
	ateco := newFakeAtecoStore([]AtecoCode{{
		Codice:       "62.10.00",
		CodiceSearch: "621000",
		Titolo:       "Attività di programmazione informatica",
	}})
	store := newMemoryCompanySearchCache()
	var upstreamAteco string
	client := newCompanySearchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		upstreamAteco = r.URL.Query().Get("atecoCode")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":    map[string]string{"source": "fresh"},
			"success": true,
			"message": "fresh",
			"error":   nil,
		})
	})
	h := &Handler{openapiit: client, companySearchCache: store, ateco: ateco}

	rec := performCompanySearch(t, h, "province=MI&dry_run=true&dataEnrichment=start&limit=10&atecoCode=62.10.00")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if upstreamAteco != "621000" {
		t.Fatalf("upstream atecoCode = %q, want 621000", upstreamAteco)
	}
	if store.len() != 1 {
		t.Fatalf("cache entries = %d, want one", store.len())
	}
	for _, entry := range store.entries {
		var params map[string]string
		if err := json.Unmarshal(entry.params, &params); err != nil {
			t.Fatalf("unmarshal cache params: %v", err)
		}
		if params["atecoCode"] != "621000" {
			t.Fatalf("cache params atecoCode = %q, want 621000", params["atecoCode"])
		}
	}
}

type fakeAtecoStore struct {
	items     map[string]AtecoCode
	lastQuery string
	lastLimit int
}

func newFakeAtecoStore(items []AtecoCode) *fakeAtecoStore {
	store := &fakeAtecoStore{items: map[string]AtecoCode{}}
	for _, item := range items {
		rememberAllowedAteco(store.items, item)
	}
	return store
}

func (s *fakeAtecoStore) ResolveAtecoCode(_ context.Context, code string) (AtecoCode, error) {
	item, ok := s.items[atecoSearchCode(code)]
	if !ok {
		return AtecoCode{}, errAtecoCodeNotFound
	}
	return item, nil
}

func (s *fakeAtecoStore) SearchAteco(_ context.Context, query string, limit int) ([]AtecoCode, error) {
	s.lastQuery = query
	s.lastLimit = limit
	out := make([]AtecoCode, 0, len(s.items))
	seen := map[string]struct{}{}
	for _, item := range s.items {
		if _, exists := seen[item.CodiceSearch]; exists {
			continue
		}
		seen[item.CodiceSearch] = struct{}{}
		out = append(out, item)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type fakeMAAI struct {
	responses []openrouter.ChatResponse
	requests  []openrouter.ChatRequest
}

func (f *fakeMAAI) Chat(_ context.Context, req openrouter.ChatRequest) (openrouter.ChatResponse, error) {
	f.requests = append(f.requests, req)
	if len(f.responses) == 0 {
		return openrouter.ChatResponse{}, errors.New("missing fake ai response")
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response, nil
}

type fakeMAWorkspaceStore struct{}

func (f *fakeMAWorkspaceStore) ListMASessions(context.Context) ([]MASessionSummary, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) CreateMASession(context.Context, maSessionCreate) (MASessionDetail, error) {
	return MASessionDetail{}, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) GetMASession(context.Context, string) (MASessionDetail, error) {
	return MASessionDetail{}, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) AddMAStrategyVersion(context.Context, string, MAStrategySpec, string) (*MAStrategyVersion, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) ReplaceMAEstimates(context.Context, string, string, string, []MAEstimate) error {
	return errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) CreateMAExecutionRun(context.Context, maExecutionRunCreate) (MAExecutionRun, error) {
	return MAExecutionRun{}, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) CompleteMAExecutionRun(context.Context, string, string, int, string) error {
	return errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) ReplaceMATargets(context.Context, string, string, []MATarget) error {
	return errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) RecordMAModelAudit(context.Context, maModelAuditWrite) error {
	return nil
}

func (f *fakeMAWorkspaceStore) ListMALLMOptions(context.Context) (MALLMOptionsResponse, error) {
	return MALLMOptionsResponse{}, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) ResolveMAModel(context.Context, string, string) (maLLMModel, error) {
	return maLLMModel{ID: "model-id", Scope: maModelScopeStrategy, Name: "Model", Model: "test-model", IsDefault: true}, nil
}

func (f *fakeMAWorkspaceStore) ResolveMAPrompt(context.Context, string, string) (maLLMPrompt, error) {
	return maLLMPrompt{ID: "prompt-id", Scope: maModelScopeStrategy, Name: "Prompt", Prompt: "Rispondi solo con JSON.", IsDefault: true}, nil
}

func (f *fakeMAWorkspaceStore) RecordMAExport(context.Context, string, string, int, string) error {
	return errors.New("not implemented")
}

var _ atecoStore = (*fakeAtecoStore)(nil)
var _ maAIClient = (*fakeMAAI)(nil)
var _ maWorkspaceStore = (*fakeMAWorkspaceStore)(nil)
