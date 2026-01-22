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

	agent := ai.NewToolLoopAgent(ai.ToolLoopAgentSettings[any]{
		Model: model,
		Tools: []providerutils.ToolDefinition{tool},
	})

	result, err := agent.Generate(ctx, ai.AgentCallOptions[any]{
		Prompt: "What time is it in Tokyo right now?",
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println(result.Text)
}
