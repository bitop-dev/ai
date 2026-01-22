package main

import (
	"context"
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

	prompt := provider.Prompt{
		Messages: []provider.ModelMessage{
			{
				Role: provider.RoleUser,
				Content: []provider.ContentPart{
					provider.TextContent{Text: "Summarize the Pacific Ocean in one sentence."},
				},
			},
		},
	}

	result, err := ai.GenerateText(ctx, model, ai.GenerateTextOptions{Prompt: prompt})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result.Text)
}
