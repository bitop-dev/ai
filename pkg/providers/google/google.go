package google

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

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providerutils"
)

const DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

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

func CreateGoogle(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_GENERATIVE_AI_API_KEY")
	}
	baseURL := strings.TrimRight(settings.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	providerName := settings.ProviderName
	if providerName == "" {
		providerName = "google"
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
	return nil, provider.NewUnsupportedFunctionalityError("google does not support image models", nil, "image")
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("google api key is required")
	}
	headers := map[string]string{
		"x-goog-api-key": p.apiKey,
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
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.endpoint(fmt.Sprintf("/models/%s:generateContent", m.modelID)), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.LanguageModelV3GenerateResult{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.LanguageModelV3GenerateResult{}, newGoogleAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	return provider.LanguageModelV3GenerateResult{}, nil
}

func (m *languageModel) DoStream(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3StreamResult, error) {
	payload, err := m.buildPayload(options)
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
	req, cancel, err := providerutils.BuildRequest(ctx, http.MethodPost, m.provider.endpoint(fmt.Sprintf("/models/%s:streamGenerateContent", m.modelID)), bytes.NewReader(body), baseHeaders, options.RequestOptions)
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
		return provider.LanguageModelV3StreamResult{}, newGoogleAPIError(resp.StatusCode, body, resp.Header, m.provider.providerID, m.modelID)
	}

	responseHeaders := cloneHeaders(resp.Header)
	responseMetadata := &provider.ResponseMetadata{
		RequestID:  responseRequestID(resp.Header),
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

	state := &googleStreamState{includeRaw: options.IncludeRawChunks}

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
				return handleGoogleEvent(stream, state, event.Data, responseMetadata)
			},
		})
		if parseErr != nil && !errors.Is(parseErr, context.Canceled) && !errors.Is(parseErr, io.EOF) {
			stream <- provider.StreamPart{Type: provider.StreamPartTypeError, Error: &provider.StreamError{Err: parseErr}}
		}
	}()

	return result, nil
}

func (m *languageModel) buildPayload(options provider.LanguageModelV3CallOptions) (map[string]any, error) {
	contents, systemInstruction, err := promptToContents(options.Prompt)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"contents": contents,
	}
	if systemInstruction != "" {
		payload["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": systemInstruction}},
		}
	}

	generationConfig := map[string]any{}
	if options.MaxOutputTokens > 0 {
		generationConfig["maxOutputTokens"] = options.MaxOutputTokens
	}
	if options.Temperature != 0 {
		generationConfig["temperature"] = options.Temperature
	}
	if options.TopP != 0 {
		generationConfig["topP"] = options.TopP
	}
	if options.TopK != 0 {
		generationConfig["topK"] = options.TopK
	}
	if len(options.StopSequences) > 0 {
		generationConfig["stopSequences"] = options.StopSequences
	}

	googleOpts := googleOptions(resolveProviderOptions(options.ProviderOptions, options.RequestOptions), m.provider.providerID)
	structuredOutputs := true
	if googleOpts != nil {
		if value, ok := googleOpts["structuredOutputs"].(bool); ok {
			structuredOutputs = value
		}
	}

	if options.ResponseFormat != nil && options.ResponseFormat.Type == provider.ResponseFormatTypeJSON {
		generationConfig["responseMimeType"] = "application/json"
		if options.ResponseFormat.Schema != nil && structuredOutputs {
			generationConfig["responseSchema"] = options.ResponseFormat.Schema
		}
	}

	if len(generationConfig) > 0 {
		payload["generationConfig"] = generationConfig
	}

	if options.ToolChoice != nil {
		payload["toolConfig"] = toolConfigPayload(*options.ToolChoice)
	}
	if tools := googleTools(googleOpts); tools != nil {
		payload["tools"] = tools
	}
	for key, value := range googleRequestOverrides(googleOpts) {
		payload[key] = value
	}

	return payload, nil
}

