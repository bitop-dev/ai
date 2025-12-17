package object

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bitop-dev/ai/internal/provider"
	"github.com/bitop-dev/ai/internal/schema"
	"github.com/bitop-dev/ai/internal/tools"
)

const ReturnToolName = "__ai_return_json"

type GenerateResult[T any] struct {
	Object T
	Raw    json.RawMessage

	LastResponse provider.Response
	Usage        provider.Usage
}

type Options struct {
	Strict        bool
	MaxRetries    int
	MaxIterations int
}

func Generate[T any](ctx context.Context, p provider.Provider, req provider.Request, exec tools.Executor, schemaJSON json.RawMessage, opts Options) (GenerateResult[T], error) {
	if len(schemaJSON) == 0 {
		return GenerateResult[T]{}, fmt.Errorf("schema is required")
	}
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = 5
	}
	if opts.MaxRetries < 0 {
		opts.MaxRetries = 0
	}
	if p == nil {
		return GenerateResult[T]{}, fmt.Errorf("provider is required")
	}
	if toolNameCollides(req.Tools, ReturnToolName) {
		return GenerateResult[T]{}, fmt.Errorf("tool name collision: %q is reserved", ReturnToolName)
	}

	// Prepare: inject system instruction + synthetic tool.
	msgs := append([]provider.Message(nil), req.Messages...)
	msgs = prependSystem(msgs)

	toolsDefs := append([]provider.ToolDefinition(nil), req.Tools...)
	toolsDefs = append(toolsDefs, provider.ToolDefinition{
		Name:        ReturnToolName,
		Description: "Return the final JSON object result.",
		InputSchema: schemaJSON,
	})

	baseReq := req
	baseReq.Messages = nil
	baseReq.Tools = nil

	var agg provider.Usage
	var last provider.Response

	// Tool loop state.
	messages := msgs

	retryCount := 0
	retryMessages := []provider.Message(nil)

	for iter := 0; iter < opts.MaxIterations; iter++ {
		callReq := baseReq
		callReq.Messages = append([]provider.Message(nil), messages...)
		callReq.Messages = append(callReq.Messages, retryMessages...)
		callReq.Tools = append([]provider.ToolDefinition(nil), toolsDefs...)

		resp, err := p.Generate(ctx, callReq)
		if err != nil {
			if errors.Is(err, provider.ErrToolsUnsupported) {
				return generateJSONOnly[T](ctx, p, baseReq, messages, schemaJSON, opts)
			}
			return GenerateResult[T]{}, err
		}

		last = resp
		agg = tools.AddUsage(agg, resp.Usage)
		messages = append(messages, resp.Message)

		if raw, ok := findReturnArgs(resp.Message); ok {
			var obj T
			if err := schema.Validate(schemaJSON, raw); err != nil {
				if !opts.Strict {
					return GenerateResult[T]{Raw: raw, LastResponse: last, Usage: agg}, err
				}
				if retryCount >= opts.MaxRetries {
					return GenerateResult[T]{}, fmt.Errorf("invalid json: %w", err)
				}
				retryCount++
				retryMessages = []provider.Message{systemText(correctionPrompt(err, raw))}
				continue
			}
			if err := json.Unmarshal(raw, &obj); err != nil {
				if !opts.Strict {
					return GenerateResult[T]{Raw: raw, LastResponse: last, Usage: agg}, err
				}
				if retryCount >= opts.MaxRetries {
					return GenerateResult[T]{}, fmt.Errorf("invalid json: %w", err)
				}
				retryCount++
				retryMessages = []provider.Message{systemText(correctionPrompt(err, raw))}
				continue
			}
			return GenerateResult[T]{Object: obj, Raw: raw, LastResponse: last, Usage: agg}, nil
		}

		calls := tools.ExtractToolCalls(resp.Message)
		if len(calls) == 0 {
			err := fmt.Errorf("model did not call %q", ReturnToolName)
			if !opts.Strict {
				return GenerateResult[T]{LastResponse: last, Usage: agg}, err
			}
			if retryCount >= opts.MaxRetries {
				return GenerateResult[T]{}, err
			}
			retryCount++
			retryMessages = []provider.Message{systemText(mustCallPrompt())}
			continue
		}

		nonReturn := filterNonReturn(calls)
		if len(nonReturn) == 0 {
			err := fmt.Errorf("model did not call %q", ReturnToolName)
			if !opts.Strict {
				return GenerateResult[T]{LastResponse: last, Usage: agg}, err
			}
			if retryCount >= opts.MaxRetries {
				return GenerateResult[T]{}, err
			}
			retryCount++
			retryMessages = []provider.Message{systemText(mustCallPrompt())}
			continue
		}

		if exec == nil {
			return GenerateResult[T]{}, fmt.Errorf("tool calls requested but no executor provided")
		}
		results, err := exec(ctx, nonReturn)
		if err != nil {
			return GenerateResult[T]{}, err
		}
		messages = append(messages, results...)
		retryMessages = nil
	}

	return GenerateResult[T]{}, fmt.Errorf("tool loop exceeded max iterations (%d)", opts.MaxIterations)
}

