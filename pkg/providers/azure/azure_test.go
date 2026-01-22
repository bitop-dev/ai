package azure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

func TestCreateAzureDefaults(t *testing.T) {
	azureProvider := CreateAzure(Settings{})
	if azureProvider.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, azureProvider.providerID)
	}
	if azureProvider.apiVersion != DefaultAPIVersion {
		t.Fatalf("expected api version %q, got %q", DefaultAPIVersion, azureProvider.apiVersion)
	}
}

func TestAzureLanguageModelRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("api-version") != "2024-10-01" {
			t.Fatalf("missing api version: %s", r.URL.RawQuery)
		}
		if r.Header.Get("api-key") != "test-key" {
			t.Fatalf("missing api key header")
		}
		if r.Header.Get("X-Custom") != "custom" {
			t.Fatalf("missing custom header")
		}
		if r.Header.Get("X-Request") != "request" {
			t.Fatalf("missing request header")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["model"] != "deploy-1" {
			t.Fatalf("unexpected model: %#v", payload["model"])
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

	client := CreateAzure(Settings{
		APIKey:     "test-key",
		BaseURL:    server.URL + "/openai/",
		APIVersion: "2024-10-01",
		Headers:    map[string]string{"X-Custom": "custom"},
	})
	model, err := client.LanguageModel("deploy-1")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	prompt := provider.Prompt{
		Messages: []provider.ModelMessage{{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}}},
	}
	result, err := model.DoStream(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt:         prompt,
		RequestOptions: provider.RequestOptions{Headers: map[string]string{"X-Request": "request"}},
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

func TestAzureDeploymentBasedURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/deploy-2/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("api-version") != "2024-09-15" {
			t.Fatalf("missing api version: %s", r.URL.RawQuery)
		}
		if r.Header.Get("api-key") != "test-key" {
			t.Fatalf("missing api key header")
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

	client := CreateAzure(Settings{
		APIKey:                 "test-key",
		BaseURL:                server.URL + "/openai",
		APIVersion:             "2024-09-15",
		UseDeploymentBasedURLs: true,
	})
	model, err := client.LanguageModel("deploy-2")
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
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
}

func TestAzureErrorMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ms-request-id", "azure-request")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"nope"}}`)
	}))
	defer server.Close()

	client := CreateAzure(Settings{
		APIKey:  "test-key",
		BaseURL: server.URL + "/openai",
	})
	model, err := client.LanguageModel("deploy-3")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt: provider.Prompt{Messages: []provider.ModelMessage{{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}}}},
	})
	var authErr *provider.AuthenticationError
	if err == nil || !errors.As(err, &authErr) {
		t.Fatalf("expected authentication error, got %v", err)
	}
	if authErr.RequestID != "azure-request" {
		t.Fatalf("expected request ID to be set, got %q", authErr.RequestID)
	}
}

func collectParts(stream <-chan provider.StreamPart) []provider.StreamPart {
	var parts []provider.StreamPart
	for part := range stream {
		parts = append(parts, part)
	}
	return parts
}
