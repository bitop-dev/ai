package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bitop-dev/ai/internal/provider"
)

const returnJSONToolName = "__ai_return_json"

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

	toolLoopMax := 5
	if req.ToolLoop != nil && req.ToolLoop.MaxIterations > 0 {
		toolLoopMax = req.ToolLoop.MaxIterations
	}

	if nameCollides(req.Tools, returnJSONToolName) {
		return nil, fmt.Errorf("tool name collision: %q is reserved", returnJSONToolName)
	}

	resp, err := generateObjectWithTools[T](ctx, p, req, strict, maxRetries, toolLoopMax)
	if err == nil {
		return resp, nil
	}
	if errors.Is(err, provider.ErrToolsUnsupported) {
		return generateObjectJSONOnly[T](ctx, p, req, strict, maxRetries)
	}
	return nil, mapProviderError(err)
}

func generateObjectWithTools[T any](ctx context.Context, p provider.Provider, req GenerateObjectRequest[T], strict bool, maxRetries int, toolLoopMax int) (*GenerateObjectResponse[T], error) {
	baseReq := req.BaseRequest
	baseReq.Messages = nil
	baseReq.Tools = nil

	tools := append([]Tool(nil), req.Tools...)
	tools = append(tools, Tool{
		Name:        returnJSONToolName,
		Description: "Return the final JSON object result.",
		InputSchema: req.Schema,
		Handler: func(ctx context.Context, input json.RawMessage) (any, error) {
			// Never executed; GenerateObject captures tool-call args directly.
			return nil, nil
		},
	})

	messages := append([]Message(nil), req.Messages...)
	messages = prependGenerateObjectSystem(messages)

	retryMessages := []Message(nil)
	var lastMsg Message
	var aggUsage Usage
	var lastFinish FinishReason
	var lastValidationErr error

	attempt := 0
	iter := 0
	for {
		callReq := baseReq
		callReq.Model = req.Model
		callReq.Messages = append([]Message(nil), messages...)
		callReq.Messages = append(callReq.Messages, retryMessages...)
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

		aggUsage = addUsage(aggUsage, usage)
		lastMsg, lastFinish = msg, finish
		messages = append(messages, msg)

		if raw, ok := findReturnToolArgs(msg); ok {
			obj, raw, vErr, err := decodeAndValidate[T](req.Schema, raw)
			lastValidationErr = vErr
			if err == nil {
				return &GenerateObjectResponse[T]{
					Object:       obj,
					RawJSON:      raw,
					Message:      lastMsg,
					Usage:        aggUsage,
					FinishReason: lastFinish,
				}, nil
			}

			if !strict {
				return &GenerateObjectResponse[T]{
					RawJSON:         raw,
					Message:         lastMsg,
					Usage:           aggUsage,
					FinishReason:    lastFinish,
					ValidationError: err,
				}, nil
			}

			if attempt >= maxRetries {
				return nil, fmt.Errorf("GenerateObject: invalid json: %w", err)
			}
			attempt++
			retryMessages = []Message{System(generateCorrectionPrompt(err, raw))}
			continue
		}

		calls := extractToolCalls(msg)
		if len(calls) == 0 {
			lastValidationErr = fmt.Errorf("model did not call %q", returnJSONToolName)
			if !strict {
				return &GenerateObjectResponse[T]{
					Message:         lastMsg,
					Usage:           aggUsage,
					FinishReason:    lastFinish,
					ValidationError: lastValidationErr,
				}, nil
			}
			if attempt >= maxRetries {
				return nil, lastValidationErr
			}
			attempt++
			retryMessages = []Message{System(generateMustCallToolPrompt())}
			continue
		}

		nonReturnCalls := filterNonReturnToolCalls(calls)
		if len(nonReturnCalls) == 0 {
			lastValidationErr = fmt.Errorf("model called %d tools but none were %q", len(calls), returnJSONToolName)
			if !strict {
				return &GenerateObjectResponse[T]{
					Message:         lastMsg,
					Usage:           aggUsage,
					FinishReason:    lastFinish,
					ValidationError: lastValidationErr,
				}, nil
			}
			if attempt >= maxRetries {
				return nil, lastValidationErr
			}
			attempt++
			retryMessages = []Message{System(generateMustCallToolPrompt())}
			continue
		}

		if iter >= toolLoopMax-1 {
			return nil, fmt.Errorf("tool loop exceeded max iterations (%d)", toolLoopMax)
		}
		iter++

		results, err := runTools(ctx, req.Tools, nonReturnCalls)
		if err != nil {
			return nil, err
		}
		messages = append(messages, results...)
		retryMessages = nil
	}
}

