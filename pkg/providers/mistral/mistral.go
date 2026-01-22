package mistral

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

const DefaultBaseURL = "https://api.mistral.ai/v1"
const DefaultProviderName = "mistral"

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

func CreateMistral(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("MISTRAL_API_KEY")
	}
	baseURL := strings.TrimRight(settings.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	providerName := settings.ProviderName
	if providerName == "" {
		providerName = DefaultProviderName
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
	return &embeddingModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return nil, provider.NewNoSuchModelError("mistral does not support image models", nil, p.providerID, modelID)
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("mistral api key is required")
	}
	headers := map[string]string{
		"Authorization": "Bearer " + p.apiKey,
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
	_, err := m.doRequest(ctx, options, false)
	if err != nil {
		return provider.LanguageModelV3GenerateResult{}, err
	}
	return provider.LanguageModelV3GenerateResult{}, nil
}

func (m *languageModel) DoStream(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3StreamResult, error) {
	payload, err := m.buildPayload(options, true)
	if err != nil {
		return provider.LanguageModelV3StreamResult{}, err
	}

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
	req, cancel, err := providerutils.BuildRequest(ctx, http.MethodPost, m.provider.endpoint("/chat/completions"), bytes.NewReader(body), baseHeaders, options.RequestOptions)
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
		return provider.LanguageModelV3StreamResult{}, newMistralAPIError(resp.StatusCode, body, resp.Header, m.provider.providerID, m.modelID)
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
				return handleChatEvent(stream, state, event.Data, responseMetadata)
			},
		})
		if parseErr != nil && !errors.Is(parseErr, context.Canceled) && !errors.Is(parseErr, io.EOF) {
			stream <- provider.StreamPart{Type: provider.StreamPartTypeError, Error: &provider.StreamError{Err: parseErr}}
		}
	}()

	return result, nil
}

func (m *languageModel) doRequest(ctx context.Context, options provider.LanguageModelV3CallOptions, stream bool) (providerutils.HTTPResponse, error) {
	payload, err := m.buildPayload(options, stream)
	if err != nil {
		return providerutils.HTTPResponse{}, err
	}
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return providerutils.HTTPResponse{}, err
	}
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.endpoint("/chat/completions"), payload, requestOptions, nil, nil)
	if err != nil {
		return response, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return response, newMistralAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	return response, nil
}

func (m *languageModel) buildPayload(options provider.LanguageModelV3CallOptions, stream bool) (map[string]any, error) {
	basePayload := map[string]any{
		"model":    string(m.modelID),
		"messages": chatMessages(options.Prompt),
	}

	mistralOpts := mistralOptions(resolveProviderOptions(options.ProviderOptions, options.RequestOptions), m.provider.providerID)
	tooling, toolChoice := mistralTooling(mistralOpts, options.ToolChoice)

	if options.MaxOutputTokens > 0 {
		basePayload["max_tokens"] = options.MaxOutputTokens
	}
	if options.Temperature != 0 {
		basePayload["temperature"] = options.Temperature
	}
	if options.TopP != 0 {
		basePayload["top_p"] = options.TopP
	}
	if options.Seed != 0 {
		basePayload["random_seed"] = options.Seed
	}
	if options.ResponseFormat != nil {
		if payload := responseFormatPayload(*options.ResponseFormat, mistralOpts); payload != nil {
			basePayload["response_format"] = payload
		}
	}
	if tooling != nil {
		basePayload["tools"] = tooling
	}
	if toolChoice != nil {
		basePayload["tool_choice"] = toolChoice
	}
	if value := boolFromAny(mistralOpts["safePrompt"]); value != nil {
		basePayload["safe_prompt"] = *value
	}
	if value := intFromAny(mistralOpts["documentImageLimit"]); value != nil {
		basePayload["document_image_limit"] = *value
	}
	if value := intFromAny(mistralOpts["documentPageLimit"]); value != nil {
		basePayload["document_page_limit"] = *value
	}
	if tooling != nil {
		if value := boolFromAny(mistralOpts["parallelToolCalls"]); value != nil {
			basePayload["parallel_tool_calls"] = *value
		}
	}
	if stream {
		basePayload["stream"] = true
	}
	for key, value := range mistralRequestOverrides(mistralOpts) {
		basePayload[key] = value
	}

	return basePayload, nil
}

