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

	add := ai.NewTool("add", ai.ToolSpec[struct {
		A int `json:"a"`
		B int `json:"b"`
	}, map[string]int]{
		Description: "Add two integers.",
		InputSchema: ai.JSONSchema([]byte(`{"type":"object","properties":{"a":{"type":"integer"},"b":{"type":"integer"}},"required":["a","b"],"additionalProperties":false}`)),
		Execute: func(ctx context.Context, input struct {
			A int `json:"a"`
			B int `json:"b"`
		}, meta ai.ToolExecutionMeta) (map[string]int, error) {
			_ = ctx
			_ = meta
			return map[string]int{"result": input.A + input.B}, nil
		},
	})

	add.OnInputStart = func(e ai.ToolInputStartEvent) {
		fmt.Printf("\nTOOL INPUT START: tool=%s id=%s index=%d\n", e.ToolName, e.ToolCallID, e.ToolCallIndex)
	}
	add.OnInputDelta = func(e ai.ToolInputDeltaEvent) {
		fmt.Printf("TOOL INPUT DELTA: %q\n", e.InputTextDelta)
	}
	add.OnInputAvailable = func(e ai.ToolInputAvailableEvent) {
		fmt.Printf("TOOL INPUT AVAILABLE: %s\n\n", string(e.Input))
	}

	stream, err := ai.StreamText(context.Background(), ai.StreamTextRequest{
		BaseRequest: ai.BaseRequest{
			Model: openai.Chat(getenv("OPENAI_MODEL", "gpt-4o-mini")),
			Messages: []ai.Message{
				ai.User(getenv("PROMPT", "Use the add tool to add 123 and 456. Then respond with just the final number.")),
			},
			Tools:    []ai.Tool{add},
			ToolLoop: &ai.ToolLoopOptions{MaxIterations: 5},
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer stream.Close()

	for stream.Next() {
		fmt.Print(stream.Delta())
	}
	if err := stream.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println()
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
