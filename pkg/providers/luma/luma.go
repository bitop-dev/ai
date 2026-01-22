package luma

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

const DefaultBaseURL = "https://api.lumalabs.ai"
const DefaultProviderName = "luma"

const defaultPollInterval = 500 * time.Millisecond
const defaultMaxPollAttempts = 120

type Settings struct {
	APIKey          string
	BaseURL         string
	Headers         map[string]string
	HTTPClient      *http.Client
	ProviderName    string
	PollInterval    time.Duration
	MaxPollAttempts int
}

type Provider struct {
	apiKey          string
	baseURL         string
	headers         map[string]string
	httpClient      *http.Client
	providerID      provider.ProviderID
	pollInterval    time.Duration
	maxPollAttempts int
}

func CreateLuma(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("LUMA_API_KEY")
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
		apiKey:          apiKey,
		baseURL:         baseURL,
		headers:         settings.Headers,
		httpClient:      settings.HTTPClient,
		providerID:      provider.ProviderID(providerName),
		pollInterval:    settings.PollInterval,
		maxPollAttempts: settings.MaxPollAttempts,
	}
}

func (p *Provider) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (p *Provider) LanguageModel(modelID provider.ModelID) (provider.LanguageModelV3, error) {
	return nil, provider.NewNoSuchModelError("luma does not support language models", nil, p.providerID, modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("luma does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return &imageModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("luma api key is required")
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

type pollOverrides struct {
	Interval    time.Duration
	MaxAttempts int
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
		return provider.ImageModelV3Result{}, newLumaAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	generation, err := parseLumaGenerationResponse(response.Body)
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}
	imageURL, err := m.resolveImageURL(ctx, generation, requestOptions, overrides)
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}
	if err := m.provider.fetchImage(ctx, imageURL); err != nil {
		return provider.ImageModelV3Result{}, err
	}
	return provider.ImageModelV3Result{}, nil
}

func (m *imageModel) endpoint() string {
	return m.provider.endpoint("/dream-machine/v1/generations/image")
}

func (m *imageModel) resolveImageURL(ctx context.Context, generation lumaGenerationResponse, options provider.RequestOptions, overrides pollOverrides) (string, error) {
	if generation.ID == "" {
		return "", provider.NewInvalidResponseDataError("luma generation response missing id", nil)
	}
	state := strings.ToLower(generation.State)
	switch state {
	case "completed":
		return extractLumaImageURL(generation)
	case "failed":
		return "", provider.NewInvalidResponseDataError(lumaFailureMessage(generation), nil)
	}
	return m.provider.awaitGeneration(ctx, generation.ID, options, overrides, m.modelID)
}

func (m *imageModel) buildPayload(options provider.ImageModelV3CallOptions) (map[string]any, pollOverrides, error) {
	payload := map[string]any{
		"model": string(m.modelID),
	}
	if options.Prompt != "" {
		payload["prompt"] = options.Prompt
	}
	if options.AspectRatio != "" {
		payload["aspect_ratio"] = options.AspectRatio
	}

	lumaOpts := lumaOptions(options.RequestOptions.ProviderOptions, m.provider.providerID)
	overrides := pollOverrides{
		Interval:    parseDurationMillis(lumaOpts["pollIntervalMillis"]),
		MaxAttempts: parseInt(lumaOpts["maxPollAttempts"]),
	}
	if err := applyLumaReferenceOptions(payload, lumaOpts); err != nil {
		return nil, pollOverrides{}, err
	}
	for key, value := range lumaOpts {
		switch key {
		case "pollIntervalMillis", "maxPollAttempts", "referenceType", "reference_type", "files", "referenceImages", "reference_images", "images":
			continue
		default:
			payload[key] = value
		}
	}
	return payload, overrides, nil
}

type lumaGenerationResponse struct {
	ID            string `json:"id"`
	State         string `json:"state"`
	FailureReason string `json:"failure_reason"`
	Assets        *struct {
		Image string `json:"image"`
	} `json:"assets"`
}

func parseLumaGenerationResponse(body []byte) (lumaGenerationResponse, error) {
	if len(body) == 0 {
		return lumaGenerationResponse{}, provider.NewInvalidResponseDataError("luma response empty", nil)
	}
	var payload lumaGenerationResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return lumaGenerationResponse{}, provider.NewInvalidResponseDataError("luma response invalid", err)
	}
	return payload, nil
}

func extractLumaImageURL(generation lumaGenerationResponse) (string, error) {
	if generation.Assets == nil || generation.Assets.Image == "" {
		return "", provider.NewInvalidResponseDataError("luma generation missing image", nil)
	}
	return generation.Assets.Image, nil
}

func lumaFailureMessage(generation lumaGenerationResponse) string {
	if generation.FailureReason != "" {
		return generation.FailureReason
	}
	return "luma generation failed"
}