type googleContent struct {
	Role  string       `json:"role"`
	Parts []googlePart `json:"parts"`
}

type googlePart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *googleFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *googleFunctionResponse `json:"functionResponse,omitempty"`
}

type googleFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type googleFunctionResponse struct {
	Name     string `json:"name"`
	Response any    `json:"response,omitempty"`
}

func promptToContents(prompt provider.Prompt) ([]googleContent, string, error) {
	contents := make([]googleContent, 0, len(prompt.Messages))
	var systemParts []string
	for _, message := range prompt.Messages {
		if message.Role == provider.RoleSystem {
			text := messageContentText(message)
			if text != "" {
				systemParts = append(systemParts, text)
			}
			continue
		}
		content, err := convertMessage(message)
		if err != nil {
			return nil, "", err
		}
		if len(content.Parts) == 0 {
			continue
		}
		contents = append(contents, content)
	}
	return contents, strings.Join(systemParts, "\n"), nil
}

func convertMessage(message provider.ModelMessage) (googleContent, error) {
	role := message.Role
	if role == provider.RoleTool {
		role = provider.RoleUser
	}
	parts, err := convertContentParts(message.Content)
	if err != nil {
		return googleContent{}, err
	}
	roleValue := "user"
	if role == provider.RoleAssistant {
		roleValue = "model"
	}
	return googleContent{Role: roleValue, Parts: parts}, nil
}

func convertContentParts(parts []provider.ContentPart) ([]googlePart, error) {
	converted := make([]googlePart, 0, len(parts))
	for _, part := range parts {
		switch typed := part.(type) {
		case provider.TextContent:
			if typed.Text != "" {
				converted = append(converted, googlePart{Text: typed.Text})
			}
		case *provider.TextContent:
			if typed != nil && typed.Text != "" {
				converted = append(converted, googlePart{Text: typed.Text})
			}
		case provider.ToolCallContent:
			converted = append(converted, googlePart{FunctionCall: toolCallPayload(typed.ToolCall)})
		case *provider.ToolCallContent:
			if typed == nil {
				continue
			}
			converted = append(converted, googlePart{FunctionCall: toolCallPayload(typed.ToolCall)})
		case provider.ToolResultContent:
			converted = append(converted, googlePart{FunctionResponse: toolResultPayload(typed.ToolResult)})
		case *provider.ToolResultContent:
			if typed == nil {
				continue
			}
			converted = append(converted, googlePart{FunctionResponse: toolResultPayload(typed.ToolResult)})
		case provider.ReasoningContent:
			if typed.Text != "" {
				converted = append(converted, googlePart{Text: typed.Text})
			}
		case *provider.ReasoningContent:
			if typed != nil && typed.Text != "" {
				converted = append(converted, googlePart{Text: typed.Text})
			}
		}
	}
	return converted, nil
}

func toolCallPayload(call provider.ToolCall) *googleFunctionCall {
	if call.Name == "" {
		return nil
	}
	return &googleFunctionCall{Name: call.Name, Args: call.Arguments}
}

func toolResultPayload(result provider.ToolResult) *googleFunctionResponse {
	if result.Name == "" {
		return nil
	}
	response := result.Result
	if err, ok := result.Result.(error); ok {
		response = err.Error()
	}
	return &googleFunctionResponse{Name: result.Name, Response: response}
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

func googleOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
	if options == nil {
		return nil
	}
	if providerID == "" {
		providerID = "google"
	}
	value, ok := options[string(providerID)]
	if !ok {
		return nil
	}
	return value
}

func googleTools(options provider.JSONObject) any {
	if options == nil {
		return nil
	}
	if tools, ok := options["tools"]; ok {
		return normalizeGoogleTools(tools)
	}
	return nil
}

