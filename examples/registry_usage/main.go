package main

import (
	"context"
	"log"

	"github.com/bitop-dev/ai/pkg/ai"
	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providers/openai"
	"github.com/bitop-dev/ai/pkg/registry"
)

func main() {
	ctx := context.Background()
	providers := map[provider.ProviderID]provider.Provider{
		"openai": openai.CreateOpenAI(openai.Settings{}),
	}

	modelRegistry := registry.CreateProviderRegistry(providers, registry.Options{})
	model, err := ai.ResolveModel(modelRegistry, "openai:gpt-4o-mini")
	if err != nil {
		log.Fatal(err)
	}

	prompt := provider.Prompt{
		Messages: []provider.ModelMessage{
			{
				Role: provider.RoleUser,
				Content: []provider.ContentPart{
					provider.TextContent{Text: "Summarize the benefits of Go in one sentence."},
				},
			},
		},
	}

	result, err := ai.GenerateText(ctx, model, ai.GenerateTextOptions{Prompt: prompt})
	if err != nil {
		log.Fatal(err)
	}

	log.Println(result.Text)
}