func (p *Provider) awaitGeneration(ctx context.Context, generationID string, options provider.RequestOptions, overrides pollOverrides, modelID provider.ModelID) (string, error) {
	pollInterval := defaultPollInterval
	if p.pollInterval > 0 {
		pollInterval = p.pollInterval
	}
	if overrides.Interval > 0 {
		pollInterval = overrides.Interval
	}
	maxAttempts := defaultMaxPollAttempts
	if p.maxPollAttempts > 0 {
		maxAttempts = p.maxPollAttempts
	}
	if overrides.MaxAttempts > 0 {
		maxAttempts = overrides.MaxAttempts
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if attempt > 0 {
			if err := waitForPoll(ctx, pollInterval); err != nil {
				return "", err
			}
		}
		generation, err := p.fetchGeneration(ctx, generationID, options, modelID)
		if err != nil {
			return "", err
		}
		state := strings.ToLower(generation.State)
		switch state {
		case "completed":
			return extractLumaImageURL(generation)
		case "failed":
			return "", provider.NewInvalidResponseDataError(lumaFailureMessage(generation), nil)
		case "queued", "dreaming", "":
			continue
		default:
			continue
		}
	}
	return "", provider.NewInvalidResponseDataError("luma generation timed out", nil)
}

func (p *Provider) fetchGeneration(ctx context.Context, generationID string, options provider.RequestOptions, modelID provider.ModelID) (lumaGenerationResponse, error) {
	url := p.endpoint("/dream-machine/v1/generations/" + generationID)
	req, cancel, err := providerutils.BuildRequest(ctx, http.MethodGet, url, nil, nil, options)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return lumaGenerationResponse{}, err
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
		return lumaGenerationResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return lumaGenerationResponse{}, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return lumaGenerationResponse{}, newLumaAPIError(resp.StatusCode, body, resp.Header, p.providerID, modelID)
	}
	return parseLumaGenerationResponse(body)
}

func (p *Provider) fetchImage(ctx context.Context, imageURL string) error {
	if imageURL == "" {
		return provider.NewInvalidResponseDataError("luma image URL missing", nil)
	}
	req, cancel, err := providerutils.BuildRequest(ctx, http.MethodGet, imageURL, nil, nil, provider.RequestOptions{})
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
		return provider.NewInvalidResponseDataError(fmt.Sprintf("luma image download failed (%d)", resp.StatusCode), nil)
	}
	if len(body) == 0 {
		return provider.NewInvalidResponseDataError("luma image download empty", nil)
	}
	return nil
}

type lumaImageConfig struct {
	Weight *float64
	ID     string
}

func applyLumaReferenceOptions(payload map[string]any, options provider.JSONObject) error {
	if options == nil {
		return nil
	}
	files, err := extractLumaFiles(options)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	referenceType := extractReferenceType(options)
	if referenceType == "" {
		referenceType = "image"
	}
	configs := extractLumaImageConfigs(options["images"])

	defaultWeights := map[string]float64{
		"image":        0.85,
		"style":        0.8,
		"modify_image": 1.0,
	}

	switch referenceType {
	case "image":
		if len(files) > 4 {
			return provider.NewInvalidRequestError("luma image supports up to 4 reference images", nil)
		}
		refs := make([]map[string]any, 0, len(files))
		for i, file := range files {
			weight := defaultWeights["image"]
			if i < len(configs) && configs[i].Weight != nil {
				weight = *configs[i].Weight
			}
			refs = append(refs, map[string]any{"url": file, "weight": weight})
		}
		payload["image"] = refs
	case "style":
		refs := make([]map[string]any, 0, len(files))
		for i, file := range files {
			weight := defaultWeights["style"]
			if i < len(configs) && configs[i].Weight != nil {
				weight = *configs[i].Weight
			}
			refs = append(refs, map[string]any{"url": file, "weight": weight})
		}
		payload["style"] = refs
	case "character":
		identities := map[string][]string{}
		for i, file := range files {
			identity := "identity0"
			if i < len(configs) && configs[i].ID != "" {
				identity = configs[i].ID
			}
			identities[identity] = append(identities[identity], file)
		}
		for identity, images := range identities {
			if len(images) > 4 {
				return provider.NewInvalidRequestError(fmt.Sprintf("luma character supports up to 4 images per identity (%s)", identity), nil)
			}
		}
		refs := map[string]any{}
		for identity, images := range identities {
			refs[identity] = map[string]any{"images": images}
		}
		payload["character"] = refs
	case "modify_image":
		if len(files) > 1 {
			return provider.NewInvalidRequestError("luma modify_image only supports a single input image", nil)
		}
		weight := defaultWeights["modify_image"]
		if len(configs) > 0 && configs[0].Weight != nil {
			weight = *configs[0].Weight
		}
		payload["modify_image"] = map[string]any{"url": files[0], "weight": weight}
	default:
		return provider.NewInvalidRequestError("luma referenceType unsupported", nil)
	}
	return nil
}

