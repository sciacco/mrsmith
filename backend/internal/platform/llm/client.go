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
	sdk openai.Client
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
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
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
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("openrouter: HTTP %d: %s", e.StatusCode, e.Body)
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
	params, err := buildChatCompletionParams(reqBody)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("openrouter: build request: %w", err)
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
			return ChatResponse{}, &APIError{StatusCode: apiErr.StatusCode, Body: errorBody(apiErr)}
		}
		return ChatResponse{}, fmt.Errorf("openrouter: request failed: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("openrouter: missing choices")
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

func buildChatCompletionParams(reqBody ChatRequest) (map[string]any, error) {
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
	if reqBody.Temperature != 0 {
		params["temperature"] = reqBody.Temperature
	}
	if reqBody.MaxTokens > 0 {
		params["max_tokens"] = reqBody.MaxTokens
	}
	if reqBody.ResponseFormat != nil {
		params["response_format"] = *reqBody.ResponseFormat
	}
	if len(reqBody.Tools) > 0 {
		params["tools"] = reqBody.Tools
	}
	if reqBody.ToolChoice != nil {
		params["tool_choice"] = reqBody.ToolChoice
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
			return "", fmt.Errorf("openrouter: empty content array")
		}
		return strings.Join(parts, "\n"), nil
	default:
		return "", fmt.Errorf("openrouter: unsupported content type %T", value)
	}
}
