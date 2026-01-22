package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

const defaultAgentMaxSteps = 20

// Generate executes the agent loop and returns the aggregated result.
func (agent *ToolLoopAgent[CallOptions]) Generate(ctx context.Context, options AgentCallOptions[CallOptions]) (AgentResult, error) {
	if agent.settings.Model == nil {
		return AgentResult{}, ErrNilModel
	}

	return agent.run(ctx, options)
}

// Stream executes the agent loop and streams all parts as they arrive.
func (agent *ToolLoopAgent[CallOptions]) Stream(ctx context.Context, options AgentStreamOptions[CallOptions]) (AgentStreamResult, error) {
	if agent.settings.Model == nil {
		return AgentStreamResult{}, ErrNilModel
	}

	callOptions := AgentCallOptions[CallOptions](options)
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan provider.StreamPart, 32)
	stream := newStream(ctx, cancel, ch)
	state := &agentStreamState{done: make(chan struct{})}

	go func() {
		result, err := agent.runStream(ctx, callOptions, ch)
		stream.err = err
		state.finish(result, err)
		close(ch)
		cancel()
	}()

	return AgentStreamResult{
		Stream:  stream,
		collect: state.collect,
	}, nil
}

type agentStreamState struct {
	done   chan struct{}
	result AgentResult
	err    error
}

func (state *agentStreamState) finish(result AgentResult, err error) {
	state.result = result
	state.err = err
	close(state.done)
}

func (state *agentStreamState) collect() (AgentResult, error) {
	<-state.done
	return state.result, state.err
}

func (agent *ToolLoopAgent[CallOptions]) run(ctx context.Context, options AgentCallOptions[CallOptions]) (AgentResult, error) {
	prompt, err := agent.buildPrompt(options)
	if err != nil {
		return AgentResult{}, err
	}

	stopConditions := agent.resolveStopConditions()
	toolMap := toolDefinitionsByName(agent.settings.Tools)

	var (
		steps      []StepResult
		allParts   []provider.StreamPart
		totalUsage *provider.LanguageModelUsage
	)

	for step := 0; ; step++ {
		if err := ctx.Err(); err != nil {
			return AgentResult{}, err
		}

		callOptions, err := agent.prepareCallOptions(ctx, options, step, steps, prompt)
		if err != nil {
			return AgentResult{}, err
		}

		result, err := GenerateText(ctx, agent.settings.Model, GenerateTextOptions(callOptions))
		if err != nil {
			return AgentResult{}, err
		}

		toolCalls, contentParts, err := parseToolLoopParts(result.Parts)
		if err != nil {
			return AgentResult{}, err
		}

		stepResult := StepResult{
			Parts:            result.Parts,
			Content:          contentParts,
			Text:             result.Text,
			ReasoningText:    reasoningText(contentParts),
			Sources:          sourcesFromContent(contentParts),
			ToolCalls:        toolCalls,
			FinishReason:     result.FinishReason,
			Usage:            result.Usage,
			Warnings:         result.Warnings,
			ResponseMetadata: result.ResponseMetadata,
			ProviderMetadata: result.ProviderMetadata,
			Request:          result.Request,
			Response:         result.Response,
		}

		if len(contentParts) > 0 {
			prompt.Messages = append(prompt.Messages, provider.ModelMessage{
				Role:    provider.RoleAssistant,
				Content: contentParts,
			})
		}

		allParts = append(allParts, result.Parts...)
		totalUsage = addUsage(totalUsage, result.Usage)

		if len(toolCalls) == 0 {
			steps = append(steps, stepResult)
			agent.emitStepFinish(ctx, stepResult)
			return agent.finalizeResult(ctx, stepResult, steps, allParts, totalUsage), nil
		}

		toolResults, err := agent.executeTools(ctx, toolMap, toolCalls)
		if err != nil {
			return AgentResult{}, err
		}
		stepResult.ToolResults = toolResults
		steps = append(steps, stepResult)

		for _, toolResult := range toolResults {
			allParts = append(allParts, providerutils.StreamPartForToolResult(toolResult))
			prompt.Messages = append(prompt.Messages, provider.ModelMessage{
				Role:       provider.RoleTool,
				ToolCallID: toolResult.ID,
				Content: []provider.ContentPart{
					provider.ToolResultContent{ToolResult: toolResult},
				},
			})
		}

		agent.emitStepFinish(ctx, stepResult)
		stop, err := IsStopConditionMet(ctx, stopConditions, steps)
		if err != nil {
			return AgentResult{}, err
		}
		if stop {
			return agent.finalizeResult(ctx, stepResult, steps, allParts, totalUsage), nil
		}
	}
}

