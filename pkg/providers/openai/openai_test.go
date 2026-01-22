package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providerutils"
)

func TestOpenAIChatStream(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("OpenAI-Organization") != "org" {
			t.Fatalf("missing organization header")
		}
		if r.Header.Get("OpenAI-Project") != "proj" {
			t.Fatalf("missing project header")
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
		if payload["model"] != "gpt-4" {
			t.Fatalf("unexpected model: %#v", payload["model"])
		}
		messages, ok := payload["messages"].([]any)
		if !ok || len(messages) != 1 {
			t.Fatalf("unexpected messages: %#v", payload["messages"])
		}
		message := messages[0].(map[string]any)
		if message["content"] != "Hello" {
			t.Fatalf("unexpected message content: %#v", message["content"])
		}
		choice := payload["tool_choice"].(map[string]any)
		function := choice["function"].(map[string]any)
		if function["name"] != "weather" {
			t.Fatalf("unexpected tool choice: %#v", choice)
		}
		tools, ok := payload["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Fatalf("missing tools: %#v", payload["tools"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
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

	client := CreateOpenAI(Settings{
		BaseURL:      server.URL,
		Organization: "org",
		Project:      "proj",
		Headers:      map[string]string{"X-Custom": "custom"},
	})
	model, err := client.LanguageModel("gpt-4")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	prompt := provider.Prompt{
		Messages: []provider.ModelMessage{
			{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
		},
	}
	options := provider.LanguageModelV3CallOptions{
		Prompt:     prompt,
		ToolChoice: &provider.ToolChoice{Type: provider.ToolChoiceTypeTool, ToolName: "weather"},
		ProviderOptions: provider.ProviderOptions{
			"openai": provider.JSONObject{
				"tools": []providerutils.ToolSpecification{
					{Name: "weather", Description: "Weather tool", Parameters: provider.JSONObject{"type": "object"}},
				},
			},
		},
		RequestOptions: provider.RequestOptions{Headers: map[string]string{"X-Request": "request"}},
	}
	result, err := model.DoStream(context.Background(), options)
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
	if parts[3].ResponseMetadata == nil || parts[3].ResponseMetadata.HTTPStatus != http.StatusOK {
		t.Fatalf("missing response metadata")
	}
}

func TestOpenAIResponsesStream(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["input"] != "Hello" {
			t.Fatalf("unexpected input: %#v", payload["input"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		write := func(data string) {
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		write(`{"type":"response.output_text.delta","delta":"Hi"}`)
		write(`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":2}}}`)
	}))
	defer server.Close()

	client := CreateOpenAI(Settings{BaseURL: server.URL})
	model, err := client.LanguageModel("gpt-4o")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	prompt := provider.Prompt{
		Messages: []provider.ModelMessage{{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}}},
	}
	result, err := model.DoStream(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt: prompt,
		ProviderOptions: provider.ProviderOptions{
			"openai": provider.JSONObject{"mode": "responses"},
		},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	parts := collectParts(result.Stream)
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	if parts[1].TextStart == nil || parts[1].TextStart.Text != "Hi" {
		t.Fatalf("unexpected text start: %#v", parts[1].TextStart)
	}
	finish := parts[2].Finish
	if finish == nil || finish.Reason != provider.FinishReasonStop {
		t.Fatalf("unexpected finish: %#v", finish)
	}
	if finish.Usage == nil || finish.Usage.TotalTokens != 3 {
		t.Fatalf("unexpected usage: %#v", finish.Usage)
	}
}

func TestOpenAIChatStreamToolCalls(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		write := func(data string) {
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		write(`{"choices":[{"delta":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"weather","arguments":"{\"city\""}}]}}]}`)
		write(`{"choices":[{"delta":{"tool_calls":[{"id":"call-1","type":"function","function":{"arguments":":\"LA\"}"}}]}}]}`)
		write(`{"choices":[{"delta":{},"finish_reason":"function_call"}]}`)
	}))
	defer server.Close()

	client := CreateOpenAI(Settings{BaseURL: server.URL})
	model, err := client.LanguageModel("gpt-4")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	result, err := model.DoStream(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt: provider.Prompt{Messages: []provider.ModelMessage{{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hi"}}}}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	parts := collectParts(result.Stream)
	if len(parts) != 7 {
		t.Fatalf("expected 7 parts, got %d", len(parts))
	}
	if parts[1].ToolInputStart == nil || parts[1].ToolInputStart.Name != "weather" {
		t.Fatalf("unexpected tool input start: %#v", parts[1].ToolInputStart)
	}
	if parts[2].ToolInputDelta == nil || parts[2].ToolInputDelta.Delta != `{"city"` {
		t.Fatalf("unexpected tool input delta: %#v", parts[2].ToolInputDelta)
	}
	if parts[3].ToolInputDelta == nil || parts[3].ToolInputDelta.Delta != `:"LA"}` {
		t.Fatalf("unexpected tool input delta: %#v", parts[3].ToolInputDelta)
	}
	if parts[5].ToolCall == nil || parts[5].ToolCall.Name != "weather" {
		t.Fatalf("unexpected tool call: %#v", parts[5].ToolCall)
	}
	if parts[6].Finish == nil || parts[6].Finish.Reason != provider.FinishReasonToolCalls {
		t.Fatalf("unexpected finish: %#v", parts[6].Finish)
	}
}

func TestOpenAIEmbeddingsRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer embed-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("X-Embed") != "ok" {
			t.Fatalf("missing embed header")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		inputs := payload["input"].([]any)
		if len(inputs) != 2 || inputs[0] != "one" {
			t.Fatalf("unexpected inputs: %#v", inputs)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"usage":{"prompt_tokens":1,"total_tokens":1}}`)
	}))
	defer server.Close()

	client := CreateOpenAI(Settings{
		BaseURL: server.URL,
		APIKey:  "embed-key",
	})
	model, err := client.EmbeddingModel("text-embedding-3")
	if err != nil {
		t.Fatalf("embedding model: %v", err)
	}
	_, err = model.DoEmbed(context.Background(), provider.EmbeddingModelV3CallOptions{
		Values:         []string{"one", "two"},
		RequestOptions: provider.RequestOptions{Headers: map[string]string{"X-Embed": "ok"}},
	})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
}

func TestOpenAIChatGenerateRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer chat-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("X-Request") != "request" {
			t.Fatalf("missing request header")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["model"] != "gpt-4" {
			t.Fatalf("unexpected model: %#v", payload["model"])
		}
		if payload["max_tokens"] != float64(128) {
			t.Fatalf("unexpected max tokens: %#v", payload["max_tokens"])
		}
		messages, ok := payload["messages"].([]any)
		if !ok || len(messages) != 1 {
			t.Fatalf("unexpected messages: %#v", payload["messages"])
		}
		message := messages[0].(map[string]any)
		if message["content"] != "Hello" {
			t.Fatalf("unexpected message content: %#v", message["content"])
		}
		if payload["tool_choice"] != "required" {
			t.Fatalf("unexpected tool choice: %#v", payload["tool_choice"])
		}
		responseFormat := payload["response_format"].(map[string]any)
		if responseFormat["type"] != "json_schema" {
			t.Fatalf("unexpected response format: %#v", responseFormat)
		}
		tools, ok := payload["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Fatalf("missing tools: %#v", payload["tools"])
		}
		if payload["user"] != "beta" {
			t.Fatalf("missing override field: %#v", payload["user"])
		}
		if payload["metadata"] != "trace" {
			t.Fatalf("missing request override: %#v", payload["metadata"])
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	client := CreateOpenAI(Settings{
		BaseURL: server.URL,
		APIKey:  "chat-key",
	})
	model, err := client.LanguageModel("gpt-4")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt: provider.Prompt{
			Messages: []provider.ModelMessage{{
				Role:    provider.RoleUser,
				Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}},
			}},
		},
		MaxOutputTokens: 128,
		ToolChoice:      &provider.ToolChoice{Type: provider.ToolChoiceTypeRequired},
		ResponseFormat: &provider.ResponseFormat{
			Type:        provider.ResponseFormatTypeJSON,
			Name:        "result",
			Description: "response",
			Schema:      provider.JSONObject{"type": "object"},
		},
		ProviderOptions: provider.ProviderOptions{
			"openai": provider.JSONObject{
				"tools": []providerutils.ToolSpecification{
					{Name: "search", Description: "Search", Parameters: provider.JSONObject{"type": "object"}},
				},
				"user": "beta",
				"request": provider.JSONObject{
					"metadata": "trace",
				},
			},
		},
		RequestOptions: provider.RequestOptions{Headers: map[string]string{"X-Request": "request"}},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestOpenAICompletionsGenerateRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["prompt"] != "Hello\nWorld" {
			t.Fatalf("unexpected prompt: %#v", payload["prompt"])
		}
		if payload["max_tokens"] != float64(42) {
			t.Fatalf("unexpected max tokens: %#v", payload["max_tokens"])
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	client := CreateOpenAI(Settings{BaseURL: server.URL, APIKey: "comp-key"})
	model, err := client.LanguageModel("text-davinci-003")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt: provider.Prompt{
			Messages: []provider.ModelMessage{
				{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
				{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "World"}}},
			},
		},
		MaxOutputTokens: 42,
		ProviderOptions: provider.ProviderOptions{
			"openai": provider.JSONObject{"mode": "completions"},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func collectParts(stream <-chan provider.StreamPart) []provider.StreamPart {
	var parts []provider.StreamPart
	for part := range stream {
		parts = append(parts, part)
	}
	return parts
}
