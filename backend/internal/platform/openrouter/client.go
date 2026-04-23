package openrouter

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
	"github.com/openai/openai-go/v3/shared"
)

type Client struct {
	sdk openai.Client
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ResponseFormat struct {
	Type string `json:"type"`
}

type ChatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Content string `json:"content"`
	Usage   Usage  `json:"usage"`
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("openrouter: HTTP %d: %s", e.StatusCode, e.Body)
}

func New(apiKey string) *Client {
	return NewWithBaseURL(apiKey, "", nil)
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
				Content any `json:"content"`
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
		ID:      decoded.ID,
		Model:   decoded.Model,
		Content: content,
		Usage:   decoded.Usage,
	}, nil
}

func buildChatCompletionParams(reqBody ChatRequest) (openai.ChatCompletionNewParams, error) {
	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(reqBody.Model),
		Messages: make([]openai.ChatCompletionMessageParamUnion, 0, len(reqBody.Messages)),
	}
	for _, message := range reqBody.Messages {
		mapped, err := mapMessage(message)
		if err != nil {
			return openai.ChatCompletionNewParams{}, err
		}
		params.Messages = append(params.Messages, mapped)
	}
	if reqBody.Temperature != 0 {
		params.Temperature = openai.Float(reqBody.Temperature)
	}
	if reqBody.MaxTokens > 0 {
		params.MaxTokens = openai.Int(int64(reqBody.MaxTokens))
	}
	if reqBody.ResponseFormat != nil {
		responseFormat, err := mapResponseFormat(*reqBody.ResponseFormat)
		if err != nil {
			return openai.ChatCompletionNewParams{}, err
		}
		params.ResponseFormat = responseFormat
	}
	return params, nil
}

func mapMessage(message Message) (openai.ChatCompletionMessageParamUnion, error) {
	switch strings.TrimSpace(message.Role) {
	case "system":
		return openai.SystemMessage(message.Content), nil
	case "developer":
		return openai.DeveloperMessage(message.Content), nil
	case "user":
		return openai.UserMessage(message.Content), nil
	case "assistant":
		return openai.AssistantMessage(message.Content), nil
	default:
		return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("unsupported role %q", message.Role)
	}
}

func mapResponseFormat(responseFormat ResponseFormat) (openai.ChatCompletionNewParamsResponseFormatUnion, error) {
	switch strings.TrimSpace(responseFormat.Type) {
	case "json_object":
		jsonObject := shared.NewResponseFormatJSONObjectParam()
		return openai.ChatCompletionNewParamsResponseFormatUnion{OfJSONObject: &jsonObject}, nil
	default:
		return openai.ChatCompletionNewParamsResponseFormatUnion{}, fmt.Errorf("unsupported response format %q", responseFormat.Type)
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