func (agent *ToolLoopAgent[CallOptions]) runStream(ctx context.Context, options AgentCallOptions[CallOptions], ch chan<- provider.StreamPart) (AgentResult, error) {
	prompt, err := agent.buildPrompt(options)
	if err != nil {
		return AgentResult{}, err
	}

	stopConditions := agent.resolveStopConditions()
	toolMap := toolDefinitionsByName(agent.settings.Tools)

	var (
		steps      []StepResult
		allParts   []provider.StreamPart
		totalUsage *provider.LanguageModelUsage
	)

	for step := 0; ; step++ {
		if err := ctx.Err(); err != nil {
			agent.sendStreamError(ctx, ch, err)
			return AgentResult{}, err
		}

		callOptions, err := agent.prepareCallOptions(ctx, options, step, steps, prompt)
		if err != nil {
			agent.sendStreamError(ctx, ch, err)
			return AgentResult{}, err
		}

		streamResult, err := StreamText(ctx, agent.settings.Model, StreamTextOptions(callOptions))
		if err != nil {
			agent.sendStreamError(ctx, ch, err)
			return AgentResult{}, err
		}

		accumulator := newStepAccumulator()
		for streamResult.Stream.Next() {
			part := streamResult.Stream.Value()
			if recordErr := accumulator.recordPart(part); recordErr != nil {
				streamResult.Stream.Close()
				agent.sendStreamError(ctx, ch, recordErr)
				return AgentResult{}, recordErr
			}
			allParts = append(allParts, part)
			if !sendPart(ctx, ch, part) {
				streamResult.Stream.Close()
				return AgentResult{}, ctx.Err()
			}
		}

		if err := streamResult.Stream.Err(); err != nil {
			agent.sendStreamError(ctx, ch, err)
			return AgentResult{}, err
		}
		streamResult.Stream.Close()

		contentParts := accumulator.contentParts()
		stepResult := accumulator.stepResult(contentParts, streamResult.Request, streamResult.Response)
		if len(contentParts) > 0 {
			prompt.Messages = append(prompt.Messages, provider.ModelMessage{
				Role:    provider.RoleAssistant,
				Content: contentParts,
			})
		}

		steps = append(steps, stepResult)
		totalUsage = addUsage(totalUsage, stepResult.Usage)

		if len(stepResult.ToolCalls) == 0 {
			agent.emitStepFinish(ctx, stepResult)
			return agent.finalizeResult(ctx, stepResult, steps, allParts, totalUsage), nil
		}

		toolResults, err := agent.executeTools(ctx, toolMap, stepResult.ToolCalls)
		if err != nil {
			agent.sendStreamError(ctx, ch, err)
			return AgentResult{}, err
		}
		stepResult.ToolResults = toolResults
		steps[len(steps)-1] = stepResult

		for _, toolResult := range toolResults {
			part := providerutils.StreamPartForToolResult(toolResult)
			allParts = append(allParts, part)
			if !sendPart(ctx, ch, part) {
				return AgentResult{}, ctx.Err()
			}
			prompt.Messages = append(prompt.Messages, provider.ModelMessage{
				Role:       provider.RoleTool,
				ToolCallID: toolResult.ID,
				Content: []provider.ContentPart{
					provider.ToolResultContent{ToolResult: toolResult},
				},
			})
		}

		agent.emitStepFinish(ctx, stepResult)
		stop, err := IsStopConditionMet(ctx, stopConditions, steps)
		if err != nil {
			agent.sendStreamError(ctx, ch, err)
			return AgentResult{}, err
		}
		if stop {
			return agent.finalizeResult(ctx, stepResult, steps, allParts, totalUsage), nil
		}
	}
}

func (agent *ToolLoopAgent[CallOptions]) emitStepFinish(ctx context.Context, step StepResult) {
	if agent.settings.OnStepFinish != nil {
		agent.settings.OnStepFinish(ctx, step)
	}
}

func (agent *ToolLoopAgent[CallOptions]) finalizeResult(ctx context.Context, step StepResult, steps []StepResult, parts []provider.StreamPart, totalUsage *provider.LanguageModelUsage) AgentResult {
	result := AgentResult{
		Text:             step.Text,
		Parts:            parts,
		FinishReason:     step.FinishReason,
		Usage:            step.Usage,
		Warnings:         step.Warnings,
		ResponseMetadata: step.ResponseMetadata,
		ProviderMetadata: step.ProviderMetadata,
		Request:          step.Request,
		Response:         step.Response,
		Steps:            append([]StepResult(nil), steps...),
		TotalUsage:       totalUsage,
	}

	if agent.settings.OnFinish != nil {
		agent.settings.OnFinish(ctx, result)
	}

	return result
}

