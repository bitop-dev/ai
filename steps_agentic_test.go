package ai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bitop-dev/ai/internal/provider"
)

func TestGenerateText_StepsAndResponseMessages(t *testing.T) {
	fp := &fakeProvider{}
	fp.generate = func(call int, req provider.Request) (provider.Response, error) {
		switch call {
		case 0:
			if got := len(req.Tools); got != 1 {
				t.Fatalf("tools=%d", got)
			}
			return provider.Response{
				Message: provider.Message{
					Role: provider.RoleAssistant,
					Content: []provider.ContentPart{
						provider.ToolCallPart{ID: "call_1", Name: "add", Args: []byte(`{"a":1,"b":2}`)},
					},
				},
				Usage:        provider.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
				FinishReason: "tool_calls",
			}, nil
		case 1:
			if got := len(req.Tools); got != 1 {
				t.Fatalf("tools=%d", got)
			}
			return provider.Response{
				Message: provider.Message{
					Role:    provider.RoleAssistant,
					Content: []provider.ContentPart{provider.TextPart{Text: "3"}},
				},
				Usage:        provider.Usage{PromptTokens: 4, CompletionTokens: 5, TotalTokens: 6},
				FinishReason: "stop",
			}, nil
		default:
			t.Fatalf("unexpected call %d", call)
			return provider.Response{}, nil
		}
	}

	providerName := registerFakeProvider(t, fp)

	var stepEvents []Step

	resp, err := GenerateText(context.Background(), GenerateTextRequest{
		BaseRequest: BaseRequest{
			Model:    testModel{provider: providerName, name: "m"},
			Messages: []Message{User("calc")},
			Tools: []Tool{
				NewTool("add", ToolSpec[struct {
					A int `json:"a"`
					B int `json:"b"`
				}, map[string]int]{
					Execute: func(ctx context.Context, input struct {
						A int `json:"a"`
						B int `json:"b"`
					}, meta ToolExecutionMeta) (map[string]int, error) {
						_ = ctx
						_ = meta
						return map[string]int{"result": input.A + input.B}, nil
					},
				}),
				NewDynamicTool("unused", DynamicToolSpec{
					Execute: func(ctx context.Context, input json.RawMessage, meta ToolExecutionMeta) (any, error) {
						_ = ctx
						_ = input
						_ = meta
						return nil, nil
					},
				}),
			},
			PrepareStep: func(event PrepareStepEvent) (PrepareStepResult, error) {
				return PrepareStepResult{ActiveTools: []string{"add"}}, nil
			},
			OnStepFinish: func(event StepFinishEvent) {
				stepEvents = append(stepEvents, event.Step)
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Text != "3" {
		t.Fatalf("Text=%q", resp.Text)
	}
	if len(resp.Steps) != 2 {
		t.Fatalf("Steps=%d", len(resp.Steps))
	}
	if len(stepEvents) != 2 {
		t.Fatalf("OnStepFinish=%d", len(stepEvents))
	}
	if got := len(resp.Response.Messages); got != 3 {
		t.Fatalf("response.messages=%d", got)
	}
	if got := resp.Usage.TotalTokens; got != 9 {
		t.Fatalf("usage.total=%d", got)
	}
}

func TestGenerateText_StopWhen(t *testing.T) {
	fp := &fakeProvider{}
	fp.generate = func(call int, req provider.Request) (provider.Response, error) {
		_ = req
		switch call {
		case 0:
			return provider.Response{
				Message: provider.Message{
					Role: provider.RoleAssistant,
					Content: []provider.ContentPart{
						provider.ToolCallPart{ID: "call_1", Name: "add", Args: []byte(`{"a":1,"b":2}`)},
					},
				},
				FinishReason: "tool_calls",
			}, nil
		default:
			t.Fatalf("unexpected call %d", call)
			return provider.Response{}, nil
		}
	}

	providerName := registerFakeProvider(t, fp)

	resp, err := GenerateText(context.Background(), GenerateTextRequest{
		BaseRequest: BaseRequest{
			Model:    testModel{provider: providerName, name: "m"},
			Messages: []Message{User("calc")},
			Tools: []Tool{
				NewTool("add", ToolSpec[struct {
					A int `json:"a"`
					B int `json:"b"`
				}, map[string]int]{
					Execute: func(ctx context.Context, input struct {
						A int `json:"a"`
						B int `json:"b"`
					}, meta ToolExecutionMeta) (map[string]int, error) {
						_ = ctx
						_ = input
						_ = meta
						return map[string]int{"result": 3}, nil
					},
				}),
			},
			ToolLoop: &ToolLoopOptions{
				MaxIterations: 5,
				StopWhen:      StepCountIs(1),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.Steps) != 1 {
		t.Fatalf("Steps=%d", len(resp.Steps))
	}
	if got := resp.FinishReason; got != FinishToolCalls {
		t.Fatalf("FinishReason=%q", got)
	}
}
