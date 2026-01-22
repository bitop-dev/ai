package deepseek

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

func TestCreateDeepSeekDefaults(t *testing.T) {
	deepseekProvider := CreateDeepSeek(Settings{})
	if deepseekProvider.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, deepseekProvider.baseURL)
	}
	if deepseekProvider.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, deepseekProvider.providerID)
	}
}

func TestDeepSeekStreamReasoningAndTools(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("X-Custom") != "custom" {
			t.Fatalf("missing custom header")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		write := func(data string) {
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		write(`{"choices":[{"delta":{"reasoning_content":"Thinking"}}]}`)
		write(`{"choices":[{"delta":{"content":"Hello"}}]}`)
		write(`{"choices":[{"delta":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"weather","arguments":"{\"city\":\""}}]}}]}`)
		write(`{"choices":[{"delta":{"tool_calls":[{"id":"call-1","type":"function","function":{"arguments":"Paris\"}"}}]}}]}`)
		write(`{"choices":[{"delta":{},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()

	client := CreateDeepSeek(Settings{
		BaseURL: server.URL,
		Headers: map[string]string{"X-Custom": "custom"},
	})
	model, err := client.LanguageModel("deepseek-chat")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	prompt := provider.Prompt{
		Messages: []provider.ModelMessage{{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}}},
	}
	result, err := model.DoStream(context.Background(), provider.LanguageModelV3CallOptions{Prompt: prompt})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	parts := collectParts(result.Stream)
	if len(parts) == 0 {
		t.Fatalf("expected stream parts")
	}
	if !hasReasoningStart(parts, "Thinking") {
		t.Fatalf("expected reasoning content")
	}
	toolCall := findToolCall(parts)
	if toolCall == nil || toolCall.Name != "weather" {
		t.Fatalf("unexpected tool call: %#v", toolCall)
	}
	city, ok := toolCall.Arguments["city"].(string)
	if !ok || city != "Paris" {
		t.Fatalf("unexpected tool call arguments: %#v", toolCall.Arguments)
	}
	finish := findFinish(parts)
	if finish == nil || finish.Reason != provider.FinishReasonStop {
		t.Fatalf("unexpected finish: %#v", finish)
	}
}

func collectParts(stream <-chan provider.StreamPart) []provider.StreamPart {
	var parts []provider.StreamPart
	for part := range stream {
		parts = append(parts, part)
	}
	return parts
}

func hasReasoningStart(parts []provider.StreamPart, text string) bool {
	for _, part := range parts {
		if part.ReasoningStart != nil && part.ReasoningStart.Text == text {
			return true
		}
	}
	return false
}

func findToolCall(parts []provider.StreamPart) *provider.ToolCall {
	for _, part := range parts {
		if part.ToolCall != nil {
			return part.ToolCall
		}
	}
	return nil
}

func findFinish(parts []provider.StreamPart) *provider.Finish {
	for _, part := range parts {
		if part.Finish != nil {
			return part.Finish
		}
	}
	return nil
}