func (agent *ToolLoopAgent[CallOptions]) buildPrompt(options AgentCallOptions[CallOptions]) (provider.Prompt, error) {
	if options.Prompt != "" && len(options.Messages) > 0 {
		return provider.Prompt{}, errors.New("prompt and messages cannot both be set")
	}
	if options.Prompt == "" && len(options.Messages) == 0 {
		return provider.Prompt{}, errors.New("prompt or messages must be provided")
	}

	prompt := provider.Prompt{}
	if agent.settings.Instructions != "" {
		prompt.Messages = append(prompt.Messages, provider.ModelMessage{
			Role:    provider.RoleSystem,
			Content: []provider.ContentPart{provider.TextContent{Text: agent.settings.Instructions}},
		})
	}

	if options.Prompt != "" {
		prompt.Messages = append(prompt.Messages, provider.ModelMessage{
			Role:    provider.RoleUser,
			Content: []provider.ContentPart{provider.TextContent{Text: options.Prompt}},
		})
		return prompt, nil
	}

	prompt.Messages = append(prompt.Messages, options.Messages...)
	return prompt, nil
}

func (agent *ToolLoopAgent[CallOptions]) resolveStopConditions() []StopCondition {
	if len(agent.settings.StopWhen) > 0 {
		return agent.settings.StopWhen
	}
	maxSteps := agent.settings.MaxSteps
	if maxSteps <= 0 {
		maxSteps = defaultAgentMaxSteps
	}
	return []StopCondition{StepCountIs(maxSteps)}
}

func (agent *ToolLoopAgent[CallOptions]) prepareCallOptions(ctx context.Context, call AgentCallOptions[CallOptions], step int, steps []StepResult, prompt provider.Prompt) (TextOptions, error) {
	options := cloneTextOptions(agent.settings.TextOptions)
	options.Prompt = prompt

	if options.ToolChoice == nil {
		if agent.settings.ToolChoice != nil {
			options.ToolChoice = agent.settings.ToolChoice
		} else if len(agent.settings.Tools) > 0 {
			options.ToolChoice = &provider.ToolChoice{Type: provider.ToolChoiceTypeAuto}
		}
	}

	if agent.settings.PrepareCall != nil {
		prepared, err := agent.settings.PrepareCall(ctx, ToolLoopAgentPrepareCallState[CallOptions]{
			Prompt:      prompt,
			Options:     call.Options,
			Step:        step,
			Steps:       append([]StepResult(nil), steps...),
			TextOptions: options,
		})
		if err != nil {
			return TextOptions{}, err
		}
		options = prepared
		if len(options.Prompt.Messages) == 0 {
			options.Prompt = prompt
		}
	}

	applyToolSpecifications(&options, agent.settings.Model, agent.settings.Tools)
	return options, nil
}

func (agent *ToolLoopAgent[CallOptions]) executeTools(ctx context.Context, toolMap map[string]providerutils.ToolDefinition, calls []provider.ToolCall) ([]provider.ToolResult, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	results := make([]provider.ToolResult, 0, len(calls))
	for _, call := range calls {
		tool, ok := toolMap[call.Name]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrToolNotFound, call.Name)
		}
		if agent.settings.Approve != nil {
			approved, err := agent.settings.Approve(ctx, call)
			if err != nil {
				return nil, err
			}
			if !approved {
				return nil, ErrToolRejected
			}
		}
		if tool.Approve != nil {
			approved, err := tool.Approve(ctx, call)
			if err != nil {
				return nil, err
			}
			if !approved {
				return nil, ErrToolRejected
			}
		}

		result, err := providerutils.ExecuteTool(ctx, tool, call)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

func (agent *ToolLoopAgent[CallOptions]) sendStreamError(ctx context.Context, ch chan<- provider.StreamPart, err error) {
	if err == nil {
		return
	}
	_ = sendPart(ctx, ch, provider.StreamPart{Type: provider.StreamPartTypeError, Error: &provider.StreamError{Err: err}})
}

