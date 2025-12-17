package main

import (
	"context"
	"encoding/json"
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

	var history []ai.Message
	history = append(history, ai.User(getenv("PROMPT", "Use the add tool to add 1 and 2, then reply with just the final number.")))

	add := ai.NewTool("add", ai.ToolSpec[struct {
		A int `json:"a"`
		B int `json:"b"`
	}, map[string]int]{
		InputSchema: ai.JSONSchema([]byte(`{"type":"object","properties":{"a":{"type":"integer"},"b":{"type":"integer"}},"required":["a","b"],"additionalProperties":false}`)),
		Execute: func(ctx context.Context, input struct {
			A int `json:"a"`
			B int `json:"b"`
		}, meta ai.ToolExecutionMeta) (map[string]int, error) {
			_ = ctx
			if meta.Report != nil {
				meta.Report(map[string]any{"status": "adding"})
			}
			return map[string]int{"result": input.A + input.B}, nil
		},
	})

	agent := ai.Agent{
		Model:         openai.Chat(getenv("OPENAI_MODEL", "gpt-4o-mini")),
		Tools:         []ai.Tool{add},
		MaxIterations: 5,
		OnToolProgress: func(e ai.ToolProgressEvent) {
			b, _ := json.Marshal(e.Data)
			fmt.Printf("tool-progress %s(%s): %s\n", e.ToolName, e.ToolCallID, string(b))
		},
		OnStepFinish: func(e ai.StepFinishEvent) {
			fmt.Printf("step %d finish=%s usage=%d toolCalls=%d toolResults=%d\n",
				e.Step.StepNumber,
				e.Step.FinishReason,
				e.Step.Usage.TotalTokens,
				len(e.Step.ToolCalls),
				len(e.Step.ToolResults),
			)
		},
	}

	resp, err := agent.Generate(context.Background(), ai.AgentGenerateRequest{
		Messages: history,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Conversation continuation: append assistant/tool messages produced by the call.
	history = append(history, resp.Response.Messages...)

	fmt.Println(resp.Text)
	_ = history
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
