package ai

import (
	"context"
	"fmt"

	"github.com/bitop-dev/ai/internal/provider"
)

type streamText struct {
	ctx context.Context
	p   provider.Provider

	toolLoopMax int
	tools       []Tool

	model    ModelRef
	messages []Message
	baseReq  BaseRequest

	iter int

	cur provider.Stream

	curDelta string
	finalMsg *Message
	aggUsage Usage
	finish   FinishReason
	err      error
	closed   bool
}

func newStreamText(ctx context.Context, p provider.Provider, req StreamTextRequest) *TextStream {
	st := &streamText{
		ctx:      ctx,
		p:        p,
		model:    req.Model,
		messages: append([]Message(nil), req.Messages...),
		tools:    append([]Tool(nil), req.Tools...),
		baseReq:  req.BaseRequest,
	}
	st.baseReq.Messages = nil
	st.baseReq.Tools = nil

	max := 5
	if req.ToolLoop != nil && req.ToolLoop.MaxIterations > 0 {
		max = req.ToolLoop.MaxIterations
	}
	st.toolLoopMax = max

	return newTextStream(st.next, st.delta, st.message, st.usage, st.finishReason, st.streamErr, st.close)
}

func (s *streamText) next() bool {
	if s.err != nil || s.closed {
		return false
	}
	if s.finalMsg != nil {
		return false
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
			s.curDelta = d.Text
			if s.curDelta == "" {
				continue
			}
			return true
		}

		if err := s.cur.Err(); err != nil {
			s.err = mapProviderError(err)
			return false
		}

		final := s.cur.Final()
		_ = s.cur.Close()
		s.cur = nil

		if final == nil {
			s.finalMsg = &Message{Role: RoleAssistant}
			return false
		}

		msg, err := fromProviderMessage(final.Message)
		if err != nil {
			s.err = err
			return false
		}

		s.aggUsage = addUsage(s.aggUsage, Usage{
			PromptTokens:     final.Usage.PromptTokens,
			CompletionTokens: final.Usage.CompletionTokens,
			TotalTokens:      final.Usage.TotalTokens,
		})
		s.finish = FinishReason(final.FinishReason)

		s.messages = append(s.messages, msg)

		toolCalls := extractToolCalls(msg)
		if len(toolCalls) == 0 {
			s.finalMsg = &msg
			return false
		}

		if err := s.runTools(toolCalls); err != nil {
			s.err = err
			return false
		}

		s.iter++
		if s.iter >= s.toolLoopMax {
			s.err = fmt.Errorf("tool loop exceeded max iterations (%d)", s.toolLoopMax)
			return false
		}
	}
}

func (s *streamText) delta() string { return s.curDelta }
func (s *streamText) message() *Message {
	return s.finalMsg
}
func (s *streamText) usage() Usage { return s.aggUsage }
func (s *streamText) finishReason() FinishReason {
	if s.finish == "" {
		return FinishUnknown
	}
	return s.finish
}
func (s *streamText) streamErr() error { return s.err }

func (s *streamText) close() error {
	s.closed = true
	if s.cur != nil {
		return s.cur.Close()
	}
	return nil
}

func (s *streamText) startStream() error {
	req := s.baseReq
	req.Model = s.model
	req.Messages = append([]Message(nil), s.messages...)
	req.Tools = append([]Tool(nil), s.tools...)

	preq, err := toProviderRequest(req)
	if err != nil {
		return err
	}
	cur, err := s.p.Stream(s.ctx, preq)
	if err != nil {
		return mapProviderError(err)
	}
	s.cur = cur
	return nil
}

func (s *streamText) runTools(calls []toolCall) error {
	results, err := runTools(s.ctx, s.tools, calls)
	if err != nil {
		return err
	}
	s.messages = append(s.messages, results...)
	return nil
}
