package ai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bitop-dev/ai/internal/provider"
)

func TestStreamText_ToolInputLifecycleHooksAndProgress(t *testing.T) {
	fp := &fakeProvider{}
	fp.stream = func(call int, req provider.Request) (provider.Stream, error) {
		_ = req
		switch call {
		case 0:
			return &fakeStream{
				deltas: []provider.Delta{
					{ToolCalls: []provider.ToolCallDelta{{Index: 0, ID: "call_1", Name: "add", ArgumentsDelta: `{"a":`}}},
					{ToolCalls: []provider.ToolCallDelta{{Index: 0, ArgumentsDelta: `1,"b":2`}}},
					{ToolCalls: []provider.ToolCallDelta{{Index: 0, ArgumentsDelta: `}`}}},
				},
				final: &provider.Response{
					Message: provider.Message{
						Role: provider.RoleAssistant,
						Content: []provider.ContentPart{
							provider.ToolCallPart{ID: "call_1", Name: "add", Args: []byte(`{"a":1,"b":2}`)},
						},
					},
					FinishReason: "tool_calls",
				},
			}, nil
		case 1:
			return &fakeStream{
				deltas: []provider.Delta{{Text: "done"}},
				final: &provider.Response{
					Message:      provider.Message{Role: provider.RoleAssistant, Content: []provider.ContentPart{provider.TextPart{Text: "done"}}},
					FinishReason: "stop",
				},
			}, nil
		default:
			t.Fatalf("unexpected stream call %d", call)
			return nil, nil
		}
	}

	providerName := registerFakeProvider(t, fp)

	var starts []ToolInputStartEvent
	var deltas []string
	var avail []ToolInputAvailableEvent
	var progress []ToolProgressEvent

	var onAvailableBeforeExecute bool

	add := NewTool("add", ToolSpec[struct {
		A int `json:"a"`
		B int `json:"b"`
	}, map[string]int]{
		InputSchema: JSONSchema([]byte(`{"type":"object","properties":{"a":{"type":"integer"},"b":{"type":"integer"}},"required":["a","b"],"additionalProperties":false}`)),
		Execute: func(ctx context.Context, input struct {
			A int `json:"a"`
			B int `json:"b"`
		}, meta ToolExecutionMeta) (map[string]int, error) {
			_ = ctx
			if meta.ToolCallID != "call_1" {
				t.Fatalf("ToolCallID=%q", meta.ToolCallID)
			}
			if meta.ToolCallIndex != 0 {
				t.Fatalf("ToolCallIndex=%d", meta.ToolCallIndex)
			}
			if meta.Report != nil {
				meta.Report(map[string]any{"status": "starting"})
				meta.Report(map[string]any{"status": "done"})
			}
			if !onAvailableBeforeExecute {
				t.Fatalf("expected OnInputAvailable before Execute")
			}
			return map[string]int{"result": input.A + input.B}, nil
		},
	})
	add.OnInputStart = func(e ToolInputStartEvent) { starts = append(starts, e) }
	add.OnInputDelta = func(e ToolInputDeltaEvent) { deltas = append(deltas, e.InputTextDelta) }
	add.OnInputAvailable = func(e ToolInputAvailableEvent) {
		avail = append(avail, e)
		onAvailableBeforeExecute = true
		if !json.Valid(e.Input) {
			t.Fatalf("input not valid json: %q", string(e.Input))
		}
	}

	stream, err := StreamText(context.Background(), StreamTextRequest{
		BaseRequest: BaseRequest{
			Model:          testModel{provider: providerName, name: "m"},
			Messages:       []Message{User("calc")},
			Tools:          []Tool{add},
			OnToolProgress: func(e ToolProgressEvent) { progress = append(progress, e) },
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

	if len(starts) != 1 {
		t.Fatalf("starts=%d", len(starts))
	}
	if starts[0].ToolCallIndex != 0 || starts[0].ToolCallID != "call_1" || starts[0].ToolName != "add" {
		t.Fatalf("start=%#v", starts[0])
	}
	if len(deltas) != 3 {
		t.Fatalf("deltas=%v", deltas)
	}
	if got := deltas[0] + deltas[1] + deltas[2]; got != `{"a":1,"b":2}` {
		t.Fatalf("combined deltas=%q", got)
	}
	if len(avail) != 1 {
		t.Fatalf("avail=%d", len(avail))
	}
	if avail[0].ToolCallIndex != 0 || avail[0].ToolCallID != "call_1" || avail[0].ToolName != "add" {
		t.Fatalf("avail=%#v", avail[0])
	}
	if len(progress) != 2 {
		t.Fatalf("progress=%d", len(progress))
	}
	if progress[0].ToolCallID != "call_1" || progress[0].ToolName != "add" {
		t.Fatalf("progress[0]=%#v", progress[0])
	}
}
