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

	resp, err := ai.GenerateText(context.Background(), ai.GenerateTextRequest{
		BaseRequest: ai.BaseRequest{
			Model: openai.Chat(getenv("OPENAI_MODEL", "gpt-5-mini")),
			Messages: []ai.Message{
				ai.User(getenv("PROMPT", "Invent a new holiday and describe its traditions.")),
			},
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(resp.Text)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