func normalizeGoogleTools(value any) any {
	switch typed := value.(type) {
	case []providerutils.ToolSpecification:
		return []map[string]any{{"functionDeclarations": convertToolSpecifications(typed)}}
	case []*providerutils.ToolSpecification:
		specs := make([]providerutils.ToolSpecification, 0, len(typed))
		for _, spec := range typed {
			if spec == nil {
				continue
			}
			specs = append(specs, *spec)
		}
		return []map[string]any{{"functionDeclarations": convertToolSpecifications(specs)}}
	case []any:
		specs := make([]providerutils.ToolSpecification, 0, len(typed))
		for _, item := range typed {
			switch spec := item.(type) {
			case providerutils.ToolSpecification:
				specs = append(specs, spec)
			case *providerutils.ToolSpecification:
				if spec == nil {
					continue
				}
				specs = append(specs, *spec)
			default:
				return value
			}
		}
		return []map[string]any{{"functionDeclarations": convertToolSpecifications(specs)}}
	default:
		return value
	}
}

func convertToolSpecifications(specs []providerutils.ToolSpecification) []map[string]any {
	converted := make([]map[string]any, 0, len(specs))
	for _, spec := range specs {
		payload := map[string]any{
			"name": spec.Name,
		}
		if spec.Description != "" {
			payload["description"] = spec.Description
		}
		if spec.Parameters != nil {
			payload["parameters"] = spec.Parameters
		}
		converted = append(converted, payload)
	}
	return converted
}

func googleRequestOverrides(options provider.JSONObject) map[string]any {
	overrides := map[string]any{}
	if options == nil {
		return overrides
	}
	for key, value := range options {
		switch key {
		case "tools", "structuredOutputs":
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

func toolConfigPayload(choice provider.ToolChoice) map[string]any {
	config := map[string]any{}
	functionConfig := map[string]any{}
	switch choice.Type {
	case provider.ToolChoiceTypeNone:
		functionConfig["mode"] = "NONE"
	case provider.ToolChoiceTypeRequired:
		functionConfig["mode"] = "ANY"
	case provider.ToolChoiceTypeTool:
		functionConfig["mode"] = "ANY"
		if choice.ToolName != "" {
			functionConfig["allowedFunctionNames"] = []string{choice.ToolName}
		}
	default:
		functionConfig["mode"] = "AUTO"
	}
	config["functionCallingConfig"] = functionConfig
	return config
}

type googleStreamState struct {
	includeRaw  bool
	textStarted bool
	finishSent  bool
}

type googleStreamChunk struct {
	Candidates    []googleCandidate    `json:"candidates"`
	UsageMetadata *googleUsageMetadata `json:"usageMetadata"`
}

type googleCandidate struct {
	Content      googleContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type googleUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

func handleGoogleEvent(stream chan<- provider.StreamPart, state *googleStreamState, data string, metadata *provider.ResponseMetadata) error {
	var chunk googleStreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return err
	}
	for _, candidate := range chunk.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				emitText(stream, state, part.Text)
			}
			if part.FunctionCall != nil {
				if err := emitToolCall(stream, part.FunctionCall); err != nil {
					return err
				}
			}
		}
		if candidate.FinishReason != "" {
			if state.finishSent {
				continue
			}
			state.finishSent = true
			finish := provider.Finish{Reason: mapFinishReason(candidate.FinishReason), Usage: usageFromGoogle(chunk.UsageMetadata)}
			stream <- provider.StreamPart{Type: provider.StreamPartTypeFinish, Finish: &finish, ResponseMetadata: metadata}
		}
	}
	return nil
}

func emitText(stream chan<- provider.StreamPart, state *googleStreamState, text string) {
	if !state.textStarted {
		state.textStarted = true
		stream <- provider.StreamPart{Type: provider.StreamPartTypeTextStart, TextStart: &provider.TextStart{Text: text}}
		return
	}
	stream <- provider.StreamPart{Type: provider.StreamPartTypeTextDelta, TextDelta: &provider.TextDelta{Delta: text}}
}

