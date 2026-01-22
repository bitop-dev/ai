package fireworks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestCreateFireworksDefaults(t *testing.T) {
	client := CreateFireworks(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestFireworksStreamMetadata(t *testing.T) {
	t.Setenv("FIREWORKS_API_KEY", "test-key")
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

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		safety, ok := payload["safety_settings"].(map[string]any)
		if !ok || safety["policy"] != "strict" {
			t.Fatalf("expected safety settings, got %#v", payload["safety_settings"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		write := func(data string) {
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		write(`{"choices":[{"delta":{"content":"Hello"}}],"response_metadata":{"latency_ms":12}}`)
		write(`{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2},"safety":{"category":"safe"}}`)
	}))
	defer server.Close()

	client := CreateFireworks(Settings{
		BaseURL: server.URL,
		Headers: map[string]string{"X-Custom": "custom"},
	})
	model, err := client.LanguageModel("accounts/fireworks/models/firefunction-v1")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	result, err := model.DoStream(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt: provider.Prompt{
			Messages: []provider.ModelMessage{{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}}},
		},
		ProviderOptions: provider.ProviderOptions{
			DefaultProviderName: provider.JSONObject{
				"safety_settings": provider.JSONObject{"policy": "strict"},
			},
		},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	parts := collectParts(result.Stream)
	for _, part := range parts {
		if part.Type == provider.StreamPartTypeRaw {
			t.Fatalf("expected raw parts to be filtered")
		}
	}
	finish := parts[len(parts)-1].Finish
	if finish == nil || finish.Usage == nil || finish.Usage.TotalTokens != 2 {
		t.Fatalf("unexpected finish: %#v", finish)
	}
	metadata := parts[len(parts)-1].ProviderMetadata
	if metadata == nil || metadata[DefaultProviderName] == nil {
		t.Fatalf("missing provider metadata")
	}
	responseMeta, ok := metadata[DefaultProviderName]["response_metadata"].(map[string]any)
	if !ok || responseMeta["latency_ms"] != float64(12) {
		t.Fatalf("unexpected response metadata: %#v", metadata[DefaultProviderName]["response_metadata"])
	}
	safety, ok := metadata[DefaultProviderName]["safety"].(map[string]any)
	if !ok || safety["category"] != "safe" {
		t.Fatalf("unexpected safety metadata: %#v", metadata[DefaultProviderName]["safety"])
	}
}

func collectParts(stream <-chan provider.StreamPart) []provider.StreamPart {
	var parts []provider.StreamPart
	for part := range stream {
		parts = append(parts, part)
	}
	return parts
}