type Stream[T any] struct {
	ctx  context.Context
	p    provider.Provider
	exec tools.Executor

	opts Options

	schemaJSON json.RawMessage

	baseReq  provider.Request
	messages []provider.Message
	tools    []provider.ToolDefinition

	iter int
	cur  provider.Stream

	rawArgs []byte
	partial map[string]any

	finalObj *T
	finalRaw json.RawMessage
	usage    provider.Usage
	err      error
	emitted  bool
	fallback bool
}

func NewStream[T any](ctx context.Context, p provider.Provider, req provider.Request, exec tools.Executor, schemaJSON json.RawMessage, opts Options) *Stream[T] {
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = 5
	}
	if opts.MaxRetries < 0 {
		opts.MaxRetries = 0
	}

	s := &Stream[T]{
		ctx:        ctx,
		p:          p,
		exec:       exec,
		opts:       opts,
		schemaJSON: schemaJSON,
		baseReq:    req,
		messages:   append([]provider.Message(nil), req.Messages...),
		tools:      append([]provider.ToolDefinition(nil), req.Tools...),
	}
	s.baseReq.Messages = nil
	s.baseReq.Tools = nil

	s.messages = prependSystem(s.messages)
	s.tools = append(s.tools, provider.ToolDefinition{Name: ReturnToolName, Description: "Return the final JSON object result.", InputSchema: schemaJSON})

	return s
}

func (s *Stream[T]) Next() bool {
	if s.err != nil {
		return false
	}
	if s.fallback {
		if s.emitted {
			return false
		}
		s.emitted = true
		return true
	}
	if s.finalObj != nil {
		return false
	}

	for {
		if s.cur == nil {
			if err := s.start(); err != nil {
				s.err = err
				return false
			}
		}

		if s.cur.Next() {
			d := s.cur.Delta()
			if s.consumeToolDeltas(d.ToolCalls) {
				return true
			}
			continue
		}

		if err := s.cur.Err(); err != nil {
			s.err = err
			return false
		}

		final := s.cur.Final()
		_ = s.cur.Close()
		s.cur = nil

		if final == nil {
			s.err = fmt.Errorf("stream ended without final response")
			return false
		}

		s.usage = tools.AddUsage(s.usage, final.Usage)
		s.messages = append(s.messages, final.Message)

		if raw, ok := findReturnArgs(final.Message); ok {
			s.finalRaw = raw
			if err := schema.Validate(s.schemaJSON, raw); err != nil {
				if s.opts.Strict {
					s.err = err
				}
				return false
			}
			var obj T
			if err := json.Unmarshal(raw, &obj); err != nil {
				if s.opts.Strict {
					s.err = err
				}
				return false
			}
			s.finalObj = &obj
			return false
		}

		calls := tools.ExtractToolCalls(final.Message)
		nonReturn := filterNonReturn(calls)
		if len(nonReturn) == 0 {
			s.err = fmt.Errorf("model did not call %q", ReturnToolName)
			return false
		}

		if s.exec == nil {
			s.err = fmt.Errorf("tool calls requested but no executor provided")
			return false
		}
		results, err := s.exec(s.ctx, nonReturn)
		if err != nil {
			s.err = err
			return false
		}
		s.messages = append(s.messages, results...)
		s.rawArgs = nil
		s.partial = nil

		s.iter++
		if s.iter >= s.opts.MaxIterations {
			s.err = fmt.Errorf("tool loop exceeded max iterations (%d)", s.opts.MaxIterations)
			return false
		}
	}
}

func (s *Stream[T]) Raw() json.RawMessage {
	if s.fallback {
		return s.finalRaw
	}
	return append(json.RawMessage(nil), s.rawArgs...)
}

func (s *Stream[T]) Partial() map[string]any {
	return s.partial
}

func (s *Stream[T]) Object() *T {
	return s.finalObj
}

func (s *Stream[T]) Usage() provider.Usage { return s.usage }
func (s *Stream[T]) Err() error            { return s.err }
func (s *Stream[T]) Close() error {
	if s.cur != nil {
		return s.cur.Close()
	}
	return nil
}

func (s *Stream[T]) start() error {
	if s.p == nil {
		return fmt.Errorf("provider is required")
	}
	callReq := s.baseReq
	callReq.Messages = append([]provider.Message(nil), s.messages...)
	callReq.Tools = append([]provider.ToolDefinition(nil), s.tools...)

	cur, err := s.p.Stream(s.ctx, callReq)
	if err != nil {
		if errors.Is(err, provider.ErrToolsUnsupported) {
			// Non-stream fallback: run Generate and expose as a single event.
			r, err2 := Generate[T](s.ctx, s.p, callReq, s.exec, s.schemaJSON, s.opts)
			if err2 != nil {
				var pe *provider.Error
				if errors.As(err2, &pe) || s.opts.Strict {
					return err2
				}
				// Best-effort: expose whatever raw JSON we captured.
				s.finalRaw = r.Raw
				s.usage = r.Usage
				s.fallback = true
				return nil
			}
			s.finalObj = &r.Object
			s.finalRaw = r.Raw
			s.usage = r.Usage
			s.fallback = true
			return nil
		}
		return err
	}
	s.cur = cur
	return nil
}

