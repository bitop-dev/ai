package cohere

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

const DefaultBaseURL = "https://api.cohere.com/v2"
const DefaultProviderName = "cohere"

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

func CreateCohere(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("COHERE_API_KEY")
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
	return nil, provider.NewNoSuchModelError("cohere does not support image models", nil, p.providerID, modelID)
}

func (p *Provider) RerankingModel(modelID provider.ModelID) (provider.RerankingModelV3, error) {
	return nil, provider.NewUnsupportedFunctionalityError("cohere reranking models are not yet supported", nil, "reranking")
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("cohere api key is required")
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
	req, cancel, err := providerutils.BuildRequest(ctx, http.MethodPost, m.provider.endpoint("/chat"), bytes.NewReader(body), baseHeaders, options.RequestOptions)
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
		return provider.LanguageModelV3StreamResult{}, newCohereAPIError(resp.StatusCode, body, resp.Header, m.provider.providerID, m.modelID)
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
				return handleCohereEvent(stream, state, event.Data, responseMetadata)
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
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.endpoint("/chat"), payload, requestOptions, nil, nil)
	if err != nil {
		return response, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return response, newCohereAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	return response, nil
}

func (m *languageModel) buildPayload(options provider.LanguageModelV3CallOptions, stream bool) (map[string]any, error) {
	messages, documents, err := promptToCohereMessages(options.Prompt)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"model":    string(m.modelID),
		"messages": messages,
	}
	if len(documents) > 0 {
		payload["documents"] = documents
	}
	if options.MaxOutputTokens > 0 {
		payload["max_tokens"] = options.MaxOutputTokens
	}
	if options.Temperature != 0 {
		payload["temperature"] = options.Temperature
	}
	if options.TopP != 0 {
		payload["p"] = options.TopP
	}
	if options.TopK != 0 {
		payload["k"] = options.TopK
	}
	if options.FrequencyPenalty != 0 {
		payload["frequency_penalty"] = options.FrequencyPenalty
	}
	if options.PresencePenalty != 0 {
		payload["presence_penalty"] = options.PresencePenalty
	}
	if options.Seed != 0 {
		payload["seed"] = options.Seed
	}
	if len(options.StopSequences) > 0 {
		payload["stop_sequences"] = options.StopSequences
	}
	if options.ResponseFormat != nil && options.ResponseFormat.Type == provider.ResponseFormatTypeJSON {
		responseFormat := map[string]any{"type": "json_object"}
		if options.ResponseFormat.Schema != nil {
			responseFormat["json_schema"] = options.ResponseFormat.Schema
		}
		payload["response_format"] = responseFormat
	}
	if stream {
		payload["stream"] = true
	}

	cohereOpts := cohereOptions(resolveProviderOptions(options.ProviderOptions, options.RequestOptions), m.provider.providerID)
	tools, toolChoice := cohereTooling(cohereOpts, options.ToolChoice)
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	if toolChoice != "" {
		payload["tool_choice"] = toolChoice
	}
	if thinking := cohereThinking(cohereOpts); thinking != nil {
		payload["thinking"] = thinking
	}
	for key, value := range cohereRequestOverrides(cohereOpts) {
		payload[key] = value
	}

	return payload, nil
}

type cohereChatMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolPlan   string           `json:"tool_plan,omitempty"`
	ToolCalls  []cohereToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type cohereToolCall struct {
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function cohereToolCallTarget `json:"function"`
}

type cohereToolCallTarget struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type cohereDocument struct {
	Data cohereDocumentData `json:"data"`
}

type cohereDocumentData struct {
	Text  string `json:"text"`
	Title string `json:"title,omitempty"`
}

