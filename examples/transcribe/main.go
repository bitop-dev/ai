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
	audioPath := os.Getenv("AUDIO_PATH")
	if audioPath == "" {
		fmt.Fprintln(os.Stderr, "AUDIO_PATH is required (path to an audio file)")
		os.Exit(1)
	}

	audioBytes, err := os.ReadFile(audioPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	openai.Configure(openai.Config{
		APIKey:    apiKey,
		BaseURL:   getenv("OPENAI_BASE_URL", ""),
		APIPrefix: getenv("OPENAI_API_PREFIX", ""),
	})

	model := getenv("OPENAI_TRANSCRIBE_MODEL", "whisper-1")
	tr, err := ai.Transcribe(context.Background(), ai.TranscribeRequest{
		Model:      openai.Transcription(model),
		AudioBytes: audioBytes,
		Filename:   audioPath,
		ProviderOptions: map[string]any{
			"openai": openai.TranscriptionOptions{
				ResponseFormat: "verbose_json",
			},
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println(tr.Text)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
