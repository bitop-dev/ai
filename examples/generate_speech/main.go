package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bitop-dev/ai"
	"github.com/bitop-dev/ai/openai"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY is required")
		os.Exit(1)
	}

	openai.Configure(openai.Config{
		APIKey:    apiKey,
		BaseURL:   getenv("OPENAI_BASE_URL", ""),
		APIPrefix: getenv("OPENAI_API_PREFIX", ""),
	})

	model := getenv("OPENAI_SPEECH_MODEL", "tts-1")
	text := getenv("TEXT", "Hello, world!")
	voice := getenv("VOICE", "alloy")
	outPath := getenv("OUT", "speech.mp3")

	audio, err := ai.GenerateSpeech(context.Background(), ai.GenerateSpeechRequest{
		Model: openai.Speech(model),
		Text:  text,
		Voice: voice,
		ProviderOptions: map[string]any{
			"openai": openai.SpeechOptions{
				Format: getenv("FORMAT", "mp3"),
			},
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := os.WriteFile(outPath, audio.AudioData, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%s)\n", outPath, audio.MediaType)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
