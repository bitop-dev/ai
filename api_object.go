package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	internalObject "github.com/bitop-dev/ai/internal/object"
	"github.com/bitop-dev/ai/internal/provider"
)

func GenerateObject[T any](ctx context.Context, req GenerateObjectRequest[T]) (*GenerateObjectResponse[T], error) {
	p, err := providerForModel(req.Model)
	if err != nil {
		return nil, err
	}

	if len(req.Schema.JSON) == 0 && req.Example == nil {
		return nil, fmt.Errorf("schema (or example) is required")
	}
	if req.Example != nil && len(req.Schema.JSON) == 0 {
		return nil, fmt.Errorf("schema derivation from example is not implemented in v0")
	}

	strict := true
	if req.Strict != nil {
		strict = *req.Strict
	}

	maxRetries := 1
	if req.MaxRetries != nil {
		maxRetries = *req.MaxRetries
	}
	if maxRetries < 0 {
		maxRetries = 0
	}

	maxIter := 5
	if req.ToolLoop != nil && req.ToolLoop.MaxIterations > 0 {
		maxIter = req.ToolLoop.MaxIterations
	}

	callReq := req.BaseRequest
	callReq.Model = req.Model
	callReq.Messages = append([]Message(nil), req.Messages...)
	callReq.Tools = append([]Tool(nil), req.Tools...)

	preq, err := toProviderRequest(callReq)
	if err != nil {
		return nil, err
	}

	exec := func(ctx context.Context, calls []provider.ToolCallPart) ([]provider.Message, error) {
		return executeToolCallsProvider(ctx, req.Tools, calls)
	}

	out, genErr := internalObject.Generate[T](ctx, p, preq, exec, req.Schema.JSON, internalObject.Options{
		Strict:        strict,
		MaxRetries:    maxRetries,
		MaxIterations: maxIter,
	})

	if genErr != nil {
		var pe *provider.Error
		if errors.As(genErr, &pe) {
			return nil, mapProviderError(genErr)
		}
		if strict {
			return nil, genErr
		}
		resp := &GenerateObjectResponse[T]{
			Object:          out.Object,
			RawJSON:         out.Raw,
			Usage:           Usage{PromptTokens: out.Usage.PromptTokens, CompletionTokens: out.Usage.CompletionTokens, TotalTokens: out.Usage.TotalTokens},
			ValidationError: genErr,
		}
		if out.LastResponse.Message.Role != "" {
			msg, _, finish, err := fromProviderResponse(out.LastResponse)
			if err != nil {
				return nil, err
			}
			resp.Message = msg
			resp.FinishReason = finish
		}
		return resp, nil
	}

	msg, _, finish, err := fromProviderResponse(out.LastResponse)
	if err != nil {
		return nil, err
	}
	return &GenerateObjectResponse[T]{
		Object:       out.Object,
		RawJSON:      out.Raw,
		Message:      msg,
		Usage:        Usage{PromptTokens: out.Usage.PromptTokens, CompletionTokens: out.Usage.CompletionTokens, TotalTokens: out.Usage.TotalTokens},
		FinishReason: finish,
	}, nil
}

func StreamObject[T any](ctx context.Context, req StreamObjectRequest[T]) (*ObjectStream[T], error) {
	p, err := providerForModel(req.Model)
	if err != nil {
		return nil, err
	}

	if len(req.Schema.JSON) == 0 && req.Example == nil {
		return nil, fmt.Errorf("schema (or example) is required")
	}
	if req.Example != nil && len(req.Schema.JSON) == 0 {
		return nil, fmt.Errorf("schema derivation from example is not implemented in v0")
	}

	strict := true
	if req.Strict != nil {
		strict = *req.Strict
	}

	maxRetries := 1
	if req.MaxRetries != nil {
		maxRetries = *req.MaxRetries
	}
	if maxRetries < 0 {
		maxRetries = 0
	}

	maxIter := 5
	if req.ToolLoop != nil && req.ToolLoop.MaxIterations > 0 {
		maxIter = req.ToolLoop.MaxIterations
	}

	callReq := req.BaseRequest
	callReq.Model = req.Model
	callReq.Messages = append([]Message(nil), req.Messages...)
	callReq.Tools = append([]Tool(nil), req.Tools...)

	preq, err := toProviderRequest(callReq)
	if err != nil {
		return nil, err
	}

	exec := func(ctx context.Context, calls []provider.ToolCallPart) ([]provider.Message, error) {
		return executeToolCallsProvider(ctx, req.Tools, calls)
	}

	impl := internalObject.NewStream[T](ctx, p, preq, exec, req.Schema.JSON, internalObject.Options{
		Strict:        strict,
		MaxRetries:    maxRetries,
		MaxIterations: maxIter,
	})

	return newObjectStream[T](
		func() bool { return impl.Next() },
		func() json.RawMessage { return impl.Raw() },
		func() map[string]any { return impl.Partial() },
		func() *T { return impl.Object() },
		func() error { return mapProviderError(impl.Err()) },
		func() error { return impl.Close() },
	), nil
}
