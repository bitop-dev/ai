package huggingface

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

func TestCreateHuggingFaceDefaults(t *testing.T) {
	client := CreateHuggingFace(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestHuggingFaceLanguageModelUsesEnvKey(t *testing.T) {
	t.Setenv("HUGGINGFACE_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("X-Region") != "global" {
			t.Fatalf("missing custom header")
		}

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

	client := CreateHuggingFace(Settings{
		BaseURL: server.URL,
		Headers: map[string]string{"X-Region": "global"},
	})
	model, err := client.LanguageModel("meta-llama/Llama-3.1-8B-Instruct")
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

func TestHuggingFaceUnsupportedModels(t *testing.T) {
	client := CreateHuggingFace(Settings{})
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