func emitToolCall(stream chan<- provider.StreamPart, call *googleFunctionCall) error {
	if call == nil || call.Name == "" {
		return nil
	}
	callID, err := providerutils.GenerateID()
	if err != nil {
		return err
	}
	args := call.Args
	if args == nil {
		args = map[string]any{}
	}
	payload, err := json.Marshal(args)
	if err != nil {
		return err
	}
	delta := string(payload)
	stream <- provider.StreamPart{Type: provider.StreamPartTypeToolInputStart, ToolInputStart: &provider.ToolInputStart{ToolCallID: callID, Name: call.Name}}
	if delta != "" {
		stream <- provider.StreamPart{Type: provider.StreamPartTypeToolInputDelta, ToolInputDelta: &provider.ToolInputDelta{ToolCallID: callID, Delta: delta}}
	}
	stream <- provider.StreamPart{Type: provider.StreamPartTypeToolInputEnd, ToolInputEnd: &provider.ToolInputEnd{ToolCallID: callID}}
	stream <- provider.StreamPart{Type: provider.StreamPartTypeToolCall, ToolCall: &provider.ToolCall{ID: callID, Name: call.Name, Arguments: args}}
	return nil
}

func usageFromGoogle(usage *googleUsageMetadata) *provider.LanguageModelUsage {
	if usage == nil {
		return nil
	}
	if usage.PromptTokenCount == 0 && usage.CandidatesTokenCount == 0 && usage.TotalTokenCount == 0 {
		return nil
	}
	total := usage.TotalTokenCount
	if total == 0 {
		total = usage.PromptTokenCount + usage.CandidatesTokenCount
	}
	return &provider.LanguageModelUsage{
		PromptTokens:     usage.PromptTokenCount,
		CompletionTokens: usage.CandidatesTokenCount,
		TotalTokens:      total,
	}
}

func mapFinishReason(reason string) provider.FinishReason {
	switch strings.ToUpper(reason) {
	case "STOP":
		return provider.FinishReasonStop
	case "MAX_TOKENS":
		return provider.FinishReasonLength
	case "SAFETY":
		return provider.FinishReasonContentFilter
	case "TOOL_CALLS":
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

	googleOpts := googleOptions(resolveProviderOptions(nil, options.RequestOptions), m.provider.providerID)
	requestOverrides := googleRequestOverrides(googleOpts)

	if len(options.Values) == 1 {
		payload := map[string]any{
			"content": map[string]any{
				"parts": []map[string]any{{"text": options.Values[0]}},
			},
		}
		for key, value := range requestOverrides {
			payload[key] = value
		}
		response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.endpoint(fmt.Sprintf("/models/%s:embedContent", m.modelID)), payload, requestOptions, nil, nil)
		if err != nil {
			return provider.EmbeddingModelV3Result{}, err
		}
		if response.StatusCode >= http.StatusBadRequest {
			return provider.EmbeddingModelV3Result{}, newGoogleAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
		}
		return provider.EmbeddingModelV3Result{}, nil
	}

	requests := make([]map[string]any, 0, len(options.Values))
	for _, value := range options.Values {
		requests = append(requests, map[string]any{
			"content": map[string]any{
				"parts": []map[string]any{{"text": value}},
			},
		})
	}
	payload := map[string]any{
		"requests": requests,
	}
	for key, value := range requestOverrides {
		payload[key] = value
	}
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.endpoint(fmt.Sprintf("/models/%s:batchEmbedContents", m.modelID)), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.EmbeddingModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.EmbeddingModelV3Result{}, newGoogleAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	return provider.EmbeddingModelV3Result{}, nil
}

func newGoogleAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("google api error (%d)", status)
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
	requestID := responseRequestID(headers)
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

func responseRequestID(headers http.Header) string {
	if headers == nil {
		return ""
	}
	if requestID := headers.Get("x-request-id"); requestID != "" {
		return requestID
	}
	return headers.Get("x-goog-request-id")
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
