package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/vercel/ai-sdk-go/pkg/ai"
	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providers/openai"
)

func main() {
	ctx := context.Background()
	openaiProvider := openai.CreateOpenAI(openai.Settings{})
	model, err := openaiProvider.LanguageModel("gpt-4o-mini")
	if err != nil {
		log.Fatal(err)
	}

	schema := provider.JSONObject{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type": "string",
			},
			"bullets": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
		"required": []string{"title", "bullets"},
	}

	prompt := provider.Prompt{
		Messages: []provider.ModelMessage{
			{
				Role: provider.RoleUser,
				Content: []provider.ContentPart{
					provider.TextContent{Text: "Summarize the latest Go release in three bullets."},
				},
			},
		},
	}

	result, err := ai.GenerateObject(ctx, model, ai.GenerateObjectOptions{
		Prompt: prompt,
		ResponseFormat: &provider.ResponseFormat{
			Type:        provider.ResponseFormatTypeJSON,
			Name:        "go_release_summary",
			Description: "A short summary of Go release highlights.",
			Schema:      schema,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	payload, err := json.MarshalIndent(result.Object, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(payload))
}
