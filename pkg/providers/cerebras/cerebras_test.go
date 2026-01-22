package cerebras

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestCreateCerebrasDefaults(t *testing.T) {
	cerebrasProvider := CreateCerebras(Settings{})
	if cerebrasProvider.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, cerebrasProvider.baseURL)
	}
	if cerebrasProvider.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, cerebrasProvider.providerID)
	}
}

func TestCerebrasLanguageModelPayload(t *testing.T) {
	t.Setenv("CEREBRAS_API_KEY", "test-key")
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
		if payload["model"] != "llama-3.3-70b" {
			t.Fatalf("unexpected model: %#v", payload["model"])
		}
		if payload["max_tokens"] != float64(128) {
			t.Fatalf("unexpected max tokens: %#v", payload["max_tokens"])
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	client := CreateCerebras(Settings{
		BaseURL: server.URL,
		Headers: map[string]string{"X-Custom": "custom"},
	})
	model, err := client.LanguageModel("llama-3.3-70b")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt:          provider.Prompt{Messages: []provider.ModelMessage{{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}}}},
		MaxOutputTokens: 128,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestCerebrasStreamFinish(t *testing.T) {
	t.Setenv("CEREBRAS_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		write := func(data string) {
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		write(`{"choices":[{"delta":{"content":"Hello"}}]}`)
		write(`{"choices":[{"delta":{},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()

	client := CreateCerebras(Settings{BaseURL: server.URL})
	model, err := client.LanguageModel("llama-3.3-70b")
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
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	if parts[2].Finish == nil || parts[2].Finish.Reason != provider.FinishReasonStop {
		t.Fatalf("unexpected finish: %#v", parts[2].Finish)
	}
}

func TestCerebrasUnsupportedModels(t *testing.T) {
	client := CreateCerebras(Settings{})
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
