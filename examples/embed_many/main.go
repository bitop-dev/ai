package main

import (
	"context"
	"fmt"
	"os"
	"strings"

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
	raw := getenv("INPUTS", "hello world|goodbye world")
	inputs := strings.Split(raw, "|")

	resp, err := ai.EmbedMany(context.Background(), ai.EmbedManyRequest{
		Model:            openai.Embed(model),
		Input:            inputs,
		MaxParallelCalls: atoi(getenv("MAX_PARALLEL_CALLS", "1")),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("count=%d dims=%d\n", len(resp.Vectors), len(resp.Vectors[0]))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func atoi(v string) int {
	var n int
	_, _ = fmt.Sscanf(v, "%d", &n)
	return n
}