func extractReferenceType(options provider.JSONObject) string {
	if value, ok := options["referenceType"]; ok {
		if typed, ok := value.(string); ok {
			return typed
		}
	}
	if value, ok := options["reference_type"]; ok {
		if typed, ok := value.(string); ok {
			return typed
		}
	}
	return ""
}

func extractLumaFiles(options provider.JSONObject) ([]string, error) {
	value := options["files"]
	if value == nil {
		value = options["referenceImages"]
	}
	if value == nil {
		value = options["reference_images"]
	}
	if value == nil {
		return nil, nil
	}
	list, ok := value.([]any)
	if !ok {
		if strings, ok := value.([]string); ok {
			return strings, nil
		}
		if images, ok := value.([]provider.ImageContent); ok {
			normalized := make([]string, 0, len(images))
			for _, image := range images {
				url, err := normalizeLumaFile(image)
				if err != nil {
					return nil, err
				}
				normalized = append(normalized, url)
			}
			return normalized, nil
		}
		return nil, nil
	}
	files := make([]string, 0, len(list))
	for _, item := range list {
		url, err := normalizeLumaFile(item)
		if err != nil {
			return nil, err
		}
		files = append(files, url)
	}
	return files, nil
}

func normalizeLumaFile(value any) (string, error) {
	if value == nil {
		return "", provider.NewInvalidRequestError("luma image content missing url", nil)
	}
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return "", provider.NewInvalidRequestError("luma image content missing url", nil)
		}
		return typed, nil
	case provider.ImageContent:
		if typed.URL == "" {
			return "", provider.NewInvalidRequestError("luma image content missing url", nil)
		}
		if len(typed.Data) > 0 {
			return "", provider.NewInvalidRequestError("luma only supports URL-based images", nil)
		}
		return typed.URL, nil
	case *provider.ImageContent:
		if typed == nil || typed.URL == "" {
			return "", provider.NewInvalidRequestError("luma image content missing url", nil)
		}
		if len(typed.Data) > 0 {
			return "", provider.NewInvalidRequestError("luma only supports URL-based images", nil)
		}
		return typed.URL, nil
	case map[string]any:
		if url, ok := typed["url"].(string); ok && url != "" {
			return url, nil
		}
		return "", provider.NewInvalidRequestError("luma image content missing url", nil)
	case provider.JSONObject:
		if url, ok := typed["url"].(string); ok && url != "" {
			return url, nil
		}
		return "", provider.NewInvalidRequestError("luma image content missing url", nil)
	default:
		return "", provider.NewInvalidRequestError("luma image content unsupported", nil)
	}
}

func extractLumaImageConfigs(value any) []lumaImageConfig {
	if value == nil {
		return nil
	}
	list, ok := value.([]any)
	if !ok {
		if configs, ok := value.([]map[string]any); ok {
			list = make([]any, 0, len(configs))
			for _, config := range configs {
				list = append(list, config)
			}
		} else {
			return nil
		}
	}
	configs := make([]lumaImageConfig, 0, len(list))
	for _, item := range list {
		config := lumaImageConfig{}
		switch typed := item.(type) {
		case map[string]any:
			config = parseLumaImageConfig(typed)
		case provider.JSONObject:
			config = parseLumaImageConfig(convertJSONObject(typed))
		default:
			config = lumaImageConfig{}
		}
		configs = append(configs, config)
	}
	return configs
}

func convertJSONObject(value provider.JSONObject) map[string]any {
	converted := make(map[string]any, len(value))
	for key, entry := range value {
		converted[key] = entry
	}
	return converted
}

func parseLumaImageConfig(value map[string]any) lumaImageConfig {
	config := lumaImageConfig{}
	if weight, ok := parseFloat(value["weight"]); ok {
		config.Weight = &weight
	}
	if id, ok := value["id"].(string); ok {
		config.ID = id
	}
	return config
}

func lumaOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func parseInt(value any) int {
	if value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
	}
	return 0
}

func parseFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		if parsed, err := typed.Float64(); err == nil {
			return parsed, true
		}
	}
	return 0, false
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

type lumaErrorDetail struct {
	Msg string `json:"msg"`
}

type lumaErrorResponse struct {
	Detail  []lumaErrorDetail `json:"detail"`
	Message string            `json:"message"`
	Error   string            `json:"error"`
}

func newLumaAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("luma api error (%d)", status)
	if parsed := parseLumaErrorMessage(body); parsed != "" {
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

func parseLumaErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload lumaErrorResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if len(payload.Detail) > 0 && payload.Detail[0].Msg != "" {
		return payload.Detail[0].Msg
	}
	if payload.Message != "" {
		return payload.Message
	}
	if payload.Error != "" {
		return payload.Error
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
