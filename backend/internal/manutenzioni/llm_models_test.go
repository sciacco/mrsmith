package manutenzioni

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/sciacco/mrsmith/internal/platform/openrouter"
)

func TestResolveLLMModelUsesRequestedScope(t *testing.T) {
	h := &Handler{maintenance: openLLMModelTestDB(t, "resolve-specific", map[string]string{
		llmModelScopeDefault:         "default-model",
		llmModelScopeAssistanceDraft: "draft-model",
	})}

	model, err := h.resolveLLMModel(context.Background(), llmModelScopeAssistanceDraft)
	if err != nil {
		t.Fatalf("resolveLLMModel returned error: %v", err)
	}
	if model != "draft-model" {
		t.Fatalf("model = %q, want draft-model", model)
	}
}

func TestResolveLLMModelFallsBackToDefault(t *testing.T) {
	h := &Handler{maintenance: openLLMModelTestDB(t, "resolve-fallback", map[string]string{
		llmModelScopeDefault: "default-model",
	})}

	model, err := h.resolveLLMModel(context.Background(), llmModelScopeAssistanceDraft)
	if err != nil {
		t.Fatalf("resolveLLMModel returned error: %v", err)
	}
	if model != "default-model" {
		t.Fatalf("model = %q, want default-model", model)
	}
}

func TestResolveLLMModelErrorsWhenDefaultMissingOrEmpty(t *testing.T) {
	tests := []struct {
		name   string
		models map[string]string
	}{
		{name: "missing", models: map[string]string{}},
		{name: "empty", models: map[string]string{llmModelScopeDefault: "   "}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{maintenance: openLLMModelTestDB(t, "resolve-default-"+tc.name, tc.models)}
			_, err := h.resolveLLMModel(context.Background(), llmModelScopeAssistanceDraft)
			if !errors.Is(err, errLLMModelUnavailable) {
				t.Fatalf("err = %v, want errLLMModelUnavailable", err)
			}
		})
	}
}

func TestHandleCreateLLMModelRejectsInvalidInputAndDuplicate(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		models     map[string]string
		wantStatus int
		wantError  string
	}{
		{
			name:       "invalid scope",
			body:       `{"scope":"Assistance Draft","model":"model-a"}`,
			models:     map[string]string{},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_llm_model_scope",
		},
		{
			name:       "empty model",
			body:       `{"scope":"assistance_draft","model":"   "}`,
			models:     map[string]string{},
			wantStatus: http.StatusBadRequest,
			wantError:  "llm_model_model_required",
		},
		{
			name:       "duplicate scope",
			body:       `{"scope":"default","model":"model-b"}`,
			models:     map[string]string{llmModelScopeDefault: "model-a"},
			wantStatus: http.StatusConflict,
			wantError:  "llm_model_already_exists",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{maintenance: openLLMModelTestDB(t, "create-"+tc.name, tc.models)}
			req := httptest.NewRequest(http.MethodPost, "/manutenzioni/v1/llm-models", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			h.handleCreateLLMModel(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.wantError) {
				t.Fatalf("body = %q, want error %q", rec.Body.String(), tc.wantError)
			}
		})
	}
}