func promptToCohereMessages(prompt provider.Prompt) ([]cohereChatMessage, []cohereDocument, error) {
	messages := make([]cohereChatMessage, 0, len(prompt.Messages))
	documents := make([]cohereDocument, 0)
	for _, message := range prompt.Messages {
		switch message.Role {
		case provider.RoleSystem:
			content := messageContentText(message)
			if content != "" {
				messages = append(messages, cohereChatMessage{Role: "system", Content: content})
			}
		case provider.RoleUser:
			content, docs, err := userMessageContent(message)
			if err != nil {
				return nil, nil, err
			}
			documents = append(documents, docs...)
			messages = append(messages, cohereChatMessage{Role: "user", Content: content})
		case provider.RoleAssistant:
			assistantMessage, err := assistantChatMessage(message)
			if err != nil {
				return nil, nil, err
			}
			if assistantMessage != nil {
				messages = append(messages, *assistantMessage)
			}
		case provider.RoleTool:
			toolMessages, err := toolResultMessages(message)
			if err != nil {
				return nil, nil, err
			}
			messages = append(messages, toolMessages...)
		default:
			return nil, nil, fmt.Errorf("unsupported role: %s", message.Role)
		}
	}
	return messages, documents, nil
}

func userMessageContent(message provider.ModelMessage) (string, []cohereDocument, error) {
	var builder strings.Builder
	documents := make([]cohereDocument, 0)
	for _, part := range message.Content {
		switch typed := part.(type) {
		case provider.TextContent:
			builder.WriteString(typed.Text)
		case *provider.TextContent:
			if typed != nil {
				builder.WriteString(typed.Text)
			}
		case provider.ReasoningContent:
			builder.WriteString(typed.Text)
		case *provider.ReasoningContent:
			if typed != nil {
				builder.WriteString(typed.Text)
			}
		case provider.FileContent:
			doc, err := fileContentDocument(typed)
			if err != nil {
				return "", nil, err
			}
			documents = append(documents, doc)
		case *provider.FileContent:
			if typed == nil {
				continue
			}
			doc, err := fileContentDocument(*typed)
			if err != nil {
				return "", nil, err
			}
			documents = append(documents, doc)
		default:
			return "", nil, provider.NewUnsupportedFunctionalityError("cohere only supports text or file content for user messages", nil, string(part.ContentType()))
		}
	}
	return builder.String(), documents, nil
}

func assistantChatMessage(message provider.ModelMessage) (*cohereChatMessage, error) {
	var builder strings.Builder
	toolCalls := make([]cohereToolCall, 0)
	for _, part := range message.Content {
		switch typed := part.(type) {
		case provider.TextContent:
			builder.WriteString(typed.Text)
		case *provider.TextContent:
			if typed != nil {
				builder.WriteString(typed.Text)
			}
		case provider.ToolCallContent:
			toolCall, err := cohereToolCallForContent(typed.ToolCall)
			if err != nil {
				return nil, err
			}
			toolCalls = append(toolCalls, toolCall)
		case *provider.ToolCallContent:
			if typed == nil {
				continue
			}
			toolCall, err := cohereToolCallForContent(typed.ToolCall)
			if err != nil {
				return nil, err
			}
			toolCalls = append(toolCalls, toolCall)
		case provider.ReasoningContent:
			builder.WriteString(typed.Text)
		case *provider.ReasoningContent:
			if typed != nil {
				builder.WriteString(typed.Text)
			}
		}
	}
	messagePayload := &cohereChatMessage{Role: "assistant"}
	if len(toolCalls) > 0 {
		messagePayload.ToolCalls = toolCalls
		return messagePayload, nil
	}
	messagePayload.Content = builder.String()
	return messagePayload, nil
}

func toolResultMessages(message provider.ModelMessage) ([]cohereChatMessage, error) {
	messages := make([]cohereChatMessage, 0)
	for _, part := range message.Content {
		switch typed := part.(type) {
		case provider.ToolResultContent:
			toolMessage, err := toolResultMessage(message.ToolCallID, typed.ToolResult)
			if err != nil {
				return nil, err
			}
			messages = append(messages, toolMessage)
		case *provider.ToolResultContent:
			if typed == nil {
				continue
			}
			toolMessage, err := toolResultMessage(message.ToolCallID, typed.ToolResult)
			if err != nil {
				return nil, err
			}
			messages = append(messages, toolMessage)
		}
	}
	return messages, nil
}

func toolResultMessage(messageToolCallID string, result provider.ToolResult) (cohereChatMessage, error) {
	toolCallID := result.ID
	if toolCallID == "" {
		toolCallID = messageToolCallID
	}
	if toolCallID == "" {
		return cohereChatMessage{}, fmt.Errorf("cohere tool results require a tool call id")
	}
	content := stringifyToolResult(result)
	return cohereChatMessage{Role: "tool", Content: content, ToolCallID: toolCallID}, nil
}

