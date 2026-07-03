package binocolo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/llm"
	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/openapiit"
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
	ai := &fakeMAAI{responses: []llm.ChatResponse{
		{
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_ateco",
					Type: "function",
					Function: llm.ToolCallFunction{
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
	service := newMAService(&fakeMAWorkspaceStore{}, nil, nil, ateco, nil, &fakeMALLMProvider{ai: ai})

	strategy, audit, err := service.draftStrategy(context.Background(), "trova aziende servizi IT", "", "", "", "")
	if err != nil {
		t.Fatalf("draftStrategy returned error: %v", err)
	}
	if len(ai.requests) != 2 {
		t.Fatalf("ai requests = %d, want 2", len(ai.requests))
	}
	// The audit request must mirror the final wire payload across the tool loop:
	// provider model string + the accumulated conversation (system+developer+user,
	// plus the assistant tool-call turn and tool result).
	var auditReq struct {
		Model    string        `json:"model"`
		Messages []llm.Message `json:"messages"`
	}
	if err := json.Unmarshal(audit.Request, &auditReq); err != nil {
		t.Fatalf("audit.Request is not valid JSON: %v (%s)", err, string(audit.Request))
	}
	if auditReq.Model != "test-model" {
		t.Fatalf("audit.Request model = %q, want provider model string %q", auditReq.Model, "test-model")
	}
	if len(auditReq.Messages) < 5 {
		t.Fatalf("audit.Request messages = %d, want the full accumulated tool-loop conversation", len(auditReq.Messages))
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
	ai := &fakeMAAI{responses: []llm.ChatResponse{
		{
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_ateco",
					Type: "function",
					Function: llm.ToolCallFunction{
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
	service := newMAService(&fakeMAWorkspaceStore{}, nil, nil, ateco, nil, &fakeMALLMProvider{ai: ai})

	_, _, err := service.draftStrategy(context.Background(), "trova aziende servizi IT", "", "", "", "")
	if !errors.Is(err, errMAStrategyInvalid) {
		t.Fatalf("err = %v, want errMAStrategyInvalid", err)
	}
}

// TestScoreWebSearchResultsAuditRequest guards the llm_call_audit contract: the
// request snapshot (model_id + messages) must be recorded, not left empty. This
// is the regression that left request = {} in the audit table.
func TestScoreWebSearchResultsAuditRequest(t *testing.T) {
	ai := &fakeMAAI{responses: []llm.ChatResponse{
		{Content: `{"scores":[{"i":0,"score":90},{"i":1,"score":40}]}`},
	}}
	provider := &fakeMALLMProvider{ai: ai}
	service := newMAService(&fakeMAWorkspaceStore{}, nil, nil, nil, nil, provider)

	results := []WebSearchResult{
		{Title: "Data center Milano", URL: "https://x.it/dc", Hostname: "x.it", Snippets: []string{"datacenter tier IV"}},
		{Title: "Chi siamo", URL: "https://x.it/about", Hostname: "x.it", Snippets: []string{"storia azienda"}},
	}
	scores, err := service.scoreWebSearchResults(context.Background(), "data center", results, "sub-1", "user@x.it")
	if err != nil {
		t.Fatalf("scoreWebSearchResults returned error: %v", err)
	}
	if scores[0] != 90 || scores[1] != 40 {
		t.Fatalf("scores = %#v, want {0:90, 1:40}", scores)
	}
	if len(provider.audits) != 1 {
		t.Fatalf("recorded audits = %d, want 1", len(provider.audits))
	}
	audit := provider.audits[0]
	if len(audit.Request) == 0 || string(audit.Request) == "{}" {
		t.Fatalf("audit.Request = %q, want a populated request snapshot", string(audit.Request))
	}
	// request must mirror the wire payload: the provider model string (not our FK)
	// and the messages actually sent.
	var req struct {
		Model    string        `json:"model"`
		Messages []llm.Message `json:"messages"`
	}
	if err := json.Unmarshal(audit.Request, &req); err != nil {
		t.Fatalf("audit.Request is not valid JSON: %v (%s)", err, string(audit.Request))
	}
	if req.Model != "test-model" {
		t.Fatalf("audit.Request model = %q, want provider model string %q", req.Model, "test-model")
	}
	if len(req.Messages) != 2 {
		t.Fatalf("audit.Request messages = %d, want 2 (system+user)", len(req.Messages))
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
	responses := make([]llm.ChatResponse, 0, maMaxToolRounds+1)
	for i := 0; i <= maMaxToolRounds; i++ {
		responses = append(responses, llm.ChatResponse{
			ID:    "resp-loop",
			Model: "test-model",
			ToolCalls: []llm.ToolCall{{
				ID:   "call_ateco",
				Type: "function",
				Function: llm.ToolCallFunction{
					Name:      maAtecoToolName,
					Arguments: `{"query":"servizi IT"}`,
				},
			}},
		})
	}
	store := &fakeMAWorkspaceStore{}
	service := newMAService(store, nil, nil, ateco, nil, &fakeMALLMProvider{ai: &fakeMAAI{responses: responses}})
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

// TestMATraceJSONRedactionPreservesUsageAndCatchesValueSecrets locks the 2026-06-17
// fix: the old substring rule (Contains(key,"token")) destroyed usage/cost fields.
// Token counts and max_tokens must survive as numbers; a credential-shaped VALUE
// under an innocent key must still be caught by the value guard; prose stays.
func TestMATraceJSONRedactionPreservesUsageAndCatchesValueSecrets(t *testing.T) {
	raw := maTraceJSON(map[string]any{
		"max_tokens": 1800,
		"usage": map[string]any{
			"prompt_tokens":     1234,
			"completion_tokens": 56,
			"total_tokens":      1290,
		},
		"echoedHeader": "Bearer sk-or-v1-0123456789abcdef0123456789abcd",
		"prompt":       "Trova aziende a Novara",
	})
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal trace json: %v", err)
	}
	if got["max_tokens"] != float64(1800) {
		t.Errorf("max_tokens = %#v, want 1800 preserved", got["max_tokens"])
	}
	usage, ok := got["usage"].(map[string]any)
	if !ok {
		t.Fatalf("usage = %#v, want object", got["usage"])
	}
	for key, want := range map[string]float64{"prompt_tokens": 1234, "completion_tokens": 56, "total_tokens": 1290} {
		if usage[key] != want {
			t.Errorf("usage[%q] = %#v, want %v preserved", key, usage[key], want)
		}
	}
	if got["echoedHeader"] != "[redacted]" {
		t.Errorf("echoedHeader = %#v, want redacted (Bearer value)", got["echoedHeader"])
	}
	if got["prompt"] != "Trova aziende a Novara" {
		t.Errorf("prompt = %#v, want preserved", got["prompt"])
	}
}

// TestLooksLikeMATraceSecretValue pins the value-shape guard: specific credential
// formats are redacted regardless of key, while audit data (JSON, UUIDs, model
// ids, prose, short codes) is preserved.
func TestLooksLikeMATraceSecretValue(t *testing.T) {
	cases := []struct {
		name   string
		value  string
		secret bool
	}{
		{"bearer header", "Bearer sk-or-v1-0123456789abcdef0123456789abcd", true},
		{"api key sk", "sk-abcdef0123456789ABCDEF", true},
		{"jwt", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", true},
		{"long opaque blob", "Zk8xQ2pLb1BqVnNZd0RmR2hUbk1iUXdFclR5VWlPcEFzRGZHaEpr", true},
		{"prose with spaces", "Sei un analista M&A. Trova aziende a Novara.", false},
		{"compact json", `{"province":"NO","atecoCode":"6201","activityStatus":"ATTIVA"}`, false},
		{"uuid", "550e8400-e29b-41d4-a716-446655440000", false},
		{"model id", "openai/gpt-5.5", false},
		{"ateco code", "62.01", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksLikeMATraceSecretValue(tc.value); got != tc.secret {
				t.Errorf("looksLikeMATraceSecretValue(%q) = %v, want %v", tc.value, got, tc.secret)
			}
		})
	}
}

func TestMAEstimatesSendDotlessAtecoToOpenAPIIT(t *testing.T) {
	var mu sync.Mutex
	upstreamAteco := []string{}
	client := newCompanySearchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// runEstimates now probes the surface concurrently, so this recorder is hit
		// from multiple goroutines.
		mu.Lock()
		upstreamAteco = append(upstreamAteco, r.URL.Query().Get("atecoCode"))
		mu.Unlock()
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
		// runEstimates queries the retrieval set, normally filled by expandStrategyAteco;
		// this test drives runEstimates directly, so it is provided here.
		AtecoQueryCandidates: []MAAtecoCandidate{
			{Code: "62.10.00", SearchCode: "621000", Description: "Attività di programmazione informatica"},
		},
	}, "", "", "")
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
	params := baseMASearchParams(strategy, "MI", "", &dryRun, limit, maEnrichmentAdvanced)

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

func TestMASurfaceProbeTreatsThousandAsExactSingleDryRun(t *testing.T) {
	seen := []map[string]string{}
	client := newCompanySearchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		seen = append(seen, map[string]string{
			"limit": query.Get("limit"),
			"skip":  query.Get("skip"),
		})
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
	result, err := service.probeMASearchSurface(context.Background(), baseMASurfaceParams(strategy, "MI", "", &dryRun), "", "")
	if err != nil {
		t.Fatalf("probeMASearchSurface returned error: %v", err)
	}
	if len(seen) != 1 {
		t.Fatalf("probe calls = %#v, want one call", seen)
	}
	if seen[0]["limit"] != "" || seen[0]["skip"] != "" {
		t.Fatalf("probe queries = %#v, want no limit or skip", seen)
	}
	if result.Status != maEstimateSurfaceExact || result.EstimatedCount != 1000 || result.ProbeCount != 1 {
		t.Fatalf("surface result = %#v, want exact 1000 with one probe", result)
	}
}

func TestMASurfaceProbeMarksAboveVendorLimitTooBroadWithExactCount(t *testing.T) {
	var calls int
	client := newCompanySearchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":    []map[string]any{},
			"success": true,
			"message": "ok",
			"error":   nil,
			"count":   3810,
			"cost":    381,
		})
	})
	service := &maService{
		openapiit: client,
		now:       func() time.Time { return time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC) },
	}
	strategy := validSurfaceStrategy()
	dryRun := 1
	result, err := service.probeMASearchSurface(context.Background(), baseMASurfaceParams(strategy, "MI", "", &dryRun), "", "")
	if err != nil {
		t.Fatalf("probeMASearchSurface returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want one", calls)
	}
	if result.Status != maEstimateSurfaceTooBroad || result.EstimatedCount != 3810 || result.EstimatedCost != 381 || result.ProbeCount != 1 {
		t.Fatalf("surface result = %#v, want too_broad exact count 3810 with one probe", result)
	}
}

func TestMACompanySurfaceToolRejectsAtecoOutsideWhitelist(t *testing.T) {
	service := newMAService(&fakeMAWorkspaceStore{}, nil, nil, nil, nil, nil)
	call := llm.ToolCall{
		ID:   "surface",
		Type: "function",
		Function: llm.ToolCallFunction{
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

func TestMAAtecoChildrenToolReturnsDescendantsAndWhitelists(t *testing.T) {
	ateco := newFakeAtecoStore(testHierarchyAtecoCodes())
	service := newMAService(&fakeMAWorkspaceStore{}, nil, nil, ateco, nil, nil)
	allowed := map[string]AtecoCode{}
	rememberAllowedAteco(allowed, mustResolveFakeAteco(t, ateco, "62"))
	call := llm.ToolCall{
		ID:   "children",
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      maAtecoChildrenToolName,
			Arguments: `{"code":"62"}`,
		},
	}

	content := service.executeMAAtecoChildrenTool(context.Background(), call, allowed)

	var got struct {
		Items []maAtecoHierarchyCode `json:"items"`
		Error string                 `json:"error"`
	}
	if err := json.Unmarshal([]byte(content), &got); err != nil {
		t.Fatalf("unmarshal tool response: %v", err)
	}
	if got.Error != "" {
		t.Fatalf("tool returned error %q", got.Error)
	}
	if len(got.Items) != 6 {
		t.Fatalf("children = %#v, want all 6 descendants below 62", got.Items)
	}
	wantCodes := []string{"62.1", "62.10", "62.10.0", "62.10.00", "62.2", "62.9"}
	for index, want := range wantCodes {
		if got.Items[index].Code != want {
			t.Fatalf("children[%d] = %q, want %q; all children = %#v", index, got.Items[index].Code, want, got.Items)
		}
	}
	if got.Items[0].ChildCount != 1 || got.Items[0].SubtreeCount != 4 {
		t.Fatalf("62.1 counts = child:%d subtree:%d, want child 1 subtree 4", got.Items[0].ChildCount, got.Items[0].SubtreeCount)
	}
	if _, ok := allowed["621"]; !ok {
		t.Fatalf("62.1 was not whitelisted: %#v", allowed)
	}
	if _, ok := allowed["622"]; !ok {
		t.Fatalf("62.2 was not whitelisted: %#v", allowed)
	}
	if _, ok := allowed["621000"]; !ok {
		t.Fatalf("62.10.00 should be whitelisted by the parent subtree call: %#v", allowed)
	}

	call.Function.Arguments = `{"code":"62.1"}`
	content = service.executeMAAtecoChildrenTool(context.Background(), call, allowed)
	got = struct {
		Items []maAtecoHierarchyCode `json:"items"`
		Error string                 `json:"error"`
	}{}
	if err := json.Unmarshal([]byte(content), &got); err != nil {
		t.Fatalf("unmarshal second tool response: %v", err)
	}
	if got.Error != "" {
		t.Fatalf("second tool returned error %q", got.Error)
	}
	if len(got.Items) != 3 || got.Items[0].Code != "62.10" || got.Items[1].Code != "62.10.0" || got.Items[2].Code != "62.10.00" {
		t.Fatalf("62.1 children = %#v, want all descendants 62.10, 62.10.0, 62.10.00", got.Items)
	}
}

func TestMAAtecoChildrenToolRejectsUnseenParent(t *testing.T) {
	ateco := newFakeAtecoStore(testHierarchyAtecoCodes())
	service := newMAService(&fakeMAWorkspaceStore{}, nil, nil, ateco, nil, nil)
	allowed := map[string]AtecoCode{}
	rememberAllowedAteco(allowed, mustResolveFakeAteco(t, ateco, "62"))
	call := llm.ToolCall{
		ID:   "children",
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      maAtecoChildrenToolName,
			Arguments: `{"code":"63"}`,
		},
	}

	content := service.executeMAAtecoChildrenTool(context.Background(), call, allowed)

	var got map[string]string
	if err := json.Unmarshal([]byte(content), &got); err != nil {
		t.Fatalf("unmarshal tool response: %v", err)
	}
	if got["error"] != "code_not_allowed" {
		t.Fatalf("tool response = %#v, want code_not_allowed", got)
	}
}

func TestResolveMAIntentAtecoUsesHierarchyScopeAndToolWhitelist(t *testing.T) {
	ateco := newFakeAtecoStore(testHierarchyAtecoCodes())
	ai := &fakeMAAI{responses: []llm.ChatResponse{
		{
			ToolCalls: []llm.ToolCall{
				{
					ID:   "children-62",
					Type: "function",
					Function: llm.ToolCallFunction{
						Name:      maAtecoChildrenToolName,
						Arguments: `{"code":"62"}`,
					},
				},
			},
		},
		{
			Content: `{"selected":[{"code":"62.10.00","fit":"strong","reason":"foglia programmazione informatica esplorata in un solo subtree call"}],"excludedPrefixes":[],"missingCriteria":[]}`,
		},
	}}
	store := &fakeMAWorkspaceStore{}
	service := newMAService(store, nil, nil, ateco, nil, &fakeMALLMProvider{ai: ai})
	trace, err := service.startTrace(context.Background(), maTraceStart{Operation: "ma_session_create"})
	if err != nil {
		t.Fatalf("start trace: %v", err)
	}
	ctx := withMATrace(context.Background(), trace)
	allowed := map[string]AtecoCode{}
	intent := MAIntent{
		Sectors: MAIntentSectors{
			Include: []MAIntentTextConstraint{{Text: "it", SourceText: "settore it"}},
		},
	}

	candidates, _, _, missing, audits, err := service.resolveMAIntentAteco(ctx, "target settore it", intent, allowed, "", "")

	if err != nil {
		t.Fatalf("resolveMAIntentAteco returned error: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing = %#v, want none", missing)
	}
	if len(candidates) != 1 || candidates[0].Code != "62.10.00" || candidates[0].SearchCode != "" {
		t.Fatalf("candidates = %#v, want selected 62.10.00 before canonicalization", candidates)
	}
	if len(audits) != 1 || audits[0].Scope != maModelScopeStrategyAtecoHierarchy {
		t.Fatalf("audits = %#v, want one hierarchy-scope audit", audits)
	}
	if len(ai.requests) != 2 {
		t.Fatalf("ai requests = %d, want 2", len(ai.requests))
	}
	if !hasTool(ai.requests[0].Tools, maAtecoChildrenToolName) {
		t.Fatalf("first request tools = %#v, want list_ateco_children", ai.requests[0].Tools)
	}
	if hasTool(ai.requests[0].Tools, maAtecoToolName) {
		t.Fatalf("first request tools = %#v, must not expose search_ateco_2025", ai.requests[0].Tools)
	}
	if !strings.Contains(ai.requests[0].Messages[0].Content, "returns every descendant") {
		t.Fatalf("system prompt does not include subtree tool override: %q", ai.requests[0].Messages[0].Content)
	}
	var payload maAtecoHierarchyInput
	if err := json.Unmarshal([]byte(ai.requests[0].Messages[1].Content), &payload); err != nil {
		t.Fatalf("unmarshal hierarchy payload: %v", err)
	}
	if len(payload.Divisions) != 2 || payload.Divisions[0].Code != "62" || payload.Divisions[1].Code != "63" {
		t.Fatalf("payload divisions = %#v, want level-2 divisions 62 and 63", payload.Divisions)
	}
	if ateco.lastQuery != "" {
		t.Fatalf("SearchAteco query = %q, want no lexical retrieval in V2.1", ateco.lastQuery)
	}
	if _, ok := allowed["62"]; !ok {
		t.Fatalf("division 62 was not whitelisted: %#v", allowed)
	}
	if _, ok := allowed["621"]; !ok {
		t.Fatalf("tool child 62.1 was not whitelisted: %#v", allowed)
	}
	if _, ok := allowed["621000"]; !ok {
		t.Fatalf("leaf 62.10.00 should be whitelisted by the parent subtree call: %#v", allowed)
	}
	summary := traceEvent(store.events, "ma_ateco_hierarchy_summary")
	if summary == nil {
		t.Fatalf("missing ma_ateco_hierarchy_summary trace event")
	}
	var metadata struct {
		ChatRoundCount    int `json:"chat_round_count"`
		ToolCallCount     int `json:"tool_call_count"`
		AllowedAtecoCount int `json:"allowed_ateco_count"`
	}
	if err := json.Unmarshal(summary.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal summary metadata: %v", err)
	}
	if metadata.ChatRoundCount != 2 || metadata.ToolCallCount != 1 {
		t.Fatalf("summary metadata = %#v, want 2 chat rounds and 1 tool call", metadata)
	}
	if metadata.AllowedAtecoCount != 8 {
		t.Fatalf("allowed count = %d, want 8", metadata.AllowedAtecoCount)
	}
}

func TestConstrainMAAtecoRerankRejectsUnseenCodes(t *testing.T) {
	allowed := map[string]AtecoCode{}
	rememberAllowedAteco(allowed, AtecoCode{Codice: "62.1", CodiceSearch: "621", Titolo: "Attività di programmazione informatica"})
	rememberAllowedAteco(allowed, AtecoCode{Codice: "95", CodiceSearch: "95", Titolo: "Riparazione e manutenzione di computer"})
	output := maAtecoRerankOutput{
		Selected: []maAtecoRerankSelected{
			{Code: "62.1", Fit: "strong", Reason: "visto"},
			{Code: "63.10", Fit: "strong", Reason: "non visto"},
		},
		ExcludedPrefixes: []maAtecoRerankExcludedPrefix{
			{Prefix: "95", Reason: "esclusione esplicita"},
			{Prefix: "94", Reason: "non visto"},
		},
	}

	candidates, missing := constrainMAAtecoRerank(
		output,
		allowed,
		[]MAIntentTextConstraint{{Text: "it", SourceText: "settore it"}},
		[]MAIntentTextConstraint{{Text: "riparazione computer", SourceText: "esclusa riparazione computer"}},
	)

	if len(candidates) != 2 {
		t.Fatalf("candidates = %#v, want selected 62.1 plus excluded 95", candidates)
	}
	if candidates[0].Code != "62.1" || candidates[0].Fit != maFitCore {
		t.Fatalf("selected candidate = %#v, want canonical 62.1 core", candidates[0])
	}
	if candidates[1].Code != "95" || candidates[1].Fit != maFitExcluded {
		t.Fatalf("excluded candidate = %#v, want canonical 95 excluded", candidates[1])
	}
	joined := strings.Join(missing, "\n")
	if !strings.Contains(joined, "ATECO selezionato fuori dai codici esplorati: 63.10") {
		t.Fatalf("missing = %#v, want unseen selected code", missing)
	}
	if !strings.Contains(joined, "Prefisso ATECO escluso fuori dai codici esplorati: 94") {
		t.Fatalf("missing = %#v, want unseen excluded code", missing)
	}
	if strings.Contains(joined, "Settore testuale non mappato") {
		t.Fatalf("missing = %#v, should not add fallback when a positive selection exists", missing)
	}
}

func TestConstrainMAAtecoRerankFiltersModelMissingBySectorTextOnly(t *testing.T) {
	output := maAtecoRerankOutput{
		MissingCriteria: []maAtecoRerankMissingCriterion{
			{Text: "target in provincia di alessandria", Reason: "criterio geografico non mappabile a codici ATECO per settore IT"},
			{Text: "massimo 50 addetti", Reason: "criterio dimensionale non mappabile a codici ATECO per IT"},
			{Text: "attività non mappabile", Reason: "contiene solo token generico"},
			{Text: "it non mappato", Reason: "settore ambiguo"},
		},
	}

	_, missing := constrainMAAtecoRerank(
		output,
		nil,
		[]MAIntentTextConstraint{{
			Text:       "it",
			SourceText: "target in provincia di alessandria, settore it, massimo 50 addetti",
		}},
		nil,
	)

	if len(missing) != 1 || missing[0] != "it non mappato: settore ambiguo" {
		t.Fatalf("missing = %#v, want only sector-grounded missing", missing)
	}
}

func TestConstrainMAAtecoRerankEmitsFallbackWhenOnlyNonSectorMissingRemain(t *testing.T) {
	output := maAtecoRerankOutput{
		MissingCriteria: []maAtecoRerankMissingCriterion{
			{Text: "target in provincia di alessandria", Reason: "criterio geografico non mappabile a codici ATECO per settore IT"},
			{Text: "massimo 50 addetti", Reason: "criterio dimensionale non mappabile a codici ATECO per IT"},
		},
	}

	_, missing := constrainMAAtecoRerank(
		output,
		nil,
		[]MAIntentTextConstraint{{Text: "it", SourceText: "settore it"}},
		nil,
	)

	if len(missing) != 1 || missing[0] != "Settore testuale non mappato con sicurezza al catalogo ATECO esplorato" {
		t.Fatalf("missing = %#v, want fallback only", missing)
	}
}

func TestMAAtecoMissingGroundingUsesWholeNonNumericTokens(t *testing.T) {
	sectors := []MAIntentTextConstraint{{Text: "it", SourceText: "settore it"}}
	if !maAtecoMissingGroundedInSectors(maAtecoRerankMissingCriterion{Text: "IT non mappato"}, sectors, nil) {
		t.Fatalf("IT token should ground a sector missing")
	}
	for _, text := range []string{"diritto societario", "attivita non mappabile"} {
		if maAtecoMissingGroundedInSectors(maAtecoRerankMissingCriterion{Text: text}, sectors, nil) {
			t.Fatalf("%q should not be grounded by sector token it", text)
		}
	}
	numericSector := []MAIntentTextConstraint{{Text: "50", SourceText: "50"}}
	if maAtecoMissingGroundedInSectors(maAtecoRerankMissingCriterion{Text: "50 addetti"}, numericSector, nil) {
		t.Fatalf("numeric tokens should not ground an ATECO missing")
	}
}

func TestMAStrategyAtecoHierarchyMigrationCoversPipelineTextValueType(t *testing.T) {
	raw, err := os.ReadFile("../../../deploy/migrations/052_anisetta_mrsmith_binocolo_ma_strategy_ateco_hierarchy.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(raw)
	for _, snippet := range []string{
		"CHECK (value_type IN ('money', 'percent', 'number', 'text'))",
		"WHEN value IN ('v2', 'monolith') THEN value",
		"value_type = 'text'",
		"'ma_strategy_ateco_hierarchy'",
		"M&A strategy ATECO hierarchical resolver v1",
		"M&A strategy intent extractor v1 thesis guard",
		"is_default = false",
	} {
		if !strings.Contains(sql, snippet) {
			t.Fatalf("migration missing %q", snippet)
		}
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
	call := llm.ToolCall{
		ID:   "province",
		Type: "function",
		Function: llm.ToolCallFunction{
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

func TestResolveMAIntentTerritoryExcludesTypoProvinceAndIncludesOutOfRegionProvince(t *testing.T) {
	cache := newFakeProvinceCache(t, []openapiit.Province{
		{Sigla: "AV", Provincia: "Avellino", Regione: "Campania"},
		{Sigla: "BN", Provincia: "Benevento", Regione: "Campania"},
		{Sigla: "CE", Provincia: "Caserta", Regione: "Campania"},
		{Sigla: "NA", Provincia: "Napoli", Regione: "Campania"},
		{Sigla: "SA", Provincia: "Salerno", Regione: "Campania"},
		{Sigla: "CS", Provincia: "Cosenza", Regione: "Calabria"},
		{Sigla: "CZ", Provincia: "Catanzaro", Regione: "Calabria"},
		{Sigla: "KR", Provincia: "Crotone", Regione: "Calabria"},
		{Sigla: "RC", Provincia: "Reggio Calabria", Regione: "Calabria"},
		{Sigla: "VV", Provincia: "Vibo Valentia", Regione: "Calabria"},
		{Sigla: "MT", Provincia: "Matera", Regione: "Basilicata"},
	})
	service := newMAService(&fakeMAWorkspaceStore{}, nil, cache, nil, nil, nil)
	allowed := map[string]openapiit.Province{}
	territory := MAIntentTerritory{
		IncludeRegions: []MAIntentTerritoryConstraint{
			{Value: "calabria", SourceText: "calabria"},
			{Value: "campania", SourceText: "campania"},
		},
		IncludeProvinces: []MAIntentTerritoryConstraint{
			{Value: "matera", SourceText: "provincia di matera"},
		},
		ExcludeProvinces: []MAIntentTerritoryConstraint{
			{Value: "vibo valenzia", SourceText: "vibo valenzia"},
			{Value: "avellino", SourceText: "avellino"},
		},
	}

	provinces, label, missing, err := service.resolveMAIntentTerritory(context.Background(), territory, allowed)

	if err != nil {
		t.Fatalf("resolveMAIntentTerritory returned error: %v", err)
	}
	want := []string{"BN", "CE", "CS", "CZ", "KR", "MT", "NA", "RC", "SA"}
	if !slices.Equal(provinces, want) {
		t.Fatalf("provinces = %#v, want %#v", provinces, want)
	}
	if len(missing) != 0 {
		t.Fatalf("missing = %#v, want none", missing)
	}
	if strings.Contains(strings.Join(provinces, ","), "AV") || strings.Contains(strings.Join(provinces, ","), "VV") {
		t.Fatalf("excluded provinces leaked into result: %#v", provinces)
	}
	if _, ok := allowed["MT"]; !ok {
		t.Fatalf("MT was not whitelisted: %#v", allowed)
	}
	if _, ok := allowed["VV"]; ok {
		t.Fatalf("VV should not be whitelisted after exclusion: %#v", allowed)
	}
	if !strings.Contains(label, "Calabria") || !strings.Contains(label, "Campania") || !strings.Contains(label, "MT") || !strings.Contains(label, "VV") || !strings.Contains(label, "AV") {
		t.Fatalf("territory label = %q, want included labels and exclusions", label)
	}
}

func TestDraftStrategyUsesProvinceToolWhitelist(t *testing.T) {
	cache := newFakeProvinceCache(t, []openapiit.Province{
		{Sigla: "MI", Provincia: "Milano", Regione: "Lombardia"},
		{Sigla: "BG", Provincia: "Bergamo", Regione: "Lombardia"},
	})
	ai := &fakeMAAI{responses: []llm.ChatResponse{
		{
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_province",
					Type: "function",
					Function: llm.ToolCallFunction{
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
	service := newMAService(&fakeMAWorkspaceStore{}, nil, cache, nil, nil, &fakeMALLMProvider{ai: ai})

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
	ai := &fakeMAAI{responses: []llm.ChatResponse{
		{
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_province",
					Type: "function",
					Function: llm.ToolCallFunction{
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
	service := newMAService(&fakeMAWorkspaceStore{}, nil, cache, nil, nil, &fakeMALLMProvider{ai: ai})

	_, _, err := service.draftStrategy(context.Background(), "trova aziende servizi IT a Milano", "", "", "", "")
	if !errors.Is(err, errMAStrategyInvalid) {
		t.Fatalf("err = %v, want errMAStrategyInvalid", err)
	}
}

func TestMACompanySurfaceToolRejectsProvinceOutsideWhitelist(t *testing.T) {
	service := newMAService(&fakeMAWorkspaceStore{}, nil, nil, nil, nil, nil)
	call := llm.ToolCall{
		ID:   "surface",
		Type: "function",
		Function: llm.ToolCallFunction{
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

func hasTool(tools []llm.Tool, name string) bool {
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

func testHierarchyAtecoCodes() []AtecoCode {
	return []AtecoCode{
		{Codice: "62", CodiceSearch: "62", Titolo: "Attività di programmazione, consulenza informatica e attività connesse", Gerarchia: testAtecoLevel(2)},
		{Codice: "62.1", CodiceSearch: "621", Titolo: "Attività di programmazione informatica", Gerarchia: testAtecoLevel(3)},
		{Codice: "62.10", CodiceSearch: "6210", Titolo: "Attività di programmazione informatica", Gerarchia: testAtecoLevel(4)},
		{Codice: "62.10.0", CodiceSearch: "62100", Titolo: "Attività di programmazione informatica", Gerarchia: testAtecoLevel(5)},
		{Codice: "62.10.00", CodiceSearch: "621000", Titolo: "Attività di programmazione informatica", Gerarchia: testAtecoLevel(6)},
		{Codice: "62.2", CodiceSearch: "622", Titolo: "Attività di consulenza informatica e di gestione di strutture informatiche", Gerarchia: testAtecoLevel(3)},
		{Codice: "62.9", CodiceSearch: "629", Titolo: "Altre attività dei servizi connessi alle tecnologie dell'informazione e dell'informatica", Gerarchia: testAtecoLevel(3)},
		{Codice: "63", CodiceSearch: "63", Titolo: "Infrastrutture informatiche, elaborazione dati, hosting e altri servizi di informazione", Gerarchia: testAtecoLevel(2)},
		{Codice: "63.1", CodiceSearch: "631", Titolo: "Infrastrutture informatiche, elaborazione dati, hosting e attività connesse", Gerarchia: testAtecoLevel(3)},
		{Codice: "63.10", CodiceSearch: "6310", Titolo: "Infrastrutture informatiche, elaborazione dati, hosting e attività connesse", Gerarchia: testAtecoLevel(4)},
	}
}

func testAtecoLevel(value int) *int {
	out := value
	return &out
}

func mustResolveFakeAteco(t *testing.T, store *fakeAtecoStore, code string) AtecoCode {
	t.Helper()
	item, err := store.ResolveAtecoCode(context.Background(), code)
	if err != nil {
		t.Fatalf("resolve fake ateco %q: %v", code, err)
	}
	return item
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

func (s *fakeAtecoStore) AtecoDivisions(context.Context) ([]AtecoCode, error) {
	out := make([]AtecoCode, 0, len(s.items))
	seen := map[string]struct{}{}
	for _, item := range s.items {
		if item.Gerarchia == nil || *item.Gerarchia != 2 {
			continue
		}
		if _, exists := seen[item.CodiceSearch]; exists {
			continue
		}
		seen[item.CodiceSearch] = struct{}{}
		out = append(out, item)
	}
	slices.SortFunc(out, func(a, b AtecoCode) int {
		return strings.Compare(a.Codice, b.Codice)
	})
	return out, nil
}

func (s *fakeAtecoStore) AtecoChildren(_ context.Context, code string) ([]AtecoHierarchyCode, error) {
	parent, ok := s.items[atecoSearchCode(code)]
	if !ok || parent.Gerarchia == nil {
		return nil, errAtecoCodeNotFound
	}
	parentSearchCode := atecoSearchCode(parent.Codice)
	out := make([]AtecoHierarchyCode, 0)
	seen := map[string]struct{}{}
	for _, item := range s.items {
		if item.Gerarchia == nil {
			continue
		}
		searchCode := atecoSearchCode(item.Codice)
		if searchCode == parentSearchCode || !strings.HasPrefix(searchCode, parentSearchCode) {
			continue
		}
		if _, exists := seen[item.CodiceSearch]; exists {
			continue
		}
		seen[item.CodiceSearch] = struct{}{}
		out = append(out, AtecoHierarchyCode{
			AtecoCode:    item,
			ChildCount:   fakeAtecoChildCount(s.items, item),
			SubtreeCount: fakeAtecoSubtreeCount(s.items, item),
		})
	}
	slices.SortFunc(out, func(a, b AtecoHierarchyCode) int {
		return strings.Compare(a.Codice, b.Codice)
	})
	return out, nil
}

func (s *fakeAtecoStore) SubtreeAtecoCodes(_ context.Context, code string) ([]AtecoCode, error) {
	code = normalizeAtecoCode(code)
	if code == "" {
		return nil, errAtecoCodeNotFound
	}
	out := make([]AtecoCode, 0, len(s.items))
	seen := map[string]struct{}{}
	rootSearchCode := atecoSearchCode(code)
	for _, item := range s.items {
		searchCode := atecoSearchCode(item.Codice)
		if searchCode != rootSearchCode && !strings.HasPrefix(searchCode, rootSearchCode) {
			continue
		}
		if _, exists := seen[item.CodiceSearch]; exists {
			continue
		}
		seen[item.CodiceSearch] = struct{}{}
		out = append(out, item)
	}
	return out, nil
}

func fakeAtecoChildCount(items map[string]AtecoCode, parent AtecoCode) int {
	if parent.Gerarchia == nil {
		return 0
	}
	parentSearchCode := atecoSearchCode(parent.Codice)
	childLevel := *parent.Gerarchia + 1
	seen := map[string]struct{}{}
	for _, item := range items {
		if item.Gerarchia == nil || *item.Gerarchia != childLevel {
			continue
		}
		searchCode := atecoSearchCode(item.Codice)
		if searchCode == parentSearchCode || !strings.HasPrefix(searchCode, parentSearchCode) {
			continue
		}
		seen[item.CodiceSearch] = struct{}{}
	}
	return len(seen)
}

func fakeAtecoSubtreeCount(items map[string]AtecoCode, parent AtecoCode) int {
	parentSearchCode := atecoSearchCode(parent.Codice)
	seen := map[string]struct{}{}
	for _, item := range items {
		searchCode := atecoSearchCode(item.Codice)
		if searchCode != parentSearchCode && !strings.HasPrefix(searchCode, parentSearchCode) {
			continue
		}
		seen[item.CodiceSearch] = struct{}{}
	}
	return len(seen)
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
	responses []llm.ChatResponse
	requests  []llm.ChatRequest
}

func (f *fakeMAAI) Chat(_ context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	f.requests = append(f.requests, req)
	if len(f.responses) == 0 {
		return llm.ChatResponse{}, errors.New("missing fake ai response")
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response, nil
}

// fakeMALLMProvider implements maLLMProvider for tests: canned model/prompt and
// a fakeMAAI as the per-call chat client.
type fakeMALLMProvider struct {
	ai     *fakeMAAI
	audits []llm.CallAudit
}

func (f *fakeMALLMProvider) ResolveModel(_ context.Context, scope, _ string) (llm.Model, error) {
	return llm.Model{ID: "model-id", App: maApp, Scope: scope, ProviderID: "provider-id", Name: "Model", Model: "test-model", IsDefault: true}, nil
}

func (f *fakeMALLMProvider) ResolvePrompt(_ context.Context, scope, _ string) (llm.Prompt, error) {
	return llm.Prompt{ID: "prompt-id", App: maApp, Scope: scope, Name: "Prompt", Prompt: "Rispondi solo con JSON.", IsDefault: true}, nil
}

func (f *fakeMALLMProvider) ClientForModel(context.Context, llm.Model) (maAIClient, error) {
	if f.ai == nil {
		return nil, errors.New("missing fake ai")
	}
	return f.ai, nil
}

func (f *fakeMALLMProvider) RecordAudit(_ context.Context, a llm.CallAudit) error {
	f.audits = append(f.audits, a)
	return nil
}

func (f *fakeMALLMProvider) ListModels(context.Context) ([]llm.Model, error) { return nil, nil }

func (f *fakeMALLMProvider) ListPrompts(context.Context) ([]llm.Prompt, error) { return nil, nil }

func (f *fakeMALLMProvider) ResolveEmbeddingModel(context.Context, string) (llm.EmbeddingModel, error) {
	return llm.EmbeddingModel{}, errors.New("embeddings not configured")
}

func (f *fakeMALLMProvider) Embed(context.Context, llm.EmbeddingModel, []string) ([][]float32, llm.Usage, error) {
	return nil, llm.Usage{}, errors.New("embeddings not configured")
}

func (f *fakeMALLMProvider) ResolveRerankModel(context.Context, string, string) (llm.RerankModel, error) {
	return llm.RerankModel{}, errors.New("rerank not configured")
}

func (f *fakeMALLMProvider) Rerank(context.Context, llm.RerankModel, string, string, []string) ([]float64, llm.Usage, error) {
	return nil, llm.Usage{}, errors.New("rerank not configured")
}

type fakeMAWorkspaceStore struct {
	traces []maTraceStart
	links  []maTraceLink
	done   []maTraceComplete
	events []maTraceEventWrite
}

func (f *fakeMAWorkspaceStore) ListMASessions(context.Context, string) ([]MASessionSummary, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) CreateMASession(context.Context, maSessionCreate) (MASessionDetail, error) {
	return MASessionDetail{}, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) GetMASession(context.Context, string) (MASessionDetail, error) {
	return MASessionDetail{}, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) GetMASessionLean(context.Context, string) (MASessionDetail, error) {
	return MASessionDetail{}, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) ListMARuns(context.Context, string) ([]MAExecutionRun, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) ListMATargetRows(context.Context, string) ([]MATargetRow, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) GetMATargetByID(context.Context, string, string) (MATarget, error) {
	return MATarget{}, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) FindMALatestTargetForCard(context.Context, string, string) (string, string, error) {
	return "", "", errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) UpdateMASessionLifecycle(context.Context, string, string, string, string) (bool, error) {
	return false, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) AddMAStrategyVersion(context.Context, string, MAStrategySpec, string) (*MAStrategyVersion, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) ReplaceMAEstimates(context.Context, string, string, string, []MAEstimate) error {
	return errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) GetMAStrategyVersion(context.Context, string, string) (MAStrategyVersion, error) {
	return MAStrategyVersion{}, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) EnqueueMAJob(context.Context, maJobEnqueue) (string, bool, error) {
	return "", false, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) SetMASessionEstimateStatus(context.Context, string, string, string) error {
	return errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) MarkMASessionExecuting(context.Context, string) error {
	return errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) HasRunningMAExecution(context.Context, string) (bool, error) {
	return false, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) AbandonMASessionExecution(context.Context, string, string) error {
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

func (f *fakeMAWorkspaceStore) MarkMATargetAdvancedEnriched(context.Context, string, json.RawMessage) error {
	return nil
}

func (f *fakeMAWorkspaceStore) InsertMATargetOutcome(context.Context, MATargetOutcome) error {
	return nil
}

func (f *fakeMAWorkspaceStore) GetMACompanyDomain(context.Context, string, string, string) (*maCompanyDomain, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) UpsertMACompanyDomain(context.Context, maCompanyDomain) error {
	return nil
}

func (f *fakeMAWorkspaceStore) AddMARescoreStrategyVersion(ctx context.Context, sessionID string, strategy MAStrategySpec, createdByEmail string) (*MAStrategyVersion, error) {
	return f.AddMAStrategyVersion(ctx, sessionID, strategy, createdByEmail)
}

func (f *fakeMAWorkspaceStore) UpsertMATargetRating(context.Context, string, MATargetRatingRequest, string, string) error {
	return nil
}

func (f *fakeMAWorkspaceStore) UpsertMAWebValidation(context.Context, maWebValidationUpsert) (MAWebValidation, error) {
	return MAWebValidation{}, nil
}

func (f *fakeMAWorkspaceStore) UpsertMASectorEvalLabel(context.Context, string, string, string, string, string, string) error {
	return nil
}

func (f *fakeMAWorkspaceStore) ListMASectorEvalLabels(context.Context, string) (map[string]MASectorEvalLabel, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) GetMASessionState(context.Context, string) (MASession, error) {
	return MASession{}, nil
}

func (f *fakeMAWorkspaceStore) ListMAParameters(context.Context) ([]MAParameter, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) UpdateMAParameter(context.Context, string, string, string) error {
	return nil
}

func (f *fakeMAWorkspaceStore) ListMACompanyLegalForms(context.Context) ([]maCompanyLegalForm, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) ListMADeepAnalysis(context.Context, []string) (map[string]MADeepAnalysis, error) {
	return map[string]MADeepAnalysis{}, nil
}

func (f *fakeMAWorkspaceStore) EnqueueMADeepAnalysis(context.Context, string, string, string, string) error {
	return nil
}

func (f *fakeMAWorkspaceStore) ListMADeepReadyPayloads(context.Context) ([]maDeepPayloadRow, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) CountMADeepByStatus(context.Context) (map[string]int, error) {
	return map[string]int{}, nil
}

func (f *fakeMAWorkspaceStore) UpdateMADeepValuation(context.Context, string, *MADeepValuation) error {
	return nil
}

func (f *fakeMAWorkspaceStore) ResolveSectorMultipleKeys(context.Context, []string) (*sectorMultiple, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) UpsertMABMFamilySuggestion(context.Context, string, string, string, string, maBMFamilySuggestion) error {
	return nil
}

func (f *fakeMAWorkspaceStore) GetMABMFamily(context.Context, string) (*MABMFamily, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) RatifyMABMFamily(context.Context, string, string, string, string) error {
	return nil
}

func (f *fakeMAWorkspaceStore) CountMADeepVintage(context.Context) (int, int, error) {
	return 0, 0, nil
}

func (f *fakeMAWorkspaceStore) UpdateMADeepScorecard(context.Context, string, *MADeepScorecard) error {
	return nil
}

func (f *fakeMAWorkspaceStore) GetMADeepByVAT(context.Context, string) (*maDeepVATRecord, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) ListMADeepReadyForBrief(context.Context) ([]maDeepBriefRow, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) UpdateMADeepBrief(context.Context, string, *MADeepBrief, string, string) error {
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

func (f *fakeMAWorkspaceStore) RecordMAExport(context.Context, string, string, int, string) error {
	return errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) CreateMAInitiative(context.Context, MAInitiative) (MAInitiative, error) {
	return MAInitiative{}, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) GetMAInitiative(context.Context, string) (MAInitiative, error) {
	return MAInitiative{}, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) ListMAInitiatives(context.Context, bool) ([]MAInitiativeSummary, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) UpdateMAInitiativeLifecycle(context.Context, string, string, string, string) (bool, error) {
	return false, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) SetMASessionInitiative(context.Context, string, string) error {
	return errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) GetMAInitiativeCard(context.Context, string, string) (*MAInitiativeCard, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) UpsertMAInitiativeCard(context.Context, MAInitiativeCard) error {
	return nil
}

func (f *fakeMAWorkspaceStore) ListMAInitiativeCards(context.Context, string) ([]MAInitiativeCard, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) ListMAActiveCardsByCompany(context.Context, []string) (map[string][]MAInitiativeCard, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) ListMARatings(context.Context, string) (map[string]int, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) InsertMACompanyFact(context.Context, MACompanyFact) (MACompanyFact, error) {
	return MACompanyFact{}, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) RevokeMACompanyFact(context.Context, string, string, string, string) (bool, error) {
	return false, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) InsertMACompanyNote(context.Context, MACompanyNote) (MACompanyNote, error) {
	return MACompanyNote{}, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) GetMACompanyRegistry(context.Context, string) (MACompanyRegistry, error) {
	return MACompanyRegistry{}, nil
}

func (f *fakeMAWorkspaceStore) ListMACompanyFactsActive(context.Context, []string) (map[string][]string, error) {
	return nil, nil
}

func (f *fakeMAWorkspaceStore) ListMASessionsByInitiative(context.Context, string) ([]MASessionSummary, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) ListMACardProvenances(context.Context, string, []string) (map[string][]MACardProvenance, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeMAWorkspaceStore) ListMAInitiativeCardEvents(context.Context, string, []string, string) ([]MATargetOutcome, error) {
	return nil, errors.New("not implemented")
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
