package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

const (
	DefaultBaseURL         = "https://api.anthropic.com/v1"
	DefaultMaxOutputTokens = 1024
)

type Settings struct {
	APIKey       string
	BaseURL      string
	Headers      map[string]string
	HTTPClient   *http.Client
	ProviderName string
}

type Provider struct {
	apiKey     string
	baseURL    string
	headers    map[string]string
	httpClient *http.Client
	providerID provider.ProviderID
}

func CreateAnthropic(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	baseURL := strings.TrimRight(settings.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	providerName := settings.ProviderName
	if providerName == "" {
		providerName = "anthropic"
	}
	return &Provider{
		apiKey:     apiKey,
		baseURL:    baseURL,
		headers:    settings.Headers,
		httpClient: settings.HTTPClient,
		providerID: provider.ProviderID(providerName),
	}
}

func (p *Provider) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (p *Provider) LanguageModel(modelID provider.ModelID) (provider.LanguageModelV3, error) {
	return &languageModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewUnsupportedFunctionalityError("anthropic does not support embedding models", nil, "embedding")
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return nil, provider.NewUnsupportedFunctionalityError("anthropic does not support image models", nil, "image")
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("anthropic api key is required")
	}
	headers := map[string]string{
		"x-api-key":         p.apiKey,
		"anthropic-version": "2023-06-01",
	}
	for key, value := range p.headers {
		headers[key] = value
	}
	return headers, nil
}

func (p *Provider) endpoint(path string) string {
	return p.baseURL + path
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
	payload, err := m.buildPayload(options)
	if err != nil {
		return provider.LanguageModelV3GenerateResult{}, err
	}
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.LanguageModelV3GenerateResult{}, err
	}
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.endpoint("/messages"), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.LanguageModelV3GenerateResult{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.LanguageModelV3GenerateResult{}, newAnthropicAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	return provider.LanguageModelV3GenerateResult{}, nil
}

func (m *languageModel) DoStream(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3StreamResult, error) {
	payload, err := m.buildPayload(options)
	if err != nil {
		return provider.LanguageModelV3StreamResult{}, err
	}
	payload["stream"] = true

	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.LanguageModelV3StreamResult{}, err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return provider.LanguageModelV3StreamResult{}, err
	}

	baseHeaders := map[string]string{"Content-Type": "application/json"}
	for key, value := range headers {
		baseHeaders[key] = value
	}
	req, cancel, err := providerutils.BuildRequest(ctx, http.MethodPost, m.provider.endpoint("/messages"), bytes.NewReader(body), baseHeaders, options.RequestOptions)
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
		return provider.LanguageModelV3StreamResult{}, newAnthropicAPIError(resp.StatusCode, body, resp.Header, m.provider.providerID, m.modelID)
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

	state := &anthropicStreamState{
		includeRaw:      options.IncludeRawChunks,
		toolAccumulator: providerutils.NewToolArgumentAccumulator(),
		blockTypes:      map[int]string{},
		toolCalls:       map[int]anthropicToolCall{},
	}

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
				if state.includeRaw {
					stream <- rawStreamPart(event.Data)
				}
				return handleAnthropicEvent(stream, state, event.Data, responseMetadata)
			},
		})
		if parseErr != nil && !errors.Is(parseErr, context.Canceled) && !errors.Is(parseErr, io.EOF) {
			stream <- provider.StreamPart{Type: provider.StreamPartTypeError, Error: &provider.StreamError{Err: parseErr}}
		}
	}()

	return result, nil
}

func (m *languageModel) buildPayload(options provider.LanguageModelV3CallOptions) (map[string]any, error) {
	messages, system, err := promptToMessages(options.Prompt)
	if err != nil {
		return nil, err
	}
	maxTokens := options.MaxOutputTokens
	if maxTokens == 0 {
		maxTokens = DefaultMaxOutputTokens
	}
	payload := map[string]any{
		"model":      string(m.modelID),
		"max_tokens": maxTokens,
		"messages":   messages,
	}
	if system != "" {
		payload["system"] = system
	}
	if options.Temperature != 0 {
		payload["temperature"] = options.Temperature
	}
	if options.TopP != 0 {
		payload["top_p"] = options.TopP
	}
	if options.TopK != 0 {
		payload["top_k"] = options.TopK
	}
	if len(options.StopSequences) > 0 {
		payload["stop_sequences"] = options.StopSequences
	}
	if options.ToolChoice != nil {
		payload["tool_choice"] = toolChoicePayload(*options.ToolChoice)
	}
	if options.ResponseFormat != nil {
		if format := responseFormatPayload(*options.ResponseFormat); format != nil {
			payload["output_format"] = format
		}
	}
	anthropicOpts := anthropicOptions(resolveProviderOptions(options.ProviderOptions, options.RequestOptions), m.provider.providerID)
	if tools := anthropicTools(anthropicOpts); tools != nil {
		payload["tools"] = tools
	}
	for key, value := range anthropicRequestOverrides(anthropicOpts) {
		payload[key] = value
	}
	return payload, nil
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   any            `json:"content,omitempty"`
	IsError   *bool          `json:"is_error,omitempty"`
}

