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