func (s *Stream[T]) consumeToolDeltas(deltas []provider.ToolCallDelta) bool {
	var advanced bool
	for _, d := range deltas {
		if d.Name != "" && d.Name != ReturnToolName {
			continue
		}
		if d.ArgumentsDelta == "" {
			continue
		}
		s.rawArgs = append(s.rawArgs, d.ArgumentsDelta...)
		advanced = true

		if json.Valid(s.rawArgs) {
			var m map[string]any
			if err := json.Unmarshal(s.rawArgs, &m); err == nil {
				s.partial = m
			}
		}
	}
	return advanced
}

func generateJSONOnly[T any](ctx context.Context, p provider.Provider, baseReq provider.Request, messages []provider.Message, schemaJSON json.RawMessage, opts Options) (GenerateResult[T], error) {
	// JSON-only prompt injection.
	msgs := append([]provider.Message(nil), messages...)
	msgs = prependJSONOnlySystem(msgs)

	var agg provider.Usage
	var last provider.Response

	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		callReq := baseReq
		callReq.Messages = append([]provider.Message(nil), msgs...)
		callReq.Tools = nil

		resp, err := p.Generate(ctx, callReq)
		if err != nil {
			return GenerateResult[T]{}, err
		}
		last = resp
		agg = tools.AddUsage(agg, resp.Usage)

		raw := json.RawMessage(extractText(resp.Message))
		var obj T
		if err := schema.Validate(schemaJSON, raw); err != nil {
			if !opts.Strict {
				return GenerateResult[T]{Raw: raw, LastResponse: last, Usage: agg}, err
			}
			if attempt == opts.MaxRetries {
				return GenerateResult[T]{}, fmt.Errorf("invalid json: %w", err)
			}
			msgs = append(msgs, systemText(correctionPrompt(err, raw)))
			continue
		}
		if err := json.Unmarshal(raw, &obj); err != nil {
			if !opts.Strict {
				return GenerateResult[T]{Raw: raw, LastResponse: last, Usage: agg}, err
			}
			if attempt == opts.MaxRetries {
				return GenerateResult[T]{}, fmt.Errorf("invalid json: %w", err)
			}
			msgs = append(msgs, systemText(correctionPrompt(err, raw)))
			continue
		}
		return GenerateResult[T]{Object: obj, Raw: raw, LastResponse: last, Usage: agg}, nil
	}
	return GenerateResult[T]{}, fmt.Errorf("unreachable")
}

func extractText(m provider.Message) string {
	var b []byte
	for _, p := range m.Content {
		if t, ok := p.(provider.TextPart); ok {
			b = append(b, t.Text...)
		}
	}
	return string(b)
}

func toolNameCollides(tools []provider.ToolDefinition, name string) bool {
	for _, t := range tools {
		if t.Name == name {
			return true
		}
	}
	return false
}

func findReturnArgs(m provider.Message) (json.RawMessage, bool) {
	for _, p := range m.Content {
		tc, ok := p.(provider.ToolCallPart)
		if !ok {
			continue
		}
		if tc.Name == ReturnToolName {
			return tc.Args, true
		}
	}
	return nil, false
}

func filterNonReturn(calls []provider.ToolCallPart) []provider.ToolCallPart {
	out := make([]provider.ToolCallPart, 0, len(calls))
	for _, c := range calls {
		if c.Name == ReturnToolName {
			continue
		}
		out = append(out, c)
	}
	return out
}

func prependSystem(msgs []provider.Message) []provider.Message {
	sys := systemText("You must return the final result by calling the tool " + ReturnToolName + " with arguments matching the provided JSON schema. Do not return the result as plain text.")
	return append([]provider.Message{sys}, msgs...)
}

func prependJSONOnlySystem(msgs []provider.Message) []provider.Message {
	sys := systemText("Return ONLY valid JSON matching the provided schema. Do not include backticks, markdown, or any extra text.")
	return append([]provider.Message{sys}, msgs...)
}

func mustCallPrompt() string {
	return "You did not call the tool " + ReturnToolName + ". Call it with the final JSON object as arguments."
}

func correctionPrompt(err error, raw json.RawMessage) string {
	const max = 4000
	s := string(raw)
	if len(s) > max {
		s = s[:max] + "…"
	}
	return fmt.Sprintf("The previous JSON was invalid or did not match the schema.\nError:\n%s\nPrevious JSON:\n%s\nReturn ONLY corrected JSON (no extra text).", err.Error(), s)
}

func systemText(text string) provider.Message {
	return provider.Message{Role: provider.RoleSystem, Content: []provider.ContentPart{provider.TextPart{Text: text}}}
}