func promptToMessages(prompt provider.Prompt) ([]anthropicMessage, string, error) {
	messages := make([]anthropicMessage, 0, len(prompt.Messages))
	var systemParts []string
	for _, message := range prompt.Messages {
		if message.Role == provider.RoleSystem {
			text := messageContentText(message)
			if text != "" {
				systemParts = append(systemParts, text)
			}
			continue
		}
		msg, err := convertMessage(message)
		if err != nil {
			return nil, "", err
		}
		messages = append(messages, msg)
	}
	return messages, strings.Join(systemParts, "\n"), nil
}

func convertMessage(message provider.ModelMessage) (anthropicMessage, error) {
	role := message.Role
	if role == provider.RoleTool {
		role = provider.RoleUser
	}
	content, err := convertContentParts(message.Content)
	if err != nil {
		return anthropicMessage{}, err
	}
	return anthropicMessage{Role: string(role), Content: content}, nil
}

func convertContentParts(parts []provider.ContentPart) ([]anthropicContent, error) {
	contents := make([]anthropicContent, 0, len(parts))
	for _, part := range parts {
		switch typed := part.(type) {
		case provider.TextContent:
			if typed.Text != "" {
				contents = append(contents, anthropicContent{Type: "text", Text: typed.Text})
			}
		case *provider.TextContent:
			if typed != nil && typed.Text != "" {
				contents = append(contents, anthropicContent{Type: "text", Text: typed.Text})
			}
		case provider.ToolCallContent:
			content, err := toolCallContent(typed.ToolCall)
			if err != nil {
				return nil, err
			}
			contents = append(contents, content)
		case *provider.ToolCallContent:
			if typed == nil {
				continue
			}
			content, err := toolCallContent(typed.ToolCall)
			if err != nil {
				return nil, err
			}
			contents = append(contents, content)
		case provider.ToolResultContent:
			content, err := toolResultContent(typed.ToolResult)
			if err != nil {
				return nil, err
			}
			contents = append(contents, content)
		case *provider.ToolResultContent:
			if typed == nil {
				continue
			}
			content, err := toolResultContent(typed.ToolResult)
			if err != nil {
				return nil, err
			}
			contents = append(contents, content)
		}
	}
	return contents, nil
}

func toolCallContent(call provider.ToolCall) (anthropicContent, error) {
	if call.ID == "" || call.Name == "" {
		return anthropicContent{}, provider.NewInvalidRequestError("tool call requires id and name", nil)
	}
	return anthropicContent{
		Type:  "tool_use",
		ID:    call.ID,
		Name:  call.Name,
		Input: call.Arguments,
	}, nil
}