func stringifyToolResult(result provider.ToolResult) string {
	if result.Result == nil {
		return ""
	}
	if err, ok := result.Result.(error); ok {
		return err.Error()
	}
	switch typed := result.Result.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		payload, err := json.Marshal(result.Result)
		if err != nil {
			return fmt.Sprintf("%v", result.Result)
		}
		return string(payload)
	}
}

func cohereToolCallForContent(call provider.ToolCall) (cohereToolCall, error) {
	callID := call.ID
	if callID == "" {
		generated, err := providerutils.GenerateID()
		if err != nil {
			return cohereToolCall{}, err
		}
		callID = generated
	}
	payload, err := json.Marshal(call.Arguments)
	if err != nil {
		return cohereToolCall{}, err
	}
	arguments := string(payload)
	if arguments == "" || arguments == "null" {
		arguments = "{}"
	}
	return cohereToolCall{
		ID:   callID,
		Type: "function",
		Function: cohereToolCallTarget{
			Name:      call.Name,
			Arguments: arguments,
		},
	}, nil
}

func fileContentDocument(file provider.FileContent) (cohereDocument, error) {
	if file.MediaType != "" {
		if !strings.HasPrefix(file.MediaType, "text/") && file.MediaType != "application/json" {
			return cohereDocument{}, provider.NewUnsupportedFunctionalityError("cohere only supports text documents", nil, file.MediaType)
		}
	}
	if len(file.Data) == 0 {
		return cohereDocument{}, provider.NewUnsupportedFunctionalityError("cohere file uploads must include data", nil, "file data")
	}
	text := string(file.Data)
	return cohereDocument{Data: cohereDocumentData{Text: text, Title: file.Name}}, nil
}

type cohereTool struct {
	Type     string             `json:"type"`
	Function cohereToolFunction `json:"function"`
}

type cohereToolFunction struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Parameters  provider.JSONSchema `json:"parameters,omitempty"`
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

func cohereOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func cohereTooling(options provider.JSONObject, toolChoice *provider.ToolChoice) ([]cohereTool, string) {
	tools := cohereTools(options)
	if len(tools) == 0 {
		return nil, ""
	}
	if toolChoice == nil {
		return tools, ""
	}
	switch toolChoice.Type {
	case provider.ToolChoiceTypeNone:
		return tools, "NONE"
	case provider.ToolChoiceTypeRequired:
		return tools, "REQUIRED"
	case provider.ToolChoiceTypeTool:
		filtered := make([]cohereTool, 0, len(tools))
		for _, tool := range tools {
			if tool.Function.Name == toolChoice.ToolName {
				filtered = append(filtered, tool)
			}
		}
		if len(filtered) == 0 {
			return tools, "REQUIRED"
		}
		return filtered, "REQUIRED"
	default:
		return tools, ""
	}
}

