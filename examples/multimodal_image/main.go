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

	imageURL := os.Getenv("IMAGE_URL")
	if imageURL == "" {
		imageURL = "https://upload.wikimedia.org/wikipedia/commons/thumb/3/3a/Cat03.jpg/640px-Cat03.jpg"
	}

	openai.Configure(openai.Config{
		APIKey:    apiKey,
		BaseURL:   getenv("OPENAI_BASE_URL", ""),
		APIPrefix: getenv("OPENAI_API_PREFIX", ""),
	})

	resp, err := ai.GenerateText(context.Background(), ai.GenerateTextRequest{
		BaseRequest: ai.BaseRequest{
			Model: openai.Chat(getenv("OPENAI_MODEL", "gpt-4o-mini")),
			Messages: []ai.Message{
				{
					Role: ai.RoleUser,
					Content: []ai.ContentPart{
						ai.TextPart{Text: getenv("PROMPT", "Describe the image in one sentence.")},
						ai.ImageURL(imageURL),
					},
				},
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
