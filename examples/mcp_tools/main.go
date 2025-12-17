package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bitop-dev/ai"
	"github.com/bitop-dev/ai/mcp"
	"github.com/bitop-dev/ai/openai"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY is required")
		os.Exit(1)
	}

	mcpURL := os.Getenv("MCP_URL")
	if mcpURL == "" {
		fmt.Fprintln(os.Stderr, "MCP_URL is required (e.g. http://localhost:8080/mcp)")
		os.Exit(1)
	}

	openai.Configure(openai.Config{
		APIKey:    apiKey,
		BaseURL:   getenv("OPENAI_BASE_URL", ""),
		APIPrefix: getenv("OPENAI_API_PREFIX", ""),
	})

	client, err := mcp.NewClient(mcp.ClientOptions{
		Transport: &mcp.HTTPTransport{
			URL:     mcpURL,
			Headers: map[string]string{
				// Optional: Authorization: "Bearer ...",
			},
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Close()

	tools, err := client.Tools(context.Background(), nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	resp, err := ai.GenerateText(context.Background(), ai.GenerateTextRequest{
		BaseRequest: ai.BaseRequest{
			Model: openai.Chat(getenv("OPENAI_MODEL", "gpt-5-mini")),
			Tools: tools,
			Messages: []ai.Message{
				ai.User(getenv("PROMPT", "Use tools to answer: what tools do you have?")),
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
