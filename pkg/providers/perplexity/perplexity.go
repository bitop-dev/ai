package perplexity

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

const DefaultBaseURL = "https://api.perplexity.ai"
const DefaultProviderName = "perplexity"

// Settings configures the Perplexity provider.
type Settings struct {
	APIKey       string
	BaseURL      string
	Headers      map[string]string
	HTTPClient   *http.Client
	ProviderName string
}

// Provider connects to the Perplexity OpenAI-compatible API.
type Provider struct {
	apiKey     string
	baseURL    string
	headers    map[string]string
	httpClient *http.Client
	providerID provider.ProviderID
}

// CreatePerplexity constructs a Perplexity provider.
func CreatePerplexity(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("PERPLEXITY_API_KEY")
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
	return nil, provider.NewNoSuchModelError("perplexity does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return nil, provider.NewNoSuchModelError("perplexity does not support image models", nil, p.providerID, modelID)
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

	headers := m.provider.requestHeaders()
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
		return provider.LanguageModelV3StreamResult{}, newPerplexityAPIError(resp.StatusCode, body, resp.Header, m.provider.providerID, m.modelID)
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

	state := &perplexityStreamState{
		includeRaw: options.IncludeRawChunks,
		citations:  map[string]struct{}{},
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
				return handlePerplexityEvent(stream, state, event.Data, responseMetadata, m.provider.providerID)
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
	headers := m.provider.requestHeaders()
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.endpoint("/chat/completions"), payload, requestOptions, nil, nil)
	if err != nil {
		return response, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return response, newPerplexityAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	return response, nil
}

func (m *languageModel) buildPayload(options provider.LanguageModelV3CallOptions, stream bool) (map[string]any, error) {
	basePayload := map[string]any{
		"model":    string(m.modelID),
		"messages": chatMessages(options.Prompt),
	}
	if stream {
		basePayload["stream"] = true
	}
	if options.MaxOutputTokens > 0 {
		basePayload["max_tokens"] = options.MaxOutputTokens
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
	if len(options.StopSequences) > 0 {
		basePayload["stop"] = options.StopSequences
	}
	if options.ResponseFormat != nil {
		basePayload["response_format"] = responseFormatPayload(*options.ResponseFormat)
	}
	perplexityOpts := perplexityOptions(resolveProviderOptions(options.ProviderOptions, options.RequestOptions), m.provider.providerID)
	for key, value := range perplexityRequestOverrides(perplexityOpts) {
		basePayload[key] = value
	}
	return basePayload, nil
}

type perplexityMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

func chatMessages(prompt provider.Prompt) []perplexityMessage {
	messages := make([]perplexityMessage, 0, len(prompt.Messages))
	for _, message := range prompt.Messages {
		content := messageContentText(message)
		entry := perplexityMessage{
			Role:    string(message.Role),
			Content: content,
		}
		if message.Name != "" {
			entry.Name = message.Name
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
		if typed == nil {
			return ""
		}
		return typed.Text
	case provider.ToolResultContent:
		return stringifyJSONValue(typed.ToolResult.Result)
	case *provider.ToolResultContent:
		if typed == nil {
			return ""
		}
		return stringifyJSONValue(typed.ToolResult.Result)
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

func resolveProviderOptions(explicit provider.ProviderOptions, requestOptions provider.RequestOptions) provider.ProviderOptions {
	if explicit != nil {
		return explicit
	}
	if requestOptions.ProviderOptions != nil {
		return requestOptions.ProviderOptions
	}
	return nil
}

func perplexityOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func perplexityRequestOverrides(options provider.JSONObject) map[string]any {
	overrides := map[string]any{}
	if options == nil {
		return overrides
	}
	for key, value := range options {
		switch key {
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

type perplexityStreamState struct {
	textStarted bool
	includeRaw  bool
	citations   map[string]struct{}
	usage       *perplexityUsage
	images      []perplexityImage
}

type perplexityChatChunk struct {
	ID        string             `json:"id"`
	Created   int64              `json:"created"`
	Model     string             `json:"model"`
	Choices   []perplexityChoice `json:"choices"`
	Citations []string           `json:"citations"`
	Images    []perplexityImage  `json:"images"`
	Usage     *perplexityUsage   `json:"usage"`
}

type perplexityChoice struct {
	Delta        perplexityDelta `json:"delta"`
	FinishReason string          `json:"finish_reason"`
}

type perplexityDelta struct {
	Content string `json:"content"`
}

type perplexityUsage struct {
	PromptTokens     int  `json:"prompt_tokens"`
	CompletionTokens int  `json:"completion_tokens"`
	TotalTokens      int  `json:"total_tokens"`
	CitationTokens   *int `json:"citation_tokens"`
	NumSearchQueries *int `json:"num_search_queries"`
	ReasoningTokens  *int `json:"reasoning_tokens"`
}

type perplexityImage struct {
	ImageURL  string `json:"image_url"`
	OriginURL string `json:"origin_url"`
	Height    int    `json:"height"`
	Width     int    `json:"width"`
}

func handlePerplexityEvent(stream chan<- provider.StreamPart, state *perplexityStreamState, data string, metadata *provider.ResponseMetadata, providerID provider.ProviderID) error {
	var chunk perplexityChatChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return err
	}
	if len(chunk.Choices) == 0 {
		return nil
	}
	if chunk.Usage != nil {
		state.usage = chunk.Usage
	}
	if len(chunk.Images) > 0 {
		state.images = chunk.Images
	}
	if len(chunk.Citations) > 0 {
		for _, citation := range chunk.Citations {
			if citation == "" {
				continue
			}
			if _, exists := state.citations[citation]; exists {
				continue
			}
			state.citations[citation] = struct{}{}
			stream <- provider.StreamPart{Type: provider.StreamPartTypeSource, Source: &provider.Source{ID: newSourceID(), URL: citation}}
		}
	}

	choice := chunk.Choices[0]
	if choice.Delta.Content != "" {
		emitText(stream, state, choice.Delta.Content)
	}
	if choice.FinishReason != "" {
		finish := provider.Finish{Reason: mapFinishReason(choice.FinishReason), Usage: usageFromPerplexity(state.usage)}
		providerMetadata := providerMetadataFromPerplexity(state.usage, state.images, providerID)
		stream <- provider.StreamPart{Type: provider.StreamPartTypeFinish, Finish: &finish, ResponseMetadata: metadata, ProviderMetadata: providerMetadata}
	}
	return nil
}

func emitText(stream chan<- provider.StreamPart, state *perplexityStreamState, text string) {
	if !state.textStarted {
		state.textStarted = true
		stream <- provider.StreamPart{Type: provider.StreamPartTypeTextStart, TextStart: &provider.TextStart{Text: text}}
		return
	}
	stream <- provider.StreamPart{Type: provider.StreamPartTypeTextDelta, TextDelta: &provider.TextDelta{Delta: text}}
}

func usageFromPerplexity(usage *perplexityUsage) *provider.LanguageModelUsage {
	if usage == nil {
		return nil
	}
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 && usage.TotalTokens == 0 {
		return nil
	}
	total := usage.TotalTokens
	if total == 0 {
		total = usage.PromptTokens + usage.CompletionTokens
	}
	return &provider.LanguageModelUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      total,
	}
}

func providerMetadataFromPerplexity(usage *perplexityUsage, images []perplexityImage, providerID provider.ProviderID) provider.ProviderMetadata {
	metadata := map[string]any{}
	if usage != nil {
		if usage.CitationTokens != nil {
			metadata["citation_tokens"] = *usage.CitationTokens
		}
		if usage.NumSearchQueries != nil {
			metadata["num_search_queries"] = *usage.NumSearchQueries
		}
		if usage.ReasoningTokens != nil {
			metadata["reasoning_tokens"] = *usage.ReasoningTokens
		}
	}
	if len(images) > 0 {
		payload := make([]map[string]any, 0, len(images))
		for _, image := range images {
			payload = append(payload, map[string]any{
				"image_url":  image.ImageURL,
				"origin_url": image.OriginURL,
				"height":     image.Height,
				"width":      image.Width,
			})
		}
		metadata["images"] = payload
	}
	if len(metadata) == 0 {
		return nil
	}
	if providerID == "" {
		providerID = DefaultProviderName
	}
	return provider.ProviderMetadata{string(providerID): metadata}
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

func newPerplexityAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("perplexity api error (%d)", status)
	if len(body) > 0 {
		var payload struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			switch {
			case payload.Error.Message != "":
				message = payload.Error.Message
			case payload.Error.Type != "":
				message = payload.Error.Type
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

func newSourceID() string {
	value, err := providerutils.GenerateID()
	if err != nil {
		return ""
	}
	return value
}
