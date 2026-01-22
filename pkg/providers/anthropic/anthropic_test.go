package anthropic

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

func TestAnthropicGenerateRequest(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Fatalf("missing version header")
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
		if payload["model"] != "claude-3" {
			t.Fatalf("unexpected model: %#v", payload["model"])
		}
		if payload["max_tokens"] != float64(128) {
			t.Fatalf("unexpected max tokens: %#v", payload["max_tokens"])
		}
		if payload["system"] != "You are helpful" {
			t.Fatalf("unexpected system: %#v", payload["system"])
		}
		messages, ok := payload["messages"].([]any)
		if !ok || len(messages) != 3 {
			t.Fatalf("unexpected messages: %#v", payload["messages"])
		}
		userMessage := messages[0].(map[string]any)
		if userMessage["role"] != "user" {
			t.Fatalf("unexpected user role: %#v", userMessage["role"])
		}
		userContent := userMessage["content"].([]any)
		if userContent[0].(map[string]any)["text"] != "Hello" {
			t.Fatalf("unexpected user text: %#v", userContent[0])
		}

		assistantMessage := messages[1].(map[string]any)
		if assistantMessage["role"] != "assistant" {
			t.Fatalf("unexpected assistant role: %#v", assistantMessage["role"])
		}
		assistantContent := assistantMessage["content"].([]any)
		toolUse := assistantContent[0].(map[string]any)
		if toolUse["type"] != "tool_use" || toolUse["name"] != "weather" {
			t.Fatalf("unexpected tool use: %#v", toolUse)
		}
		input := toolUse["input"].(map[string]any)
		if input["city"] != "LA" {
			t.Fatalf("unexpected tool input: %#v", input)
		}

		toolMessage := messages[2].(map[string]any)
		if toolMessage["role"] != "user" {
			t.Fatalf("unexpected tool role: %#v", toolMessage["role"])
		}
		toolContent := toolMessage["content"].([]any)
		toolResult := toolContent[0].(map[string]any)
		if toolResult["type"] != "tool_result" || toolResult["tool_use_id"] != "call-1" {
			t.Fatalf("unexpected tool result: %#v", toolResult)
		}
		resultPayload := toolResult["content"].(map[string]any)
		if resultPayload["temp"] != "72" {
			t.Fatalf("unexpected tool result content: %#v", resultPayload)
		}

		toolChoice := payload["tool_choice"].(map[string]any)
		if toolChoice["type"] != "any" {
			t.Fatalf("unexpected tool choice: %#v", toolChoice)
		}
		tools, ok := payload["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Fatalf("missing tools: %#v", payload["tools"])
		}
		tool := tools[0].(map[string]any)
		if tool["name"] != "weather" {
			t.Fatalf("unexpected tool: %#v", tool)
		}
		outputFormat := payload["output_format"].(map[string]any)
		if outputFormat["type"] != "json_schema" {
			t.Fatalf("unexpected output format: %#v", outputFormat)
		}
		if payload["metadata"] != "trace" {
			t.Fatalf("missing request override: %#v", payload["metadata"])
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	client := CreateAnthropic(Settings{BaseURL: server.URL, Headers: map[string]string{"X-Custom": "custom"}})
	model, err := client.LanguageModel("claude-3")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	prompt := provider.Prompt{
		Messages: []provider.ModelMessage{
			{Role: provider.RoleSystem, Content: []provider.ContentPart{provider.TextContent{Text: "You are helpful"}}},
			{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
			{Role: provider.RoleAssistant, Content: []provider.ContentPart{provider.ToolCallContent{ToolCall: provider.ToolCall{ID: "call-1", Name: "weather", Arguments: map[string]any{"city": "LA"}}}}},
			{Role: provider.RoleTool, Content: []provider.ContentPart{provider.ToolResultContent{ToolResult: provider.ToolResult{ID: "call-1", Name: "weather", Result: map[string]any{"temp": "72"}}}}},
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
			"anthropic": provider.JSONObject{
				"tools": []providerutils.ToolSpecification{{Name: "weather", Description: "Weather", Parameters: provider.JSONObject{"type": "object"}}},
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

func TestAnthropicStream(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["stream"] != true {
			t.Fatalf("expected stream=true, got %#v", payload["stream"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", "req-123")
		flusher, _ := w.(http.Flusher)

		fmt.Fprint(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":3}}}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"Consider\"}}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_stop\",\"index\":1}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_start\",\"index\":2,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"weather\",\"input\":{}}}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"index\":2,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"city\\\":\\\"LA\\\"}\"}}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_stop\",\"index\":2}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":4}}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"message_stop\"}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer server.Close()

	client := CreateAnthropic(Settings{BaseURL: server.URL})
	model, err := client.LanguageModel("claude-3")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	result, err := model.DoStream(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt: provider.Prompt{Messages: []provider.ModelMessage{{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hi"}}}}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var parts []provider.StreamPart
	for part := range result.Stream {
		parts = append(parts, part)
	}
	if len(parts) < 10 {
		t.Fatalf("expected stream parts, got %d", len(parts))
	}
	if parts[0].Type != provider.StreamPartTypeStreamStart {
		t.Fatalf("expected stream start, got %v", parts[0].Type)
	}
	if parts[1].Type != provider.StreamPartTypeTextStart {
		t.Fatalf("expected text start, got %v", parts[1].Type)
	}
	if parts[2].Type != provider.StreamPartTypeTextDelta || parts[2].TextDelta == nil || parts[2].TextDelta.Delta != "Hello" {
		t.Fatalf("unexpected text delta: %#v", parts[2])
	}
	if parts[3].Type != provider.StreamPartTypeReasoningStart {
		t.Fatalf("expected reasoning start, got %v", parts[3].Type)
	}
	if parts[4].Type != provider.StreamPartTypeReasoningDelta || parts[4].ReasoningDelta == nil || parts[4].ReasoningDelta.Delta != "Consider" {
		t.Fatalf("unexpected reasoning delta: %#v", parts[4])
	}
	if parts[5].Type != provider.StreamPartTypeToolInputStart {
		t.Fatalf("expected tool input start, got %v", parts[5].Type)
	}
	if parts[6].Type != provider.StreamPartTypeToolInputDelta || parts[6].ToolInputDelta == nil {
		t.Fatalf("unexpected tool input delta: %#v", parts[6])
	}
	if parts[7].Type != provider.StreamPartTypeToolInputEnd {
		t.Fatalf("expected tool input end, got %v", parts[7].Type)
	}
	if parts[8].Type != provider.StreamPartTypeToolCall || parts[8].ToolCall == nil || parts[8].ToolCall.Arguments["city"] != "LA" {
		t.Fatalf("unexpected tool call: %#v", parts[8])
	}
	if parts[9].Type != provider.StreamPartTypeFinish || parts[9].Finish == nil {
		t.Fatalf("unexpected finish: %#v", parts[9])
	}
	if parts[9].Finish.Reason != provider.FinishReasonStop {
		t.Fatalf("unexpected finish reason: %#v", parts[9].Finish.Reason)
	}
	if parts[9].Finish.Usage == nil || parts[9].Finish.Usage.PromptTokens != 3 || parts[9].Finish.Usage.CompletionTokens != 4 {
		t.Fatalf("unexpected usage: %#v", parts[9].Finish.Usage)
	}
	if parts[9].ResponseMetadata == nil || parts[9].ResponseMetadata.RequestID != "req-123" {
		t.Fatalf("unexpected response metadata: %#v", parts[9].ResponseMetadata)
	}
}

func TestAnthropicErrorMapping(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-request-id", "req-err")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"rate limited"}}`)
	}))
	defer server.Close()

	client := CreateAnthropic(Settings{BaseURL: server.URL})
	model, err := client.LanguageModel("claude-3")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt: provider.Prompt{Messages: []provider.ModelMessage{{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hi"}}}}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	var rateLimitErr *provider.RateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("expected rate limit error, got %T", err)
	}
	if rateLimitErr.RequestID != "req-err" {
		t.Fatalf("unexpected request id: %s", rateLimitErr.RequestID)
	}
	if rateLimitErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("unexpected status: %d", rateLimitErr.StatusCode)
	}
	if rateLimitErr.Message != "rate limited" {
		t.Fatalf("unexpected message: %s", rateLimitErr.Message)
	}
}
