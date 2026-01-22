package blackforestlabs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providerutils"
)

const DefaultBaseURL = "https://api.bfl.ai/v1"
const DefaultProviderName = "black-forest-labs"

const defaultPollInterval = 500 * time.Millisecond
const defaultPollTimeout = 60 * time.Second

type Settings struct {
	APIKey       string
	BaseURL      string
	Headers      map[string]string
	HTTPClient   *http.Client
	ProviderName string
	PollInterval time.Duration
	PollTimeout  time.Duration
}

type Provider struct {
	apiKey       string
	baseURL      string
	headers      map[string]string
	httpClient   *http.Client
	providerID   provider.ProviderID
	pollInterval time.Duration
	pollTimeout  time.Duration
}

func CreateBlackForestLabs(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("BFL_API_KEY")
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
		apiKey:       apiKey,
		baseURL:      baseURL,
		headers:      settings.Headers,
		httpClient:   settings.HTTPClient,
		providerID:   provider.ProviderID(providerName),
		pollInterval: settings.PollInterval,
		pollTimeout:  settings.PollTimeout,
	}
}

func (p *Provider) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (p *Provider) LanguageModel(modelID provider.ModelID) (provider.LanguageModelV3, error) {
	return nil, provider.NewNoSuchModelError("black forest labs does not support language models", nil, p.providerID, modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("black forest labs does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return &imageModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("black forest labs api key is required")
	}
	headers := map[string]string{
		"x-key": p.apiKey,
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

type pollOverrides struct {
	Interval time.Duration
	Timeout  time.Duration
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
	payload, overrides, err := m.buildPayload(options)
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}

	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.endpoint(), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.ImageModelV3Result{}, newBlackForestLabsAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	submit, err := parseBlackForestLabsSubmitResponse(response.Body)
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}

	pollURL := ensurePollURL(submit.PollingURL, submit.ID)
	imageSample, err := m.provider.awaitImage(ctx, pollURL, requestOptions, overrides, m.modelID)
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}
	if err := m.provider.handleImageSample(ctx, imageSample, requestOptions, m.modelID); err != nil {
		return provider.ImageModelV3Result{}, err
	}
	return provider.ImageModelV3Result{}, nil
}

func (m *imageModel) endpoint() string {
	return m.provider.endpoint("/" + string(m.modelID))
}

func (m *imageModel) buildPayload(options provider.ImageModelV3CallOptions) (map[string]any, pollOverrides, error) {
	payload := map[string]any{}
	if options.Prompt != "" {
		payload["prompt"] = options.Prompt
	}
	if options.Seed > 0 {
		payload["seed"] = options.Seed
	}
	finalAspectRatio := options.AspectRatio
	if finalAspectRatio == "" && options.Size != "" {
		finalAspectRatio = convertSizeToAspectRatio(options.Size)
	}
	if finalAspectRatio != "" {
		payload["aspect_ratio"] = finalAspectRatio
	}
	width, height := parseBlackForestLabsSize(options.Size)
	if width > 0 {
		payload["width"] = width
	}
	if height > 0 {
		payload["height"] = height
	}

	bflOpts := blackForestLabsOptions(options.RequestOptions.ProviderOptions, m.provider.providerID)
	overrides := pollOverrides{
		Interval: parseDurationMillis(bflOpts["pollIntervalMillis"]),
		Timeout:  parseDurationMillis(bflOpts["pollTimeoutMillis"]),
	}
	if err := applyBlackForestLabsImageOptions(payload, bflOpts); err != nil {
		return nil, pollOverrides{}, err
	}
	return payload, overrides, nil
}

type blackForestLabsSubmitResponse struct {
	ID         string   `json:"id"`
	PollingURL string   `json:"polling_url"`
	Cost       *float64 `json:"cost"`
	InputMP    *float64 `json:"input_mp"`
	OutputMP   *float64 `json:"output_mp"`
}

func parseBlackForestLabsSubmitResponse(body []byte) (blackForestLabsSubmitResponse, error) {
	if len(body) == 0 {
		return blackForestLabsSubmitResponse{}, provider.NewInvalidResponseDataError("black forest labs response empty", nil)
	}
	var payload blackForestLabsSubmitResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return blackForestLabsSubmitResponse{}, provider.NewInvalidResponseDataError("black forest labs response invalid", err)
	}
	if payload.ID == "" || payload.PollingURL == "" {
		return blackForestLabsSubmitResponse{}, provider.NewInvalidResponseDataError("black forest labs response missing polling data", nil)
	}
	return payload, nil
}

func ensurePollURL(rawURL string, requestID string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	if query.Get("id") == "" && requestID != "" {
		query.Set("id", requestID)
		parsed.RawQuery = query.Encode()
	}
	return parsed.String()
}

