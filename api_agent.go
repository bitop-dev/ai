package ai

import (
	"context"
	"fmt"
	"time"
)

// Agent is a reusable configuration wrapper for agentic loops (LLM + tools + loop control).
//
// By default, when neither MaxIterations nor StopWhen are set, the agent runs for a single step.
type Agent struct {
	Model  ModelRef
	System string
	Tools  []Tool

	// Loop controls. If both are unset, Agent defaults to 1 step.
	MaxIterations int
	StopWhen      StopCondition

	// Request controls.
	Headers    map[string]string
	MaxRetries *int
	Timeout    time.Duration

	// Optional hooks.
	OnToolProgress func(event ToolProgressEvent)
	OnStepFinish   func(event StepFinishEvent)
	PrepareStep    func(event PrepareStepEvent) (PrepareStepResult, error)
}

type AgentGenerateRequest struct {
	Prompt string

	// Messages are optional; if provided, they are used as the base conversation.
	// If Prompt is also set, it is appended as a user message.
	Messages []Message
}

func (a Agent) Generate(ctx context.Context, req AgentGenerateRequest) (*GenerateTextResponse, error) {
	base, err := a.baseRequest(req)
	if err != nil {
		return nil, err
	}
	return generateTextFromBaseRequest(ctx, base)
}

type AgentStreamRequest = AgentGenerateRequest

func (a Agent) Stream(ctx context.Context, req AgentStreamRequest) (*TextStream, error) {
	base, err := a.baseRequest(req)
	if err != nil {
		return nil, err
	}
	return streamTextFromBaseRequest(ctx, base)
}

func (a Agent) baseRequest(req AgentGenerateRequest) (BaseRequest, error) {
	if a.Model == nil {
		return BaseRequest{}, fmt.Errorf("agent model is required")
	}

	msgs := append([]Message(nil), req.Messages...)
	if a.System != "" && !startsWithSystem(msgs) {
		msgs = append([]Message{System(a.System)}, msgs...)
	}
	if req.Prompt != "" {
		msgs = append(msgs, User(req.Prompt))
	}

	maxIter := a.MaxIterations
	if maxIter <= 0 && a.StopWhen == nil {
		maxIter = 1
	}
	if maxIter <= 0 {
		maxIter = 5
	}

	toolLoop := &ToolLoopOptions{
		MaxIterations: maxIter,
		StopWhen:      a.StopWhen,
	}

	return BaseRequest{
		Model:          a.Model,
		Messages:       msgs,
		Tools:          append([]Tool(nil), a.Tools...),
		ToolLoop:       toolLoop,
		Headers:        cloneStringMap(a.Headers),
		MaxRetries:     a.MaxRetries,
		Timeout:        a.Timeout,
		OnToolProgress: a.OnToolProgress,
		OnStepFinish:   a.OnStepFinish,
		PrepareStep:    a.PrepareStep,
	}, nil
}

func startsWithSystem(msgs []Message) bool {
	return len(msgs) > 0 && msgs[0].Role == RoleSystem
}