func TestGenerateAssistanceDraftSendsResolvedModelToOpenRouter(t *testing.T) {
	db := openLLMModelTestDB(t, "assistance-chat-model", map[string]string{
		llmModelScopeDefault:         "default-model",
		llmModelScopeAssistanceDraft: "dynamic-draft-model",
	})
	var capturedRequest openrouter.ChatRequest
	var serverMu sync.Mutex
	var serverErr error
	setServerErr := func(err error) {
		serverMu.Lock()
		defer serverMu.Unlock()
		if serverErr == nil {
			serverErr = err
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			setServerErr(errors.New("unexpected path: " + r.URL.Path))
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		var req openrouter.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			setServerErr(err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		capturedRequest = req
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "chatcmpl-test",
			"model": req.Model,
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{
							"texts": {"title_it": "Titolo AI"},
							"service_taxonomy": [],
							"reason_classes": [],
							"impact_effects": [],
							"quality_flags": [],
							"summary": "Bozza generata."
						}`,
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
	}))
	defer server.Close()

	h := &Handler{
		maintenance: db,
		ai:          openrouter.NewWithBaseURL("test-key", server.URL, server.Client()),
	}
	req := httptest.NewRequest(http.MethodPost, "/manutenzioni/v1/maintenances/42/assistance/draft", nil)
	detail := MaintenanceDetail{
		MaintenanceID:   42,
		TitleIT:         "Titolo originale",
		TechnicalDomain: ReferenceItem{ID: 1},
	}

	response, err := h.generateAssistanceDraft(req, detail, assistanceReferenceBundle{}, assistanceDraftRequest{})
	if err != nil {
		t.Fatalf("generateAssistanceDraft returned error: %v", err)
	}
	serverMu.Lock()
	defer serverMu.Unlock()
	if serverErr != nil {
		t.Fatalf("openrouter test server error: %v", serverErr)
	}
	if capturedRequest.Model != "dynamic-draft-model" {
		t.Fatalf("captured model = %q, want dynamic-draft-model", capturedRequest.Model)
	}
	if len(capturedRequest.Messages) != 2 {
		t.Fatalf("captured %d messages, want 2", len(capturedRequest.Messages))
	}
	if capturedRequest.Messages[0].Role != "system" {
		t.Fatalf("first message role = %q, want system", capturedRequest.Messages[0].Role)
	}
	if strings.TrimSpace(capturedRequest.Messages[0].Content) == "" {
		t.Fatalf("first message content should not be empty")
	}
	if capturedRequest.Messages[1].Role != "user" {
		t.Fatalf("second message role = %q, want user", capturedRequest.Messages[1].Role)
	}
	if !strings.Contains(capturedRequest.Messages[1].Content, `"maintenance"`) {
		t.Fatalf("second message content = %q, want maintenance payload", capturedRequest.Messages[1].Content)
	}
	if capturedRequest.Temperature != 0.2 {
		t.Fatalf("temperature = %v, want 0.2", capturedRequest.Temperature)
	}
	if capturedRequest.MaxTokens != 4096 {
		t.Fatalf("max_tokens = %d, want 4096", capturedRequest.MaxTokens)
	}
	if capturedRequest.ResponseFormat == nil {
		t.Fatalf("response_format should be set")
	}
	if capturedRequest.ResponseFormat.Type != "json_object" {
		t.Fatalf("response_format.type = %q, want json_object", capturedRequest.ResponseFormat.Type)
	}
	if response.Audit.Model != "dynamic-draft-model" {
		t.Fatalf("audit model = %q, want dynamic-draft-model", response.Audit.Model)
	}
}

func openLLMModelTestDB(t *testing.T, mode string, models map[string]string) *sql.DB {
	t.Helper()
	registerLLMModelTestDriver()
	state := &llmModelTestState{models: map[string]string{}}
	for scope, model := range models {
		state.models[scope] = model
	}
	llmModelTestStates.Store(mode, state)

	db, err := sql.Open(llmModelTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open llm model test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		llmModelTestStates.Delete(mode)
	})
	return db
}

const llmModelTestDriverName = "manutenzioni_llm_model_test_driver"

var (
	registerLLMModelDriverOnce sync.Once
	llmModelTestStates         sync.Map
)

type llmModelTestState struct {
	mu     sync.Mutex
	models map[string]string
}

func registerLLMModelTestDriver() {
	registerLLMModelDriverOnce.Do(func() {
		sql.Register(llmModelTestDriverName, llmModelTestDriver{})
	})
}

type llmModelTestDriver struct{}

func (llmModelTestDriver) Open(name string) (driver.Conn, error) {
	return &llmModelTestConn{mode: name}, nil
}

type llmModelTestConn struct {
	mode string
}

func (c *llmModelTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *llmModelTestConn) Close() error { return nil }

func (c *llmModelTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *llmModelTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	state := llmModelStateForMode(c.mode)
	state.mu.Lock()
	defer state.mu.Unlock()

	switch {
	case strings.Contains(query, `SELECT model FROM maintenance.llm_model WHERE scope = $1`):
		scope := llmModelNamedString(args, 0)
		model, ok := state.models[scope]
		if !ok {
			return &llmModelRows{columns: []string{"model"}}, nil
		}
		return &llmModelRows{columns: []string{"model"}, values: [][]driver.Value{{model}}}, nil
	case strings.Contains(query, `SELECT scope, model FROM maintenance.llm_model ORDER BY scope`):
		scopes := make([]string, 0, len(state.models))
		for scope := range state.models {
			scopes = append(scopes, scope)
		}
		sort.Strings(scopes)
		values := make([][]driver.Value, 0, len(scopes))
		for _, scope := range scopes {
			values = append(values, []driver.Value{scope, state.models[scope]})
		}
		return &llmModelRows{columns: []string{"scope", "model"}, values: values}, nil
	case strings.Contains(query, `INSERT INTO maintenance.llm_model`):
		scope := llmModelNamedString(args, 0)
		model := llmModelNamedString(args, 1)
		if _, exists := state.models[scope]; exists {
			return nil, errors.New(`pq: duplicate key value violates unique constraint "llm_model_pkey" (SQLSTATE 23505)`)
		}
		state.models[scope] = model
		return &llmModelRows{columns: []string{"scope", "model"}, values: [][]driver.Value{{scope, model}}}, nil
	case strings.Contains(query, `UPDATE maintenance.llm_model`):
		model := llmModelNamedString(args, 0)
		scope := llmModelNamedString(args, 1)
		if _, exists := state.models[scope]; !exists {
			return &llmModelRows{columns: []string{"scope", "model"}}, nil
		}
		state.models[scope] = model
		return &llmModelRows{columns: []string{"scope", "model"}, values: [][]driver.Value{{scope, model}}}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

var _ driver.QueryerContext = (*llmModelTestConn)(nil)

func llmModelStateForMode(mode string) *llmModelTestState {
	if state, ok := llmModelTestStates.Load(mode); ok {
		return state.(*llmModelTestState)
	}
	state := &llmModelTestState{models: map[string]string{}}
	llmModelTestStates.Store(mode, state)
	return state
}

func llmModelNamedString(args []driver.NamedValue, index int) string {
	if len(args) <= index {
		return ""
	}
	if value, ok := args[index].Value.(string); ok {
		return value
	}
	return ""
}

type llmModelRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *llmModelRows) Columns() []string {
	return r.columns
}

func (r *llmModelRows) Close() error { return nil }

func (r *llmModelRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
