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

	model := getenv("OPENAI_IMAGE_MODEL", "dall-e-3")
	prompt := getenv("PROMPT", "Santa Claus driving a Cadillac")
	outPath := getenv("OUT", "image.png")

	resp, err := ai.GenerateImage(context.Background(), ai.GenerateImageRequest{
		Model:  openai.Image(model),
		Prompt: prompt,
		Size:   getenv("SIZE", "1024x1024"),
		N:      1,
		ProviderOptions: map[string]any{
			"openai": openai.ImageOptions{
				Quality: getenv("QUALITY", ""),
				Style:   getenv("STYLE", ""),
			},
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := os.WriteFile(outPath, resp.Image.Uint8Array, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n", outPath)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
