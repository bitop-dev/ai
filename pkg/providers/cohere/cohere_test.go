package cohere

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

func TestCreateCohereDefaults(t *testing.T) {
	cohereProvider := CreateCohere(Settings{})
	if cohereProvider.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, cohereProvider.baseURL)
	}
	if cohereProvider.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, cohereProvider.providerID)
	}
}

func TestCohereStreamRequestMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat" {
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
		if payload["model"] != "command" {
			t.Fatalf("unexpected model: %#v", payload["model"])
		}
		if payload["stream"] != true {
			t.Fatalf("missing stream flag: %#v", payload["stream"])
		}
		if payload["max_tokens"] != float64(128) {
			t.Fatalf("missing max tokens: %#v", payload["max_tokens"])
		}
		if payload["temperature"] != 0.7 {
			t.Fatalf("missing temperature: %#v", payload["temperature"])
		}
		if payload["p"] != 0.9 {
			t.Fatalf("missing top p: %#v", payload["p"])
		}
		if payload["k"] != float64(5) {
			t.Fatalf("missing top k: %#v", payload["k"])
		}
		if payload["frequency_penalty"] != 0.2 {
			t.Fatalf("missing frequency penalty: %#v", payload["frequency_penalty"])
		}
		if payload["presence_penalty"] != 0.3 {
			t.Fatalf("missing presence penalty: %#v", payload["presence_penalty"])
		}
		if payload["seed"] != float64(42) {
			t.Fatalf("missing seed: %#v", payload["seed"])
		}
		if payload["tool_choice"] != "REQUIRED" {
			t.Fatalf("missing tool choice: %#v", payload["tool_choice"])
		}
		if payload["preamble"] != "system" {
			t.Fatalf("missing request override: %#v", payload["preamble"])
		}
		stopSequences, ok := payload["stop_sequences"].([]any)
		if !ok || len(stopSequences) != 1 || stopSequences[0] != "END" {
			t.Fatalf("unexpected stop sequences: %#v", payload["stop_sequences"])
		}
		responseFormat := payload["response_format"].(map[string]any)
		if responseFormat["type"] != "json_object" {
			t.Fatalf("unexpected response format: %#v", responseFormat)
		}
		if _, ok := responseFormat["json_schema"]; !ok {
			t.Fatalf("missing json schema")
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
		thinking := payload["thinking"].(map[string]any)
		if thinking["type"] != "enabled" {
			t.Fatalf("unexpected thinking type: %#v", thinking["type"])
		}
		if thinking["token_budget"] != float64(120) {
			t.Fatalf("unexpected token budget: %#v", thinking["token_budget"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", "req-123")
		flusher := w.(http.Flusher)
		write := func(data string) {
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		write(`{"type":"content-start","index":0,"delta":{"message":{"content":{"type":"text","text":"Hello"}}}}`)
		write(`{"type":"content-delta","index":0,"delta":{"message":{"content":{"text":" world"}}}}`)
		write(`{"type":"content-end","index":0}`)
		write(`{"type":"message-end","delta":{"finish_reason":"COMPLETE","usage":{"tokens":{"input_tokens":2,"output_tokens":3}}}}`)
	}))
	defer server.Close()

	client := CreateCohere(Settings{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Headers: map[string]string{"X-Custom": "custom"},
	})
	model, err := client.LanguageModel("command")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	prompt := provider.Prompt{
		Messages: []provider.ModelMessage{{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}}},
	}
	result, err := model.DoStream(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt:           prompt,
		MaxOutputTokens:  128,
		Temperature:      0.7,
		TopP:             0.9,
		TopK:             5,
		FrequencyPenalty: 0.2,
		PresencePenalty:  0.3,
		Seed:             42,
		StopSequences:    []string{"END"},
		ToolChoice:       &provider.ToolChoice{Type: provider.ToolChoiceTypeRequired},
		ResponseFormat: &provider.ResponseFormat{
			Type:        provider.ResponseFormatTypeJSON,
			Schema:      provider.JSONObject{"type": "object"},
			Name:        "payload",
			Description: "payload schema",
		},
		ProviderOptions: provider.ProviderOptions{
			"cohere": provider.JSONObject{
				"thinking": provider.JSONObject{
					"type":        "enabled",
					"tokenBudget": 120,
				},
				"tools": []providerutils.ToolSpecification{
					{Name: "weather", Description: "Weather tool", Parameters: provider.JSONObject{"type": "object"}},
				},
				"request": provider.JSONObject{"preamble": "system"},
			},
		},
		RequestOptions: provider.RequestOptions{Headers: map[string]string{"X-Request": "request"}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	parts := collectParts(result.Stream)
	if len(parts) != 5 {
		t.Fatalf("expected 5 parts, got %d", len(parts))
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
	if parts[3].Type != provider.StreamPartTypeTextEnd {
		t.Fatalf("expected text end, got %s", parts[3].Type)
	}
	finish := parts[4].Finish
	if finish == nil || finish.Reason != provider.FinishReasonStop {
		t.Fatalf("unexpected finish: %#v", finish)
	}
	if finish.Usage == nil || finish.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected usage: %#v", finish.Usage)
	}
	if parts[4].ResponseMetadata == nil || parts[4].ResponseMetadata.RequestID != "req-123" {
		t.Fatalf("missing response metadata")
	}
}

func TestCohereEmbeddingRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embed" {
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
		if payload["input_type"] != "search_document" {
			t.Fatalf("unexpected input type: %#v", payload["input_type"])
		}
		if payload["truncate"] != "END" {
			t.Fatalf("unexpected truncate: %#v", payload["truncate"])
		}
		if payload["user"] != "client" {
			t.Fatalf("missing request override: %#v", payload["user"])
		}
		if values, ok := payload["texts"].([]any); !ok || len(values) != 1 {
			t.Fatalf("unexpected input: %#v", payload["texts"])
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"embeddings":{"float":[]},"meta":{"billed_units":{"input_tokens":1}}}`)
	}))
	defer server.Close()

	client := CreateCohere(Settings{APIKey: "test-key", BaseURL: server.URL})
	model, err := client.EmbeddingModel("embed-model")
	if err != nil {
		t.Fatalf("embedding model: %v", err)
	}
	_, err = model.DoEmbed(context.Background(), provider.EmbeddingModelV3CallOptions{
		Values: []string{"hello"},
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"cohere": provider.JSONObject{
					"inputType": "search_document",
					"truncate":  "END",
					"request":   provider.JSONObject{"user": "client"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
}

func TestCohereErrorMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"nope"}`)
	}))
	defer server.Close()

	client := CreateCohere(Settings{APIKey: "test-key", BaseURL: server.URL})
	model, err := client.LanguageModel("command")
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
