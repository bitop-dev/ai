package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bitop-dev/ai/internal/provider"
)

type streamObject[T any] struct {
	ctx context.Context
	p   provider.Provider

	req         StreamObjectRequest[T]
	toolLoopMax int

	tools    []Tool
	messages []Message
	baseReq  BaseRequest

	iter int

	cur provider.Stream

	rawArgs  []byte
	partial  map[string]any
	finalObj *T
	finalRaw json.RawMessage
	err      error
	closed   bool
	emitted  bool
	fallback bool
}

func newStreamObject[T any](ctx context.Context, p provider.Provider, req StreamObjectRequest[T]) *ObjectStream[T] {
	so := &streamObject[T]{
		ctx:      ctx,
		p:        p,
		req:      req,
		tools:    append([]Tool(nil), req.Tools...),
		messages: append([]Message(nil), req.Messages...),
		baseReq:  req.BaseRequest,
	}
	so.baseReq.Messages = nil
	so.baseReq.Tools = nil

	max := 5
	if req.ToolLoop != nil && req.ToolLoop.MaxIterations > 0 {
		max = req.ToolLoop.MaxIterations
	}
	so.toolLoopMax = max

	return newObjectStream[T](so.next, so.raw, so.partialObject, so.object, so.streamErr, so.close)
}

func (s *streamObject[T]) next() bool {
	if s.err != nil || s.closed {
		return false
	}
	if s.finalObj != nil || s.fallback {
		if s.emitted {
			return false
		}
		s.emitted = true
		return true
	}

	for {
		if s.cur == nil {
			if err := s.startStream(); err != nil {
				s.err = err
				return false
			}
		}

		if s.cur.Next() {
			d := s.cur.Delta()
			if s.consumeToolDeltas(d.ToolCalls) {
				return true
			}
			// Ignore text deltas for object streaming.
			continue
		}

		if err := s.cur.Err(); err != nil {
			s.err = mapProviderError(err)
			return false
		}

		final := s.cur.Final()
		_ = s.cur.Close()
		s.cur = nil

		if final == nil {
			s.err = fmt.Errorf("stream ended without final response")
			return false
		}

		msg, err := fromProviderMessage(final.Message)
		if err != nil {
			s.err = err
			return false
		}
		s.messages = append(s.messages, msg)

		if raw, ok := findReturnToolArgs(msg); ok {
			s.finalRaw = raw

			if err := validateJSONAgainstSchema(s.req.Schema, raw); err != nil {
				s.err = err
				return false
			}

			var obj T
			if err := json.Unmarshal(raw, &obj); err != nil {
				s.err = err
				return false
			}
			s.finalObj = &obj
			return false
		}

		calls := extractToolCalls(msg)
		if len(calls) == 0 {
			s.err = fmt.Errorf("model did not call %q", returnJSONToolName)
			return false
		}

		nonReturnCalls := filterNonReturnToolCalls(calls)
		if len(nonReturnCalls) == 0 {
			s.err = fmt.Errorf("model did not call %q", returnJSONToolName)
			return false
		}

		if s.iter >= s.toolLoopMax-1 {
			s.err = fmt.Errorf("tool loop exceeded max iterations (%d)", s.toolLoopMax)
			return false
		}
		s.iter++

		results, err := runTools(s.ctx, s.req.Tools, nonReturnCalls)
		if err != nil {
			s.err = err
			return false
		}
		s.messages = append(s.messages, results...)
		s.rawArgs = nil
		s.partial = nil
	}
}

func (s *streamObject[T]) consumeToolDeltas(deltas []provider.ToolCallDelta) bool {
	var advanced bool
	for _, d := range deltas {
		if d.Name != "" && d.Name != returnJSONToolName {
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

func (s *streamObject[T]) raw() json.RawMessage {
	if s.fallback {
		return s.finalRaw
	}
	return append(json.RawMessage(nil), s.rawArgs...)
}

func (s *streamObject[T]) partialObject() map[string]any {
	if s.fallback && s.finalObj != nil {
		// Best-effort: expose the decoded object as a map too.
		b, _ := json.Marshal(s.finalObj)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		return m
	}
	return s.partial
}

func (s *streamObject[T]) object() *T {
	return s.finalObj
}

func (s *streamObject[T]) streamErr() error { return s.err }

func (s *streamObject[T]) close() error {
	s.closed = true
	if s.cur != nil {
		return s.cur.Close()
	}
	return nil
}

func (s *streamObject[T]) startStream() error {
	if len(s.req.Schema.JSON) == 0 && s.req.Example == nil {
		return fmt.Errorf("schema (or example) is required")
	}
	if s.req.Example != nil && len(s.req.Schema.JSON) == 0 {
		return fmt.Errorf("schema derivation from example is not implemented in v0")
	}
	if nameCollides(s.req.Tools, returnJSONToolName) {
		return fmt.Errorf("tool name collision: %q is reserved", returnJSONToolName)
	}

	msgs := append([]Message(nil), s.messages...)
	msgs = prependGenerateObjectSystem(msgs)

	tools := append([]Tool(nil), s.tools...)
	tools = append(tools, Tool{
		Name:        returnJSONToolName,
		Description: "Return the final JSON object result.",
		InputSchema: s.req.Schema,
		Handler: func(ctx context.Context, input json.RawMessage) (any, error) {
			return nil, nil
		},
	})

	req := s.baseReq
	req.Model = s.req.Model
	req.Messages = msgs
	req.Tools = tools

	preq, err := toProviderRequest(req)
	if err != nil {
		return err
	}
	cur, err := s.p.Stream(s.ctx, preq)
	if err != nil {
		if errors.Is(err, provider.ErrToolsUnsupported) {
			// Fallback: compute object via non-stream JSON-only path and expose as a single-event stream.
			r, err2 := GenerateObject[T](s.ctx, GenerateObjectRequest[T](s.req))
			if err2 != nil {
				return err2
			}
			obj := r.Object
			s.finalObj = &obj
			s.finalRaw = r.RawJSON
			s.fallback = true
			return nil
		}
		return mapProviderError(err)
	}
	s.cur = cur
	return nil
}