func toolResultContent(result provider.ToolResult) (anthropicContent, error) {
	if result.ID == "" {
		return anthropicContent{}, provider.NewInvalidRequestError("tool result requires id", nil)
	}
	payload := result.Result
	isError := result.IsError
	if err, ok := result.Result.(error); ok {
		payload = err.Error()
		isError = true
	}
	return anthropicContent{
		Type:      "tool_result",
		ToolUseID: result.ID,
		Content:   payload,
		IsError:   &isError,
	}, nil
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

func contentPartText(part provider.ContentPart) string {
	switch typed := part.(type) {
	case provider.TextContent:
		return typed.Text
	case *provider.TextContent:
		if typed == nil {
			return ""
		}
		return typed.Text
	case provider.ReasoningContent:
		return typed.Text
	case *provider.ReasoningContent:
		if typed == nil {
			return ""
		}
		return typed.Text
	default:
		return ""
	}
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

func anthropicOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
	if options == nil {
		return nil
	}
	if providerID == "" {
		providerID = "anthropic"
	}
	value, ok := options[string(providerID)]
	if !ok {
		return nil
	}
	return value
}

func anthropicTools(options provider.JSONObject) any {
	if options == nil {
		return nil
	}
	if tools, ok := options["tools"]; ok {
		return normalizeAnthropicTools(tools)
	}
	return nil
}

func normalizeAnthropicTools(value any) any {
	switch typed := value.(type) {
	case []providerutils.ToolSpecification:
		return convertToolSpecifications(typed)
	case []*providerutils.ToolSpecification:
		specs := make([]providerutils.ToolSpecification, 0, len(typed))
		for _, spec := range typed {
			if spec == nil {
				continue
			}
			specs = append(specs, *spec)
		}
		return convertToolSpecifications(specs)
	case []any:
		converted := make([]any, 0, len(typed))
		for _, item := range typed {
			switch spec := item.(type) {
			case providerutils.ToolSpecification:
				converted = append(converted, convertToolSpecification(spec))
			case *providerutils.ToolSpecification:
				if spec == nil {
					continue
				}
				converted = append(converted, convertToolSpecification(*spec))
			default:
				converted = append(converted, item)
			}
		}
		return converted
	default:
		return value
	}
}

func convertToolSpecifications(specs []providerutils.ToolSpecification) []map[string]any {
	converted := make([]map[string]any, 0, len(specs))
	for _, spec := range specs {
		converted = append(converted, convertToolSpecification(spec))
	}
	return converted
}

func convertToolSpecification(spec providerutils.ToolSpecification) map[string]any {
	payload := map[string]any{
		"name": spec.Name,
	}
	if spec.Description != "" {
		payload["description"] = spec.Description
	}
	if spec.Parameters != nil {
		payload["input_schema"] = spec.Parameters
	}
	return payload
}

func anthropicRequestOverrides(options provider.JSONObject) map[string]any {
	overrides := map[string]any{}
	if options == nil {
		return overrides
	}
	for key, value := range options {
		switch key {
		case "tools":
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

func toolChoicePayload(choice provider.ToolChoice) any {
	switch choice.Type {
	case provider.ToolChoiceTypeRequired:
		return map[string]any{"type": "any"}
	case provider.ToolChoiceTypeTool:
		return map[string]any{"type": "tool", "name": choice.ToolName}
	default:
		return map[string]any{"type": "auto"}
	}
}

func responseFormatPayload(format provider.ResponseFormat) any {
	if format.Type != provider.ResponseFormatTypeJSON || format.Schema == nil {
		return nil
	}
	payload := map[string]any{
		"type":   "json_schema",
		"schema": format.Schema,
	}
	if format.Name != "" {
		payload["name"] = format.Name
	}
	if format.Description != "" {
		payload["description"] = format.Description
	}
	return payload
}

func mergeRequestOptions(options provider.RequestOptions, headers map[string]string) provider.RequestOptions {
	if len(headers) == 0 {
		return options
	}
	merged := options
	merged.Headers = providerutils.MergeHeaders(headers, options.Headers)
	return merged
}

type anthropicStreamState struct {
	includeRaw      bool
	toolAccumulator *providerutils.ToolArgumentAccumulator
	blockTypes      map[int]string
	toolCalls       map[int]anthropicToolCall
	usage           anthropicUsage
	finishReason    string
	finishSent      bool
}

type anthropicToolCall struct {
	ID   string
	Name string
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicMessageDelta struct {
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
}

type anthropicContentBlock struct {
	Type     string         `json:"type"`
	Text     string         `json:"text"`
	Thinking string         `json:"thinking"`
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Input    map[string]any `json:"input"`
}

type anthropicContentDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	Thinking    string `json:"thinking"`
	PartialJSON string `json:"partial_json"`
}

func handleAnthropicEvent(stream chan<- provider.StreamPart, state *anthropicStreamState, data string, metadata *provider.ResponseMetadata) error {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		return err
	}

	switch envelope.Type {
	case "ping":
		return nil
	case "message_start":
		var event struct {
			Message struct {
				Usage anthropicUsage `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return err
		}
		if event.Message.Usage.InputTokens != 0 {
			state.usage.InputTokens = event.Message.Usage.InputTokens
		}
		return nil
	case "content_block_start":
		var event struct {
			Index        int                   `json:"index"`
			ContentBlock anthropicContentBlock `json:"content_block"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return err
		}
		switch event.ContentBlock.Type {
		case "text":
			state.blockTypes[event.Index] = "text"
			stream <- provider.StreamPart{Type: provider.StreamPartTypeTextStart, TextStart: &provider.TextStart{Text: event.ContentBlock.Text}}
		case "thinking", "redacted_thinking":
			state.blockTypes[event.Index] = "reasoning"
			text := event.ContentBlock.Thinking
			stream <- provider.StreamPart{Type: provider.StreamPartTypeReasoningStart, ReasoningStart: &provider.ReasoningStart{Text: text}}
		case "tool_use":
			state.blockTypes[event.Index] = "tool_use"
			state.toolCalls[event.Index] = anthropicToolCall{ID: event.ContentBlock.ID, Name: event.ContentBlock.Name}
			state.toolAccumulator.Start(event.ContentBlock.ID, event.ContentBlock.Name)
			stream <- provider.StreamPart{Type: provider.StreamPartTypeToolInputStart, ToolInputStart: &provider.ToolInputStart{ToolCallID: event.ContentBlock.ID, Name: event.ContentBlock.Name}}
			if len(event.ContentBlock.Input) > 0 {
				payload, err := json.Marshal(event.ContentBlock.Input)
				if err != nil {
					return err
				}
				delta := string(payload)
				if err := state.toolAccumulator.AddDelta(event.ContentBlock.ID, delta); err != nil {
					return err
				}
				stream <- provider.StreamPart{Type: provider.StreamPartTypeToolInputDelta, ToolInputDelta: &provider.ToolInputDelta{ToolCallID: event.ContentBlock.ID, Delta: delta}}
			}
		}
		return nil
	case "content_block_delta":
		var event struct {
			Index int                   `json:"index"`
			Delta anthropicContentDelta `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return err
		}
		switch event.Delta.Type {
		case "text_delta":
			if event.Delta.Text != "" {
				stream <- provider.StreamPart{Type: provider.StreamPartTypeTextDelta, TextDelta: &provider.TextDelta{Delta: event.Delta.Text}}
			}
		case "thinking_delta":
			if event.Delta.Thinking != "" {
				stream <- provider.StreamPart{Type: provider.StreamPartTypeReasoningDelta, ReasoningDelta: &provider.ReasoningDelta{Delta: event.Delta.Thinking}}
			}
		case "input_json_delta":
			call, ok := state.toolCalls[event.Index]
			if !ok {
				return nil
			}
			if event.Delta.PartialJSON != "" {
				if err := state.toolAccumulator.AddDelta(call.ID, event.Delta.PartialJSON); err != nil {
					return err
				}
				stream <- provider.StreamPart{Type: provider.StreamPartTypeToolInputDelta, ToolInputDelta: &provider.ToolInputDelta{ToolCallID: call.ID, Delta: event.Delta.PartialJSON}}
			}
		}
		return nil
	case "content_block_stop":
		var event struct {
			Index int `json:"index"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return err
		}
		blockType := state.blockTypes[event.Index]
		delete(state.blockTypes, event.Index)
		if blockType == "tool_use" {
			call, ok := state.toolCalls[event.Index]
			if !ok {
				return nil
			}
			delete(state.toolCalls, event.Index)
			toolCall, err := state.toolAccumulator.End(call.ID)
			if err != nil {
				stream <- provider.StreamPart{Type: provider.StreamPartTypeError, Error: &provider.StreamError{Err: err}}
				return nil
			}
			stream <- provider.StreamPart{Type: provider.StreamPartTypeToolInputEnd, ToolInputEnd: &provider.ToolInputEnd{ToolCallID: call.ID}}
			stream <- provider.StreamPart{Type: provider.StreamPartTypeToolCall, ToolCall: &toolCall}
		}
		return nil
	case "message_delta":
		var event struct {
			Delta anthropicMessageDelta `json:"delta"`
			Usage anthropicUsage        `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return err
		}
		if event.Usage.OutputTokens != 0 {
			state.usage.OutputTokens = event.Usage.OutputTokens
		}
		if event.Delta.StopReason != "" {
			state.finishReason = event.Delta.StopReason
			emitAnthropicFinish(stream, state, metadata)
		}
		return nil
	case "message_stop":
		emitAnthropicFinish(stream, state, metadata)
		return nil
	default:
		return nil
	}
}

func emitAnthropicFinish(stream chan<- provider.StreamPart, state *anthropicStreamState, metadata *provider.ResponseMetadata) {
	if state.finishSent {
		return
	}
	state.finishSent = true
	finish := provider.Finish{Reason: mapAnthropicFinishReason(state.finishReason), Usage: usageFromAnthropic(state.usage)}
	stream <- provider.StreamPart{Type: provider.StreamPartTypeFinish, Finish: &finish, ResponseMetadata: metadata}
}

func mapAnthropicFinishReason(reason string) provider.FinishReason {
	switch reason {
	case "pause_turn", "end_turn", "stop_sequence":
		return provider.FinishReasonStop
	case "refusal":
		return provider.FinishReasonContentFilter
	case "tool_use":
		return provider.FinishReasonToolCalls
	case "max_tokens", "model_context_window_exceeded":
		return provider.FinishReasonLength
	default:
		return provider.FinishReasonOther
	}
}

func usageFromAnthropic(usage anthropicUsage) *provider.LanguageModelUsage {
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

func rawStreamPart(data string) provider.StreamPart {
	var payload any
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		payload = data
	}
	return provider.StreamPart{Type: provider.StreamPartTypeRaw, Raw: payload}
}

func newAnthropicAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("anthropic api error (%d)", status)
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
		return &provider.RateLimitError{ApiCallError: base}
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