type blackForestLabsPollResponse struct {
	Status string `json:"status"`
	State  string `json:"state"`
	Result *struct {
		Sample    string `json:"sample"`
		Seed      *int   `json:"seed"`
		StartTime *int64 `json:"start_time"`
		EndTime   *int64 `json:"end_time"`
		Duration  *int64 `json:"duration"`
	} `json:"result"`
}

func (p *Provider) awaitImage(ctx context.Context, pollURL string, options provider.RequestOptions, overrides pollOverrides, modelID provider.ModelID) (string, error) {
	pollInterval := defaultPollInterval
	if p.pollInterval > 0 {
		pollInterval = p.pollInterval
	}
	if overrides.Interval > 0 {
		pollInterval = overrides.Interval
	}
	pollTimeout := defaultPollTimeout
	if p.pollTimeout > 0 {
		pollTimeout = p.pollTimeout
	}
	if overrides.Timeout > 0 {
		pollTimeout = overrides.Timeout
	}
	attempts := int(pollTimeout / maxDuration(pollInterval, time.Millisecond))
	if attempts <= 0 {
		attempts = 1
	}

	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if attempt > 0 {
			if err := waitForPoll(ctx, pollInterval); err != nil {
				return "", err
			}
		}
		pollResponse, err := p.fetchPoll(ctx, pollURL, options, modelID)
		if err != nil {
			return "", err
		}
		status := pollResponse.Status
		if status == "" {
			status = pollResponse.State
		}
		switch status {
		case "Ready":
			if pollResponse.Result == nil || pollResponse.Result.Sample == "" {
				return "", provider.NewInvalidResponseDataError("black forest labs poll response missing sample", nil)
			}
			return pollResponse.Result.Sample, nil
		case "Error", "Failed", "Request Moderated":
			return "", provider.NewInvalidResponseDataError("black forest labs generation failed", nil)
		case "Pending":
			continue
		default:
			if status == "" {
				return "", provider.NewInvalidResponseDataError("black forest labs poll response missing status", nil)
			}
			continue
		}
	}
	return "", provider.NewInvalidResponseDataError("black forest labs generation timed out", nil)
}

func (p *Provider) fetchPoll(ctx context.Context, pollURL string, options provider.RequestOptions, modelID provider.ModelID) (blackForestLabsPollResponse, error) {
	req, cancel, err := providerutils.BuildRequest(ctx, http.MethodGet, pollURL, nil, nil, options)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return blackForestLabsPollResponse{}, err
	}
	if cancel != nil {
		defer cancel()
	}

	client := p.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return blackForestLabsPollResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return blackForestLabsPollResponse{}, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return blackForestLabsPollResponse{}, newBlackForestLabsAPIError(resp.StatusCode, body, resp.Header, p.providerID, modelID)
	}
	if len(body) == 0 {
		return blackForestLabsPollResponse{}, provider.NewInvalidResponseDataError("black forest labs poll response empty", nil)
	}
	var payload blackForestLabsPollResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return blackForestLabsPollResponse{}, provider.NewInvalidResponseDataError("black forest labs poll response invalid", err)
	}
	return payload, nil
}

func (p *Provider) handleImageSample(ctx context.Context, sample string, options provider.RequestOptions, modelID provider.ModelID) error {
	if sample == "" {
		return provider.NewInvalidResponseDataError("black forest labs image sample missing", nil)
	}
	if isURL(sample) {
		return p.fetchImage(ctx, sample, options, modelID)
	}
	if strings.HasPrefix(sample, "data:") {
		return validateDataURL(sample)
	}
	if _, err := base64.StdEncoding.DecodeString(sample); err != nil {
		return provider.NewInvalidResponseDataError("black forest labs image sample invalid", err)
	}
	return nil
}

func (p *Provider) fetchImage(ctx context.Context, imageURL string, options provider.RequestOptions, modelID provider.ModelID) error {
	req, cancel, err := providerutils.BuildRequest(ctx, http.MethodGet, imageURL, nil, nil, options)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return err
	}
	if cancel != nil {
		defer cancel()
	}
	client := p.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return provider.NewInvalidResponseDataError(fmt.Sprintf("black forest labs image download failed (%d)", resp.StatusCode), nil)
	}
	if len(body) == 0 {
		return provider.NewInvalidResponseDataError("black forest labs image download empty", nil)
	}
	return nil
}

func blackForestLabsOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func applyBlackForestLabsImageOptions(payload map[string]any, options provider.JSONObject) error {
	if options == nil {
		return nil
	}
	fieldMapping := map[string]string{
		"imagePrompt":         "image_prompt",
		"imagePromptStrength": "image_prompt_strength",
		"outputFormat":        "output_format",
		"promptUpsampling":    "prompt_upsampling",
		"safetyTolerance":     "safety_tolerance",
		"webhookSecret":       "webhook_secret",
		"webhookUrl":          "webhook_url",
	}
	for key, value := range options {
		if key == "pollIntervalMillis" || key == "pollTimeoutMillis" {
			continue
		}
		if mapped, ok := normalizeInputImageKey(key); ok {
			normalized, err := normalizeBlackForestLabsFile(value)
			if err != nil {
				return err
			}
			if normalized != "" {
				payload[mapped] = normalized
			}
			continue
		}
		if key == "mask" || key == "maskUrl" || key == "mask_url" {
			normalized, err := normalizeBlackForestLabsFile(value)
			if err != nil {
				return err
			}
			if normalized != "" {
				payload["mask"] = normalized
			}
			continue
		}
		if mapped, ok := fieldMapping[key]; ok {
			key = mapped
		}
		payload[key] = value
	}
	return nil
}

func normalizeInputImageKey(key string) (string, bool) {
	switch key {
	case "inputImage", "input_image":
		return "input_image", true
	case "inputImage2", "input_image2", "input_image_2":
		return "input_image_2", true
	case "inputImage3", "input_image3", "input_image_3":
		return "input_image_3", true
	case "inputImage4", "input_image4", "input_image_4":
		return "input_image_4", true
	case "inputImage5", "input_image5", "input_image_5":
		return "input_image_5", true
	case "inputImage6", "input_image6", "input_image_6":
		return "input_image_6", true
	case "inputImage7", "input_image7", "input_image_7":
		return "input_image_7", true
	case "inputImage8", "input_image8", "input_image_8":
		return "input_image_8", true
	case "inputImage9", "input_image9", "input_image_9":
		return "input_image_9", true
	case "inputImage10", "input_image10", "input_image_10":
		return "input_image_10", true
	default:
		return "", false
	}
}

func normalizeBlackForestLabsFile(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	switch typed := value.(type) {
	case string:
		return typed, nil
	case []byte:
		return base64.StdEncoding.EncodeToString(typed), nil
	case provider.ImageContent:
		return encodeBlackForestLabsFile(typed.URL, typed.Data)
	case *provider.ImageContent:
		if typed == nil {
			return "", nil
		}
		return encodeBlackForestLabsFile(typed.URL, typed.Data)
	case provider.FileContent:
		return encodeBlackForestLabsFile(typed.URL, typed.Data)
	case *provider.FileContent:
		if typed == nil {
			return "", nil
		}
		return encodeBlackForestLabsFile(typed.URL, typed.Data)
	default:
		return "", provider.NewInvalidRequestError("black forest labs file content unsupported", nil)
	}
}

func encodeBlackForestLabsFile(url string, data []byte) (string, error) {
	if url != "" {
		return url, nil
	}
	if len(data) == 0 {
		return "", provider.NewInvalidRequestError("black forest labs file content missing data", nil)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func parseBlackForestLabsSize(size string) (int, int) {
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

func convertSizeToAspectRatio(size string) string {
	width, height := parseBlackForestLabsSize(size)
	if width <= 0 || height <= 0 {
		return ""
	}
	divisor := gcd(width, height)
	if divisor == 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", width/divisor, height/divisor)
}

func gcd(a int, b int) int {
	x := a
	if x < 0 {
		x = -x
	}
	y := b
	if y < 0 {
		y = -y
	}
	for y != 0 {
		x, y = y, x%y
	}
	return x
}

func parseDurationMillis(value any) time.Duration {
	if value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return time.Duration(typed) * time.Millisecond
		}
	case int64:
		if typed > 0 {
			return time.Duration(typed) * time.Millisecond
		}
	case float64:
		if typed > 0 {
			return time.Duration(typed) * time.Millisecond
		}
	case json.Number:
		if parsed, err := typed.Int64(); err == nil && parsed > 0 {
			return time.Duration(parsed) * time.Millisecond
		}
	}
	return 0
}

func waitForPoll(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func maxDuration(a time.Duration, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func isURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	return parsed.Scheme != "" && parsed.Host != ""
}

func validateDataURL(value string) error {
	parts := strings.SplitN(value, ",", 2)
	if len(parts) != 2 {
		return provider.NewInvalidResponseDataError("black forest labs image sample invalid", nil)
	}
	if _, err := base64.StdEncoding.DecodeString(parts[1]); err != nil {
		return provider.NewInvalidResponseDataError("black forest labs image sample invalid", err)
	}
	return nil
}

func newBlackForestLabsAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("black forest labs api error (%d)", status)
	if parsed := parseBlackForestLabsErrorMessage(body); parsed != "" {
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

func parseBlackForestLabsErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload struct {
		Message string `json:"message"`
		Detail  any    `json:"detail"`
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
