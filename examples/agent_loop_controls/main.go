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

	search := ai.NewTool("search", ai.ToolSpec[struct {
		Query string `json:"query"`
	}, map[string]any]{
		Description: "Search the web (fake).",
		InputSchema: ai.JSONSchema([]byte(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"],"additionalProperties":false}`)),
		Execute: func(ctx context.Context, input struct {
			Query string `json:"query"`
		}, meta ai.ToolExecutionMeta) (map[string]any, error) {
			_ = ctx
			if meta.Report != nil {
				meta.Report(map[string]any{"phase": "search", "query": input.Query})
			}
			return map[string]any{
				"results": []map[string]string{
					{"title": "Result A", "url": "https://example.com/a"},
					{"title": "Result B", "url": "https://example.com/b"},
				},
			}, nil
		},
	})

	summarize := ai.NewTool("summarize", ai.ToolSpec[struct {
		Items any `json:"items"`
	}, map[string]any]{
		Description: "Summarize search results (fake).",
		InputSchema: ai.JSONSchema([]byte(`{"type":"object","properties":{"items":{}},"required":["items"],"additionalProperties":false}`)),
		Execute: func(ctx context.Context, input struct {
			Items any `json:"items"`
		}, meta ai.ToolExecutionMeta) (map[string]any, error) {
			_ = ctx
			if meta.Report != nil {
				meta.Report(map[string]any{"phase": "summarize"})
			}
			return map[string]any{"summary": "Two example results were found."}, nil
		},
	})

	agent := ai.Agent{
		Model:         openai.Chat(getenv("OPENAI_MODEL", "gpt-4o-mini")),
		System:        "You are a concise research assistant. Use the tools when helpful.",
		Tools:         []ai.Tool{search, summarize},
		MaxIterations: 10,
		OnToolProgress: func(e ai.ToolProgressEvent) {
			b, _ := json.Marshal(e.Data)
			fmt.Printf("tool-progress %s(%s): %s\n", e.ToolName, e.ToolCallID, string(b))
		},
		OnStepFinish: func(e ai.StepFinishEvent) {
			fmt.Printf("step %d finish=%s toolCalls=%d toolResults=%d\n",
				e.Step.StepNumber,
				e.Step.FinishReason,
				len(e.Step.ToolCalls),
				len(e.Step.ToolResults),
			)
		},
		PrepareStep: func(e ai.PrepareStepEvent) (ai.PrepareStepResult, error) {
			// Phase 0: require search
			if e.StepNumber == 0 {
				return ai.PrepareStepResult{ActiveTools: []string{"search"}}, nil
			}
			// Phase 1: summarize
			if e.StepNumber == 1 {
				return ai.PrepareStepResult{ActiveTools: []string{"summarize"}}, nil
			}
			// Final phase: no tools (let the model answer)
			return ai.PrepareStepResult{ActiveTools: []string{}}, nil
		},
		StopWhen: func(e ai.StopConditionEvent) bool {
			// Stop once summarize was called at least once.
			return ai.HasToolCall("summarize")(e)
		},
	}

	resp, err := agent.Generate(context.Background(), ai.AgentGenerateRequest{
		Prompt: getenv("PROMPT", "Find two examples of Go web frameworks, then summarize."),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("\nFINAL:")
	fmt.Println(resp.Text)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
