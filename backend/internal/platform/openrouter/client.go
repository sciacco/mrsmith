package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	apiKey  string
	baseURL string
	httpCli *http.Client
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
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		httpCli: httpCli,
	}
}

func (c *Client) Chat(ctx context.Context, reqBody ChatRequest) (ChatResponse, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("openrouter: marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("openrouter: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("openrouter: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("openrouter: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ChatResponse{}, &APIError{StatusCode: resp.StatusCode, Body: string(body)}
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
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ChatResponse{}, fmt.Errorf("openrouter: decode response: %w", err)
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
