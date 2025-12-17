package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

type ToolExecutionMeta struct {
	ToolName      string
	ToolCallID    string
	ToolCallIndex int

	// Report emits a progress event during tool execution (if enabled on the request).
	Report func(data any)
}

type ToolSpec[Input any, Output any] struct {
	Description string
	InputSchema Schema
	Execute     func(ctx context.Context, input Input, meta ToolExecutionMeta) (Output, error)
}

type toolExecutionMetaKey struct{}

func toolExecutionMetaFromContext(ctx context.Context) ToolExecutionMeta {
	if ctx == nil {
		return ToolExecutionMeta{}
	}
	meta, _ := ctx.Value(toolExecutionMetaKey{}).(ToolExecutionMeta)
	return meta
}

// NewTool creates a Tool with typed input/output. The returned Tool.Handler:
// - validates input against InputSchema (if provided)
// - unmarshals into Input
// - calls Execute
func NewTool[Input any, Output any](name string, spec ToolSpec[Input, Output]) Tool {
	if name == "" {
		panic("tool name is required")
	}
	if spec.Execute == nil {
		panic(fmt.Sprintf("tool %q Execute is required", name))
	}
	return Tool{
		Name:        name,
		Description: spec.Description,
		InputSchema: spec.InputSchema,
		Handler: func(ctx context.Context, input json.RawMessage) (any, error) {
			if err := validateJSONAgainstSchema(spec.InputSchema, input); err != nil {
				return nil, err
			}
			var v Input
			if err := json.Unmarshal(input, &v); err != nil {
				return nil, err
			}
			return spec.Execute(ctx, v, toolExecutionMetaFromContext(ctx))
		},
	}
}

type DynamicToolSpec struct {
	Description string
	InputSchema Schema
	Execute     func(ctx context.Context, input json.RawMessage, meta ToolExecutionMeta) (any, error)
}

// NewDynamicTool creates a Tool where input is left as json.RawMessage for runtime
// validation/casting.
func NewDynamicTool(name string, spec DynamicToolSpec) Tool {
	if name == "" {
		panic("tool name is required")
	}
	if spec.Execute == nil {
		panic(fmt.Sprintf("tool %q Execute is required", name))
	}
	return Tool{
		Name:        name,
		Description: spec.Description,
		InputSchema: spec.InputSchema,
		Handler: func(ctx context.Context, input json.RawMessage) (any, error) {
			if err := validateJSONAgainstSchema(spec.InputSchema, input); err != nil {
				return nil, err
			}
			return spec.Execute(ctx, input, toolExecutionMetaFromContext(ctx))
		},
	}
}
