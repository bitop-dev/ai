package ai

import (
	"context"
	"fmt"

	"github.com/bitop-dev/ai/internal/agents"
	"github.com/bitop-dev/ai/internal/provider"
	"github.com/bitop-dev/ai/internal/text"
)

func GenerateText(ctx context.Context, req GenerateTextRequest) (*GenerateTextResponse, error) {
	return generateTextFromBaseRequest(ctx, req.BaseRequest)
}

func StreamText(ctx context.Context, req StreamTextRequest) (*TextStream, error) {
	return streamTextFromBaseRequest(ctx, req.BaseRequest)
}

func generateTextFromBaseRequest(ctx context.Context, base BaseRequest) (*GenerateTextResponse, error) {
	ctx, cancel := applyTimeout(ctx, base.Timeout)
	defer cancel()

	p, err := providerForModel(base.Model)
	if err != nil {
		return nil, err
	}

	maxIter := 5
	if base.ToolLoop != nil && base.ToolLoop.MaxIterations > 0 {
		maxIter = base.ToolLoop.MaxIterations
	}

	preq, err := toProviderRequest(base)
	if err != nil {
		return nil, err
	}

	exec := func(ctx context.Context, calls []provider.ToolCallPart) ([]provider.Message, error) {
		return executeToolCallsProviderWithOptions(ctx, base.Tools, calls, toolExecOptions{
			onProgress: base.OnToolProgress,
		})
	}

	opts := text.Options{
		MaxIterations: maxIter,
	}
	if base.ToolLoop != nil && base.ToolLoop.StopWhen != nil {
		opts.StopWhen = func(event text.StopWhenEvent) bool {
			steps, err := stepsFromProviderSteps(event.Steps)
			if err != nil {
				return false
			}
			return base.ToolLoop.StopWhen(StopConditionEvent{Steps: steps})
		}
	}
	if base.PrepareStep != nil {
		opts.PrepareStep = func(event text.PrepareStepEvent) (text.PrepareStepResult, error) {
			steps, err := stepsFromProviderSteps(event.Steps)
			if err != nil {
				return text.PrepareStepResult{}, err
			}
			msgs, err := messagesFromProviderMessages(event.Messages)
			if err != nil {
				return text.PrepareStepResult{}, err
			}
			res, err := base.PrepareStep(PrepareStepEvent{
				StepNumber: event.StepNumber,
				Steps:      steps,
				Messages:   msgs,
			})
			if err != nil {
				return text.PrepareStepResult{}, err
			}
			var model string
			if res.Model != nil {
				if res.Model.Provider() != base.Model.Provider() {
					return text.PrepareStepResult{}, fmt.Errorf("PrepareStep model provider mismatch (%q != %q)", res.Model.Provider(), base.Model.Provider())
				}
				model = res.Model.Name()
			}
			var outMsgs []provider.Message
			if res.Messages != nil {
				outMsgs, err = toProviderMessages(res.Messages)
				if err != nil {
					return text.PrepareStepResult{}, err
				}
			}
			return text.PrepareStepResult{
				Model:       model,
				Messages:    outMsgs,
				ActiveTools: append([]string(nil), res.ActiveTools...),
			}, nil
		}
	}
	if base.OnStepFinish != nil {
		opts.OnStepFinish = func(event text.StepFinishEvent) {
			step, err := stepFromProviderStep(event.Step)
			if err != nil {
				return
			}
			base.OnStepFinish(StepFinishEvent{Step: step})
		}
	}

	out, err := agents.Generate(ctx, p, preq, exec, opts)
	if err != nil {
		return nil, mapProviderError(err)
	}

	msg, err := fromProviderMessage(out.Response.Message)
	if err != nil {
		return nil, err
	}

	usage := Usage{
		PromptTokens:     out.AggregatedUsage.PromptTokens,
		CompletionTokens: out.AggregatedUsage.CompletionTokens,
		TotalTokens:      out.AggregatedUsage.TotalTokens,
	}

	steps, err := stepsFromProviderSteps(out.Steps)
	if err != nil {
		return nil, err
	}
	respMsgs, err := messagesFromProviderMessages(out.ResponseMessages)
	if err != nil {
		return nil, err
	}

	return &GenerateTextResponse{
		Text:         extractTextFromMessage(msg),
		Message:      msg,
		Usage:        usage,
		FinishReason: FinishReason(out.Response.FinishReason),
		Steps:        steps,
		Response:     Response{Messages: respMsgs},
	}, nil
}

