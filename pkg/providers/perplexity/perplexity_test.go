package perplexity

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

func TestCreatePerplexityDefaults(t *testing.T) {
	client := CreatePerplexity(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestPerplexityStreamCitations(t *testing.T) {
	t.Setenv("PERPLEXITY_API_KEY", "test-key")
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
		write(`{"id":"chat-1","created":123,"model":"sonar-pro","choices":[{"delta":{"role":"assistant","content":"Hello "}}],"citations":["https://example.com"],"images":[{"image_url":"https://img","origin_url":"https://origin","height":10,"width":20}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3,"citation_tokens":4,"num_search_queries":1}}`)
		write(`{"id":"chat-1","created":123,"model":"sonar-pro","choices":[{"delta":{"role":"assistant","content":"world"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()

	client := CreatePerplexity(Settings{
		BaseURL: server.URL,
		Headers: map[string]string{"X-Custom": "custom"},
	})
	model, err := client.LanguageModel("sonar-pro")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	result, err := model.DoStream(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt: provider.Prompt{Messages: []provider.ModelMessage{{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}}}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	parts := collectParts(result.Stream)

	var source *provider.Source
	var finish *provider.Finish
	var metadata provider.ProviderMetadata
	for _, part := range parts {
		if part.Type == provider.StreamPartTypeSource {
			source = part.Source
		}
		if part.Type == provider.StreamPartTypeFinish {
			finish = part.Finish
			metadata = part.ProviderMetadata
		}
	}
	if source == nil || source.URL != "https://example.com" {
		t.Fatalf("expected source citation, got %#v", source)
	}
	if finish == nil || finish.Reason != provider.FinishReasonStop {
		t.Fatalf("unexpected finish: %#v", finish)
	}
	if finish.Usage == nil || finish.Usage.TotalTokens != 3 {
		t.Fatalf("unexpected usage: %#v", finish.Usage)
	}
	if metadata == nil || metadata[DefaultProviderName] == nil {
		t.Fatalf("missing provider metadata")
	}
	providerMeta := metadata[DefaultProviderName]
	if providerMeta["citation_tokens"] != 4 {
		t.Fatalf("expected citation tokens 4, got %#v", providerMeta["citation_tokens"])
	}
	if providerMeta["num_search_queries"] != 1 {
		t.Fatalf("expected num search queries 1, got %#v", providerMeta["num_search_queries"])
	}
	images, ok := providerMeta["images"].([]map[string]any)
	if !ok || len(images) != 1 {
		t.Fatalf("expected images metadata, got %#v", providerMeta["images"])
	}
}

func TestPerplexityUnsupportedModels(t *testing.T) {
	client := CreatePerplexity(Settings{})
	_, err := client.EmbeddingModel("embed")
	var noSuch *provider.NoSuchModelError
	if err == nil || !errors.As(err, &noSuch) {
		t.Fatalf("expected no such model error, got %v", err)
	}
	_, err = client.ImageModel("image")
	if err == nil || !errors.As(err, &noSuch) {
		t.Fatalf("expected no such model error, got %v", err)
	}
}

func collectParts(stream <-chan provider.StreamPart) []provider.StreamPart {
	var parts []provider.StreamPart
	for part := range stream {
		parts = append(parts, part)
	}
	return parts
}
