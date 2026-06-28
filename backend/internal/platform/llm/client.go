package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type Client struct {
	sdk   openai.Client
	label string // provider name for error messages; falls back to "llm"
}

// name returns the provider label used in error messages.
func (c *Client) name() string {
	if c != nil && strings.TrimSpace(c.label) != "" {
		return c.label
	}
	return "llm"
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ResponseFormat struct {
	Type string `json:"type"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	// Params carries ALL provider sampling parameters (temperature, max_tokens,
	// reasoning_effort, top_p, …) as a dynamic bag — sourced verbatim from
	// llm_model.params, or built inline by callers without a registry model. It is
	// merged into the request body by BuildRequestBody, so a new knob needs no Go
	// change. Not a wire field itself: it is flattened into the top-level body.
	Params         map[string]any  `json:"-"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	Tools          []Tool          `json:"tools,omitempty"`
	ToolChoice     any             `json:"tool_choice,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	ID        string     `json:"id"`
	Model     string     `json:"model"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Usage     Usage      `json:"usage"`
}

type APIError struct {
	Provider   string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	provider := strings.TrimSpace(e.Provider)
	if provider == "" {
		provider = "llm"
	}
	return fmt.Sprintf("%s: HTTP %d: %s", provider, e.StatusCode, e.Body)
}

func NewWithBaseURL(apiKey, baseURL string, httpCli *http.Client) *Client {
	if strings.TrimSpace(apiKey) == "" {
		return nil
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	if httpCli == nil {
		httpCli = &http.Client{Timeout: 60 * time.Second}
	}
	return &Client{
		sdk: openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL(strings.TrimRight(baseURL, "/")),
			option.WithHTTPClient(httpCli),
			option.WithMaxRetries(2),
		),
	}
}

func (c *Client) Chat(ctx context.Context, reqBody ChatRequest) (ChatResponse, error) {
	params, err := BuildRequestBody(reqBody)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("%s: build request: %w", c.name(), err)
	}

	var decoded struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content   any        `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}
	if err := c.sdk.Post(ctx, "chat/completions", params, &decoded); err != nil {
		var apiErr *openai.Error
		if errors.As(err, &apiErr) {
			return ChatResponse{}, &APIError{Provider: c.name(), StatusCode: apiErr.StatusCode, Body: errorBody(apiErr)}
		}
		return ChatResponse{}, fmt.Errorf("%s: request failed: %w", c.name(), err)
	}
	if len(decoded.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("%s: missing choices", c.name())
	}

	content, err := flattenContent(decoded.Choices[0].Message.Content)
	if err != nil {
		return ChatResponse{}, err
	}

	return ChatResponse{
		ID:        decoded.ID,
		Model:     decoded.Model,
		Content:   content,
		ToolCalls: decoded.Choices[0].Message.ToolCalls,
		Usage:     decoded.Usage,
	}, nil
}

// Embed produces one embedding vector per input via the provider's
// OpenAI-compatible /embeddings endpoint. Inputs are sent verbatim (the caller
// owns any instruction wrapping, e.g. the Qwen "Instruct: …\nQuery: …" form), and
// vectors are returned in input order. The token usage is returned for cost
// accounting/tracing.
func (c *Client) Embed(ctx context.Context, model string, inputs []string) ([][]float32, Usage, error) {
	if len(inputs) == 0 {
		return nil, Usage{}, nil
	}
	body := map[string]any{
		"model": model,
		"input": inputs,
	}
	var decoded struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Usage Usage `json:"usage"`
	}
	if err := c.sdk.Post(ctx, "embeddings", body, &decoded); err != nil {
		var apiErr *openai.Error
		if errors.As(err, &apiErr) {
			return nil, Usage{}, &APIError{Provider: c.name(), StatusCode: apiErr.StatusCode, Body: errorBody(apiErr)}
		}
		return nil, Usage{}, fmt.Errorf("%s: embeddings request failed: %w", c.name(), err)
	}
	if len(decoded.Data) != len(inputs) {
		return nil, Usage{}, fmt.Errorf("%s: embeddings returned %d vectors for %d inputs", c.name(), len(decoded.Data), len(inputs))
	}
	out := make([][]float32, len(inputs))
	for _, d := range decoded.Data {
		if d.Index < 0 || d.Index >= len(out) {
			return nil, Usage{}, fmt.Errorf("%s: embeddings index %d out of range", c.name(), d.Index)
		}
		out[d.Index] = d.Embedding
	}
	for i, v := range out {
		if len(v) == 0 {
			return nil, Usage{}, fmt.Errorf("%s: embeddings missing vector at index %d", c.name(), i)
		}
	}
	return out, decoded.Usage, nil
}

// Rerank scores each prompt via the provider's /embeddings endpoint in
// return_logits mode: the reranker emits logits for the given "no"/"yes" token
// ids and, with normalize=true, softmaxes them so each data[i].embedding is
// [no_prob, yes_prob]. Returns the yes-probability per prompt, in input order.
func (c *Client) Rerank(ctx context.Context, model string, prompts []string, noTokenID, yesTokenID int) ([]float64, Usage, error) {
	if len(prompts) == 0 {
		return nil, Usage{}, nil
	}
	body := map[string]any{
		"model":         model,
		"input":         prompts,
		"return_logits": []int{noTokenID, yesTokenID},
		"normalize":     true,
	}
	var decoded struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Usage Usage `json:"usage"`
	}
	if err := c.sdk.Post(ctx, "embeddings", body, &decoded); err != nil {
		var apiErr *openai.Error
		if errors.As(err, &apiErr) {
			return nil, Usage{}, &APIError{Provider: c.name(), StatusCode: apiErr.StatusCode, Body: errorBody(apiErr)}
		}
		return nil, Usage{}, fmt.Errorf("%s: rerank request failed: %w", c.name(), err)
	}
	if len(decoded.Data) != len(prompts) {
		return nil, Usage{}, fmt.Errorf("%s: rerank returned %d scores for %d prompts", c.name(), len(decoded.Data), len(prompts))
	}
	out := make([]float64, len(prompts))
	seen := make([]bool, len(prompts))
	for _, d := range decoded.Data {
		if d.Index < 0 || d.Index >= len(out) {
			return nil, Usage{}, fmt.Errorf("%s: rerank index %d out of range", c.name(), d.Index)
		}
		if len(d.Embedding) < 2 {
			return nil, Usage{}, fmt.Errorf("%s: rerank logits at index %d have %d values, want 2", c.name(), d.Index, len(d.Embedding))
		}
		out[d.Index] = d.Embedding[1] // yes-probability
		seen[d.Index] = true
	}
	for i, ok := range seen {
		if !ok {
			return nil, Usage{}, fmt.Errorf("%s: rerank missing score at index %d", c.name(), i)
		}
	}
	return out, decoded.Usage, nil
}

// bodyStructuralKeys are set explicitly from typed ChatRequest fields and must
// never be overridden by the dynamic Params bag.
var bodyStructuralKeys = map[string]struct{}{
	"model": {}, "messages": {}, "response_format": {}, "tools": {}, "tool_choice": {},
}

// BuildRequestBody assembles the chat/completions request body. Structural fields
// come from typed ChatRequest fields; the dynamic Params bag (from llm_model.params)
// is merged on top so DB-configured sampling params (temperature, max_tokens,
// reasoning_effort, …) are forwarded verbatim without a Go change per key. The
// returned map is exactly what goes on the wire, so callers also use it to record a
// faithful audit request.
func BuildRequestBody(reqBody ChatRequest) (map[string]any, error) {
	params := map[string]any{
		"model":    reqBody.Model,
		"messages": make([]map[string]any, 0, len(reqBody.Messages)),
	}
	messages := params["messages"].([]map[string]any)
	for _, message := range reqBody.Messages {
		mapped, err := mapMessage(message)
		if err != nil {
			return nil, err
		}
		messages = append(messages, mapped)
	}
	params["messages"] = messages
	if reqBody.ResponseFormat != nil {
		params["response_format"] = *reqBody.ResponseFormat
	}
	if len(reqBody.Tools) > 0 {
		params["tools"] = reqBody.Tools
	}
	if reqBody.ToolChoice != nil {
		params["tool_choice"] = reqBody.ToolChoice
	}
	// Dynamic params win for sampling keys; structural keys are protected.
	for k, v := range reqBody.Params {
		if _, structural := bodyStructuralKeys[k]; structural {
			continue
		}
		params[k] = v
	}
	return params, nil
}

func mapMessage(message Message) (map[string]any, error) {
	switch strings.TrimSpace(message.Role) {
	case "system", "developer", "user":
		return map[string]any{"role": strings.TrimSpace(message.Role), "content": message.Content}, nil
	case "assistant":
		out := map[string]any{"role": "assistant"}
		if message.Content != "" {
			out["content"] = message.Content
		}
		if len(message.ToolCalls) > 0 {
			out["tool_calls"] = message.ToolCalls
		}
		return out, nil
	case "tool":
		if strings.TrimSpace(message.ToolCallID) == "" {
			return nil, fmt.Errorf("tool message missing tool_call_id")
		}
		return map[string]any{"role": "tool", "tool_call_id": message.ToolCallID, "content": message.Content}, nil
	default:
		return nil, fmt.Errorf("unsupported role %q", message.Role)
	}
}

func errorBody(err *openai.Error) string {
	body := err.RawJSON()
	if body != "" {
		return body
	}
	if err.Response != nil && err.Response.Body != nil {
		if raw, readErr := io.ReadAll(err.Response.Body); readErr == nil && len(raw) > 0 {
			return string(raw)
		}
	}
	return err.Error()
}

func flattenContent(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			text, _ := m["text"].(string)
			if text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) == 0 {
			return "", fmt.Errorf("llm: empty content array")
		}
		return strings.Join(parts, "\n"), nil
	default:
		return "", fmt.Errorf("llm: unsupported content type %T", value)
	}
}
