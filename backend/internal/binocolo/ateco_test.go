package binocolo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/openapiit"
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
	service := newMAService(&fakeMAWorkspaceStore{}, nil, nil, ateco, nil, ai)

	strategy, _, err := service.draftStrategy(context.Background(), "trova aziende servizi IT", "", "", "", "")
	if err != nil {
		t.Fatalf("draftStrategy returned error: %v", err)
	}
	if len(ai.requests) != 2 {
		t.Fatalf("ai requests = %d, want 2", len(ai.requests))
	}
	if !hasTool(ai.requests[0].Tools, maAtecoToolName) || !hasTool(ai.requests[0].Tools, maProvinceRegionToolName) || !hasTool(ai.requests[0].Tools, maCompanySurfaceToolName) {
		t.Fatalf("first request tools = %#v, want ateco, province, and surface tools", ai.requests[0].Tools)
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
	service := newMAService(&fakeMAWorkspaceStore{}, nil, nil, ateco, nil, ai)

	_, _, err := service.draftStrategy(context.Background(), "trova aziende servizi IT", "", "", "", "")
	if !errors.Is(err, errMAStrategyInvalid) {
		t.Fatalf("err = %v, want errMAStrategyInvalid", err)
	}
}

func TestHandleCreateMASessionTracesPreSessionValidationFailure(t *testing.T) {
	store := &fakeMAWorkspaceStore{}
	h := &Handler{ma: newMAService(store, nil, nil, nil, nil, nil)}
	req := httptest.NewRequest(http.MethodPost, "/binocolo/v1/ma/sessions", strings.NewReader(`{"prompt":"   "}`))
	req = req.WithContext(logging.WithRequestID(req.Context(), "req-pre-session"))
	rec := httptest.NewRecorder()

	h.handleCreateMASession(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(store.traces) != 1 {
		t.Fatalf("traces = %d, want 1", len(store.traces))
	}
	if store.traces[0].RequestID != "req-pre-session" || store.traces[0].Operation != "ma_session_create" {
		t.Fatalf("trace start = %#v", store.traces[0])
	}
	if len(store.done) != 1 {
		t.Fatalf("completed traces = %d, want 1", len(store.done))
	}
	if got := store.done[0]; got.Status != maTraceStatusFailed || got.HTTPStatus != http.StatusBadRequest || got.ErrorCode != "invalid_ma_request" {
		t.Fatalf("trace completion = %#v, want failed invalid_ma_request", got)
	}
}

func TestDraftStrategyTracesToolLoopFailure(t *testing.T) {
	ateco := newFakeAtecoStore([]AtecoCode{{
		Codice:       "62.10.00",
		CodiceSearch: "621000",
		Titolo:       "Attività di programmazione informatica",
	}})
	responses := make([]openrouter.ChatResponse, 0, maMaxToolRounds+1)
	for i := 0; i <= maMaxToolRounds; i++ {
		responses = append(responses, openrouter.ChatResponse{
			ID:    "resp-loop",
			Model: "test-model",
			ToolCalls: []openrouter.ToolCall{{
				ID:   "call_ateco",
				Type: "function",
				Function: openrouter.ToolCallFunction{
					Name:      maAtecoToolName,
					Arguments: `{"query":"servizi IT"}`,
				},
			}},
		})
	}
	store := &fakeMAWorkspaceStore{}
	service := newMAService(store, nil, nil, ateco, nil, &fakeMAAI{responses: responses})
	trace, err := service.startTrace(context.Background(), maTraceStart{Operation: "ma_session_create", Request: maTraceJSON(map[string]string{"prompt": "trova aziende servizi IT"})})
	if err != nil {
		t.Fatalf("start trace: %v", err)
	}
	ctx := withMATrace(context.Background(), trace)

	_, _, err = service.draftStrategy(ctx, "trova aziende servizi IT", "", "", "", "")

	if !errors.Is(err, errMAStrategyInvalid) {
		t.Fatalf("err = %v, want errMAStrategyInvalid", err)
	}
	if got := traceEventCount(store.events, "openrouter_chat"); got != maMaxToolRounds+1 {
		t.Fatalf("openrouter events = %d, want %d", got, maMaxToolRounds+1)
	}
	if got := traceEventCount(store.events, "ma_strategy_tool"); got != maMaxToolRounds {
		t.Fatalf("tool events = %d, want %d", got, maMaxToolRounds)
	}
	limitEvent := traceEvent(store.events, "ma_strategy_tool_loop_limit")
	if limitEvent == nil || limitEvent.Status != maTraceEventFailed || !strings.Contains(limitEvent.Error, "ateco tool loop") {
		t.Fatalf("loop limit event = %#v, want failed ateco tool loop", limitEvent)
	}
}

func TestMATraceJSONRedactsOnlyTokenFields(t *testing.T) {
	raw := maTraceJSON(map[string]any{
		"authorization": "Bearer secret",
		"prompt":        "keep this",
		"nested": map[string]any{
			"accessToken": "secret",
			"password":    "keep password field by policy",
		},
	})
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal trace json: %v", err)
	}
	if got["authorization"] != "[redacted]" {
		t.Fatalf("authorization = %#v, want redacted", got["authorization"])
	}
	if got["prompt"] != "keep this" {
		t.Fatalf("prompt = %#v, want preserved", got["prompt"])
	}
	nested, _ := got["nested"].(map[string]any)
	if nested["accessToken"] != "[redacted]" {
		t.Fatalf("accessToken = %#v, want redacted", nested["accessToken"])
	}
	if nested["password"] != "keep password field by policy" {
		t.Fatalf("password = %#v, want preserved", nested["password"])
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
	if params["limit"] != "" {
		t.Fatalf("surface estimate params should not include limit, got %q", params["limit"])
	}
	if estimates[0].ExecutionLimit != 10 {
		t.Fatalf("execution limit = %d, want 10", estimates[0].ExecutionLimit)
	}
	if estimates[0].SurfaceStatus != maEstimateSurfaceExact {
		t.Fatalf("surface status = %q, want exact", estimates[0].SurfaceStatus)
	}
}

func TestMASurfaceProbeExactRunsWithoutLimit(t *testing.T) {
	var seenLimit string
	var seenSkip string
	client := newCompanySearchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		seenLimit = r.URL.Query().Get("limit")
		seenSkip = r.URL.Query().Get("skip")
		if got := r.URL.Query().Get("dryRun"); got != "1" {
			t.Fatalf("dryRun = %q, want 1", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":    []map[string]any{},
			"success": true,
			"message": "ok",
			"error":   nil,
			"count":   42,
			"cost":    0.01,
		})
	})
	service := &maService{
		openapiit: client,
		now:       func() time.Time { return time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC) },
	}
	strategy := validSurfaceStrategy()
	dryRun := 1
	limit := 25
	params := baseMASearchParams(strategy, "MI", &dryRun, limit)

	result, err := service.probeMASearchSurface(context.Background(), params, "", "")
	if err != nil {
		t.Fatalf("probeMASearchSurface returned error: %v", err)
	}
	if seenLimit != "" || seenSkip != "" {
		t.Fatalf("first surface probe query limit=%q skip=%q, want both empty", seenLimit, seenSkip)
	}
	if result.Status != maEstimateSurfaceExact || result.EstimatedCount != 42 || result.ProbeCount != 1 {
		t.Fatalf("surface result = %#v, want exact 42 with one probe", result)
	}
	var storedParams map[string]string
	if err := json.Unmarshal(result.Params, &storedParams); err != nil {
		t.Fatalf("unmarshal surface params: %v", err)
	}
	if _, ok := storedParams["limit"]; ok {
		t.Fatalf("stored surface params should not include limit: %#v", storedParams)
	}
}