func cohereTools(options provider.JSONObject) []cohereTool {
	if options == nil {
		return nil
	}
	raw, ok := options["tools"]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case []cohereTool:
		return typed
	case []providerutils.ToolSpecification:
		tools := make([]cohereTool, 0, len(typed))
		for _, tool := range typed {
			tools = append(tools, cohereTool{
				Type: "function",
				Function: cohereToolFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			})
		}
		return tools
	case []any:
		tools := make([]cohereTool, 0, len(typed))
		for _, entry := range typed {
			tool, ok := parseCohereTool(entry)
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

func parseCohereTool(entry any) (cohereTool, bool) {
	switch typed := entry.(type) {
	case cohereTool:
		return typed, true
	case providerutils.ToolSpecification:
		return cohereTool{
			Type: "function",
			Function: cohereToolFunction{
				Name:        typed.Name,
				Description: typed.Description,
				Parameters:  typed.Parameters,
			},
		}, true
	case map[string]any:
		function, ok := typed["function"].(map[string]any)
		if !ok {
			return cohereTool{}, false
		}
		name, _ := function["name"].(string)
		if name == "" {
			return cohereTool{}, false
		}
		description, _ := function["description"].(string)
		parameters, _ := function["parameters"].(map[string]any)
		return cohereTool{
			Type: "function",
			Function: cohereToolFunction{
				Name:        name,
				Description: description,
				Parameters:  parameters,
			},
		}, true
	default:
		return cohereTool{}, false
	}
}

func cohereThinking(options provider.JSONObject) map[string]any {
	if options == nil {
		return nil
	}
	raw, ok := options["thinking"]
	if !ok {
		return nil
	}
	data := normalizeObject(raw)
	if data == nil {
		return nil
	}
	payload := map[string]any{}
	if value, ok := data["type"].(string); ok && value != "" {
		payload["type"] = value
	}
	if value, ok := data["tokenBudget"]; ok {
		if tokenBudget := intFromAny(value); tokenBudget != nil {
			payload["token_budget"] = *tokenBudget
		}
	}
	if len(payload) == 0 {
		return nil
	}
	return payload
}

func cohereRequestOverrides(options provider.JSONObject) map[string]any {
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

func messageContentText(message provider.ModelMessage) string {
	var builder strings.Builder
	for _, part := range message.Content {
		text := contentPartText(part)
		if text == "" {
			continue
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

type streamState struct {
	includeRaw      bool
	textStarted     bool
	reasoningActive bool
	toolAccumulator *providerutils.ToolArgumentAccumulator
	pendingToolCall string
}

type cohereEventHeader struct {
	Type string `json:"type"`
}

type cohereContentEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Message struct {
			Content struct {
				Type     string `json:"type"`
				Text     string `json:"text,omitempty"`
				Thinking string `json:"thinking,omitempty"`
			} `json:"content"`
		} `json:"message"`
	} `json:"delta"`
}

type cohereContentDeltaEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Message struct {
			Content struct {
				Text     string `json:"text,omitempty"`
				Thinking string `json:"thinking,omitempty"`
			} `json:"content"`
		} `json:"message"`
	} `json:"delta"`
}

type cohereToolCallEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Message struct {
			ToolCalls struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"delta"`
}

type cohereToolCallDeltaEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Message struct {
			ToolCalls struct {
				Function struct {
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"delta"`
}

type cohereMessageEndEvent struct {
	Type  string `json:"type"`
	Delta struct {
		FinishReason string `json:"finish_reason"`
		Usage        struct {
			Tokens cohereUsageTokens `json:"tokens"`
		} `json:"usage"`
	} `json:"delta"`
}

type cohereUsageTokens struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func handleCohereEvent(stream chan<- provider.StreamPart, state *streamState, data string, metadata *provider.ResponseMetadata) error {
	var header cohereEventHeader
	if err := json.Unmarshal([]byte(data), &header); err != nil {
		return err
	}
	switch header.Type {
	case "content-start":
		var event cohereContentEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return err
		}
		content := event.Delta.Message.Content
		if content.Type == "thinking" {
			state.reasoningActive = true
			if content.Thinking != "" {
				stream <- provider.StreamPart{Type: provider.StreamPartTypeReasoningStart, ReasoningStart: &provider.ReasoningStart{Text: content.Thinking}}
			}
			return nil
		}
		state.textStarted = true
		if content.Text != "" {
			stream <- provider.StreamPart{Type: provider.StreamPartTypeTextStart, TextStart: &provider.TextStart{Text: content.Text}}
		}
		return nil
	case "content-delta":
		var event cohereContentDeltaEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return err
		}
		content := event.Delta.Message.Content
		if content.Thinking != "" {
			stream <- provider.StreamPart{Type: provider.StreamPartTypeReasoningDelta, ReasoningDelta: &provider.ReasoningDelta{Delta: content.Thinking}}
			return nil
		}
		if content.Text != "" {
			if !state.textStarted {
				state.textStarted = true
				stream <- provider.StreamPart{Type: provider.StreamPartTypeTextStart, TextStart: &provider.TextStart{Text: content.Text}}
			} else {
				stream <- provider.StreamPart{Type: provider.StreamPartTypeTextDelta, TextDelta: &provider.TextDelta{Delta: content.Text}}
			}
		}
		return nil
	case "content-end":
		if state.reasoningActive {
			stream <- provider.StreamPart{Type: provider.StreamPartTypeReasoningEnd, ReasoningEnd: &provider.ReasoningEnd{Text: ""}}
			state.reasoningActive = false
			return nil
		}
		stream <- provider.StreamPart{Type: provider.StreamPartTypeTextEnd, TextEnd: &provider.TextEnd{Text: ""}}
		return nil
	case "tool-call-start":
		var event cohereToolCallEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return err
		}
		call := event.Delta.Message.ToolCalls
		if call.ID == "" || call.Function.Name == "" {
			return nil
		}
		state.pendingToolCall = call.ID
		state.toolAccumulator.Start(call.ID, call.Function.Name)
		stream <- provider.StreamPart{Type: provider.StreamPartTypeToolInputStart, ToolInputStart: &provider.ToolInputStart{ToolCallID: call.ID, Name: call.Function.Name}}
		if call.Function.Arguments != "" {
			if err := state.toolAccumulator.AddDelta(call.ID, call.Function.Arguments); err != nil {
				return err
			}
			stream <- provider.StreamPart{Type: provider.StreamPartTypeToolInputDelta, ToolInputDelta: &provider.ToolInputDelta{ToolCallID: call.ID, Delta: call.Function.Arguments}}
		}
		return nil
	case "tool-call-delta":
		var event cohereToolCallDeltaEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return err
		}
		if state.pendingToolCall == "" {
			return nil
		}
		delta := event.Delta.Message.ToolCalls.Function.Arguments
		if delta == "" {
			return nil
		}
		if err := state.toolAccumulator.AddDelta(state.pendingToolCall, delta); err != nil {
			return err
		}
		stream <- provider.StreamPart{Type: provider.StreamPartTypeToolInputDelta, ToolInputDelta: &provider.ToolInputDelta{ToolCallID: state.pendingToolCall, Delta: delta}}
		return nil
	case "tool-call-end":
		if state.pendingToolCall == "" {
			return nil
		}
		callID := state.pendingToolCall
		state.pendingToolCall = ""
		call, err := state.toolAccumulator.End(callID)
		if err != nil {
			return err
		}
		stream <- provider.StreamPart{Type: provider.StreamPartTypeToolInputEnd, ToolInputEnd: &provider.ToolInputEnd{ToolCallID: callID}}
		stream <- provider.StreamPart{Type: provider.StreamPartTypeToolCall, ToolCall: &call}
		return nil
	case "message-end":
		var event cohereMessageEndEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return err
		}
		usage := usageFromCohere(event.Delta.Usage.Tokens)
		finish := provider.Finish{Reason: mapFinishReason(event.Delta.FinishReason), Usage: usage}
		stream <- provider.StreamPart{Type: provider.StreamPartTypeFinish, Finish: &finish, ResponseMetadata: metadata}
		return nil
	default:
		return nil
	}
}

