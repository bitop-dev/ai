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

	Steps            []Step
	ResponseMessages []provider.Message
}

func Generate(ctx context.Context, p provider.Provider, req provider.Request, exec tools.Executor, opts Options) (GenerateResult, error) {
	maxIterations := opts.MaxIterations
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
	var steps []Step
	var responseMessages []provider.Message

	for iter := 0; iter < maxIterations; iter++ {
		stepNumber := iter

		stepReq := req
		stepMessages := append([]provider.Message(nil), messages...)
		stepTools := append([]provider.ToolDefinition(nil), toolDefs...)
		activeTools := []string(nil)

		if opts.PrepareStep != nil {
			res, err := opts.PrepareStep(PrepareStepEvent{
				StepNumber: stepNumber,
				Steps:      append([]Step(nil), steps...),
				Request:    stepReq,
				Messages:   append([]provider.Message(nil), stepMessages...),
				Tools:      append([]provider.ToolDefinition(nil), stepTools...),
			})
			if err != nil {
				return GenerateResult{}, err
			}
			if res.Model != "" {
				stepReq.Model = res.Model
				req.Model = res.Model
			}
			if res.Messages != nil {
				stepMessages = append([]provider.Message(nil), res.Messages...)
				messages = append([]provider.Message(nil), res.Messages...)
			}
			if res.ActiveTools != nil {
				activeTools = append([]string(nil), res.ActiveTools...)
			}
		}

		callTools := stepTools
		if len(activeTools) > 0 {
			allowed := make(map[string]struct{}, len(activeTools))
			for _, n := range activeTools {
				allowed[n] = struct{}{}
			}
			filtered := make([]provider.ToolDefinition, 0, len(stepTools))
			for _, td := range stepTools {
				if _, ok := allowed[td.Name]; ok {
					filtered = append(filtered, td)
				}
			}
			callTools = filtered
		}

		callReq := req
		callReq.Model = stepReq.Model
		callReq.Messages = append([]provider.Message(nil), stepMessages...)
		callReq.Tools = append([]provider.ToolDefinition(nil), callTools...)

		resp, err := p.Generate(ctx, callReq)
		if err != nil {
			return GenerateResult{}, err
		}
		agg = tools.AddUsage(agg, resp.Usage)

		messages = append(messages, resp.Message)
		responseMessages = append(responseMessages, resp.Message)

		calls := tools.ExtractToolCalls(resp.Message)
		step := Step{
			StepNumber:  stepNumber,
			Response:    resp,
			ToolCalls:   append([]provider.ToolCallPart(nil), calls...),
			ActiveTools: activeTools,
		}
		if len(calls) == 0 {
			steps = append(steps, step)
			if opts.OnStepFinish != nil {
				opts.OnStepFinish(StepFinishEvent{Step: step})
			}
			return GenerateResult{
				Response:         resp,
				AggregatedUsage:  agg,
				Steps:            steps,
				ResponseMessages: responseMessages,
			}, nil
		}
		if exec == nil {
			return GenerateResult{}, fmt.Errorf("tool calls requested but no executor provided")
		}
		results, err := exec(ctx, calls)
		if err != nil {
			return GenerateResult{}, err
		}
		messages = append(messages, results...)
		responseMessages = append(responseMessages, results...)
		step.ToolResults = append([]provider.Message(nil), results...)
		steps = append(steps, step)
		if opts.OnStepFinish != nil {
			opts.OnStepFinish(StepFinishEvent{Step: step})
		}

		if opts.StopWhen != nil {
			if opts.StopWhen(StopWhenEvent{
				StepNumber: stepNumber,
				Steps:      append([]Step(nil), steps...),
				Messages:   append([]provider.Message(nil), messages...),
			}) {
				return GenerateResult{
					Response:         resp,
					AggregatedUsage:  agg,
					Steps:            steps,
					ResponseMessages: responseMessages,
				}, nil
			}
		}
	}

	return GenerateResult{}, fmt.Errorf("tool loop exceeded max iterations (%d)", maxIterations)
}
