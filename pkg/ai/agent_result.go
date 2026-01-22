package ai

import "github.com/bitop-dev/ai/pkg/provider"

// StepResult captures the output of a single agent step.
type StepResult struct {
	Parts            []provider.StreamPart
	Content          []provider.ContentPart
	Text             string
	ReasoningText    string
	Sources          []provider.Source
	ToolCalls        []provider.ToolCall
	ToolResults      []provider.ToolResult
	FinishReason     provider.FinishReason
	Usage            *provider.LanguageModelUsage
	Warnings         []provider.Warning
	ResponseMetadata *provider.ResponseMetadata
	ProviderMetadata provider.ProviderMetadata
	Request          *provider.LanguageModelRequest
	Response         *provider.LanguageModelResponse
}

// AgentResult captures the aggregated output of an agent run.
type AgentResult struct {
	Text             string
	Parts            []provider.StreamPart
	FinishReason     provider.FinishReason
	Usage            *provider.LanguageModelUsage
	Warnings         []provider.Warning
	ResponseMetadata *provider.ResponseMetadata
	ProviderMetadata provider.ProviderMetadata
	Request          *provider.LanguageModelRequest
	Response         *provider.LanguageModelResponse
	Steps            []StepResult
	TotalUsage       *provider.LanguageModelUsage
}