type streamState struct {
	textStarted     bool
	includeRaw      bool
	toolAccumulator *providerutils.ToolArgumentAccumulator
	toolCalls       map[string]struct{}
}

type mistralChatChunk struct {
	Choices []mistralChatChoice `json:"choices"`
	Usage   *mistralUsage       `json:"usage"`
}

type mistralChatChoice struct {
	Delta        mistralChatDelta `json:"delta"`
	FinishReason string           `json:"finish_reason"`
}

type mistralChatDelta struct {
	Content   json.RawMessage        `json:"content"`
	ToolCalls []mistralToolCallDelta `json:"tool_calls"`
}

type mistralToolCallDelta struct {
	ID       string              `json:"id"`
	Function mistralToolFunction `json:"function"`
}

type mistralToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type mistralUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func handleChatEvent(stream chan<- provider.StreamPart, state *streamState, data string, metadata *provider.ResponseMetadata) error {
	var chunk mistralChatChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return err
	}
	if len(chunk.Choices) == 0 {
		return nil
	}
	choice := chunk.Choices[0]
	text := extractTextContent(choice.Delta.Content)
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
		finalizeTools(stream, state)
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

func handleToolCallDelta(stream chan<- provider.StreamPart, state *streamState, deltas []mistralToolCallDelta) error {
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

func usageFromChatChunk(usage *mistralUsage) *provider.LanguageModelUsage {
	if usage == nil {
		return nil
	}
	return &provider.LanguageModelUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func mapFinishReason(reason string) provider.FinishReason {
	switch reason {
	case "stop":
		return provider.FinishReasonStop
	case "length", "model_length":
		return provider.FinishReasonLength
	case "tool_calls":
		return provider.FinishReasonToolCalls
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
	mistralOpts := mistralOptions(options.RequestOptions.ProviderOptions, m.provider.providerID)
	for key, value := range mistralRequestOverrides(mistralOpts) {
		payload[key] = value
	}
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.EmbeddingModelV3Result{}, err
	}
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.endpoint("/embeddings"), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.EmbeddingModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.EmbeddingModelV3Result{}, newMistralAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	return provider.EmbeddingModelV3Result{}, nil
}

type mistralTool struct {
	Type     string          `json:"type"`
	Function mistralToolSpec `json:"function"`
}

type mistralToolSpec struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Parameters  provider.JSONSchema `json:"parameters"`
	Strict      *bool               `json:"strict,omitempty"`
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

func mistralOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func mistralTooling(options provider.JSONObject, toolChoice *provider.ToolChoice) ([]mistralTool, any) {
	tools := mistralTools(options)
	if toolChoice == nil {
		if len(tools) == 0 {
			return nil, nil
		}
		return tools, nil
	}
	switch toolChoice.Type {
	case provider.ToolChoiceTypeAuto:
		return tools, "auto"
	case provider.ToolChoiceTypeNone:
		return tools, "none"
	case provider.ToolChoiceTypeRequired:
		return tools, "any"
	case provider.ToolChoiceTypeTool:
		if len(tools) == 0 {
			return nil, "any"
		}
		filtered := make([]mistralTool, 0, len(tools))
		for _, tool := range tools {
			if tool.Function.Name == toolChoice.ToolName {
				filtered = append(filtered, tool)
			}
		}
		return filtered, "any"
	default:
		return tools, "auto"
	}
}

func mistralTools(options provider.JSONObject) []mistralTool {
	if options == nil {
		return nil
	}
	rawTools, ok := options["tools"]
	if !ok {
		return nil
	}
	switch typed := rawTools.(type) {
	case []mistralTool:
		return typed
	case []providerutils.ToolSpecification:
		tools := make([]mistralTool, 0, len(typed))
		for _, tool := range typed {
			tools = append(tools, mistralTool{
				Type: "function",
				Function: mistralToolSpec{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			})
		}
		return tools
	case []any:
		tools := make([]mistralTool, 0, len(typed))
		for _, entry := range typed {
			tool, ok := parseMistralTool(entry)
			if !ok {
				continue
			}
			tools = append(tools, tool)
		}
		return tools
	default:
		return nil
	}
}

func parseMistralTool(entry any) (mistralTool, bool) {
	switch typed := entry.(type) {
	case mistralTool:
		return typed, true
	case providerutils.ToolSpecification:
		return mistralTool{
			Type: "function",
			Function: mistralToolSpec{
				Name:        typed.Name,
				Description: typed.Description,
				Parameters:  typed.Parameters,
			},
		}, true
	case map[string]any:
		if function, ok := typed["function"].(map[string]any); ok {
			name, _ := function["name"].(string)
			if name == "" {
				return mistralTool{}, false
			}
			description, _ := function["description"].(string)
			strict := boolFromAny(function["strict"])
			parameters, _ := function["parameters"].(map[string]any)
			tool := mistralTool{
				Type: "function",
				Function: mistralToolSpec{
					Name:        name,
					Description: description,
					Parameters:  parameters,
					Strict:      strict,
				},
			}
			return tool, true
		}
	}
	return mistralTool{}, false
}

func responseFormatPayload(format provider.ResponseFormat, options provider.JSONObject) any {
	if format.Type != provider.ResponseFormatTypeJSON {
		return map[string]any{"type": "text"}
	}
	structuredOutputs := true
	if value := boolFromAny(options["structuredOutputs"]); value != nil {
		structuredOutputs = *value
	}
	strictSchema := false
	if value := boolFromAny(options["strictJsonSchema"]); value != nil {
		strictSchema = *value
	}
	if format.Schema != nil && structuredOutputs {
		payload := map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"schema": format.Schema,
				"strict": strictSchema,
			},
		}
		name := format.Name
		if name == "" {
			name = "response"
		}
		payload["json_schema"].(map[string]any)["name"] = name
		if format.Description != "" {
			payload["json_schema"].(map[string]any)["description"] = format.Description
		}
		return payload
	}
	return map[string]any{"type": "json_object"}
}

func mistralRequestOverrides(options provider.JSONObject) map[string]any {
	overrides := map[string]any{}
	if options == nil {
		return overrides
	}
	raw, ok := options["request"]
	if !ok {
		return overrides
	}
	for key, value := range normalizeObject(raw) {
		overrides[key] = value
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

func boolFromAny(value any) *bool {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case bool:
		return &typed
	case *bool:
		return typed
	default:
		return nil
	}
}

func intFromAny(value any) *int {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case int:
		return &typed
	case int32:
		converted := int(typed)
		return &converted
	case int64:
		converted := int(typed)
		return &converted
	case float64:
		converted := int(typed)
		return &converted
	default:
		return nil
	}
}

type mistralChatMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content,omitempty"`
	Name       string `json:"name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

func chatMessages(prompt provider.Prompt) []mistralChatMessage {
	messages := make([]mistralChatMessage, 0, len(prompt.Messages))
	for _, message := range prompt.Messages {
		content := messageContentText(message)
		entry := mistralChatMessage{
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

func extractTextContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}
	var builder strings.Builder
	for _, part := range parts {
		if part.Type != "text" || part.Text == "" {
			continue
		}
		builder.WriteString(part.Text)
	}
	return builder.String()
}

func newMistralAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("mistral api error (%d)", status)
	if len(body) > 0 {
		var payload struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			if payload.Message != "" {
				message = payload.Message
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

func mergeRequestOptions(options provider.RequestOptions, headers map[string]string) provider.RequestOptions {
	if len(headers) == 0 {
		return options
	}
	merged := options
	merged.Headers = providerutils.MergeHeaders(headers, options.Headers)
	return merged
}