type stepAccumulator struct {
	parts            []provider.StreamPart
	textBuilder      strings.Builder
	reasoningBuilder strings.Builder
	sources          []provider.Source
	toolCalls        []provider.ToolCall
	warnings         []provider.Warning
	providerMetadata provider.ProviderMetadata
	responseMetadata *provider.ResponseMetadata
	finishReason     provider.FinishReason
	usage            *provider.LanguageModelUsage
	toolArgs         *providerutils.ToolArgumentAccumulator
}

func newStepAccumulator() *stepAccumulator {
	return &stepAccumulator{toolArgs: providerutils.NewToolArgumentAccumulator()}
}

func (accumulator *stepAccumulator) recordPart(part provider.StreamPart) error {
	accumulator.parts = append(accumulator.parts, part)
	accumulator.warnings = append(accumulator.warnings, part.Warnings...)
	accumulator.providerMetadata = mergeProviderMetadata(accumulator.providerMetadata, part.ProviderMetadata)
	if part.ResponseMetadata != nil {
		accumulator.responseMetadata = part.ResponseMetadata
		accumulator.providerMetadata = mergeProviderMetadata(accumulator.providerMetadata, part.ResponseMetadata.ProviderMetadata)
	}

	switch part.Type {
	case provider.StreamPartTypeTextStart:
		if part.TextStart != nil {
			accumulator.textBuilder.WriteString(part.TextStart.Text)
		}
	case provider.StreamPartTypeTextDelta:
		if part.TextDelta != nil {
			accumulator.textBuilder.WriteString(part.TextDelta.Delta)
		}
	case provider.StreamPartTypeTextEnd:
		if part.TextEnd != nil {
			accumulator.textBuilder.WriteString(part.TextEnd.Text)
		}
	case provider.StreamPartTypeReasoningStart:
		if part.ReasoningStart != nil {
			accumulator.reasoningBuilder.WriteString(part.ReasoningStart.Text)
		}
	case provider.StreamPartTypeReasoningDelta:
		if part.ReasoningDelta != nil {
			accumulator.reasoningBuilder.WriteString(part.ReasoningDelta.Delta)
		}
	case provider.StreamPartTypeReasoningEnd:
		if part.ReasoningEnd != nil {
			accumulator.reasoningBuilder.WriteString(part.ReasoningEnd.Text)
		}
	case provider.StreamPartTypeSource:
		if part.Source != nil {
			accumulator.sources = append(accumulator.sources, *part.Source)
		}
	case provider.StreamPartTypeToolCall:
		if part.ToolCall != nil {
			accumulator.toolCalls = append(accumulator.toolCalls, *part.ToolCall)
		}
	case provider.StreamPartTypeToolInputStart:
		if part.ToolInputStart != nil {
			accumulator.toolArgs.Start(part.ToolInputStart.ToolCallID, part.ToolInputStart.Name)
		}
	case provider.StreamPartTypeToolInputDelta:
		if part.ToolInputDelta != nil {
			if err := accumulator.toolArgs.AddDelta(part.ToolInputDelta.ToolCallID, part.ToolInputDelta.Delta); err != nil {
				return err
			}
		}
	case provider.StreamPartTypeToolInputEnd:
		if part.ToolInputEnd != nil {
			call, err := accumulator.toolArgs.End(part.ToolInputEnd.ToolCallID)
			if err != nil {
				return err
			}
			accumulator.toolCalls = append(accumulator.toolCalls, call)
		}
	case provider.StreamPartTypeFinish:
		if part.Finish != nil {
			accumulator.finishReason = part.Finish.Reason
			accumulator.usage = part.Finish.Usage
		}
	case provider.StreamPartTypeError:
		if part.Error != nil && part.Error.Err != nil {
			return part.Error.Err
		}
	}

	return nil
}

func (accumulator *stepAccumulator) contentParts() []provider.ContentPart {
	contentParts := make([]provider.ContentPart, 0, len(accumulator.toolCalls)+2)
	if accumulator.textBuilder.Len() > 0 {
		contentParts = append(contentParts, provider.TextContent{Text: accumulator.textBuilder.String()})
	}
	if accumulator.reasoningBuilder.Len() > 0 {
		contentParts = append(contentParts, provider.ReasoningContent{Text: accumulator.reasoningBuilder.String()})
	}
	for _, source := range accumulator.sources {
		contentParts = append(contentParts, provider.SourceContent{Source: source})
	}
	for _, call := range accumulator.toolCalls {
		contentParts = append(contentParts, provider.ToolCallContent{ToolCall: call})
	}
	return contentParts
}

