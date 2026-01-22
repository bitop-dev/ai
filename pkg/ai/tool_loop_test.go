package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providerutils"
)

type toolLoopModel struct {
	responses   [][]provider.StreamPart
	callOptions []provider.LanguageModelV3CallOptions
}

func (toolLoopModel) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (toolLoopModel) ProviderID() provider.ProviderID { return provider.ProviderID("stub") }

func (toolLoopModel) ModelID() provider.ModelID { return provider.ModelID("tool-loop") }

func (toolLoopModel) SupportedURLs() provider.SupportedURLPatterns { return nil }

func (m *toolLoopModel) DoGenerate(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3GenerateResult, error) {
	_ = ctx
	_ = options
	return provider.LanguageModelV3GenerateResult{}, nil
}

func (m *toolLoopModel) DoStream(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3StreamResult, error) {
	_ = ctx
	m.callOptions = append(m.callOptions, options)
	idx := len(m.callOptions) - 1
	var parts []provider.StreamPart
	if idx < len(m.responses) {
		parts = m.responses[idx]
	}
	stream := make(chan provider.StreamPart, len(parts))
	for _, part := range parts {
		stream <- part
	}
	close(stream)
	return provider.LanguageModelV3StreamResult{Stream: stream}, nil
}

func TestGenerateTextWithToolsExecutesToolCalls(t *testing.T) {
	model := &toolLoopModel{responses: [][]provider.StreamPart{
		{
			{Type: provider.StreamPartTypeToolCall, ToolCall: &provider.ToolCall{ID: "call-1", Name: "weather", Arguments: map[string]any{"city": "SF"}}},
			{Type: provider.StreamPartTypeFinish, Finish: &provider.Finish{Reason: provider.FinishReasonToolCalls}},
		},
		{
			{Type: provider.StreamPartTypeTextStart, TextStart: &provider.TextStart{Text: "done"}},
			{Type: provider.StreamPartTypeFinish, Finish: &provider.Finish{Reason: provider.FinishReasonStop}},
		},
	}}

	tool := providerutils.ToolDefinition{
		Name: "weather",
		Execute: func(ctx context.Context, call providerutils.ToolCall) (providerutils.ToolOutput, error) {
			_ = ctx
			_ = call
			return providerutils.ToolTextOutput{Text: "sunny"}, nil
		},
	}

	result, err := GenerateTextWithTools(context.Background(), model, ToolLoopOptions{
		TextOptions: TextOptions{
			Prompt: provider.Prompt{Messages: []provider.ModelMessage{{
				Role:    provider.RoleUser,
				Content: []provider.ContentPart{provider.TextContent{Text: "hi"}},
			}}},
		},
		Tools:    []providerutils.ToolDefinition{tool},
		MaxSteps: 3,
	})
	if err != nil {
		t.Fatalf("GenerateTextWithTools returned error: %v", err)
	}
	if result.Text != "done" {
		t.Fatalf("Text mismatch: got %q want %q", result.Text, "done")
	}
	if len(model.callOptions) != 2 {
		t.Fatalf("expected 2 model calls, got %d", len(model.callOptions))
	}

	secondPrompt := model.callOptions[1].Prompt.Messages
	if len(secondPrompt) != 3 {
		t.Fatalf("expected 3 prompt messages, got %d", len(secondPrompt))
	}
	if secondPrompt[1].Role != provider.RoleAssistant {
		t.Fatalf("assistant role mismatch: got %q", secondPrompt[1].Role)
	}

	var toolCall provider.ToolCall
	for _, part := range secondPrompt[1].Content {
		if callPart, ok := part.(provider.ToolCallContent); ok {
			toolCall = callPart.ToolCall
			break
		}
	}
	if toolCall.Name != "weather" {
		t.Fatalf("tool call mismatch: got %q want %q", toolCall.Name, "weather")
	}

	if secondPrompt[2].Role != provider.RoleTool {
		t.Fatalf("tool role mismatch: got %q", secondPrompt[2].Role)
	}
	if len(secondPrompt[2].Content) != 1 {
		t.Fatalf("expected 1 tool content part, got %d", len(secondPrompt[2].Content))
	}
	resultPart, ok := secondPrompt[2].Content[0].(provider.ToolResultContent)
	if !ok {
		t.Fatalf("expected ToolResultContent, got %T", secondPrompt[2].Content[0])
	}
	if resultPart.ToolResult.Result != "sunny" {
		t.Fatalf("tool result mismatch: got %#v", resultPart.ToolResult.Result)
	}
}

func TestGenerateTextWithToolsRejectsTool(t *testing.T) {
	model := &toolLoopModel{responses: [][]provider.StreamPart{
		{{Type: provider.StreamPartTypeToolCall, ToolCall: &provider.ToolCall{ID: "call-1", Name: "deny", Arguments: map[string]any{}}}},
	}}

	tool := providerutils.ToolDefinition{
		Name: "deny",
		Approve: func(ctx context.Context, call providerutils.ToolCall) (bool, error) {
			_ = ctx
			_ = call
			return false, nil
		},
		Execute: func(ctx context.Context, call providerutils.ToolCall) (providerutils.ToolOutput, error) {
			_ = ctx
			_ = call
			return providerutils.ToolTextOutput{Text: "nope"}, nil
		},
	}

	_, err := GenerateTextWithTools(context.Background(), model, ToolLoopOptions{
		TextOptions: TextOptions{
			Prompt: provider.Prompt{Messages: []provider.ModelMessage{{
				Role:    provider.RoleUser,
				Content: []provider.ContentPart{provider.TextContent{Text: "hi"}},
			}}},
		},
		Tools:    []providerutils.ToolDefinition{tool},
		MaxSteps: 2,
	})
	if !errors.Is(err, ErrToolRejected) {
		t.Fatalf("expected ErrToolRejected, got %v", err)
	}
	if len(model.callOptions) != 1 {
		t.Fatalf("expected 1 model call, got %d", len(model.callOptions))
	}
}

func TestGenerateTextWithToolsMaxSteps(t *testing.T) {
	model := &toolLoopModel{responses: [][]provider.StreamPart{
		{{Type: provider.StreamPartTypeToolCall, ToolCall: &provider.ToolCall{ID: "call-1", Name: "loop", Arguments: map[string]any{}}}},
	}}

	tool := providerutils.ToolDefinition{
		Name: "loop",
		Execute: func(ctx context.Context, call providerutils.ToolCall) (providerutils.ToolOutput, error) {
			_ = ctx
			_ = call
			return providerutils.ToolTextOutput{Text: "ok"}, nil
		},
	}

	_, err := GenerateTextWithTools(context.Background(), model, ToolLoopOptions{
		TextOptions: TextOptions{
			Prompt: provider.Prompt{Messages: []provider.ModelMessage{{
				Role:    provider.RoleUser,
				Content: []provider.ContentPart{provider.TextContent{Text: "hi"}},
			}}},
		},
		Tools:    []providerutils.ToolDefinition{tool},
		MaxSteps: 1,
	})
	if !errors.Is(err, ErrToolLoopMaxSteps) {
		t.Fatalf("expected ErrToolLoopMaxSteps, got %v", err)
	}
}