func generateObjectJSONOnly[T any](ctx context.Context, p provider.Provider, req GenerateObjectRequest[T], strict bool, maxRetries int) (*GenerateObjectResponse[T], error) {
	baseReq := req.BaseRequest
	baseReq.Messages = nil
	baseReq.Tools = nil

	messages := append([]Message(nil), req.Messages...)
	messages = prependGenerateObjectJSONOnlySystem(messages, req.Schema)

	var lastMsg Message
	var aggUsage Usage
	var lastFinish FinishReason
	var lastRaw json.RawMessage

	for attempt := 0; ; attempt++ {
		callReq := baseReq
		callReq.Model = req.Model
		callReq.Messages = append([]Message(nil), messages...)
		callReq.Tools = nil

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
		aggUsage = addUsage(aggUsage, usage)
		lastMsg, lastFinish = msg, finish

		raw := json.RawMessage(extractTextFromMessage(msg))
		lastRaw = raw

		obj, raw, _, err := decodeAndValidate[T](req.Schema, raw)
		if err == nil {
			return &GenerateObjectResponse[T]{
				Object:       obj,
				RawJSON:      raw,
				Message:      lastMsg,
				Usage:        aggUsage,
				FinishReason: lastFinish,
			}, nil
		}

		if !strict {
			return &GenerateObjectResponse[T]{
				RawJSON:         raw,
				Message:         lastMsg,
				Usage:           aggUsage,
				FinishReason:    lastFinish,
				ValidationError: err,
			}, nil
		}

		if attempt >= maxRetries {
			return nil, fmt.Errorf("GenerateObject: invalid json: %w", err)
		}
		messages = append(messages, System(generateCorrectionPrompt(err, lastRaw)))
	}
}

func decodeAndValidate[T any](schema Schema, raw json.RawMessage) (T, json.RawMessage, error, error) {
	var zero T
	if len(raw) == 0 {
		return zero, raw, fmt.Errorf("empty json"), fmt.Errorf("empty json")
	}
	if err := validateJSONAgainstSchema(schema, raw); err != nil {
		return zero, raw, err, err
	}
	var obj T
	if err := json.Unmarshal(raw, &obj); err != nil {
		return zero, raw, err, err
	}
	return obj, raw, nil, nil
}

func findReturnToolArgs(m Message) (json.RawMessage, bool) {
	for _, p := range m.Content {
		tc, ok := p.(ToolCallPart)
		if !ok {
			continue
		}
		if tc.Name == returnJSONToolName {
			return tc.Args, true
		}
	}
	return nil, false
}

func filterNonReturnToolCalls(calls []toolCall) []toolCall {
	out := make([]toolCall, 0, len(calls))
	for _, c := range calls {
		if c.Name == returnJSONToolName {
			continue
		}
		out = append(out, c)
	}
	return out
}

func prependGenerateObjectSystem(msgs []Message) []Message {
	sys := System("You must return the final result by calling the tool " + returnJSONToolName + " with arguments matching the provided JSON schema. Do not return the result as plain text.")
	return append([]Message{sys}, msgs...)
}

func prependGenerateObjectJSONOnlySystem(msgs []Message, schema Schema) []Message {
	sys := System("Return ONLY valid JSON matching the provided schema. Do not include backticks, markdown, or any extra text.")
	return append([]Message{sys}, msgs...)
}

func generateMustCallToolPrompt() string {
	return "You did not call the tool " + returnJSONToolName + ". Call it with the final JSON object as arguments."
}

func generateCorrectionPrompt(err error, raw json.RawMessage) string {
	const max = 4000
	s := string(raw)
	if len(s) > max {
		s = s[:max] + "…"
	}
	return fmt.Sprintf("The previous JSON was invalid or did not match the schema.\nError:\n%s\nPrevious JSON:\n%s\nReturn ONLY corrected JSON (no extra text).", err.Error(), s)
}

func nameCollides(tools []Tool, name string) bool {
	for _, t := range tools {
		if t.Name == name {
			return true
		}
	}
	return false
}
