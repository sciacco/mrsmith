package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func TestChatSupportsToolCalls(t *testing.T) {
	var captured ChatRequest
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q, want /chat/completions", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "chatcmpl-tool",
			"model": captured.Model,
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": nil,
						"tool_calls": []map[string]any{
							{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "search_ateco_2025",
									"arguments": `{"query":"servizi IT"}`,
								},
							},
						},
					},
				},
			},
			"usage": map[string]any{"total_tokens": 12},
		})
	})

	client := NewWithBaseURL("test-key", "http://llm.local", httputil.NewMockClient(handler))
	response, err := client.Chat(context.Background(), ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "trova servizi IT"},
		},
		Tools: []Tool{
			{
				Type: "function",
				Function: ToolFunction{
					Name: "search_ateco_2025",
					Parameters: map[string]any{
						"type": "object",
					},
				},
			},
		},
		ToolChoice: "auto",
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if len(captured.Tools) != 1 || captured.Tools[0].Function.Name != "search_ateco_2025" {
		t.Fatalf("captured tools = %#v, want search_ateco_2025", captured.Tools)
	}
	if captured.ToolChoice != "auto" {
		t.Fatalf("tool choice = %#v, want auto", captured.ToolChoice)
	}
	if response.Content != "" {
		t.Fatalf("content = %q, want empty content for tool call", response.Content)
	}
	if len(response.ToolCalls) != 1 {
		t.Fatalf("tool calls = %#v, want one", response.ToolCalls)
	}
	if response.ToolCalls[0].Function.Name != "search_ateco_2025" {
		t.Fatalf("tool call name = %q, want search_ateco_2025", response.ToolCalls[0].Function.Name)
	}
}
