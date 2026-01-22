package main

import (
	"context"
	"log"
	"time"

	"github.com/bitop-dev/ai/pkg/ai"
	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providers/openai"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

func main() {
	ctx := context.Background()
	openaiProvider := openai.CreateOpenAI(openai.Settings{})
	model, err := openaiProvider.LanguageModel("gpt-4o-mini")
	if err != nil {
		log.Fatal(err)
	}

	tool := providerutils.ToolDefinition{
		Name:        "current_time",
		Description: "Return the current time in RFC3339 format.",
		Parameters: provider.JSONObject{
			"type": "object",
			"properties": map[string]any{
				"timezone": map[string]any{
					"type":        "string",
					"description": "IANA time zone name, e.g. America/Los_Angeles",
				},
			},
		},
		Execute: func(ctx context.Context, call providerutils.ToolCall) (providerutils.ToolOutput, error) {
			locationName, _ := call.Arguments["timezone"].(string)
			if locationName == "" {
				locationName = "UTC"
			}
			location, err := time.LoadLocation(locationName)
			if err != nil {
				return providerutils.ToolErrorOutput{Err: err}, err
			}
			return providerutils.ToolTextOutput{Text: time.Now().In(location).Format(time.RFC3339)}, nil
		},
	}

	prompt := provider.Prompt{
		Messages: []provider.ModelMessage{
			{
				Role: provider.RoleUser,
				Content: []provider.ContentPart{
					provider.TextContent{Text: "What time is it in Tokyo right now?"},
				},
			},
		},
	}

	options := ai.ToolLoopOptions{
		TextOptions: ai.TextOptions{
			Prompt: prompt,
			RequestOptions: provider.RequestOptions{
				ProviderOptions: provider.ProviderOptions{
					"openai": provider.JSONObject{
						"tools": []providerutils.ToolSpecification{tool.Specification()},
					},
				},
			},
		},
		Tools: []providerutils.ToolDefinition{tool},
	}

	result, err := ai.GenerateTextWithTools(ctx, model, options)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(result.Text)
}
