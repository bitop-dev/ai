package text

import (
	"context"
	"fmt"

	"github.com/bitop-dev/ai/internal/provider"
	"github.com/bitop-dev/ai/internal/tools"
)

type GenerateResult struct {
	Response        provider.Response
	AggregatedUsage provider.Usage
}

func Generate(ctx context.Context, p provider.Provider, req provider.Request, exec tools.Executor, maxIterations int) (GenerateResult, error) {
	if maxIterations <= 0 {
		maxIterations = 5
	}
	if p == nil {
		return GenerateResult{}, fmt.Errorf("provider is required")
	}

	messages := append([]provider.Message(nil), req.Messages...)
	toolDefs := append([]provider.ToolDefinition(nil), req.Tools...)
	req.Messages = nil
	req.Tools = nil

	var agg provider.Usage

	for iter := 0; iter < maxIterations; iter++ {
		callReq := req
		callReq.Messages = append([]provider.Message(nil), messages...)
		callReq.Tools = append([]provider.ToolDefinition(nil), toolDefs...)

		resp, err := p.Generate(ctx, callReq)
		if err != nil {
			return GenerateResult{}, err
		}
		agg = tools.AddUsage(agg, resp.Usage)

		messages = append(messages, resp.Message)

		calls := tools.ExtractToolCalls(resp.Message)
		if len(calls) == 0 {
			return GenerateResult{Response: resp, AggregatedUsage: agg}, nil
		}
		if exec == nil {
			return GenerateResult{}, fmt.Errorf("tool calls requested but no executor provided")
		}
		results, err := exec(ctx, calls)
		if err != nil {
			return GenerateResult{}, err
		}
		messages = append(messages, results...)
	}

	return GenerateResult{}, fmt.Errorf("tool loop exceeded max iterations (%d)", maxIterations)
}
