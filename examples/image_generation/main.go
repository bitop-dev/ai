package main

import (
	"context"
	"log"

	"github.com/vercel/ai-sdk-go/pkg/ai"
	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providers/openai"
)

func main() {
	ctx := context.Background()
	openaiProvider := openai.CreateOpenAI(openai.Settings{})
	model, err := openaiProvider.ImageModel("gpt-image-1")
	if err != nil {
		log.Fatal(err)
	}

	_, err = ai.GenerateImage(ctx, model, provider.ImageModelV3CallOptions{
		Prompt:      "A watercolor illustration of a fox reading a book.",
		Size:        "1024x1024",
		AspectRatio: "1:1",
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("image generation request sent")
}
