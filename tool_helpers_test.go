package ai

import (
	"context"
	"testing"
)

func TestToolHelper_ValidatesSchemaAndUnmarshals(t *testing.T) {
	type args struct {
		Location string `json:"location"`
	}
	type result struct {
		Ok bool `json:"ok"`
	}

	tool := NewTool("weather", ToolSpec[args, result]{
		InputSchema: JSONSchema([]byte(`{"type":"object","properties":{"location":{"type":"string"}},"required":["location"],"additionalProperties":false}`)),
		Execute: func(ctx context.Context, input args, meta ToolExecutionMeta) (result, error) {
			_ = ctx
			_ = meta
			return result{Ok: input.Location != ""}, nil
		},
	})

	if _, err := tool.Handler(context.Background(), []byte(`{"location":"sf"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := tool.Handler(context.Background(), []byte(`{"wrong":true}`)); err == nil {
		t.Fatalf("expected schema validation error")
	}
}
