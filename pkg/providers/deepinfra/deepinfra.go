package deepinfra

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providers/openaicompatible"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

const DefaultBaseURL = "https://api.deepinfra.com/v1"
const DefaultProviderName = "deepinfra"

// Settings configures the DeepInfra provider.
type Settings struct {
	APIKey       string
	BaseURL      string
	Headers      map[string]string
	HTTPClient   *http.Client
	ProviderName string
}

// Provider connects to DeepInfra using OpenAI-compatible endpoints.
type Provider struct {
	client     *openaicompatible.Provider
	apiKey     string
	baseURL    string
	headers    map[string]string
	httpClient *http.Client
	providerID provider.ProviderID
}

// CreateDeepInfra constructs a DeepInfra provider.
func CreateDeepInfra(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("DEEPINFRA_API_KEY")
	}
	baseURL := strings.TrimRight(settings.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	providerName := settings.ProviderName
	if providerName == "" {
		providerName = DefaultProviderName
	}
	client := openaicompatible.CreateOpenAICompatible(openaicompatible.Settings{
		APIKey:       apiKey,
		BaseURL:      baseURL + "/openai",
		Headers:      settings.Headers,
		HTTPClient:   settings.HTTPClient,
		ProviderName: providerName,
	})
	return &Provider{
		client:     client,
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
	return p.client.LanguageModel(modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return p.client.EmbeddingModel(modelID)
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

func (p *Provider) inferenceEndpoint(modelID provider.ModelID) string {
	return fmt.Sprintf("%s/inference/%s", p.baseURL, modelID)
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
	payload := map[string]any{}
	if options.Prompt != "" {
		payload["prompt"] = options.Prompt
	}
	if options.N > 0 {
		payload["num_images"] = options.N
	}
	if options.AspectRatio != "" {
		payload["aspect_ratio"] = options.AspectRatio
	}
	if options.Seed > 0 {
		payload["seed"] = options.Seed
	}
	if width, height := splitSize(options.Size); width != "" && height != "" {
		payload["width"] = width
		payload["height"] = height
	}

	deepinfraOpts := deepinfraOptions(options.RequestOptions.ProviderOptions, m.provider.providerID)
	for key, value := range deepinfraRequestOverrides(deepinfraOpts) {
		payload[key] = value
	}

	headers := m.provider.requestHeaders()
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.provider.inferenceEndpoint(m.modelID), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.ImageModelV3Result{}, newDeepInfraAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	return provider.ImageModelV3Result{}, nil
}

func splitSize(size string) (string, string) {
	if size == "" {
		return "", ""
	}
	parts := strings.SplitN(size, "x", 3)
	if len(parts) != 2 {
		return "", ""
	}
	width := strings.TrimSpace(parts[0])
	height := strings.TrimSpace(parts[1])
	if width == "" || height == "" {
		return "", ""
	}
	return width, height
}

func deepinfraOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func deepinfraRequestOverrides(options provider.JSONObject) map[string]any {
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

func newDeepInfraAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("deepinfra api error (%d)", status)
	if len(body) > 0 {
		var payload struct {
			Detail struct {
				Error string `json:"error"`
			} `json:"detail"`
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			switch {
			case payload.Detail.Error != "":
				message = payload.Detail.Error
			case payload.Error.Message != "":
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
