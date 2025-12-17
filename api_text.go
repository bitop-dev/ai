package ai

import (
	"context"
	"fmt"

	"github.com/bitop-dev/ai/internal/provider"
	"github.com/bitop-dev/ai/internal/text"
)

func GenerateText(ctx context.Context, req GenerateTextRequest) (*GenerateTextResponse, error) {
	p, err := providerForModel(req.Model)
	if err != nil {
		return nil, err
	}

	maxIter := 5
	if req.ToolLoop != nil && req.ToolLoop.MaxIterations > 0 {
		maxIter = req.ToolLoop.MaxIterations
	}

	preq, err := toProviderRequest(req.BaseRequest)
	if err != nil {
		return nil, err
	}

	exec := func(ctx context.Context, calls []provider.ToolCallPart) ([]provider.Message, error) {
		return executeToolCallsProvider(ctx, req.Tools, calls)
	}

	out, err := text.Generate(ctx, p, preq, exec, maxIter)
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

	return &GenerateTextResponse{
		Text:         extractTextFromMessage(msg),
		Message:      msg,
		Usage:        usage,
		FinishReason: FinishReason(out.Response.FinishReason),
	}, nil
}

func StreamText(ctx context.Context, req StreamTextRequest) (*TextStream, error) {
	p, err := providerForModel(req.Model)
	if err != nil {
		return nil, err
	}

	maxIter := 5
	if req.ToolLoop != nil && req.ToolLoop.MaxIterations > 0 {
		maxIter = req.ToolLoop.MaxIterations
	}

	preq, err := toProviderRequest(req.BaseRequest)
	if err != nil {
		return nil, err
	}

	exec := func(ctx context.Context, calls []provider.ToolCallPart) ([]provider.Message, error) {
		return executeToolCallsProvider(ctx, req.Tools, calls)
	}

	impl := text.NewStream(ctx, p, preq, exec, maxIter)

	var finalMsg *Message
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