func TestMASurfaceProbeExtendsSaturatedFirstWindowWithSkip(t *testing.T) {
	seen := []map[string]string{}
	client := newCompanySearchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		seen = append(seen, map[string]string{
			"limit": query.Get("limit"),
			"skip":  query.Get("skip"),
		})
		count := 1000
		if query.Get("skip") == "1000" {
			count = 37
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":    []map[string]any{},
			"success": true,
			"message": "ok",
			"error":   nil,
			"count":   count,
			"cost":    0.01,
		})
	})
	service := &maService{
		openapiit: client,
		now:       func() time.Time { return time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC) },
	}
	strategy := validSurfaceStrategy()
	dryRun := 1
	result, err := service.probeMASearchSurface(context.Background(), baseMASurfaceParams(strategy, "MI", &dryRun), "", "")
	if err != nil {
		t.Fatalf("probeMASearchSurface returned error: %v", err)
	}
	if len(seen) != 2 {
		t.Fatalf("probe calls = %#v, want two calls", seen)
	}
	if seen[0]["limit"] != "" || seen[0]["skip"] != "" || seen[1]["limit"] != "" || seen[1]["skip"] != "1000" {
		t.Fatalf("probe queries = %#v, want no limit and second skip=1000", seen)
	}
	if result.Status != maEstimateSurfaceExact || result.EstimatedCount != 1037 || result.ProbeCount != 2 {
		t.Fatalf("surface result = %#v, want exact 1037 with two probes", result)
	}
}

