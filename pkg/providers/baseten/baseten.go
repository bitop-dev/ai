package baseten

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

const DefaultBaseURL = "https://inference.baseten.co/v1"
const DefaultProviderName = "baseten"

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

func CreateBaseten(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("BASETEN_API_KEY")
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
	return nil, provider.NewNoSuchModelError("baseten does not support language models", nil, p.providerID, modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("baseten does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return &imageModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("baseten api key is required")
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
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)
	payload := m.buildPayload(options)
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.endpoint(), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.ImageModelV3Result{}, newBasetenAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	if err := parseBasetenImageResponse(response.Body); err != nil {
		return provider.ImageModelV3Result{}, err
	}
	return provider.ImageModelV3Result{}, nil
}

func (m *imageModel) endpoint() string {
	return m.provider.endpoint("/models/" + string(m.modelID) + "/predict")
}

func (m *imageModel) buildPayload(options provider.ImageModelV3CallOptions) map[string]any {
	payload := map[string]any{}
	if options.Prompt != "" {
		payload["prompt"] = options.Prompt
	}
	if options.N > 0 {
		payload["num_images"] = options.N
	}
	if options.Seed > 0 {
		payload["seed"] = options.Seed
	}
	if options.AspectRatio != "" {
		payload["aspect_ratio"] = options.AspectRatio
	}
	width, height := parseBasetenSize(options.Size)
	if width > 0 && height > 0 {
		payload["width"] = width
		payload["height"] = height
	}
	basetenOpts := basetenOptions(options.RequestOptions.ProviderOptions, m.provider.providerID)
	applyBasetenImageOptions(payload, basetenOpts)
	for key, value := range basetenRequestOverrides(basetenOpts) {
		payload[key] = value
	}
	return payload
}

func parseBasetenSize(size string) (int, int) {
	if size == "" {
		return 0, 0
	}
	parts := strings.SplitN(size, "x", 3)
	if len(parts) != 2 {
		return 0, 0
	}
	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || width <= 0 {
		return 0, 0
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || height <= 0 {
		return 0, 0
	}
	return width, height
}

func basetenOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func applyBasetenImageOptions(payload map[string]any, options provider.JSONObject) {
	if options == nil {
		return
	}
	fieldMapping := map[string]string{
		"numImages":       "num_images",
		"num_images":      "num_images",
		"guidanceScale":   "guidance_scale",
		"guidance_scale":  "guidance_scale",
		"negativePrompt":  "negative_prompt",
		"negative_prompt": "negative_prompt",
		"imageUrl":        "image_url",
		"imageURL":        "image_url",
		"image_url":       "image_url",
	}
	for key, value := range options {
		if key == "request" {
			continue
		}
		if mapped, ok := fieldMapping[key]; ok {
			key = mapped
		}
		payload[key] = value
	}
}

func basetenRequestOverrides(options provider.JSONObject) map[string]any {
	if options == nil {
		return nil
	}
	request, ok := options["request"]
	if !ok {
		return nil
	}
	return normalizeBasetenObject(request)
}

func normalizeBasetenObject(value any) map[string]any {
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

func parseBasetenImageResponse(body []byte) error {
	if len(body) == 0 {
		return provider.NewInvalidResponseDataError("baseten response empty", nil)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return provider.NewInvalidResponseDataError("baseten response invalid", err)
	}
	if message, ok := payload["error"].(string); ok && message != "" {
		return provider.NewInvalidResponseDataError(message, nil)
	}
	if !containsBasetenImageValue(payload) {
		return provider.NewInvalidResponseDataError("baseten response missing image data", nil)
	}
	return nil
}

func containsBasetenImageValue(value any) bool {
	if value == nil {
		return false
	}
	switch typed := value.(type) {
	case string:
		return typed != ""
	case []any:
		for _, item := range typed {
			if containsBasetenImageValue(item) {
				return true
			}
		}
	case map[string]any:
		if url, ok := typed["url"].(string); ok && url != "" {
			return true
		}
		if url, ok := typed["image_url"].(string); ok && url != "" {
			return true
		}
		if url, ok := typed["imageUrl"].(string); ok && url != "" {
			return true
		}
		if data, ok := typed["data"].(string); ok && data != "" {
			return true
		}
		if data, ok := typed["base64"].(string); ok && data != "" {
			return true
		}
		if data, ok := typed["b64"].(string); ok && data != "" {
			return true
		}
		if images, ok := typed["images"]; ok && containsBasetenImageValue(images) {
			return true
		}
		if image, ok := typed["image"]; ok && containsBasetenImageValue(image) {
			return true
		}
		if output, ok := typed["output"]; ok && containsBasetenImageValue(output) {
			return true
		}
		if data, ok := typed["data"]; ok && containsBasetenImageValue(data) {
			return true
		}
	}
	return false
}

func newBasetenAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("baseten api error (%d)", status)
	if parsed := parseBasetenErrorMessage(body); parsed != "" {
		message = parsed
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

func parseBasetenErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload struct {
		Error   any    `json:"error"`
		Message string `json:"message"`
		Detail  any    `json:"detail"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if message := normalizeBasetenErrorValue(payload.Error); message != "" {
		return message
	}
	if payload.Message != "" {
		return payload.Message
	}
	if message := normalizeBasetenErrorValue(payload.Detail); message != "" {
		return message
	}
	return ""
}

func normalizeBasetenErrorValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]any:
		if message, ok := typed["message"].(string); ok {
			return message
		}
		if message, ok := typed["error"].(string); ok {
			return message
		}
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
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return seconds
}
