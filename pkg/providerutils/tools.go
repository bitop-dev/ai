package providerutils

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

type ToolCall = provider.ToolCall
type ToolResult = provider.ToolResult

type ToolExecuteFunc func(ctx context.Context, call ToolCall) (ToolOutput, error)
type ToolApprovalFunc func(ctx context.Context, call ToolCall) (bool, error)

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  provider.JSONSchema
	Execute     ToolExecuteFunc
	Approve     ToolApprovalFunc
}

type ToolSpecification struct {
	Name        string
	Description string
	Parameters  provider.JSONSchema
}

func (definition ToolDefinition) Specification() ToolSpecification {
	return ToolSpecification{
		Name:        definition.Name,
		Description: definition.Description,
		Parameters:  definition.Parameters,
	}
}

type ToolOutputType string

const (
	ToolOutputTypeText    ToolOutputType = "text"
	ToolOutputTypeJSON    ToolOutputType = "json"
	ToolOutputTypeError   ToolOutputType = "error"
	ToolOutputTypeContent ToolOutputType = "content"
)

type ToolOutput interface {
	OutputType() ToolOutputType
}

type ToolTextOutput struct {
	Text string
}

func (ToolTextOutput) OutputType() ToolOutputType { return ToolOutputTypeText }

type ToolJSONOutput struct {
	Data provider.JSONValue
}

func (ToolJSONOutput) OutputType() ToolOutputType { return ToolOutputTypeJSON }

type ToolErrorOutput struct {
	Err error
}

func (ToolErrorOutput) OutputType() ToolOutputType { return ToolOutputTypeError }

type ToolContentOutput struct {
	Content []provider.ContentPart
}

func (ToolContentOutput) OutputType() ToolOutputType { return ToolOutputTypeContent }

func ExecuteTool(ctx context.Context, tool ToolDefinition, call ToolCall) (ToolResult, error) {
	if tool.Execute == nil {
		return ToolResult{}, fmt.Errorf("tool %q has no execute function", tool.Name)
	}

	output, err := tool.Execute(ctx, call)
	if err != nil {
		return ToolResult{ID: call.ID, Name: call.Name, Result: err, IsError: true}, err
	}

	return ToolResultFromOutput(call, output), nil
}

func ToolResultFromOutput(call ToolCall, output ToolOutput) ToolResult {
	result := ToolResult{ID: call.ID, Name: call.Name}
	if output == nil {
		return result
	}

	switch typed := output.(type) {
	case ToolTextOutput:
		result.Result = typed.Text
	case *ToolTextOutput:
		result.Result = typed.Text
	case ToolJSONOutput:
		result.Result = typed.Data
	case *ToolJSONOutput:
		result.Result = typed.Data
	case ToolContentOutput:
		result.Result = typed.Content
	case *ToolContentOutput:
		result.Result = typed.Content
	case ToolErrorOutput:
		result.Result = typed.Err
		result.IsError = true
	case *ToolErrorOutput:
		result.Result = typed.Err
		result.IsError = true
	default:
		result.Result = output
	}

	return result
}

func ToolResultForError(call ToolCall, err error) ToolResult {
	return ToolResult{ID: call.ID, Name: call.Name, Result: err, IsError: true}
}

func StreamPartForToolCall(call ToolCall) provider.StreamPart {
	return provider.StreamPart{Type: provider.StreamPartTypeToolCall, ToolCall: &call}
}

func StreamPartForToolResult(result ToolResult) provider.StreamPart {
	return provider.StreamPart{Type: provider.StreamPartTypeToolResult, ToolResult: &result}
}

func ToolCallFromStreamPart(part provider.StreamPart) (ToolCall, bool) {
	if part.ToolCall == nil {
		return ToolCall{}, false
	}

	return *part.ToolCall, true
}

func ToolResultFromStreamPart(part provider.StreamPart) (ToolResult, bool) {
	if part.ToolResult == nil {
		return ToolResult{}, false
	}

	return *part.ToolResult, true
}

type ToolNameMapper struct {
	toolToProvider map[string]string
	providerToTool map[string]string
}

func NewToolNameMapper(toolToProvider map[string]string) ToolNameMapper {
	mapper := ToolNameMapper{
		toolToProvider: make(map[string]string, len(toolToProvider)),
		providerToTool: make(map[string]string, len(toolToProvider)),
	}

	for toolName, providerName := range toolToProvider {
		mapper.toolToProvider[toolName] = providerName
		mapper.providerToTool[providerName] = toolName
	}

	return mapper
}

func (mapper ToolNameMapper) ProviderName(toolName string) string {
	if mapped, ok := mapper.toolToProvider[toolName]; ok {
		return mapped
	}

	return toolName
}

func (mapper ToolNameMapper) ToolName(providerName string) string {
	if mapped, ok := mapper.providerToTool[providerName]; ok {
		return mapped
	}

	return providerName
}

type ToolArgumentAccumulator struct {
	mu    sync.Mutex
	calls map[string]*toolArgumentBuffer
}

type toolArgumentBuffer struct {
	name    string
	builder strings.Builder
}

func NewToolArgumentAccumulator() *ToolArgumentAccumulator {
	return &ToolArgumentAccumulator{calls: map[string]*toolArgumentBuffer{}}
}

func (accumulator *ToolArgumentAccumulator) Start(callID, name string) {
	accumulator.mu.Lock()
	defer accumulator.mu.Unlock()

	accumulator.calls[callID] = &toolArgumentBuffer{name: name}
}

func (accumulator *ToolArgumentAccumulator) AddDelta(callID, delta string) error {
	accumulator.mu.Lock()
	defer accumulator.mu.Unlock()

	buffer, ok := accumulator.calls[callID]
	if !ok {
		return fmt.Errorf("tool call %q not started", callID)
	}

	buffer.builder.WriteString(delta)
	return nil
}

func (accumulator *ToolArgumentAccumulator) End(callID string) (ToolCall, error) {
	accumulator.mu.Lock()
	buffer, ok := accumulator.calls[callID]
	if !ok {
		accumulator.mu.Unlock()
		return ToolCall{}, fmt.Errorf("tool call %q not started", callID)
	}
	delete(accumulator.calls, callID)
	accumulator.mu.Unlock()

	arguments, err := parseToolArguments(callID, buffer.builder.String())
	if err != nil {
		return ToolCall{}, err
	}

	return ToolCall{ID: callID, Name: buffer.name, Arguments: arguments}, nil
}

func parseToolArguments(callID, raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}, nil
	}

	var arguments map[string]any
	if err := json.Unmarshal([]byte(trimmed), &arguments); err != nil {
		return nil, fmt.Errorf("parse tool arguments for %q: %w", callID, err)
	}

	return arguments, nil
}
