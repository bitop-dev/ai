package ai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bitop-dev/ai/internal/provider"
)

func TestAgent_DefaultsToOneStep(t *testing.T) {
	fp := &fakeProvider{}
	fp.generate = func(call int, req provider.Request) (provider.Response, error) {
		if call != 0 {
			t.Fatalf("unexpected call %d", call)
		}
		// If the agent defaulted to 1 step, it should not run a tool loop.
		return provider.Response{
			Message: provider.Message{
				Role:    provider.RoleAssistant,
				Content: []provider.ContentPart{provider.TextPart{Text: "ok"}},
			},
			FinishReason: "stop",
		}, nil
	}
	providerName := registerFakeProvider(t, fp)

	a := Agent{
		Model: testModel{provider: providerName, name: "m"},
		Tools: []Tool{
			{
				Name: "noop",
				Handler: func(ctx context.Context, input json.RawMessage) (any, error) {
					_ = ctx
					_ = input
					t.Fatal("tool should not be executed")
					return nil, nil
				},
			},
		},
	}

	resp, err := a.Generate(context.Background(), AgentGenerateRequest{Prompt: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "ok" {
		t.Fatalf("Text=%q", resp.Text)
	}
	if got := len(fp.Requests()); got != 1 {
		t.Fatalf("provider calls=%d", got)
	}
}

func TestAgent_MultiStepWhenConfigured(t *testing.T) {
	fp := &fakeProvider{}
	fp.generate = func(call int, req provider.Request) (provider.Response, error) {
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
		case 1:
			return provider.Response{
				Message: provider.Message{
					Role:    provider.RoleAssistant,
					Content: []provider.ContentPart{provider.TextPart{Text: "3"}},
				},
				FinishReason: "stop",
			}, nil
		default:
			t.Fatalf("unexpected call %d", call)
			return provider.Response{}, nil
		}
	}
	providerName := registerFakeProvider(t, fp)

	a := Agent{
		Model:         testModel{provider: providerName, name: "m"},
		MaxIterations: 5,
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
		},
	}

	resp, err := a.Generate(context.Background(), AgentGenerateRequest{Prompt: "calc"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "3" {
		t.Fatalf("Text=%q", resp.Text)
	}
	if got := len(fp.Requests()); got != 2 {
		t.Fatalf("provider calls=%d", got)
	}
}