func streamTextFromBaseRequest(ctx context.Context, base BaseRequest) (*TextStream, error) {
	ctx, cancel := applyTimeout(ctx, base.Timeout)
	defer cancel()

	p, err := providerForModel(base.Model)
	if err != nil {
		return nil, err
	}

	maxIter := 5
	if base.ToolLoop != nil && base.ToolLoop.MaxIterations > 0 {
		maxIter = base.ToolLoop.MaxIterations
	}

	preq, err := toProviderRequest(base)
	if err != nil {
		return nil, err
	}

	lifecycle := newToolInputLifecycle(base.Tools)

	exec := func(ctx context.Context, calls []provider.ToolCallPart) ([]provider.Message, error) {
		return executeToolCallsProviderWithOptions(ctx, base.Tools, calls, toolExecOptions{
			toolCallIndexByID: lifecycle.toolCallIndexByID,
			onInputAvailable:  lifecycle.onInputAvailable,
			onProgress:        base.OnToolProgress,
		})
	}

	opts := text.Options{MaxIterations: maxIter}
	if base.ToolLoop != nil && base.ToolLoop.StopWhen != nil {
		opts.StopWhen = func(event text.StopWhenEvent) bool {
			steps, err := stepsFromProviderSteps(event.Steps)
			if err != nil {
				return false
			}
			return base.ToolLoop.StopWhen(StopConditionEvent{Steps: steps})
		}
	}
	if base.PrepareStep != nil {
		opts.PrepareStep = func(event text.PrepareStepEvent) (text.PrepareStepResult, error) {
			steps, err := stepsFromProviderSteps(event.Steps)
			if err != nil {
				return text.PrepareStepResult{}, err
			}
			msgs, err := messagesFromProviderMessages(event.Messages)
			if err != nil {
				return text.PrepareStepResult{}, err
			}
			res, err := base.PrepareStep(PrepareStepEvent{
				StepNumber: event.StepNumber,
				Steps:      steps,
				Messages:   msgs,
			})
			if err != nil {
				return text.PrepareStepResult{}, err
			}
			var model string
			if res.Model != nil {
				if res.Model.Provider() != base.Model.Provider() {
					return text.PrepareStepResult{}, fmt.Errorf("PrepareStep model provider mismatch (%q != %q)", res.Model.Provider(), base.Model.Provider())
				}
				model = res.Model.Name()
			}
			var outMsgs []provider.Message
			if res.Messages != nil {
				outMsgs, err = toProviderMessages(res.Messages)
				if err != nil {
					return text.PrepareStepResult{}, err
				}
			}
			return text.PrepareStepResult{
				Model:       model,
				Messages:    outMsgs,
				ActiveTools: append([]string(nil), res.ActiveTools...),
			}, nil
		}
	}
	if base.OnStepFinish != nil {
		opts.OnStepFinish = func(event text.StepFinishEvent) {
			step, err := stepFromProviderStep(event.Step)
			if err != nil {
				return
			}
			base.OnStepFinish(StepFinishEvent{Step: step})
		}
	}

	impl := agents.NewStream(ctx, p, preq, exec, opts, lifecycle.onDelta)

	var finalMsg *Message
	var cachedSteps []Step
	var cachedResp []Message
	return newTextStream(
		func() bool { return impl.Next() },
		func() string { return impl.Delta() },
		func() *Message {
			if finalMsg != nil {
				return finalMsg
			}
			final := impl.Final()
			if final == nil {
				return nil
			}
			m, err := fromProviderMessage(final.Message)
			if err != nil {
				// surface via Err()
				return nil
			}
			finalMsg = &m
			return finalMsg
		},
		func() Usage {
			u := impl.Usage()
			return Usage{PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens, TotalTokens: u.TotalTokens}
		},
		func() FinishReason {
			final := impl.Final()
			if final == nil {
				return FinishUnknown
			}
			return FinishReason(final.FinishReason)
		},
		func() []Step {
			if cachedSteps != nil {
				return append([]Step(nil), cachedSteps...)
			}
			ps := impl.Steps()
			steps, err := stepsFromProviderSteps(ps)
			if err != nil {
				return nil
			}
			cachedSteps = steps
			return append([]Step(nil), cachedSteps...)
		},
		func() Response {
			if cachedResp != nil {
				return Response{Messages: append([]Message(nil), cachedResp...)}
			}
			msgs, err := messagesFromProviderMessages(impl.ResponseMessages())
			if err != nil {
				return Response{}
			}
			cachedResp = msgs
			return Response{Messages: append([]Message(nil), cachedResp...)}
		},
		func() error { return mapProviderError(impl.Err()) },
		func() error { return impl.Close() },
	), nil
}

func providerForModel(m ModelRef) (provider.Provider, error) {
	if m == nil {
		return nil, fmt.Errorf("model is required")
	}
	name := m.Provider()
	if name == "" {
		return nil, fmt.Errorf("model provider is required")
	}
	p, ok := provider.Get(name)
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return p, nil
}