func TestMASurfaceProbeStopsAfterTwoSaturatedWindows(t *testing.T) {
	var calls int
	client := newCompanySearchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":    []map[string]any{},
			"success": true,
			"message": "ok",
			"error":   nil,
			"count":   1000,
			"cost":    0.01,
		})
	})
	service := &maService{
		openapiit: client,
		now:       func() time.Time { return time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC) },
	}
	strategy := validSurfaceStrategy()
	dryRun := 1
	result, err := service.probeMASearchSurface(context.Background(), baseMASurfaceParams(strategy, "MI", &dryRun), "", "")
	if err != nil {
		t.Fatalf("probeMASearchSurface returned error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want two", calls)
	}
	if result.Status != maEstimateSurfaceTooBroad || result.EstimatedCount != 2000 || result.ProbeCount != 2 {
		t.Fatalf("surface result = %#v, want too_broad lower bound 2000", result)
	}
}

func TestMACompanySurfaceToolRejectsAtecoOutsideWhitelist(t *testing.T) {
	service := newMAService(&fakeMAWorkspaceStore{}, nil, nil, nil, nil, nil)
	call := openrouter.ToolCall{
		ID:   "surface",
		Type: "function",
		Function: openrouter.ToolCallFunction{
			Name:      maCompanySurfaceToolName,
			Arguments: `{"province":"MI","atecoCode":"62.10.00"}`,
		},
	}
	content := service.executeMACompanySurfaceTool(context.Background(), call, map[string]AtecoCode{}, nil, "", "")
	var got map[string]string
	if err := json.Unmarshal([]byte(content), &got); err != nil {
		t.Fatalf("unmarshal tool response: %v", err)
	}
	if got["error"] != "ateco_not_allowed" {
		t.Fatalf("tool response = %#v, want ateco_not_allowed", got)
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

func TestProvinceRegionToolReturnsRegionsAndWhitelistsProvinces(t *testing.T) {
	cache := newFakeProvinceCache(t, []openapiit.Province{
		{Sigla: "MI", Provincia: "Milano", Regione: "Lombardia"},
		{Sigla: "BG", Provincia: "Bergamo", Regione: "Lombardia"},
		{Sigla: "RM", Provincia: "Roma", Regione: "Lazio"},
	})
	service := newMAService(&fakeMAWorkspaceStore{}, nil, cache, nil, nil, nil)
	allowed := map[string]openapiit.Province{}
	call := openrouter.ToolCall{
		ID:   "province",
		Type: "function",
		Function: openrouter.ToolCallFunction{
			Name:      maProvinceRegionToolName,
			Arguments: `{"region":"lombardia"}`,
		},
	}

	content := service.executeMAProvinceRegionTool(context.Background(), call, allowed)

	var got struct {
		Regions []maRegionToolItem   `json:"regions"`
		Items   []maProvinceToolItem `json:"provinces"`
		Error   string               `json:"error"`
	}
	if err := json.Unmarshal([]byte(content), &got); err != nil {
		t.Fatalf("unmarshal tool response: %v", err)
	}
	if got.Error != "" {
		t.Fatalf("tool returned error %q", got.Error)
	}
	if len(got.Items) != 2 || got.Items[0].Code != "BG" || got.Items[1].Code != "MI" {
		t.Fatalf("provinces = %#v, want BG and MI", got.Items)
	}
	if len(got.Regions) != 1 || got.Regions[0].Name != "Lombardia" || !slices.Equal(got.Regions[0].Provinces, []string{"BG", "MI"}) {
		t.Fatalf("regions = %#v, want Lombardia with BG, MI", got.Regions)
	}
	if _, ok := allowed["BG"]; !ok {
		t.Fatalf("BG was not whitelisted: %#v", allowed)
	}
	if _, ok := allowed["MI"]; !ok {
		t.Fatalf("MI was not whitelisted: %#v", allowed)
	}
	if _, ok := allowed["RM"]; ok {
		t.Fatalf("RM should not be whitelisted by Lombardia filter: %#v", allowed)
	}
}

func TestDraftStrategyUsesProvinceToolWhitelist(t *testing.T) {
	cache := newFakeProvinceCache(t, []openapiit.Province{
		{Sigla: "MI", Provincia: "Milano", Regione: "Lombardia"},
		{Sigla: "BG", Provincia: "Bergamo", Regione: "Lombardia"},
	})
	ai := &fakeMAAI{responses: []openrouter.ChatResponse{
		{
			ToolCalls: []openrouter.ToolCall{
				{
					ID:   "call_province",
					Type: "function",
					Function: openrouter.ToolCallFunction{
						Name:      maProvinceRegionToolName,
						Arguments: `{"query":"Milano"}`,
					},
				},
			},
		},
		{
			Content: `{"strategy":{"sectorDescription":"servizi IT","provinces":["MI"],"activityStatus":"ATTIVA","searchLimit":100,"atecoCandidates":[],"keywords":[],"rationale":"ok","missingCriteria":[]}}`,
		},
	}}
	service := newMAService(&fakeMAWorkspaceStore{}, nil, cache, nil, nil, ai)

	strategy, _, err := service.draftStrategy(context.Background(), "trova aziende servizi IT a Milano", "", "", "", "")
	if err != nil {
		t.Fatalf("draftStrategy returned error: %v", err)
	}
	if !slices.Equal(strategy.Provinces, []string{"MI"}) {
		t.Fatalf("provinces = %#v, want MI", strategy.Provinces)
	}
}

func TestDraftStrategyRejectsProvinceOutsideToolWhitelist(t *testing.T) {
	cache := newFakeProvinceCache(t, []openapiit.Province{
		{Sigla: "MI", Provincia: "Milano", Regione: "Lombardia"},
	})
	ai := &fakeMAAI{responses: []openrouter.ChatResponse{
		{
			ToolCalls: []openrouter.ToolCall{
				{
					ID:   "call_province",
					Type: "function",
					Function: openrouter.ToolCallFunction{
						Name:      maProvinceRegionToolName,
						Arguments: `{"query":"Milano"}`,
					},
				},
			},
		},
		{
			Content: `{"strategy":{"sectorDescription":"servizi IT","provinces":["RM"],"activityStatus":"ATTIVA","searchLimit":100,"atecoCandidates":[],"keywords":[],"rationale":"ok","missingCriteria":[]}}`,
		},
	}}
	service := newMAService(&fakeMAWorkspaceStore{}, nil, cache, nil, nil, ai)

	_, _, err := service.draftStrategy(context.Background(), "trova aziende servizi IT a Milano", "", "", "", "")
	if !errors.Is(err, errMAStrategyInvalid) {
		t.Fatalf("err = %v, want errMAStrategyInvalid", err)
	}
}

func TestMACompanySurfaceToolRejectsProvinceOutsideWhitelist(t *testing.T) {
	service := newMAService(&fakeMAWorkspaceStore{}, nil, nil, nil, nil, nil)
	call := openrouter.ToolCall{
		ID:   "surface",
		Type: "function",
		Function: openrouter.ToolCallFunction{
			Name:      maCompanySurfaceToolName,
			Arguments: `{"province":"MI"}`,
		},
	}
	content := service.executeMACompanySurfaceTool(context.Background(), call, nil, map[string]openapiit.Province{}, "", "")
	var got map[string]string
	if err := json.Unmarshal([]byte(content), &got); err != nil {
		t.Fatalf("unmarshal tool response: %v", err)
	}
	if got["error"] != "province_not_allowed" {
		t.Fatalf("tool response = %#v, want province_not_allowed", got)
	}
}

type fakeAtecoStore struct {
	items     map[string]AtecoCode
	lastQuery string
	lastLimit int
}

func validSurfaceStrategy() MAStrategySpec {
	strategy, err := validateMAStrategy(MAStrategySpec{
		SectorDescription: "servizi IT",
		ActivityStatus:    "ATTIVA",
		SearchLimit:       100,
	})
	if err != nil {
		panic(err)
	}
	return strategy
}

func hasTool(tools []openrouter.Tool, name string) bool {
	for _, tool := range tools {
		if tool.Function.Name == name {
			return true
		}
	}
	return false
}

func traceEventCount(events []maTraceEventWrite, eventType string) int {
	count := 0
	for _, event := range events {
		if event.EventType == eventType {
			count++
		}
	}
	return count
}

func traceEvent(events []maTraceEventWrite, eventType string) *maTraceEventWrite {
	for index := range events {
		if events[index].EventType == eventType {
			return &events[index]
		}
	}
	return nil
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

type fakeMAWorkspaceStore struct {
	traces []maTraceStart
	links  []maTraceLink
	done   []maTraceComplete
	events []maTraceEventWrite
}

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

func (f *fakeMAWorkspaceStore) StartMATrace(_ context.Context, input maTraceStart) (string, error) {
	if input.ID == "" {
		input.ID = "trace-id"
	}
	f.traces = append(f.traces, input)
	return input.ID, nil
}

func (f *fakeMAWorkspaceStore) LinkMATrace(_ context.Context, input maTraceLink) error {
	f.links = append(f.links, input)
	return nil
}

func (f *fakeMAWorkspaceStore) CompleteMATrace(_ context.Context, input maTraceComplete) error {
	f.done = append(f.done, input)
	return nil
}

func (f *fakeMAWorkspaceStore) RecordMATraceEvent(_ context.Context, input maTraceEventWrite) error {
	f.events = append(f.events, input)
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

type fakeProvinceCache struct {
	response json.RawMessage
}

func newFakeProvinceCache(t *testing.T, provinces []openapiit.Province) *fakeProvinceCache {
	t.Helper()
	raw, err := json.Marshal(openapiit.Envelope[[]openapiit.Province]{
		Data:    provinces,
		Success: true,
		Message: "ok",
	})
	if err != nil {
		t.Fatalf("marshal province cache: %v", err)
	}
	return &fakeProvinceCache{response: raw}
}

func (f *fakeProvinceCache) GetValidProvinceCache(context.Context, time.Time) (*provinceCacheEntry, error) {
	return &provinceCacheEntry{Response: append(json.RawMessage(nil), f.response...)}, nil
}

func (f *fakeProvinceCache) WithProvinceCacheLock(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (f *fakeProvinceCache) UpsertProvinceCache(_ context.Context, input provinceCacheWrite) error {
	f.response = append(json.RawMessage(nil), input.Response...)
	return nil
}

var _ atecoStore = (*fakeAtecoStore)(nil)
var _ maAIClient = (*fakeMAAI)(nil)
var _ maWorkspaceStore = (*fakeMAWorkspaceStore)(nil)
var _ provinceCacheStore = (*fakeProvinceCache)(nil)
