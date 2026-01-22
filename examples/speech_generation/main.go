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
	model, err := openaiProvider.SpeechModel("gpt-4o-mini-tts")
	if err != nil {
		log.Fatal(err)
	}

	_, err = ai.GenerateSpeech(ctx, model, provider.SpeechModelV3CallOptions{
		Text:         "Hello! This is a quick text-to-speech demo.",
		Voice:        "alloy",
		OutputFormat: "mp3",
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("speech generation request sent")
}
