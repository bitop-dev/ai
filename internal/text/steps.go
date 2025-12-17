package text

import "github.com/bitop-dev/ai/internal/provider"

type Step struct {
	StepNumber int

	Response    provider.Response
	ToolCalls   []provider.ToolCallPart
	ToolResults []provider.Message

	ActiveTools []string
}

type StopWhenEvent struct {
	StepNumber int
	Steps      []Step
	Messages   []provider.Message
}

type StopWhenFunc func(event StopWhenEvent) bool

type PrepareStepEvent struct {
	StepNumber int
	Steps      []Step

	Request  provider.Request
	Messages []provider.Message
	Tools    []provider.ToolDefinition
}

type PrepareStepResult struct {
	// Model overrides provider.Request.Model for this step.
	Model string

	// Messages overrides the messages used for this step (and becomes the base
	// for following steps).
	Messages []provider.Message

	// ActiveTools restricts tools available to the model for this step.
	// When empty/nil, all tools are active.
	ActiveTools []string
}

type StepFinishEvent struct {
	Step Step
}

type Options struct {
	MaxIterations int
	StopWhen      StopWhenFunc
	PrepareStep   func(event PrepareStepEvent) (PrepareStepResult, error)
	OnStepFinish  func(event StepFinishEvent)
}