func usageFromCohere(tokens cohereUsageTokens) *provider.LanguageModelUsage {
	if tokens.InputTokens == 0 && tokens.OutputTokens == 0 {
		return nil
	}
	total := tokens.InputTokens + tokens.OutputTokens
	return &provider.LanguageModelUsage{
		PromptTokens:     tokens.InputTokens,
		CompletionTokens: tokens.OutputTokens,
		TotalTokens:      total,
	}
}

func mapFinishReason(reason string) provider.FinishReason {
	switch reason {
	case "COMPLETE", "STOP_SEQUENCE":
		return provider.FinishReasonStop
	case "MAX_TOKENS":
		return provider.FinishReasonLength
	case "ERROR":
		return provider.FinishReasonError
	case "TOOL_CALL":
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
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.EmbeddingModelV3Result{}, err
	}
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)

	cohereOpts := cohereOptions(resolveProviderOptions(nil, options.RequestOptions), m.provider.providerID)
	requestOverrides := cohereRequestOverrides(cohereOpts)

	payload := map[string]any{
		"model":           string(m.modelID),
		"embedding_types": []string{"float"},
		"texts":           options.Values,
	}
	if value, ok := cohereOpts["inputType"].(string); ok && value != "" {
		payload["input_type"] = value
	}
	if value, ok := cohereOpts["truncate"].(string); ok && value != "" {
		payload["truncate"] = value
	}
	for key, value := range requestOverrides {
		payload[key] = value
	}

	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.endpoint("/embed"), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.EmbeddingModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.EmbeddingModelV3Result{}, newCohereAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	return provider.EmbeddingModelV3Result{}, nil
}

func newCohereAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("cohere api error (%d)", status)
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
