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

	model := getenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
	input := getenv("INPUT", "hello world")

	dims := 0
	if v := os.Getenv("DIMENSIONS"); v != "" {
		_, _ = fmt.Sscanf(v, "%d", &dims)
	}

	var opts map[string]any
	if dims > 0 {
		opts = map[string]any{"openai": openai.EmbeddingOptions{Dimensions: &dims}}
	}

	resp, err := ai.Embed(context.Background(), ai.EmbedRequest{
		Model:           openai.Embed(model),
		Input:           input,
		ProviderOptions: opts,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("dims=%d\n", len(resp.Vector))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
