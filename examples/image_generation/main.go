package main

import (
	"context"
	"log"

	"github.com/bitop-dev/ai/pkg/ai"
	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providers/openai"
)

func main() {
	ctx := context.Background()
	openaiProvider := openai.CreateOpenAI(openai.Settings{})
	model, err := openaiProvider.ImageModel("gpt-image-1")
	if err != nil {
		log.Fatal(err)
	}

	_, err = ai.GenerateImage(ctx, model, provider.ImageModelCallOptions{
		Prompt:      "A watercolor illustration of a fox reading a book.",
		Size:        "1024x1024",
		AspectRatio: "1:1",
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("image generation request sent")
}
