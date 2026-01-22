package lmnt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

const DefaultBaseURL = "https://api.lmnt.com"
const DefaultProviderName = "lmnt"
const DefaultSpeechVoiceID = "ava"
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

func CreateLMNT(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("LMNT_API_KEY")
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
	return nil, provider.NewNoSuchModelError("lmnt does not support language models", nil, p.providerID, modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("lmnt does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return nil, provider.NewNoSuchModelError("lmnt does not support image models", nil, p.providerID, modelID)
}

func (p *Provider) SpeechModel(modelID provider.ModelID) (provider.SpeechModelV3, error) {
	return &speechModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) TranscriptionModel(modelID provider.ModelID) (provider.TranscriptionModelV3, error) {
	return nil, provider.NewNoSuchModelError("lmnt does not support transcription models", nil, p.providerID, modelID)
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("lmnt api key is required")
	}
	headers := map[string]string{
		"x-api-key": p.apiKey,
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
		return provider.SpeechModelV3Result{}, provider.NewInvalidRequestError("lmnt speech text is required", nil)
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
	outputFormat := resolveLMNTOutputFormat(options.OutputFormat)
	if outputFormat == "" {
		outputFormat = DefaultSpeechOutputFormat
	}

	payload := map[string]any{
		"model":           string(m.modelID),
		"text":            options.Text,
		"voice":           voice,
		"response_format": outputFormat,
	}
	if options.Language != "" {
		payload["language"] = options.Language
	}
	if options.Speed != 0 {
		payload["speed"] = options.Speed
	}
	if lmntOpts := lmntOptions(options.RequestOptions.ProviderOptions, m.provider.providerID); lmntOpts != nil {
		applyLMNTSpeechOptions(payload, lmntOpts)
	}

	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.endpoint("/v1/ai/speech/bytes"), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.SpeechModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.SpeechModelV3Result{}, newLMNTAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	if len(response.Body) == 0 {
		return provider.SpeechModelV3Result{}, provider.NewInvalidResponseDataError("lmnt speech response empty", nil)
	}
	return provider.SpeechModelV3Result{}, nil
}

func resolveLMNTOutputFormat(format string) string {
	if format == "" {
		return ""
	}
	format = strings.ToLower(format)
	switch format {
	case "aac", "mp3", "mulaw", "raw", "wav":
		return format
	default:
		return ""
	}
}

func applyLMNTSpeechOptions(payload map[string]any, options provider.JSONObject) {
	if options == nil {
		return
	}
	if value, ok := lookupOption(options, "conversational"); ok {
		payload["conversational"] = value
	}
	if value, ok := lookupOption(options, "length"); ok {
		payload["length"] = value
	}
	if value, ok := lookupOption(options, "seed"); ok {
		payload["seed"] = value
	}
	if value, ok := lookupOption(options, "speed"); ok {
		payload["speed"] = value
	}
	if value, ok := lookupOption(options, "temperature"); ok {
		payload["temperature"] = value
	}
	if value, ok := lookupOption(options, "topP", "top_p"); ok {
		payload["top_p"] = value
	}
	if value, ok := lookupOption(options, "sampleRate", "sample_rate"); ok {
		payload["sample_rate"] = value
	}
}

func lmntOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func newLMNTAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("lmnt api error (%d)", status)
	if parsed := parseLMNTErrorMessage(body); parsed != "" {
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

func parseLMNTErrorMessage(body []byte) string {
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
