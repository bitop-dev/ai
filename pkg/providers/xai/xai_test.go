package xai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestCreateXAIDefaults(t *testing.T) {
	xaiProvider := CreateXAI(Settings{})
	if xaiProvider.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, xaiProvider.baseURL)
	}
	if xaiProvider.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, xaiProvider.providerID)
	}
}

func TestXAILanguageModelReasoningMetadata(t *testing.T) {
	t.Setenv("XAI_API_KEY", "test-key")
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
		write(`{"choices":[{"delta":{"reasoning":"Thinking"}}]}`)
		write(`{"choices":[{"delta":{"content":"Hello"}}]}`)
		write(`{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5,"completion_tokens_details":{"reasoning_tokens":7}}}`)
	}))
	defer server.Close()

	client := CreateXAI(Settings{
		BaseURL: server.URL,
		Headers: map[string]string{"X-Custom": "custom"},
	})
	model, err := client.LanguageModel("grok-2")
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
	if len(parts) != 5 {
		t.Fatalf("expected 5 parts, got %d", len(parts))
	}
	if parts[1].ReasoningStart == nil || parts[1].ReasoningStart.Text != "Thinking" {
		t.Fatalf("unexpected reasoning start: %#v", parts[1].ReasoningStart)
	}
	if parts[2].TextStart == nil || parts[2].TextStart.Text != "Hello" {
		t.Fatalf("unexpected text start: %#v", parts[2].TextStart)
	}
	if parts[3].ReasoningEnd == nil {
		t.Fatalf("expected reasoning end")
	}
	finish := parts[4].Finish
	if finish == nil || finish.Usage == nil || finish.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected finish usage: %#v", finish)
	}
	metadata := parts[4].ProviderMetadata
	if metadata == nil || metadata[DefaultProviderName]["reasoning_tokens"] != 7 {
		t.Fatalf("unexpected provider metadata: %#v", metadata)
	}
}

func collectParts(stream <-chan provider.StreamPart) []provider.StreamPart {
	var parts []provider.StreamPart
	for part := range stream {
		parts = append(parts, part)
	}
	return parts
}