func (accumulator *stepAccumulator) stepResult(contentParts []provider.ContentPart, request *provider.LanguageModelRequest, response *provider.LanguageModelResponse) StepResult {
	return StepResult{
		Parts:            accumulator.parts,
		Content:          contentParts,
		Text:             accumulator.textBuilder.String(),
		ReasoningText:    accumulator.reasoningBuilder.String(),
		Sources:          append([]provider.Source(nil), accumulator.sources...),
		ToolCalls:        append([]provider.ToolCall(nil), accumulator.toolCalls...),
		FinishReason:     accumulator.finishReason,
		Usage:            accumulator.usage,
		Warnings:         append([]provider.Warning(nil), accumulator.warnings...),
		ResponseMetadata: accumulator.responseMetadata,
		ProviderMetadata: accumulator.providerMetadata,
		Request:          request,
		Response:         response,
	}
}

func toolDefinitionsByName(tools []providerutils.ToolDefinition) map[string]providerutils.ToolDefinition {
	if len(tools) == 0 {
		return map[string]providerutils.ToolDefinition{}
	}
	toolMap := make(map[string]providerutils.ToolDefinition, len(tools))
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}
	return toolMap
}

func cloneTextOptions(options TextOptions) TextOptions {
	cloned := options
	if options.StopSequences != nil {
		cloned.StopSequences = append([]string(nil), options.StopSequences...)
	}
	cloned.RequestOptions = cloneRequestOptions(options.RequestOptions)
	return cloned
}

func cloneRequestOptions(options provider.RequestOptions) provider.RequestOptions {
	cloned := options
	if options.Headers != nil {
		cloned.Headers = map[string]string{}
		for key, value := range options.Headers {
			cloned.Headers[key] = value
		}
	}
	if options.Metadata != nil {
		cloned.Metadata = map[string]any{}
		for key, value := range options.Metadata {
			cloned.Metadata[key] = value
		}
	}
	cloned.ProviderOptions = cloneProviderOptions(options.ProviderOptions)
	return cloned
}

func cloneProviderOptions(options provider.ProviderOptions) provider.ProviderOptions {
	if options == nil {
		return nil
	}
	cloned := provider.ProviderOptions{}
	for key, value := range options {
		if value == nil {
			cloned[key] = nil
			continue
		}
		copied := provider.JSONObject{}
		for innerKey, innerValue := range value {
			copied[innerKey] = innerValue
		}
		cloned[key] = copied
	}
	return cloned
}

func applyToolSpecifications(options *TextOptions, model provider.LanguageModel, tools []providerutils.ToolDefinition) {
	if model == nil || len(tools) == 0 {
		return
	}
	providerID := string(model.ProviderID())
	if options.RequestOptions.ProviderOptions == nil {
		options.RequestOptions.ProviderOptions = provider.ProviderOptions{}
	}

	providerOptions := options.RequestOptions.ProviderOptions[providerID]
	if providerOptions == nil {
		providerOptions = provider.JSONObject{}
	} else {
		if _, ok := providerOptions["tools"]; ok {
			return
		}
		copied := provider.JSONObject{}
		for key, value := range providerOptions {
			copied[key] = value
		}
		providerOptions = copied
	}

	specifications := make([]providerutils.ToolSpecification, 0, len(tools))
	for _, tool := range tools {
		specifications = append(specifications, tool.Specification())
	}
	providerOptions["tools"] = specifications
	options.RequestOptions.ProviderOptions[providerID] = providerOptions
}

func reasoningText(parts []provider.ContentPart) string {
	var builder strings.Builder
	for _, part := range parts {
		switch typed := part.(type) {
		case provider.ReasoningContent:
			builder.WriteString(typed.Text)
		}
	}
	return builder.String()
}

func sourcesFromContent(parts []provider.ContentPart) []provider.Source {
	var sources []provider.Source
	for _, part := range parts {
		switch typed := part.(type) {
		case provider.SourceContent:
			sources = append(sources, typed.Source)
		}
	}
	return sources
}

func addUsage(total *provider.LanguageModelUsage, usage *provider.LanguageModelUsage) *provider.LanguageModelUsage {
	if usage == nil {
		return total
	}
	if total == nil {
		total = &provider.LanguageModelUsage{}
	}
	total.PromptTokens += usage.PromptTokens
	total.CompletionTokens += usage.CompletionTokens
	total.TotalTokens += usage.TotalTokens
	return total
}

func sendPart(ctx context.Context, ch chan<- provider.StreamPart, part provider.StreamPart) bool {
	select {
	case <-ctx.Done():
		return false
	case ch <- part:
		return true
	}
}
