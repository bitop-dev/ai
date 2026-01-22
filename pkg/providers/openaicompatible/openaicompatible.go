package openaicompatible

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providerutils"
)

const DefaultBaseURL = "https://api.openai.com/v1"
const DefaultProviderName = "openai-compatible"

type Settings struct {
	APIKey       string
	BaseURL      string
	Headers      map[string]string
	QueryParams  map[string]string
	HTTPClient   *http.Client
	ProviderName string
}

type Provider struct {
	apiKey      string
	baseURL     string
	headers     map[string]string
	queryParams map[string]string
	httpClient  *http.Client
	providerID  provider.ProviderID
}

func CreateOpenAICompatible(settings Settings) *Provider {
	baseURL := strings.TrimRight(settings.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	providerName := settings.ProviderName
	if providerName == "" {
		providerName = DefaultProviderName
	}
	return &Provider{
		apiKey:      settings.APIKey,
		baseURL:     baseURL,
		headers:     settings.Headers,
		queryParams: settings.QueryParams,
		httpClient:  settings.HTTPClient,
		providerID:  provider.ProviderID(providerName),
	}
}

func (p *Provider) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (p *Provider) LanguageModel(modelID provider.ModelID) (provider.LanguageModelV3, error) {
	return &languageModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return &embeddingModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return &imageModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) requestHeaders() map[string]string {
	headers := map[string]string{}
	if p.apiKey != "" {
		headers["Authorization"] = "Bearer " + p.apiKey
	}
	for key, value := range p.headers {
		headers[key] = value
	}
	return headers
}

func (p *Provider) endpoint(path string) string {
	base := p.baseURL + path
	if len(p.queryParams) == 0 {
		return base
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return base
	}
	query := parsed.Query()
	for key, value := range p.queryParams {
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

type languageModel struct {
	provider *Provider
	modelID  provider.ModelID
}

func (m *languageModel) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (m *languageModel) ProviderID() provider.ProviderID {
	return m.provider.providerID
}

func (m *languageModel) ModelID() provider.ModelID {
	return m.modelID
}

func (m *languageModel) SupportedURLs() provider.SupportedURLPatterns {
	return nil
}

func (m *languageModel) DoGenerate(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3GenerateResult, error) {
	_, err := m.doRequest(ctx, options, false)
	if err != nil {
		return provider.LanguageModelV3GenerateResult{}, err
	}
	return provider.LanguageModelV3GenerateResult{}, nil
}

func (m *languageModel) DoStream(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3StreamResult, error) {
	payload, endpoint, err := m.buildPayload(options, true)
	if err != nil {
		return provider.LanguageModelV3StreamResult{}, err
	}

	headers := m.provider.requestHeaders()

	body, err := json.Marshal(payload)
	if err != nil {
		return provider.LanguageModelV3StreamResult{}, err
	}

	baseHeaders := map[string]string{"Content-Type": "application/json"}
	for key, value := range headers {
		baseHeaders[key] = value
	}
	req, cancel, err := providerutils.BuildRequest(ctx, http.MethodPost, m.provider.endpoint(endpoint), bytes.NewReader(body), baseHeaders, options.RequestOptions)
	if err != nil {
		return provider.LanguageModelV3StreamResult{}, err
	}
	if cancel != nil {
		defer cancel()
	}

	client := m.provider.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return provider.LanguageModelV3StreamResult{}, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return provider.LanguageModelV3StreamResult{}, newOpenAICompatibleAPIError(resp.StatusCode, body, resp.Header, m.provider.providerID, m.modelID)
	}

	responseHeaders := cloneHeaders(resp.Header)
	responseMetadata := &provider.ResponseMetadata{
		RequestID:  resp.Header.Get("x-request-id"),
		HTTPStatus: resp.StatusCode,
		Headers:    responseHeaders,
	}

	stream := make(chan provider.StreamPart)
	result := provider.LanguageModelV3StreamResult{
		Stream: stream,
		Request: &provider.LanguageModelV3Request{
			Body: payload,
		},
		Response: &provider.LanguageModelV3Response{Headers: responseHeaders},
	}

	state := &streamState{
		includeRaw:      options.IncludeRawChunks,
		toolAccumulator: providerutils.NewToolArgumentAccumulator(),
		toolCalls:       map[string]struct{}{},
	}
	mode := m.resolveMode(options)

	go func() {
		defer close(stream)
		defer resp.Body.Close()
		stream <- provider.StreamPart{
			Type:        provider.StreamPartTypeStreamStart,
			StreamStart: &provider.StreamStart{ProviderID: m.provider.providerID, ModelID: m.modelID},
		}
		parseErr := providerutils.ParseSSE(ctx, resp.Body, providerutils.SSEParseOptions{
			OnEvent: func(event providerutils.SSEEvent) error {
				if event.Data == "" {
					return nil
				}
				if event.Data == "[DONE]" {
					return nil
				}
				if state.includeRaw {
					stream <- rawStreamPart(event.Data)
				}
				switch mode {
				case modeResponses:
					return handleResponsesEvent(stream, state, event.Data, responseMetadata)
				case modeCompletions:
					return handleChatEvent(stream, state, event.Data, responseMetadata, m.provider.providerID, true)
				default:
					return handleChatEvent(stream, state, event.Data, responseMetadata, m.provider.providerID, false)
				}
			},
		})
		if parseErr != nil && !errors.Is(parseErr, context.Canceled) && !errors.Is(parseErr, io.EOF) {
			stream <- provider.StreamPart{Type: provider.StreamPartTypeError, Error: &provider.StreamError{Err: parseErr}}
		}
	}()

	return result, nil
}

func (m *languageModel) doRequest(ctx context.Context, options provider.LanguageModelV3CallOptions, stream bool) (providerutils.HTTPResponse, error) {
	payload, endpoint, err := m.buildPayload(options, stream)
	if err != nil {
		return providerutils.HTTPResponse{}, err
	}
	headers := m.provider.requestHeaders()
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.endpoint(endpoint), payload, requestOptions, nil, nil)
	if err != nil {
		return response, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return response, newOpenAICompatibleAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	return response, nil
}

type openAICompatibleMode string

const (
	modeChat        openAICompatibleMode = "chat"
	modeResponses   openAICompatibleMode = "responses"
	modeCompletions openAICompatibleMode = "completions"
)

func (m *languageModel) resolveMode(options provider.LanguageModelV3CallOptions) openAICompatibleMode {
	resolved := modeChat
	if opts := openAICompatibleOptions(resolveProviderOptions(options.ProviderOptions, options.RequestOptions), m.provider.providerID); opts != nil {
		if modeValue, ok := opts["mode"].(string); ok {
			switch modeValue {
			case "responses":
				resolved = modeResponses
			case "completions":
				resolved = modeCompletions
			}
		}
	}
	return resolved
}

func (m *languageModel) buildPayload(options provider.LanguageModelV3CallOptions, stream bool) (map[string]any, string, error) {
	mode := m.resolveMode(options)
	basePayload := map[string]any{}
	endpoint := ""
	compatOpts := openAICompatibleOptions(resolveProviderOptions(options.ProviderOptions, options.RequestOptions), m.provider.providerID)
	requestOverrides := openAICompatibleRequestOverrides(compatOpts)
	tools := openAICompatibleTools(compatOpts)

	switch mode {
	case modeResponses:
		endpoint = "/responses"
		basePayload["model"] = string(m.modelID)
		basePayload["input"] = promptToText(options.Prompt)
		if options.MaxOutputTokens > 0 {
			basePayload["max_output_tokens"] = options.MaxOutputTokens
		}
	case modeCompletions:
		endpoint = "/completions"
		basePayload["model"] = string(m.modelID)
		basePayload["prompt"] = promptToText(options.Prompt)
		if options.MaxOutputTokens > 0 {
			basePayload["max_tokens"] = options.MaxOutputTokens
		}
	default:
		endpoint = "/chat/completions"
		basePayload["model"] = string(m.modelID)
		basePayload["messages"] = chatMessages(options.Prompt)
		if options.MaxOutputTokens > 0 {
			basePayload["max_tokens"] = options.MaxOutputTokens
		}
	}

	if stream {
		basePayload["stream"] = true
	}
	if options.Temperature != 0 {
		basePayload["temperature"] = options.Temperature
	}
	if options.TopP != 0 {
		basePayload["top_p"] = options.TopP
	}
	if options.TopK != 0 {
		basePayload["top_k"] = options.TopK
	}
	if options.PresencePenalty != 0 {
		basePayload["presence_penalty"] = options.PresencePenalty
	}
	if options.FrequencyPenalty != 0 {
		basePayload["frequency_penalty"] = options.FrequencyPenalty
	}
	if options.Seed != 0 {
		basePayload["seed"] = options.Seed
	}
	if len(options.StopSequences) > 0 {
		basePayload["stop"] = options.StopSequences
	}
	if options.ToolChoice != nil {
		basePayload["tool_choice"] = toolChoicePayload(*options.ToolChoice)
	}
	if options.ResponseFormat != nil {
		basePayload["response_format"] = responseFormatPayload(*options.ResponseFormat)
	}
	if tools != nil {
		basePayload["tools"] = tools
	}
	for key, value := range requestOverrides {
		basePayload[key] = value
	}

	return basePayload, endpoint, nil
}

func resolveProviderOptions(explicit provider.ProviderOptions, requestOptions provider.RequestOptions) provider.ProviderOptions {
	if explicit != nil {
		return explicit
	}
	if requestOptions.ProviderOptions != nil {
		return requestOptions.ProviderOptions
	}
	return nil
}

func openAICompatibleOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
	if options == nil {
		return nil
	}
	if providerID == "" {
		providerID = DefaultProviderName
	}
	value, ok := options[string(providerID)]
	if !ok {
		return nil
	}
	return value
}

func openAICompatibleTools(options provider.JSONObject) any {
	if options == nil {
		return nil
	}
	if tools, ok := options["tools"]; ok {
		return tools
	}
	return nil
}

func openAICompatibleRequestOverrides(options provider.JSONObject) map[string]any {
	overrides := map[string]any{}
	if options == nil {
		return overrides
	}
	for key, value := range options {
		switch key {
		case "mode", "tools":
			continue
		case "request":
			for requestKey, requestValue := range normalizeObject(value) {
				overrides[requestKey] = requestValue
			}
		default:
			overrides[key] = value
		}
	}
	return overrides
}

func normalizeObject(value any) map[string]any {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	if typed, ok := value.(provider.JSONObject); ok {
		converted := make(map[string]any, len(typed))
		for key, inner := range typed {
			converted[key] = inner
		}
		return converted
	}
	return nil
}

type openAICompatibleChatMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content,omitempty"`
	Name       string `json:"name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

func chatMessages(prompt provider.Prompt) []openAICompatibleChatMessage {
	messages := make([]openAICompatibleChatMessage, 0, len(prompt.Messages))
	for _, message := range prompt.Messages {
		content := messageContentText(message)
		entry := openAICompatibleChatMessage{
			Role:    string(message.Role),
			Content: content,
		}
		if message.Name != "" {
			entry.Name = message.Name
		}
		if message.ToolCallID != "" {
			entry.ToolCallID = message.ToolCallID
		}
		messages = append(messages, entry)
	}
	return messages
}

func messageContentText(message provider.ModelMessage) string {
	var builder strings.Builder
	for _, part := range message.Content {
		text := contentPartText(part)
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(text)
	}
	return builder.String()
}

func promptToText(prompt provider.Prompt) string {
	var builder strings.Builder
	for _, message := range prompt.Messages {
		text := messageContentText(message)
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(text)
	}
	return builder.String()
}

func contentPartText(part provider.ContentPart) string {
	switch typed := part.(type) {
	case provider.TextContent:
		return typed.Text
	case *provider.TextContent:
		return typed.Text
	case provider.ToolResultContent:
		return stringifyJSONValue(typed.ToolResult.Result)
	case *provider.ToolResultContent:
		return stringifyJSONValue(typed.ToolResult.Result)
	case provider.ReasoningContent:
		return typed.Text
	case *provider.ReasoningContent:
		return typed.Text
	default:
		return ""
	}
}

func stringifyJSONValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		payload, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(payload)
	}
}

func toolChoicePayload(choice provider.ToolChoice) any {
	switch choice.Type {
	case provider.ToolChoiceTypeNone:
		return "none"
	case provider.ToolChoiceTypeRequired:
		return "required"
	case provider.ToolChoiceTypeTool:
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": choice.ToolName,
			},
		}
	default:
		return "auto"
	}
}

func responseFormatPayload(format provider.ResponseFormat) any {
	if format.Type == provider.ResponseFormatTypeJSON {
		if format.Schema != nil {
			payload := map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"schema": format.Schema,
				},
			}
			if format.Name != "" {
				payload["json_schema"].(map[string]any)["name"] = format.Name
			}
			if format.Description != "" {
				payload["json_schema"].(map[string]any)["description"] = format.Description
			}
			return payload
		}
		return map[string]any{"type": "json_object"}
	}
	return map[string]any{"type": "text"}
}

type streamState struct {
	textStarted     bool
	reasoningActive bool
	includeRaw      bool
	toolAccumulator *providerutils.ToolArgumentAccumulator
	toolCalls       map[string]struct{}
}

type openAICompatibleChatChunk struct {
	Choices []openAICompatibleChatChoice `json:"choices"`
	Usage   *openAICompatibleUsage       `json:"usage"`
}

type openAICompatibleChatChoice struct {
	Delta        openAICompatibleChatDelta `json:"delta"`
	Text         string                    `json:"text"`
	FinishReason string                    `json:"finish_reason"`
}

type openAICompatibleChatDelta struct {
	Content   string                      `json:"content"`
	Reasoning string                      `json:"reasoning"`
	ToolCalls []openAICompatibleToolDelta `json:"tool_calls"`
}

type openAICompatibleToolDelta struct {
	ID       string                      `json:"id"`
	Type     string                      `json:"type"`
	Function openAICompatibleToolRequest `json:"function"`
}

type openAICompatibleToolRequest struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAICompatibleUsage struct {
	PromptTokens            int                           `json:"prompt_tokens"`
	CompletionTokens        int                           `json:"completion_tokens"`
	TotalTokens             int                           `json:"total_tokens"`
	CompletionTokensDetails *openAICompatibleTokenDetails `json:"completion_tokens_details"`
	PromptTokensDetails     *openAICompatibleTokenDetails `json:"prompt_tokens_details"`
}

type openAICompatibleTokenDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

type openAICompatibleResponsesChunk struct {
	Type     string                        `json:"type"`
	Delta    string                        `json:"delta"`
	Response openAICompatibleResponsesData `json:"response"`
}

type openAICompatibleResponsesData struct {
	Usage             openAICompatibleResponsesUsage `json:"usage"`
	IncompleteDetails openAICompatibleIncomplete     `json:"incomplete_details"`
}

type openAICompatibleResponsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type openAICompatibleIncomplete struct {
	Reason string `json:"reason"`
}

func handleChatEvent(stream chan<- provider.StreamPart, state *streamState, data string, metadata *provider.ResponseMetadata, providerID provider.ProviderID, isCompletion bool) error {
	var chunk openAICompatibleChatChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return err
	}
	if len(chunk.Choices) == 0 {
		return nil
	}
	choice := chunk.Choices[0]
	text := choice.Delta.Content
	if isCompletion && choice.Text != "" {
		text = choice.Text
	}
	if choice.Delta.Reasoning != "" {
		emitReasoning(stream, state, choice.Delta.Reasoning)
	}
	if text != "" {
		emitText(stream, state, text)
	}
	if len(choice.Delta.ToolCalls) > 0 {
		if err := handleToolCallDelta(stream, state, choice.Delta.ToolCalls); err != nil {
			return err
		}
	}
	if choice.FinishReason != "" {
		usage := usageFromChatChunk(chunk.Usage)
		finish := provider.Finish{Reason: mapFinishReason(choice.FinishReason), Usage: usage}
		providerMetadata := usageMetadataFromChatChunk(chunk.Usage, providerID)
		finalizeTools(stream, state)
		finalizeReasoning(stream, state)
		stream <- provider.StreamPart{Type: provider.StreamPartTypeFinish, Finish: &finish, ResponseMetadata: metadata, ProviderMetadata: providerMetadata}
	}
	return nil
}

func handleResponsesEvent(stream chan<- provider.StreamPart, state *streamState, data string, metadata *provider.ResponseMetadata) error {
	var chunk openAICompatibleResponsesChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return err
	}
	switch chunk.Type {
	case "response.output_text.delta":
		if chunk.Delta != "" {
			emitText(stream, state, chunk.Delta)
		}
	case "response.completed", "response.incomplete":
		reason := provider.FinishReasonStop
		if chunk.Type == "response.incomplete" {
			reason = provider.FinishReasonOther
			if chunk.Response.IncompleteDetails.Reason == "max_tokens" {
				reason = provider.FinishReasonLength
			}
		}
		usage := usageFromResponsesChunk(chunk.Response.Usage)
		finish := provider.Finish{Reason: reason, Usage: usage}
		stream <- provider.StreamPart{Type: provider.StreamPartTypeFinish, Finish: &finish, ResponseMetadata: metadata}
	}
	return nil
}

func emitText(stream chan<- provider.StreamPart, state *streamState, text string) {
	if !state.textStarted {
		state.textStarted = true
		stream <- provider.StreamPart{Type: provider.StreamPartTypeTextStart, TextStart: &provider.TextStart{Text: text}}
		return
	}
	stream <- provider.StreamPart{Type: provider.StreamPartTypeTextDelta, TextDelta: &provider.TextDelta{Delta: text}}
}

func emitReasoning(stream chan<- provider.StreamPart, state *streamState, text string) {
	if !state.reasoningActive {
		state.reasoningActive = true
		stream <- provider.StreamPart{Type: provider.StreamPartTypeReasoningStart, ReasoningStart: &provider.ReasoningStart{Text: text}}
		return
	}
	stream <- provider.StreamPart{Type: provider.StreamPartTypeReasoningDelta, ReasoningDelta: &provider.ReasoningDelta{Delta: text}}
}

func handleToolCallDelta(stream chan<- provider.StreamPart, state *streamState, deltas []openAICompatibleToolDelta) error {
	for _, delta := range deltas {
		if delta.ID == "" {
			continue
		}
		if _, exists := state.toolCalls[delta.ID]; !exists && delta.Function.Name != "" {
			state.toolAccumulator.Start(delta.ID, delta.Function.Name)
			state.toolCalls[delta.ID] = struct{}{}
			stream <- provider.StreamPart{Type: provider.StreamPartTypeToolInputStart, ToolInputStart: &provider.ToolInputStart{ToolCallID: delta.ID, Name: delta.Function.Name}}
		}
		if delta.Function.Arguments != "" {
			if err := state.toolAccumulator.AddDelta(delta.ID, delta.Function.Arguments); err != nil {
				return err
			}
			stream <- provider.StreamPart{Type: provider.StreamPartTypeToolInputDelta, ToolInputDelta: &provider.ToolInputDelta{ToolCallID: delta.ID, Delta: delta.Function.Arguments}}
		}
	}
	return nil
}

func finalizeTools(stream chan<- provider.StreamPart, state *streamState) {
	for callID := range state.toolCalls {
		call, err := state.toolAccumulator.End(callID)
		if err != nil {
			stream <- provider.StreamPart{Type: provider.StreamPartTypeError, Error: &provider.StreamError{Err: err}}
			continue
		}
		stream <- provider.StreamPart{Type: provider.StreamPartTypeToolInputEnd, ToolInputEnd: &provider.ToolInputEnd{ToolCallID: callID}}
		stream <- provider.StreamPart{Type: provider.StreamPartTypeToolCall, ToolCall: &call}
	}
}

func finalizeReasoning(stream chan<- provider.StreamPart, state *streamState) {
	if !state.reasoningActive {
		return
	}
	stream <- provider.StreamPart{Type: provider.StreamPartTypeReasoningEnd, ReasoningEnd: &provider.ReasoningEnd{Text: ""}}
	state.reasoningActive = false
}

func usageFromChatChunk(usage *openAICompatibleUsage) *provider.LanguageModelUsage {
	if usage == nil {
		return nil
	}
	return &provider.LanguageModelUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func usageMetadataFromChatChunk(usage *openAICompatibleUsage, providerID provider.ProviderID) provider.ProviderMetadata {
	if usage == nil {
		return nil
	}
	metadata := map[string]any{}
	if usage.CompletionTokensDetails != nil && usage.CompletionTokensDetails.ReasoningTokens != 0 {
		metadata["reasoning_tokens"] = usage.CompletionTokensDetails.ReasoningTokens
	}
	if len(metadata) == 0 {
		return nil
	}
	return provider.ProviderMetadata{string(providerID): metadata}
}

func usageFromResponsesChunk(usage openAICompatibleResponsesUsage) *provider.LanguageModelUsage {
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return nil
	}
	total := usage.InputTokens + usage.OutputTokens
	return &provider.LanguageModelUsage{
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      total,
	}
}

func mapFinishReason(reason string) provider.FinishReason {
	switch reason {
	case "stop":
		return provider.FinishReasonStop
	case "length":
		return provider.FinishReasonLength
	case "tool_calls":
		return provider.FinishReasonToolCalls
	case "function_call":
		return provider.FinishReasonToolCalls
	case "content_filter":
		return provider.FinishReasonContentFilter
	default:
		return provider.FinishReasonOther
	}
}

func rawStreamPart(data string) provider.StreamPart {
	var payload any
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		payload = data
	}
	return provider.StreamPart{Type: provider.StreamPartTypeRaw, Raw: payload}
}

type embeddingModel struct {
	provider *Provider
	modelID  provider.ModelID
}

func (m *embeddingModel) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (m *embeddingModel) ProviderID() provider.ProviderID {
	return m.provider.providerID
}

func (m *embeddingModel) ModelID() provider.ModelID {
	return m.modelID
}

func (m *embeddingModel) DoEmbed(ctx context.Context, options provider.EmbeddingModelV3CallOptions) (provider.EmbeddingModelV3Result, error) {
	if len(options.Values) == 0 {
		return provider.EmbeddingModelV3Result{}, provider.NewInvalidRequestError("embedding values are required", nil)
	}
	payload := map[string]any{
		"model": string(m.modelID),
		"input": options.Values,
	}
	compatOpts := openAICompatibleOptions(options.RequestOptions.ProviderOptions, m.provider.providerID)
	for key, value := range openAICompatibleRequestOverrides(compatOpts) {
		payload[key] = value
	}
	headers := m.provider.requestHeaders()
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.endpoint("/embeddings"), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.EmbeddingModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.EmbeddingModelV3Result{}, newOpenAICompatibleAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	return provider.EmbeddingModelV3Result{}, nil
}

type imageModel struct {
	provider *Provider
	modelID  provider.ModelID
}

func (m *imageModel) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (m *imageModel) ProviderID() provider.ProviderID {
	return m.provider.providerID
}

func (m *imageModel) ModelID() provider.ModelID {
	return m.modelID
}

func (m *imageModel) DoGenerate(ctx context.Context, options provider.ImageModelV3CallOptions) (provider.ImageModelV3Result, error) {
	payload := map[string]any{
		"model": string(m.modelID),
	}
	if options.Prompt != "" {
		payload["prompt"] = options.Prompt
	}
	if options.N > 0 {
		payload["n"] = options.N
	}
	if options.Size != "" {
		payload["size"] = options.Size
	}
	if options.AspectRatio != "" {
		payload["aspect_ratio"] = options.AspectRatio
	}
	if options.Seed > 0 {
		payload["seed"] = options.Seed
	}
	compatOpts := openAICompatibleOptions(options.RequestOptions.ProviderOptions, m.provider.providerID)
	for key, value := range openAICompatibleRequestOverrides(compatOpts) {
		payload[key] = value
	}
	headers := m.provider.requestHeaders()
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.endpoint("/images/generations"), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.ImageModelV3Result{}, newOpenAICompatibleAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	return provider.ImageModelV3Result{}, nil
}

func newOpenAICompatibleAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("openai compatible api error (%d)", status)
	if len(body) > 0 {
		var payload struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			if payload.Error.Message != "" {
				message = payload.Error.Message
			}
		}
	}
	requestID := headers.Get("x-request-id")
	base := provider.ApiCallError{
		AISDKError: provider.AISDKError{
			Category: provider.ErrorCategoryAPICall,
			Kind:     provider.ErrorKindAPICall,
			Message:  message,
		},
		StatusCode:      status,
		RequestID:       requestID,
		ResponseHeaders: cloneHeaders(headers),
		ResponseBody:    string(body),
		ProviderID:      providerID,
		ModelID:         modelID,
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return &provider.AuthenticationError{ApiCallError: base}
	}
	if status == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(headers)
		return &provider.RateLimitError{ApiCallError: base, RetryAfterSeconds: retryAfter}
	}
	if status == http.StatusBadRequest {
		return &provider.InvalidRequestError{ApiCallError: base}
	}
	if status == http.StatusNotFound {
		return &provider.NoSuchModelError{
			AISDKError: provider.AISDKError{Category: provider.ErrorCategorySDK, Kind: provider.ErrorKindNoSuchModel, Message: message},
			ProviderID: providerID,
			ModelID:    modelID,
		}
	}
	if status >= http.StatusInternalServerError {
		return &provider.InternalServerError{ApiCallError: base}
	}
	return &base
}

func cloneHeaders(headers http.Header) map[string][]string {
	if headers == nil {
		return nil
	}
	cloned := make(map[string][]string, len(headers))
	for key, values := range headers {
		copied := make([]string, len(values))
		copy(copied, values)
		cloned[key] = copied
	}
	return cloned
}

func mergeRequestOptions(options provider.RequestOptions, headers map[string]string) provider.RequestOptions {
	if len(headers) == 0 {
		return options
	}
	merged := options
	merged.Headers = providerutils.MergeHeaders(headers, options.Headers)
	return merged
}

func parseRetryAfter(headers http.Header) int {
	if headers == nil {
		return 0
	}
	value := headers.Get("Retry-After")
	if value == "" {
		return 0
	}
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return seconds
}
