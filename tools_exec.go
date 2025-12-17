package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bitop-dev/ai/internal/provider"
)

func findTool(tools []Tool, name string) (Tool, bool) {
	for _, t := range tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

type toolExecOptions struct {
	toolCallIndexByID func(toolCallID string) int
	onInputAvailable  func(tool Tool, call provider.ToolCallPart, toolCallIndex int)
	onProgress        func(event ToolProgressEvent)
}

func executeToolCallsProvider(ctx context.Context, tools []Tool, calls []provider.ToolCallPart) ([]provider.Message, error) {
	return executeToolCallsProviderWithOptions(ctx, tools, calls, toolExecOptions{})
}

func executeToolCallsProviderWithOptions(ctx context.Context, tools []Tool, calls []provider.ToolCallPart, opts toolExecOptions) ([]provider.Message, error) {
	if len(calls) == 0 {
		return nil, nil
	}
	if len(tools) == 0 {
		return nil, fmt.Errorf("model requested tool calls but no tools were provided")
	}

	results := make([]provider.Message, 0, len(calls))
	for _, call := range calls {
		if call.ID == "" {
			return nil, fmt.Errorf("tool call missing id")
		}
		t, ok := findTool(tools, call.Name)
		if !ok {
			return nil, &NoSuchToolError{ToolName: call.Name}
		}
		if t.Handler == nil {
			return nil, fmt.Errorf("tool %q missing handler", call.Name)
		}

		toolCallIndex := -1
		if opts.toolCallIndexByID != nil {
			toolCallIndex = opts.toolCallIndexByID(call.ID)
		}

		if len(t.InputSchema.JSON) > 0 {
			if err := validateJSONAgainstSchema(t.InputSchema, call.Args); err != nil {
				return nil, &InvalidToolInputError{ToolName: t.Name, ToolCallID: call.ID, Cause: err}
			}
		}

		if opts.onInputAvailable != nil {
			opts.onInputAvailable(t, call, toolCallIndex)
		}

		meta := ToolExecutionMeta{
			ToolName:      t.Name,
			ToolCallID:    call.ID,
			ToolCallIndex: toolCallIndex,
		}
		if opts.onProgress != nil {
			meta.Report = func(data any) {
				opts.onProgress(ToolProgressEvent{
					ToolName:      t.Name,
					ToolCallID:    call.ID,
					ToolCallIndex: toolCallIndex,
					Data:          data,
				})
			}
		}

		execCtx := context.WithValue(ctx, toolExecutionMetaKey{}, meta)

		val, err := t.Handler(execCtx, call.Args)
		if err != nil {
			return nil, &ToolExecutionError{ToolName: t.Name, ToolCallID: call.ID, Cause: err}
		}
		results = append(results, toolResultProvider(call.ID, t.Name, val))
	}
	return results, nil
}

func toolResultProvider(toolCallID, toolName string, value any) provider.Message {
	raw, err := json.Marshal(value)
	if err != nil {
		raw = json.RawMessage(fmt.Sprintf(`{"error":%q}`, err.Error()))
	}
	return provider.Message{
		Role:       provider.RoleTool,
		ToolCallID: toolCallID,
		Content:    []provider.ContentPart{provider.TextPart{Text: string(raw)}},
		Name:       toolName,
	}
}
