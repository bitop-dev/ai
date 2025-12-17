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

	opts Options

	onDelta func(provider.Delta)

	baseReq  provider.Request
	messages []provider.Message
	tools    []provider.ToolDefinition

	stepNumber int

	cur provider.Stream

	curDelta string
	final    *provider.Response
	aggUsage provider.Usage
	steps    []Step

	responseMessages []provider.Message
	curActiveTools   []string
	err              error
}

func NewStream(ctx context.Context, p provider.Provider, req provider.Request, exec tools.Executor, opts Options, onDelta func(provider.Delta)) *Stream {
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = 5
	}
	s := &Stream{
		ctx:      ctx,
		p:        p,
		exec:     exec,
		opts:     opts,
		onDelta:  onDelta,
		baseReq:  req,
		messages: append([]provider.Message(nil), req.Messages...),
		tools:    append([]provider.ToolDefinition(nil), req.Tools...),
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
			if s.onDelta != nil && (len(d.ToolCalls) > 0 || d.Text != "") {
				s.onDelta(d)
			}
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
		s.responseMessages = append(s.responseMessages, final.Message)

		calls := tools.ExtractToolCalls(final.Message)
		step := Step{
			StepNumber:  s.stepNumber,
			Response:    *final,
			ToolCalls:   append([]provider.ToolCallPart(nil), calls...),
			ActiveTools: append([]string(nil), s.curActiveTools...),
		}
		if len(calls) == 0 {
			s.steps = append(s.steps, step)
			if s.opts.OnStepFinish != nil {
				s.opts.OnStepFinish(StepFinishEvent{Step: step})
			}
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
		s.responseMessages = append(s.responseMessages, results...)
		step.ToolResults = append([]provider.Message(nil), results...)
		s.steps = append(s.steps, step)
		if s.opts.OnStepFinish != nil {
			s.opts.OnStepFinish(StepFinishEvent{Step: step})
		}

		if s.opts.StopWhen != nil {
			if s.opts.StopWhen(StopWhenEvent{
				StepNumber: s.stepNumber,
				Steps:      append([]Step(nil), s.steps...),
				Messages:   append([]provider.Message(nil), s.messages...),
			}) {
				s.final = final
				return false
			}
		}

		s.stepNumber++
		if s.stepNumber >= s.opts.MaxIterations {
			s.err = fmt.Errorf("tool loop exceeded max iterations (%d)", s.opts.MaxIterations)
			return false
		}
	}
}

func (s *Stream) Delta() string             { return s.curDelta }
func (s *Stream) Final() *provider.Response { return s.final }
func (s *Stream) Usage() provider.Usage     { return s.aggUsage }
func (s *Stream) Steps() []Step             { return append([]Step(nil), s.steps...) }
func (s *Stream) ResponseMessages() []provider.Message {
	return append([]provider.Message(nil), s.responseMessages...)
}
func (s *Stream) Err() error { return s.err }
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

	activeTools := []string(nil)
	if s.opts.PrepareStep != nil {
		res, err := s.opts.PrepareStep(PrepareStepEvent{
			StepNumber: s.stepNumber,
			Steps:      append([]Step(nil), s.steps...),
			Request:    req,
			Messages:   append([]provider.Message(nil), req.Messages...),
			Tools:      append([]provider.ToolDefinition(nil), s.tools...),
		})
		if err != nil {
			return err
		}
		if res.Model != "" {
			s.baseReq.Model = res.Model
			req.Model = res.Model
		}
		if res.Messages != nil {
			s.messages = append([]provider.Message(nil), res.Messages...)
			req.Messages = append([]provider.Message(nil), res.Messages...)
		}
		if res.ActiveTools != nil {
			activeTools = append([]string(nil), res.ActiveTools...)
		}
	}

	s.curActiveTools = append([]string(nil), activeTools...)

	callTools := append([]provider.ToolDefinition(nil), s.tools...)
	if len(activeTools) > 0 {
		allowed := make(map[string]struct{}, len(activeTools))
		for _, n := range activeTools {
			allowed[n] = struct{}{}
		}
		filtered := make([]provider.ToolDefinition, 0, len(callTools))
		for _, td := range callTools {
			if _, ok := allowed[td.Name]; ok {
				filtered = append(filtered, td)
			}
		}
		callTools = filtered
	}
	req.Tools = append([]provider.ToolDefinition(nil), callTools...)

	cur, err := s.p.Stream(s.ctx, req)
	if err != nil {
		return err
	}
	s.cur = cur
	return nil
}
