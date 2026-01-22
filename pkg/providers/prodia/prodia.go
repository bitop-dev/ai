package prodia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

const DefaultBaseURL = "https://inference.prodia.com/v2"
const DefaultProviderName = "prodia"

const prodiaAcceptHeader = "multipart/form-data; image/png"

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

func CreateProdia(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("PRODIA_TOKEN")
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
	return nil, provider.NewNoSuchModelError("prodia does not support language models", nil, p.providerID, modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("prodia does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return &imageModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("prodia api token is required")
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
	requestOptions.Headers = providerutils.MergeHeaders(map[string]string{"Accept": prodiaAcceptHeader}, requestOptions.Headers)
	payload := m.buildPayload(options)

	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.endpoint(), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.ImageModelV3Result{}, newProdiaAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	if err := parseProdiaMultipartResponse(response.Body, response.Headers); err != nil {
		return provider.ImageModelV3Result{}, err
	}
	return provider.ImageModelV3Result{}, nil
}

func (m *imageModel) endpoint() string {
	return m.provider.endpoint("/job")
}

func (m *imageModel) buildPayload(options provider.ImageModelV3CallOptions) map[string]any {
	config := map[string]any{}
	if options.Prompt != "" {
		config["prompt"] = options.Prompt
	}
	if options.Seed > 0 {
		config["seed"] = options.Seed
	}
	width, height := parseProdiaSize(options.Size)
	prodiaOpts := prodiaOptions(options.RequestOptions.ProviderOptions, m.provider.providerID)
	if value, ok := prodiaOpts["width"]; ok {
		config["width"] = value
	} else if width > 0 {
		config["width"] = width
	}
	if value, ok := prodiaOpts["height"]; ok {
		config["height"] = value
	} else if height > 0 {
		config["height"] = height
	}
	if value, ok := prodiaOpts["steps"]; ok {
		config["steps"] = value
	}
	if value, ok := prodiaOpts["stylePreset"]; ok {
		config["style_preset"] = value
	}
	if value, ok := prodiaOpts["loras"]; ok {
		config["loras"] = value
	}
	if value, ok := prodiaOpts["progressive"]; ok {
		config["progressive"] = value
	}
	return map[string]any{
		"type":   string(m.modelID),
		"config": config,
	}
}

type prodiaJobResult struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ExpiresAt string `json:"expires_at"`
	State     *struct {
		Current string `json:"current"`
	} `json:"state"`
	Config *struct {
		Seed *int `json:"seed"`
	} `json:"config"`
	Metrics *struct {
		Elapsed *float64 `json:"elapsed"`
		IPS     *float64 `json:"ips"`
	} `json:"metrics"`
}

func parseProdiaMultipartResponse(body []byte, headers http.Header) error {
	if len(body) == 0 {
		return provider.NewInvalidResponseDataError("prodia response empty", nil)
	}
	contentType := ""
	if headers != nil {
		contentType = headers.Get("Content-Type")
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return provider.NewInvalidResponseDataError("prodia response missing multipart content type", err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		return provider.NewInvalidResponseDataError("prodia response missing multipart boundary", nil)
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	var jobResult *prodiaJobResult
	var imageBytes []byte
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return provider.NewInvalidResponseDataError("prodia response invalid multipart data", err)
		}
		partBody, err := io.ReadAll(part)
		_ = part.Close()
		if err != nil {
			return provider.NewInvalidResponseDataError("prodia response invalid multipart part", err)
		}
		name := part.FormName()
		contentType := part.Header.Get("Content-Type")
		switch {
		case name == "job":
			var parsed prodiaJobResult
			if err := json.Unmarshal(partBody, &parsed); err != nil {
				return provider.NewInvalidResponseDataError("prodia response invalid job payload", err)
			}
			jobResult = &parsed
		case name == "output" || strings.HasPrefix(contentType, "image/"):
			if len(partBody) > 0 {
				imageBytes = partBody
			}
		}
	}

	if jobResult == nil || jobResult.ID == "" {
		return provider.NewInvalidResponseDataError("prodia response missing job data", nil)
	}
	if len(imageBytes) == 0 {
		return provider.NewInvalidResponseDataError("prodia response missing output image", nil)
	}
	return nil
}

func parseProdiaSize(size string) (int, int) {
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

func prodiaOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func newProdiaAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("prodia api error (%d)", status)
	if parsed := parseProdiaErrorMessage(body); parsed != "" {
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

func parseProdiaErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload struct {
		Message string `json:"message"`
		Detail  any    `json:"detail"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if detail, ok := payload.Detail.(string); ok && detail != "" {
		return detail
	}
	if payload.Detail != nil {
		if encoded, err := json.Marshal(payload.Detail); err == nil {
			if string(encoded) != "null" {
				return string(encoded)
			}
		}
	}
	if payload.Error != "" {
		return payload.Error
	}
	if payload.Message != "" {
		return payload.Message
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
