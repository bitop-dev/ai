package providerutils

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

func TestToolResultFromOutput(t *testing.T) {
	call := ToolCall{ID: "call-1", Name: "weather"}

	cases := []struct {
		name      string
		output    ToolOutput
		wantError bool
		wantValue any
	}{
		{name: "text", output: ToolTextOutput{Text: "ok"}, wantValue: "ok"},
		{name: "json", output: ToolJSONOutput{Data: provider.JSONObject{"ok": true}}, wantValue: provider.JSONObject{"ok": true}},
		{name: "content", output: ToolContentOutput{Content: []provider.ContentPart{provider.TextContent{Text: "hello"}}}, wantValue: []provider.ContentPart{provider.TextContent{Text: "hello"}}},
		{name: "error", output: ToolErrorOutput{Err: errors.New("boom")}, wantError: true, wantValue: "boom"},
	}

	for _, entry := range cases {
		t.Run(entry.name, func(t *testing.T) {
			result := ToolResultFromOutput(call, entry.output)
			if result.ID != call.ID || result.Name != call.Name {
				t.Fatalf("result did not preserve call identifiers")
			}

			if result.IsError != entry.wantError {
				t.Fatalf("result error flag mismatch: got %v want %v", result.IsError, entry.wantError)
			}

			if entry.wantError {
				errValue, ok := result.Result.(error)
				if !ok {
					t.Fatalf("expected error result, got %T", result.Result)
				}
				if errValue.Error() != entry.wantValue {
					t.Fatalf("error message mismatch: got %q want %q", errValue.Error(), entry.wantValue)
				}
				return
			}

			if !reflect.DeepEqual(result.Result, entry.wantValue) {
				t.Fatalf("result mismatch: got %#v want %#v", result.Result, entry.wantValue)
			}
		})
	}
}

func TestExecuteTool(t *testing.T) {
	tool := ToolDefinition{
		Name: "echo",
		Execute: func(ctx context.Context, call ToolCall) (ToolOutput, error) {
			if ctx == nil {
				t.Fatalf("expected context to be provided")
			}
			return ToolTextOutput{Text: "ok"}, nil
		},
	}

	result, err := ExecuteTool(context.Background(), tool, ToolCall{ID: "1", Name: "echo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Result != "ok" {
		t.Fatalf("unexpected result: %#v", result.Result)
	}
}

func TestToolNameMapper(t *testing.T) {
	mapper := NewToolNameMapper(map[string]string{"weather": "tool_1"})

	if got := mapper.ProviderName("weather"); got != "tool_1" {
		t.Fatalf("provider name mismatch: got %q", got)
	}

	if got := mapper.ProviderName("other"); got != "other" {
		t.Fatalf("unexpected provider name fallback: %q", got)
	}

	if got := mapper.ToolName("tool_1"); got != "weather" {
		t.Fatalf("tool name mismatch: got %q", got)
	}

	if got := mapper.ToolName("tool_2"); got != "tool_2" {
		t.Fatalf("unexpected tool name fallback: %q", got)
	}
}

func TestToolArgumentAccumulator(t *testing.T) {
	accumulator := NewToolArgumentAccumulator()
	accumulator.Start("call-1", "weather")

	if err := accumulator.AddDelta("call-1", "{\"city\":\"lon"); err != nil {
		t.Fatalf("unexpected delta error: %v", err)
	}

	if err := accumulator.AddDelta("call-1", "don\",\"units\":\"c\"}"); err != nil {
		t.Fatalf("unexpected delta error: %v", err)
	}

	call, err := accumulator.End("call-1")
	if err != nil {
		t.Fatalf("unexpected end error: %v", err)
	}

	expected := map[string]any{"city": "london", "units": "c"}
	if call.Name != "weather" || call.ID != "call-1" {
		t.Fatalf("call identifiers mismatch")
	}

	if !reflect.DeepEqual(call.Arguments, expected) {
		t.Fatalf("arguments mismatch: got %#v want %#v", call.Arguments, expected)
	}
}

func TestToolArgumentAccumulatorMissingStart(t *testing.T) {
	accumulator := NewToolArgumentAccumulator()
	if err := accumulator.AddDelta("call-1", "{}"); err == nil {
		t.Fatalf("expected error for missing start")
	}
}
