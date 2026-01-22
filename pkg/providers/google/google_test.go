package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providerutils"
)

func TestGoogleGenerateRequest(t *testing.T) {
	t.Setenv("GOOGLE_GENERATIVE_AI_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/gemini-2.5-flash:generateContent" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "test-key" {
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
		contents, ok := payload["contents"].([]any)
		if !ok || len(contents) != 3 {
			t.Fatalf("unexpected contents: %#v", payload["contents"])
		}
		userMessage := contents[0].(map[string]any)
		if userMessage["role"] != "user" {
			t.Fatalf("unexpected user role: %#v", userMessage["role"])
		}
		userParts := userMessage["parts"].([]any)
		if userParts[0].(map[string]any)["text"] != "Hello" {
			t.Fatalf("unexpected user text: %#v", userParts[0])
		}
		assistantMessage := contents[1].(map[string]any)
		if assistantMessage["role"] != "model" {
			t.Fatalf("unexpected assistant role: %#v", assistantMessage["role"])
		}
		assistantParts := assistantMessage["parts"].([]any)
		functionCall := assistantParts[0].(map[string]any)["functionCall"].(map[string]any)
		if functionCall["name"] != "weather" {
			t.Fatalf("unexpected function call: %#v", functionCall)
		}
		args := functionCall["args"].(map[string]any)
		if args["city"] != "LA" {
			t.Fatalf("unexpected function call args: %#v", args)
		}
		toolMessage := contents[2].(map[string]any)
		if toolMessage["role"] != "user" {
			t.Fatalf("unexpected tool role: %#v", toolMessage["role"])
		}
		toolParts := toolMessage["parts"].([]any)
		functionResponse := toolParts[0].(map[string]any)["functionResponse"].(map[string]any)
		if functionResponse["name"] != "weather" {
			t.Fatalf("unexpected function response: %#v", functionResponse)
		}
		if functionResponse["response"].(map[string]any)["temp"] != "72" {
			t.Fatalf("unexpected function response payload: %#v", functionResponse["response"])
		}

		systemInstruction := payload["systemInstruction"].(map[string]any)
		systemParts := systemInstruction["parts"].([]any)
		if systemParts[0].(map[string]any)["text"] != "You are helpful" {
			t.Fatalf("unexpected system instruction: %#v", systemInstruction)
		}

		toolConfig := payload["toolConfig"].(map[string]any)
		functionConfig := toolConfig["functionCallingConfig"].(map[string]any)
		if functionConfig["mode"] != "ANY" {
			t.Fatalf("unexpected tool config: %#v", toolConfig)
		}
		tools := payload["tools"].([]any)
		tool := tools[0].(map[string]any)
		declarations := tool["functionDeclarations"].([]any)
		declaration := declarations[0].(map[string]any)
		if declaration["name"] != "weather" {
			t.Fatalf("unexpected tool declaration: %#v", declaration)
		}

		generationConfig := payload["generationConfig"].(map[string]any)
		if generationConfig["maxOutputTokens"] != float64(128) {
			t.Fatalf("unexpected max tokens: %#v", generationConfig["maxOutputTokens"])
		}
		if generationConfig["responseMimeType"] != "application/json" {
			t.Fatalf("unexpected response mime type: %#v", generationConfig["responseMimeType"])
		}
		if generationConfig["responseSchema"].(map[string]any)["type"] != "object" {
			t.Fatalf("unexpected response schema: %#v", generationConfig["responseSchema"])
		}
		if payload["cachedContent"] != "cachedContents/1" {
			t.Fatalf("unexpected cached content: %#v", payload["cachedContent"])
		}
		if payload["candidateCount"] != float64(2) {
			t.Fatalf("unexpected candidate count: %#v", payload["candidateCount"])
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	client := CreateGoogle(Settings{BaseURL: server.URL, Headers: map[string]string{"X-Custom": "custom"}})
	model, err := client.LanguageModel("gemini-2.5-flash")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	prompt := provider.Prompt{
		Messages: []provider.ModelMessage{
			{Role: provider.RoleSystem, Content: []provider.ContentPart{provider.TextContent{Text: "You are helpful"}}},
			{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
			{Role: provider.RoleAssistant, Content: []provider.ContentPart{provider.ToolCallContent{ToolCall: provider.ToolCall{Name: "weather", Arguments: map[string]any{"city": "LA"}}}}},
			{Role: provider.RoleTool, Content: []provider.ContentPart{provider.ToolResultContent{ToolResult: provider.ToolResult{Name: "weather", Result: map[string]any{"temp": "72"}}}}},
		},
	}
	_, err = model.DoGenerate(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt:          prompt,
		MaxOutputTokens: 128,
		ToolChoice:      &provider.ToolChoice{Type: provider.ToolChoiceTypeRequired},
		ResponseFormat: &provider.ResponseFormat{
			Type:   provider.ResponseFormatTypeJSON,
			Schema: provider.JSONObject{"type": "object"},
		},
		ProviderOptions: provider.ProviderOptions{
			"google": provider.JSONObject{
				"tools":         []providerutils.ToolSpecification{{Name: "weather", Description: "Weather", Parameters: provider.JSONObject{"type": "object"}}},
				"cachedContent": "cachedContents/1",
				"request": provider.JSONObject{
					"candidateCount": 2,
				},
			},
		},
		RequestOptions: provider.RequestOptions{Headers: map[string]string{"X-Request": "request"}},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestGoogleStream(t *testing.T) {
	t.Setenv("GOOGLE_GENERATIVE_AI_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/gemini-2.5-flash:streamGenerateContent" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", "req-123")
		flusher, _ := w.(http.Flusher)

		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello\"}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\" world\"}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"functionCall\":{\"name\":\"weather\",\"args\":{\"city\":\"LA\"}}}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"candidates\":[{\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":2,\"candidatesTokenCount\":3,\"totalTokenCount\":5}}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer server.Close()

	client := CreateGoogle(Settings{BaseURL: server.URL})
	model, err := client.LanguageModel("gemini-2.5-flash")
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
	if len(parts) != 8 {
		t.Fatalf("expected 8 parts, got %d", len(parts))
	}
	if parts[0].Type != provider.StreamPartTypeStreamStart {
		t.Fatalf("expected stream start, got %v", parts[0].Type)
	}
	if parts[1].TextStart == nil || parts[1].TextStart.Text != "Hello" {
		t.Fatalf("unexpected text start: %#v", parts[1].TextStart)
	}
	if parts[2].TextDelta == nil || parts[2].TextDelta.Delta != " world" {
		t.Fatalf("unexpected text delta: %#v", parts[2].TextDelta)
	}
	if parts[3].ToolInputStart == nil || parts[3].ToolInputStart.Name != "weather" {
		t.Fatalf("unexpected tool input start: %#v", parts[3].ToolInputStart)
	}
	if parts[4].ToolInputDelta == nil || !strings.Contains(parts[4].ToolInputDelta.Delta, "city") {
		t.Fatalf("unexpected tool input delta: %#v", parts[4].ToolInputDelta)
	}
	if parts[6].ToolCall == nil || parts[6].ToolCall.Name != "weather" {
		t.Fatalf("unexpected tool call: %#v", parts[6].ToolCall)
	}
	if parts[6].ToolCall.Arguments["city"] != "LA" {
		t.Fatalf("unexpected tool args: %#v", parts[6].ToolCall.Arguments)
	}
	finish := parts[7].Finish
	if finish == nil || finish.Reason != provider.FinishReasonStop {
		t.Fatalf("unexpected finish: %#v", finish)
	}
	if finish.Usage == nil || finish.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected usage: %#v", finish.Usage)
	}
	if parts[7].ResponseMetadata == nil || parts[7].ResponseMetadata.RequestID != "req-123" {
		t.Fatalf("unexpected response metadata: %#v", parts[7].ResponseMetadata)
	}
}

func TestGoogleEmbeddingBatchRequest(t *testing.T) {
	t.Setenv("GOOGLE_GENERATIVE_AI_API_KEY", "embed-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/gemini-embedding-001:batchEmbedContents" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "embed-key" {
			t.Fatalf("missing auth header")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requests, ok := payload["requests"].([]any)
		if !ok || len(requests) != 2 {
			t.Fatalf("unexpected requests: %#v", payload["requests"])
		}
		first := requests[0].(map[string]any)
		content := first["content"].(map[string]any)
		parts := content["parts"].([]any)
		if parts[0].(map[string]any)["text"] != "one" {
			t.Fatalf("unexpected request content: %#v", parts[0])
		}
		if payload["outputDimensionality"] != float64(512) {
			t.Fatalf("unexpected output dimensionality: %#v", payload["outputDimensionality"])
		}
		if payload["taskType"] != "SEMANTIC_SIMILARITY" {
			t.Fatalf("unexpected task type: %#v", payload["taskType"])
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	client := CreateGoogle(Settings{BaseURL: server.URL})
	model, err := client.EmbeddingModel("gemini-embedding-001")
	if err != nil {
		t.Fatalf("embedding model: %v", err)
	}
	_, err = model.DoEmbed(context.Background(), provider.EmbeddingModelV3CallOptions{
		Values: []string{"one", "two"},
		RequestOptions: provider.RequestOptions{ProviderOptions: provider.ProviderOptions{
			"google": provider.JSONObject{
				"outputDimensionality": 512,
				"taskType":             "SEMANTIC_SIMILARITY",
			},
		}},
	})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
}

func collectParts(stream <-chan provider.StreamPart) []provider.StreamPart {
	var parts []provider.StreamPart
	for part := range stream {
		parts = append(parts, part)
	}
	return parts
}
