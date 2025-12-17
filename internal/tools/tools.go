package tools

import (
	"context"

	"github.com/bitop-dev/ai/internal/provider"
)

type Executor func(ctx context.Context, calls []provider.ToolCallPart) ([]provider.Message, error)

func ExtractToolCalls(m provider.Message) []provider.ToolCallPart {
	var out []provider.ToolCallPart
	for _, p := range m.Content {
		tc, ok := p.(provider.ToolCallPart)
		if !ok {
			continue
		}
		out = append(out, tc)
	}
	return out
}

func AddUsage(a, b provider.Usage) provider.Usage {
	return provider.Usage{
		PromptTokens:     a.PromptTokens + b.PromptTokens,
		CompletionTokens: a.CompletionTokens + b.CompletionTokens,
		TotalTokens:      a.TotalTokens + b.TotalTokens,
	}
}
