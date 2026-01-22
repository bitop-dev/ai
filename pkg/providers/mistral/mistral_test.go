package mistral

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providerutils"
)

func TestCreateMistralDefaults(t *testing.T) {
	mistralProvider := CreateMistral(Settings{})
	if mistralProvider.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, mistralProvider.baseURL)
	}
	if mistralProvider.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, mistralProvider.providerID)
	}
}

func TestMistralStreamRequestMapping(t *testing.T) {
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
		if r.Header.Get("X-Request") != "request" {
			t.Fatalf("missing request header")
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["model"] != "mistral-large" {
			t.Fatalf("unexpected model: %#v", payload["model"])
		}
		if payload["safe_prompt"] != true {
			t.Fatalf("missing safe_prompt: %#v", payload["safe_prompt"])
		}
		if payload["parallel_tool_calls"] != false {
			t.Fatalf("missing parallel_tool_calls: %#v", payload["parallel_tool_calls"])
		}
		if payload["user"] != "client" {
			t.Fatalf("missing request override: %#v", payload["user"])
		}
		tools, ok := payload["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Fatalf("missing tools: %#v", payload["tools"])
		}
		tool := tools[0].(map[string]any)
		if tool["type"] != "function" {
			t.Fatalf("unexpected tool type: %#v", tool["type"])
		}
		function := tool["function"].(map[string]any)
		if function["name"] != "weather" {
			t.Fatalf("unexpected tool name: %#v", function["name"])
		}
		responseFormat := payload["response_format"].(map[string]any)
		if responseFormat["type"] != "json_schema" {
			t.Fatalf("unexpected response format: %#v", responseFormat)
		}
		jsonSchema := responseFormat["json_schema"].(map[string]any)
		if jsonSchema["name"] != "payload" {
			t.Fatalf("unexpected schema name: %#v", jsonSchema["name"])
		}
		if jsonSchema["strict"] != true {
			t.Fatalf("unexpected strict schema: %#v", jsonSchema["strict"])
		}
		if payload["tool_choice"] != "any" {
			t.Fatalf("unexpected tool choice: %#v", payload["tool_choice"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", "req-123")
		flusher := w.(http.Flusher)
		write := func(data string) {
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		write(`{"choices":[{"delta":{"content":"Hello"}}]}`)
		write(`{"choices":[{"delta":{"content":" world"}}]}`)
		write(`{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`)
	}))
	defer server.Close()

	client := CreateMistral(Settings{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Headers: map[string]string{"X-Custom": "custom"},
	})
	model, err := client.LanguageModel("mistral-large")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	prompt := provider.Prompt{
		Messages: []provider.ModelMessage{{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}}},
	}
	result, err := model.DoStream(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt:     prompt,
		ToolChoice: &provider.ToolChoice{Type: provider.ToolChoiceTypeRequired},
		ResponseFormat: &provider.ResponseFormat{
			Type:        provider.ResponseFormatTypeJSON,
			Schema:      provider.JSONObject{"type": "object"},
			Name:        "payload",
			Description: "payload schema",
		},
		ProviderOptions: provider.ProviderOptions{
			"mistral": provider.JSONObject{
				"safePrompt":        true,
				"parallelToolCalls": false,
				"strictJsonSchema":  true,
				"tools": []providerutils.ToolSpecification{
					{Name: "weather", Description: "Weather tool", Parameters: provider.JSONObject{"type": "object"}},
				},
				"request": provider.JSONObject{"user": "client"},
			},
		},
		RequestOptions: provider.RequestOptions{Headers: map[string]string{"X-Request": "request"}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	parts := collectParts(result.Stream)
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(parts))
	}
	if parts[0].Type != provider.StreamPartTypeStreamStart {
		t.Fatalf("expected stream start, got %s", parts[0].Type)
	}
	if parts[1].TextStart == nil || parts[1].TextStart.Text != "Hello" {
		t.Fatalf("unexpected text start: %#v", parts[1].TextStart)
	}
	if parts[2].TextDelta == nil || parts[2].TextDelta.Delta != " world" {
		t.Fatalf("unexpected text delta: %#v", parts[2].TextDelta)
	}
	finish := parts[3].Finish
	if finish == nil || finish.Reason != provider.FinishReasonStop {
		t.Fatalf("unexpected finish: %#v", finish)
	}
	if finish.Usage == nil || finish.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected usage: %#v", finish.Usage)
	}
	if parts[3].ResponseMetadata == nil || parts[3].ResponseMetadata.RequestID != "req-123" {
		t.Fatalf("missing response metadata")
	}
}

func TestMistralEmbeddingRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing auth header")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["model"] != "embed-model" {
			t.Fatalf("unexpected model: %#v", payload["model"])
		}
		if payload["user"] != "client" {
			t.Fatalf("missing request override: %#v", payload["user"])
		}
		if values, ok := payload["input"].([]any); !ok || len(values) != 1 {
			t.Fatalf("unexpected input: %#v", payload["input"])
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()

	client := CreateMistral(Settings{APIKey: "test-key", BaseURL: server.URL})
	model, err := client.EmbeddingModel("embed-model")
	if err != nil {
		t.Fatalf("embedding model: %v", err)
	}
	_, err = model.DoEmbed(context.Background(), provider.EmbeddingModelV3CallOptions{
		Values: []string{"hello"},
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"mistral": provider.JSONObject{"request": provider.JSONObject{"user": "client"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
}

func TestMistralErrorMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"nope"}`)
	}))
	defer server.Close()

	client := CreateMistral(Settings{APIKey: "test-key", BaseURL: server.URL})
	model, err := client.LanguageModel("mistral-large")
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
}

func collectParts(stream <-chan provider.StreamPart) []provider.StreamPart {
	var parts []provider.StreamPart
	for part := range stream {
		parts = append(parts, part)
	}
	return parts
}
