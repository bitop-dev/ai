package ai

import (
	"context"
	"fmt"

	"github.com/bitop-dev/ai/internal/provider"
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

	messages := append([]Message(nil), req.Messages...)
	tools := append([]Tool(nil), req.Tools...)
	baseReq := req.BaseRequest
	baseReq.Messages = nil
	baseReq.Tools = nil

	for iter := 0; ; iter++ {
		callReq := baseReq
		callReq.Model = req.Model
		callReq.Messages = append([]Message(nil), messages...)
		callReq.Tools = append([]Tool(nil), tools...)

		preq, err := toProviderRequest(callReq)
		if err != nil {
			return nil, err
		}
		resp, err := p.Generate(ctx, preq)
		if err != nil {
			return nil, mapProviderError(err)
		}

		msg, usage, finish, err := fromProviderResponse(resp)
		if err != nil {
			return nil, err
		}

		messages = append(messages, msg)
		calls := extractToolCalls(msg)
		if len(calls) == 0 {
			return &GenerateTextResponse{
				Text:         extractTextFromMessage(msg),
				Message:      msg,
				Usage:        usage,
				FinishReason: finish,
			}, nil
		}

		if iter >= maxIter-1 {
			return nil, fmt.Errorf("tool loop exceeded max iterations (%d)", maxIter)
		}

		results, err := runTools(ctx, tools, calls)
		if err != nil {
			return nil, err
		}
		messages = append(messages, results...)
	}
}

func StreamText(ctx context.Context, req StreamTextRequest) (*TextStream, error) {
	p, err := providerForModel(req.Model)
	if err != nil {
		return nil, err
	}
	return newStreamText(ctx, p, req), nil
}

func StreamObject[T any](ctx context.Context, req StreamObjectRequest[T]) (*ObjectStream[T], error) {
	p, err := providerForModel(req.Model)
	if err != nil {
		return nil, err
	}
	return newStreamObject[T](ctx, p, req), nil
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
