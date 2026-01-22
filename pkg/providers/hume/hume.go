package hume

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providerutils"
)

const DefaultBaseURL = "https://api.hume.ai"
const DefaultProviderName = "hume"
const DefaultSpeechVoiceID = "d8ab67c6-953d-4bd8-9370-8fa53a0f1453"
const DefaultSpeechOutputFormat = "mp3"

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

func CreateHume(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("HUME_API_KEY")
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
	return nil, provider.NewNoSuchModelError("hume does not support language models", nil, p.providerID, modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("hume does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return nil, provider.NewNoSuchModelError("hume does not support image models", nil, p.providerID, modelID)
}

func (p *Provider) SpeechModel(modelID provider.ModelID) (provider.SpeechModelV3, error) {
	return &speechModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) TranscriptionModel(modelID provider.ModelID) (provider.TranscriptionModelV3, error) {
	return nil, provider.NewNoSuchModelError("hume does not support transcription models", nil, p.providerID, modelID)
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("hume api key is required")
	}
	headers := map[string]string{
		"X-Hume-Api-Key": p.apiKey,
	}
	for key, value := range p.headers {
		headers[key] = value
	}
	return headers, nil
}

func (p *Provider) endpoint(path string) string {
	return p.baseURL + path
}

type speechModel struct {
	provider *Provider
	modelID  provider.ModelID
}

func (m *speechModel) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (m *speechModel) ProviderID() provider.ProviderID {
	return m.provider.providerID
}

func (m *speechModel) ModelID() provider.ModelID {
	return m.modelID
}

func (m *speechModel) DoGenerate(ctx context.Context, options provider.SpeechModelV3CallOptions) (provider.SpeechModelV3Result, error) {
	if options.Text == "" {
		return provider.SpeechModelV3Result{}, provider.NewInvalidRequestError("hume speech text is required", nil)
	}
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.SpeechModelV3Result{}, err
	}
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)

	voice := options.Voice
	if voice == "" {
		voice = DefaultSpeechVoiceID
	}
	format := resolveHumeOutputFormat(options.OutputFormat)
	if format == "" {
		format = DefaultSpeechOutputFormat
	}
	utterance := map[string]any{
		"text": options.Text,
		"voice": map[string]any{
			"id":       voice,
			"provider": "HUME_AI",
		},
	}
	if options.Instructions != "" {
		utterance["description"] = options.Instructions
	}
	if options.Speed != 0 {
		utterance["speed"] = options.Speed
	}

	payload := map[string]any{
		"utterances": []map[string]any{utterance},
		"format": map[string]any{
			"type": format,
		},
	}
	if humeOpts := humeOptions(options.RequestOptions.ProviderOptions, m.provider.providerID); humeOpts != nil {
		applyHumeSpeechOptions(payload, humeOpts)
	}

	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.endpoint("/v0/tts/file"), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.SpeechModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.SpeechModelV3Result{}, newHumeAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	if len(response.Body) == 0 {
		return provider.SpeechModelV3Result{}, provider.NewInvalidResponseDataError("hume speech response empty", nil)
	}
	return provider.SpeechModelV3Result{}, nil
}

func resolveHumeOutputFormat(format string) string {
	if format == "" {
		return ""
	}
	normalized := strings.ToLower(format)
	switch normalized {
	case "mp3", "pcm", "wav":
		return normalized
	default:
		return ""
	}
}

func applyHumeSpeechOptions(payload map[string]any, options provider.JSONObject) {
	if options == nil {
		return
	}
	contextValue, ok := lookupOption(options, "context")
	if !ok {
		return
	}
	contextOptions := normalizeJSONObject(contextValue)
	if contextOptions == nil {
		return
	}
	if generationID, ok := lookupOption(contextOptions, "generationId", "generation_id"); ok {
		payload["context"] = map[string]any{"generation_id": generationID}
		return
	}
	if utterancesValue, ok := lookupOption(contextOptions, "utterances"); ok {
		if utterances := normalizeHumeUtterances(utterancesValue); len(utterances) > 0 {
			payload["context"] = map[string]any{"utterances": utterances}
		}
	}
}

func normalizeHumeUtterances(value any) []map[string]any {
	switch typed := value.(type) {
	case []provider.JSONObject:
		items := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if normalized := normalizeHumeUtterance(item); normalized != nil {
				items = append(items, normalized)
			}
		}
		return items
	case []any:
		items := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if normalized := normalizeHumeUtterance(item); normalized != nil {
				items = append(items, normalized)
			}
		}
		return items
	default:
		return nil
	}
}

func normalizeHumeUtterance(value any) map[string]any {
	utterance := normalizeJSONObject(value)
	if utterance == nil {
		return nil
	}
	normalized := map[string]any{}
	if text, ok := lookupOption(utterance, "text"); ok {
		normalized["text"] = text
	}
	if description, ok := lookupOption(utterance, "description"); ok {
		normalized["description"] = description
	}
	if speed, ok := lookupOption(utterance, "speed"); ok {
		normalized["speed"] = speed
	}
	if trailingSilence, ok := lookupOption(utterance, "trailingSilence", "trailing_silence"); ok {
		normalized["trailing_silence"] = trailingSilence
	}
	if voiceValue, ok := lookupOption(utterance, "voice"); ok {
		if voice := normalizeHumeVoice(voiceValue); len(voice) > 0 {
			normalized["voice"] = voice
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeHumeVoice(value any) map[string]any {
	switch typed := value.(type) {
	case string:
		return map[string]any{"id": typed}
	case map[string]any:
		return normalizeHumeVoiceMap(typed)
	case provider.JSONObject:
		converted := make(map[string]any, len(typed))
		for key, value := range typed {
			converted[key] = value
		}
		return normalizeHumeVoiceMap(converted)
	default:
		return nil
	}
}

func normalizeHumeVoiceMap(voice map[string]any) map[string]any {
	normalized := map[string]any{}
	if id, ok := voice["id"]; ok {
		normalized["id"] = id
	}
	if name, ok := voice["name"]; ok {
		normalized["name"] = name
	}
	if providerValue, ok := voice["provider"]; ok {
		normalized["provider"] = providerValue
	}
	return normalized
}

func humeOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func lookupOption(options provider.JSONObject, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := options[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func normalizeJSONObject(value any) provider.JSONObject {
	switch typed := value.(type) {
	case provider.JSONObject:
		return typed
	case map[string]any:
		converted := make(provider.JSONObject, len(typed))
		for key, value := range typed {
			converted[key] = value
		}
		return converted
	default:
		return nil
	}
}

func newHumeAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("hume api error (%d)", status)
	if parsed := parseHumeErrorMessage(body); parsed != "" {
		message = parsed
	}
	requestID := headers.Get("x-request-id")
	if requestID == "" {
		requestID = headers.Get("request-id")
	}
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

func parseHumeErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if payload.Error.Message != "" {
		return payload.Error.Message
	}
	return ""
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
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}
