package main

import (
	"context"
	"fmt"
	"log"

	"github.com/bitop-dev/ai/pkg/ai"
	"github.com/bitop-dev/ai/pkg/gateway"
	"github.com/bitop-dev/ai/pkg/provider"
)

func main() {
	ctx := context.Background()
	gatewayProvider := gateway.CreateGateway()
	model, err := gatewayProvider.LanguageModel("openai/gpt-4o")
	if err != nil {
		log.Fatal(err)
	}

	prompt := provider.Prompt{
		Messages: []provider.ModelMessage{
			{
				Role: provider.RoleUser,
				Content: []provider.ContentPart{
					provider.TextContent{Text: "Write a short haiku about streaming APIs."},
				},
			},
		},
	}

	result, err := ai.StreamText(ctx, model, ai.StreamTextOptions{Prompt: prompt})
	if err != nil {
		log.Fatal(err)
	}
	defer result.Stream.Close()

	for result.Stream.Next() {
		part := result.Stream.Value()
		switch part.Type {
		case provider.StreamPartTypeTextStart:
			if part.TextStart != nil {
				fmt.Print(part.TextStart.Text)
			}
		case provider.StreamPartTypeTextDelta:
			if part.TextDelta != nil {
				fmt.Print(part.TextDelta.Delta)
			}
		case provider.StreamPartTypeTextEnd:
			if part.TextEnd != nil {
				fmt.Print(part.TextEnd.Text)
			}
		}
	}

	if err := result.Stream.Err(); err != nil {
		log.Fatal(err)
	}
	fmt.Println()
}
