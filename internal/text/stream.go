package text

import (
	"context"
	"fmt"

	"github.com/bitop-dev/ai/internal/provider"
	"github.com/bitop-dev/ai/internal/tools"
)

type Stream struct {
	ctx  context.Context
	p    provider.Provider
	exec tools.Executor

	maxIterations int

	baseReq  provider.Request
	messages []provider.Message
	tools    []provider.ToolDefinition

	iter int

	cur provider.Stream

	curDelta string
	final    *provider.Response
	aggUsage provider.Usage
	err      error
}

func NewStream(ctx context.Context, p provider.Provider, req provider.Request, exec tools.Executor, maxIterations int) *Stream {
	if maxIterations <= 0 {
		maxIterations = 5
	}
	s := &Stream{
		ctx:           ctx,
		p:             p,
		exec:          exec,
		maxIterations: maxIterations,
		baseReq:       req,
		messages:      append([]provider.Message(nil), req.Messages...),
		tools:         append([]provider.ToolDefinition(nil), req.Tools...),
	}
	s.baseReq.Messages = nil
	s.baseReq.Tools = nil
	return s
}

func (s *Stream) Next() bool {
	if s.err != nil || s.final != nil {
		return false
	}
	s.curDelta = ""

	for {
		if s.cur == nil {
			if err := s.start(); err != nil {
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
			s.err = err
			return false
		}

		final := s.cur.Final()
		_ = s.cur.Close()
		s.cur = nil

		if final == nil {
			s.final = &provider.Response{Message: provider.Message{Role: provider.RoleAssistant}}
			return false
		}

		s.aggUsage = tools.AddUsage(s.aggUsage, final.Usage)
		s.messages = append(s.messages, final.Message)

		calls := tools.ExtractToolCalls(final.Message)
		if len(calls) == 0 {
			s.final = final
			return false
		}

		if s.exec == nil {
			s.err = fmt.Errorf("tool calls requested but no executor provided")
			return false
		}

		results, err := s.exec(s.ctx, calls)
		if err != nil {
			s.err = err
			return false
		}
		s.messages = append(s.messages, results...)

		s.iter++
		if s.iter >= s.maxIterations {
			s.err = fmt.Errorf("tool loop exceeded max iterations (%d)", s.maxIterations)
			return false
		}
	}
}

func (s *Stream) Delta() string             { return s.curDelta }
func (s *Stream) Final() *provider.Response { return s.final }
func (s *Stream) Usage() provider.Usage     { return s.aggUsage }
func (s *Stream) Err() error                { return s.err }
func (s *Stream) Close() error {
	if s.cur != nil {
		return s.cur.Close()
	}
	return nil
}

func (s *Stream) start() error {
	if s.p == nil {
		return fmt.Errorf("provider is required")
	}
	req := s.baseReq
	req.Messages = append([]provider.Message(nil), s.messages...)
	req.Tools = append([]provider.ToolDefinition(nil), s.tools...)

	cur, err := s.p.Stream(s.ctx, req)
	if err != nil {
		return err
	}
	s.cur = cur
	return nil
}
