package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

const defaultToolLoopMaxSteps = 5

var (
	ErrToolLoopMaxSteps = errors.New("tool loop exceeded max steps")
	ErrToolNotFound     = errors.New("tool not found")
	ErrToolRejected     = errors.New("tool call rejected")
)

type ToolLoopOptions struct {
	TextOptions
	Tools    []providerutils.ToolDefinition
	MaxSteps int
	Approve  providerutils.ToolApprovalFunc
}

func GenerateTextWithTools(ctx context.Context, model provider.LanguageModelV3, options ToolLoopOptions) (GenerateTextResult, error) {
	if len(options.Tools) == 0 {
		return GenerateText(ctx, model, GenerateTextOptions(options.TextOptions))
	}
	if model == nil {
		return GenerateTextResult{}, ErrNilModel
	}

	prompt := options.Prompt
	maxSteps := options.MaxSteps
	if maxSteps <= 0 {
		maxSteps = defaultToolLoopMaxSteps
	}
	toolMap := make(map[string]providerutils.ToolDefinition, len(options.Tools))
	for _, tool := range options.Tools {
		toolMap[tool.Name] = tool
	}

	callOptions := options.TextOptions
	if callOptions.ToolChoice == nil {
		callOptions.ToolChoice = &provider.ToolChoice{Type: provider.ToolChoiceTypeAuto}
	}

	for step := 0; step < maxSteps; step++ {
		callOptions.Prompt = prompt
		result, err := GenerateText(ctx, model, GenerateTextOptions(callOptions))
		if err != nil {
			return GenerateTextResult{}, err
		}

		toolCalls, contentParts, err := parseToolLoopParts(result.Parts)
		if err != nil {
			return GenerateTextResult{}, err
		}
		if len(contentParts) > 0 {
			prompt.Messages = append(prompt.Messages, provider.ModelMessage{
				Role:    provider.RoleAssistant,
				Content: contentParts,
			})
		}
		if len(toolCalls) == 0 {
			return result, nil
		}

		toolResults := make([]provider.ToolResult, 0, len(toolCalls))
		for _, call := range toolCalls {
			tool, ok := toolMap[call.Name]
			if !ok {
				return GenerateTextResult{}, fmt.Errorf("%w: %s", ErrToolNotFound, call.Name)
			}
			if options.Approve != nil {
				approved, err := options.Approve(ctx, call)
				if err != nil {
					return GenerateTextResult{}, err
				}
				if !approved {
					return GenerateTextResult{}, ErrToolRejected
				}
			}
			if tool.Approve != nil {
				approved, err := tool.Approve(ctx, call)
				if err != nil {
					return GenerateTextResult{}, err
				}
				if !approved {
					return GenerateTextResult{}, ErrToolRejected
				}
			}

			result, err := providerutils.ExecuteTool(ctx, tool, call)
			if err != nil {
				return GenerateTextResult{}, err
			}
			toolResults = append(toolResults, result)
		}

		for _, result := range toolResults {
			prompt.Messages = append(prompt.Messages, provider.ModelMessage{
				Role:       provider.RoleTool,
				ToolCallID: result.ID,
				Content: []provider.ContentPart{
					provider.ToolResultContent{ToolResult: result},
				},
			})
		}
	}

	return GenerateTextResult{}, ErrToolLoopMaxSteps
}

func parseToolLoopParts(parts []provider.StreamPart) ([]provider.ToolCall, []provider.ContentPart, error) {
	accumulator := providerutils.NewToolArgumentAccumulator()
	var toolCalls []provider.ToolCall
	var textBuilder strings.Builder
	var reasoningBuilder strings.Builder
	var sources []provider.Source

	for _, part := range parts {
		switch part.Type {
		case provider.StreamPartTypeTextStart:
			if part.TextStart != nil {
				textBuilder.WriteString(part.TextStart.Text)
			}
		case provider.StreamPartTypeTextDelta:
			if part.TextDelta != nil {
				textBuilder.WriteString(part.TextDelta.Delta)
			}
		case provider.StreamPartTypeTextEnd:
			if part.TextEnd != nil {
				textBuilder.WriteString(part.TextEnd.Text)
			}
		case provider.StreamPartTypeReasoningStart:
			if part.ReasoningStart != nil {
				reasoningBuilder.WriteString(part.ReasoningStart.Text)
			}
		case provider.StreamPartTypeReasoningDelta:
			if part.ReasoningDelta != nil {
				reasoningBuilder.WriteString(part.ReasoningDelta.Delta)
			}
		case provider.StreamPartTypeReasoningEnd:
			if part.ReasoningEnd != nil {
				reasoningBuilder.WriteString(part.ReasoningEnd.Text)
			}
		case provider.StreamPartTypeSource:
			if part.Source != nil {
				sources = append(sources, *part.Source)
			}
		case provider.StreamPartTypeToolCall:
			if part.ToolCall != nil {
				toolCalls = append(toolCalls, *part.ToolCall)
			}
		case provider.StreamPartTypeToolInputStart:
			if part.ToolInputStart != nil {
				accumulator.Start(part.ToolInputStart.ToolCallID, part.ToolInputStart.Name)
			}
		case provider.StreamPartTypeToolInputDelta:
			if part.ToolInputDelta != nil {
				if err := accumulator.AddDelta(part.ToolInputDelta.ToolCallID, part.ToolInputDelta.Delta); err != nil {
					return nil, nil, err
				}
			}
		case provider.StreamPartTypeToolInputEnd:
			if part.ToolInputEnd != nil {
				call, err := accumulator.End(part.ToolInputEnd.ToolCallID)
				if err != nil {
					return nil, nil, err
				}
				toolCalls = append(toolCalls, call)
			}
		}
	}

	contentParts := make([]provider.ContentPart, 0, len(toolCalls)+2)
	if textBuilder.Len() > 0 {
		contentParts = append(contentParts, provider.TextContent{Text: textBuilder.String()})
	}
	if reasoningBuilder.Len() > 0 {
		contentParts = append(contentParts, provider.ReasoningContent{Text: reasoningBuilder.String()})
	}
	for _, source := range sources {
		contentParts = append(contentParts, provider.SourceContent{Source: source})
	}
	for _, call := range toolCalls {
		contentParts = append(contentParts, provider.ToolCallContent{ToolCall: call})
	}

	return toolCalls, contentParts, nil
}
