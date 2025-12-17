package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

type ToolExecutionMeta struct {
	ToolCallID string
}

type ToolSpec[Input any, Output any] struct {
	Description string
	InputSchema Schema
	Execute     func(ctx context.Context, input Input, meta ToolExecutionMeta) (Output, error)
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
			return spec.Execute(ctx, v, ToolExecutionMeta{})
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
			return spec.Execute(ctx, input, ToolExecutionMeta{})
		},
	}
}
