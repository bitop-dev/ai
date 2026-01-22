package ai

import (
	"context"
	"errors"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

// Agent defines the interface for multi-step tool loop agents.
type Agent[CallOptions any] interface {
	ID() string
	Tools() []providerutils.ToolDefinition
	Generate(ctx context.Context, options AgentCallOptions[CallOptions]) (AgentResult, error)
	Stream(ctx context.Context, options AgentStreamOptions[CallOptions]) (AgentStreamResult, error)
}

// AgentCallOptions configures a single agent invocation.
type AgentCallOptions[CallOptions any] struct {
	Prompt   string
	Messages []provider.ModelMessage
	Options  CallOptions
}

// AgentStreamOptions configures a streaming agent invocation.
type AgentStreamOptions[CallOptions any] AgentCallOptions[CallOptions]

// AgentStreamResult exposes a streaming response from an agent.
type AgentStreamResult struct {
	Stream  *Stream[provider.StreamPart]
	collect func() (AgentResult, error)
}

// Collect drains the stream and returns the aggregated agent result.
func (result AgentStreamResult) Collect() (AgentResult, error) {
	if result.collect == nil {
		return AgentResult{}, errors.New("stream is nil")
	}
	return result.collect()
}

// ToolLoopAgentOnStepFinishCallback is invoked after each completed step.
type ToolLoopAgentOnStepFinishCallback func(ctx context.Context, step StepResult)

// ToolLoopAgentOnFinishCallback is invoked when the agent finishes.
type ToolLoopAgentOnFinishCallback func(ctx context.Context, result AgentResult)

// ToolLoopAgentPrepareCallState provides context for preparing model calls.
type ToolLoopAgentPrepareCallState[CallOptions any] struct {
	Prompt      provider.Prompt
	Options     CallOptions
	Step        int
	Steps       []StepResult
	TextOptions TextOptions
}

// ToolLoopAgentPrepareCallFunc customizes call options for each step.
type ToolLoopAgentPrepareCallFunc[CallOptions any] func(ctx context.Context, state ToolLoopAgentPrepareCallState[CallOptions]) (TextOptions, error)

// ToolLoopAgentSettings configures a tool loop agent.
type ToolLoopAgentSettings[CallOptions any] struct {
	ID           string
	Model        provider.LanguageModel
	Tools        []providerutils.ToolDefinition
	ToolChoice   *provider.ToolChoice
	Instructions string
	StopWhen     []StopCondition
	MaxSteps     int
	Approve      providerutils.ToolApprovalFunc
	OnStepFinish ToolLoopAgentOnStepFinishCallback
	OnFinish     ToolLoopAgentOnFinishCallback
	TextOptions  TextOptions
	PrepareCall  ToolLoopAgentPrepareCallFunc[CallOptions]
}

// ToolLoopAgent orchestrates multi-step tool calling.
type ToolLoopAgent[CallOptions any] struct {
	settings ToolLoopAgentSettings[CallOptions]
}

// NewToolLoopAgent constructs a tool loop agent.
func NewToolLoopAgent[CallOptions any](settings ToolLoopAgentSettings[CallOptions]) *ToolLoopAgent[CallOptions] {
	return &ToolLoopAgent[CallOptions]{settings: settings}
}

// ID returns the agent identifier, if any.
func (agent *ToolLoopAgent[CallOptions]) ID() string {
	return agent.settings.ID
}

// Tools returns the tool definitions available to the agent.
func (agent *ToolLoopAgent[CallOptions]) Tools() []providerutils.ToolDefinition {
	return agent.settings.Tools
}
