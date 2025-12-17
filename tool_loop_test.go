package ai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bitop-dev/ai/internal/provider"
)

func TestGenerateText_ToolLoop(t *testing.T) {
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
			var sawToolResult bool
			for _, m := range req.Messages {
				if m.Role == provider.RoleTool && m.ToolCallID == "call_1" {
					sawToolResult = true
					break
				}
			}
			if !sawToolResult {
				t.Fatalf("second request missing tool result message")
			}
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

	resp, err := GenerateText(context.Background(), GenerateTextRequest{
		BaseRequest: BaseRequest{
			Model:    testModel{provider: providerName, name: "m"},
			Messages: []Message{User("calc")},
			Tools: []Tool{
				NewTool("add", ToolSpec[struct {
					A int `json:"a"`
					B int `json:"b"`
				}, map[string]any]{
					Description: "add numbers",
					InputSchema: JSONSchema([]byte(`{"type":"object","properties":{"a":{"type":"integer"},"b":{"type":"integer"}},"required":["a","b"],"additionalProperties":false}`)),
					Execute: func(ctx context.Context, input struct {
						A int `json:"a"`
						B int `json:"b"`
					}, meta ToolExecutionMeta) (map[string]any, error) {
						_ = ctx
						_ = meta
						return map[string]any{"result": input.A + input.B}, nil
					},
				}),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "3" {
		t.Fatalf("Text=%q", resp.Text)
	}
	if len(fp.Requests()) != 2 {
		t.Fatalf("provider calls=%d", len(fp.Requests()))
	}
}

func TestStreamText_UsageAggregatesAcrossIterations(t *testing.T) {
	fp := &fakeProvider{}
	fp.stream = func(call int, req provider.Request) (provider.Stream, error) {
		_ = req
		switch call {
		case 0:
			return &fakeStream{
				deltas: []provider.Delta{{Text: "hi "}},
				final: &provider.Response{
					Message: provider.Message{
						Role: provider.RoleAssistant,
						Content: []provider.ContentPart{
							provider.ToolCallPart{ID: "call_1", Name: "add", Args: []byte(`{"a":1,"b":2}`)},
						},
					},
					Usage:        provider.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
					FinishReason: "tool_calls",
				},
			}, nil
		case 1:
			return &fakeStream{
				deltas: []provider.Delta{{Text: "3"}},
				final: &provider.Response{
					Message: provider.Message{
						Role:    provider.RoleAssistant,
						Content: []provider.ContentPart{provider.TextPart{Text: "3"}},
					},
					Usage:        provider.Usage{PromptTokens: 4, CompletionTokens: 5, TotalTokens: 6},
					FinishReason: "stop",
				},
			}, nil
		default:
			t.Fatalf("unexpected stream call %d", call)
			return nil, nil
		}
	}

	providerName := registerFakeProvider(t, fp)

	stream, err := StreamText(context.Background(), StreamTextRequest{
		BaseRequest: BaseRequest{
			Model:    testModel{provider: providerName, name: "m"},
			Messages: []Message{User("calc")},
			Tools: []Tool{
				{
					Name: "add",
					Handler: func(ctx context.Context, input json.RawMessage) (any, error) {
						_ = ctx
						_ = input
						return map[string]any{"result": 3}, nil
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	for stream.Next() {
		_ = stream.Delta()
	}
	if err := stream.Err(); err != nil {
		t.Fatal(err)
	}

	u := stream.Usage()
	if u.TotalTokens != 9 || u.PromptTokens != 5 || u.CompletionTokens != 7 {
		t.Fatalf("usage=%#v", u)
	}
	if got := stream.FinishReason(); got != FinishStop {
		t.Fatalf("FinishReason=%q", got)
	}
}
