package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

type toolCall struct {
	ID   string
	Name string
	Args json.RawMessage
}

func extractToolCalls(m Message) []toolCall {
	var out []toolCall
	for _, p := range m.Content {
		tc, ok := p.(ToolCallPart)
		if !ok {
			continue
		}
		out = append(out, toolCall{
			ID:   tc.ID,
			Name: tc.Name,
			Args: tc.Args,
		})
	}
	return out
}

func findTool(tools []Tool, name string) (Tool, bool) {
	for _, t := range tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

func runTools(ctx context.Context, tools []Tool, calls []toolCall) ([]Message, error) {
	if len(calls) == 0 {
		return nil, nil
	}
	if len(tools) == 0 {
		return nil, fmt.Errorf("model requested tool calls but no tools were provided")
	}
	results := make([]Message, 0, len(calls))
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

		if len(t.InputSchema.JSON) > 0 {
			if err := validateJSONAgainstSchema(t.InputSchema, call.Args); err != nil {
				return nil, &InvalidToolInputError{ToolName: t.Name, ToolCallID: call.ID, Cause: err}
			}
		}

		val, err := t.Handler(ctx, call.Args)
		if err != nil {
			return nil, &ToolExecutionError{ToolName: t.Name, ToolCallID: call.ID, Cause: err}
		}
		results = append(results, ToolResultForCall(call.ID, t.Name, val))
	}
	return results, nil
}
