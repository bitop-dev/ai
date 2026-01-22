package anthropic

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
