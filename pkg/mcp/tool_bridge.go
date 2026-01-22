package mcp

import (
	"context"
	"fmt"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

type ToolSetOptions struct {
	ToolName func(Tool) string
	Filter   func(Tool) bool
	Params   *PaginatedParams
}

type ToolSet struct {
	Tools      []providerutils.ToolDefinition
	NameMapper providerutils.ToolNameMapper
}

func (client *Client) Tools(ctx context.Context, options ToolSetOptions) (ToolSet, error) {
	if client == nil {
		return ToolSet{}, fmt.Errorf("mcp client is nil")
	}

	toolToProvider := map[string]string{}
	var definitions []providerutils.ToolDefinition
	params := options.Params
	for {
		result, err := client.ListTools(ctx, params)
		if err != nil {
			return ToolSet{}, err
		}

		for _, tool := range result.Tools {
			if options.Filter != nil && !options.Filter(tool) {
				continue
			}

			mappedName := tool.Name
			if options.ToolName != nil {
				mappedName = options.ToolName(tool)
			}
			if mappedName == "" {
				return ToolSet{}, fmt.Errorf("mcp tool name mapping returned empty for %q", tool.Name)
			}

			toolToProvider[mappedName] = tool.Name
			description := tool.Description
			if description == "" {
				description = tool.Title
			}

			remoteName := tool.Name
			inputSchema := tool.InputSchema
			definitions = append(definitions, providerutils.ToolDefinition{
				Name:        mappedName,
				Description: description,
				Parameters:  inputSchema,
				Execute: func(ctx context.Context, call providerutils.ToolCall) (providerutils.ToolOutput, error) {
					result, err := client.CallTool(ctx, remoteName, call.Arguments)
					if err != nil {
						return nil, err
					}
					toolResult := ToolResultFromCall(call, result)
					return providerutils.ToolResultOutput{Result: toolResult.Result, IsError: toolResult.IsError}, nil
				},
			})
		}

		if result.NextCursor == "" {
			break
		}
		params = &PaginatedParams{Cursor: result.NextCursor}
	}

	return ToolSet{Tools: definitions, NameMapper: providerutils.NewToolNameMapper(toolToProvider)}, nil
}

func StreamPartForToolCall(call provider.ToolCall) provider.StreamPart {
	return providerutils.StreamPartForToolCall(call)
}

func StreamPartForToolResult(call provider.ToolCall, result CallToolResult) provider.StreamPart {
	toolResult := ToolResultFromCall(call, result)
	return providerutils.StreamPartForToolResult(toolResult)
}
