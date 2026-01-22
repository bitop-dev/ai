package replicate

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

const DefaultBaseURL = "https://api.replicate.com/v1"
const DefaultProviderName = "replicate"

const defaultPollInterval = 500 * time.Millisecond

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

func CreateReplicate(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("REPLICATE_API_TOKEN")
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
	return nil, provider.NewNoSuchModelError("replicate does not support language models", nil, p.providerID, modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("replicate does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return &imageModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("replicate api token is required")
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
	payload, preferHeader := m.buildPayload(options)
	createOptions := requestOptions
	if preferHeader != "" {
		createOptions.Headers = providerutils.MergeHeaders(requestOptions.Headers, map[string]string{"Prefer": preferHeader})
	}

	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.endpoint(), payload, createOptions, nil, nil)
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.ImageModelV3Result{}, newReplicateAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}

	prediction, err := decodePrediction(response.Body)
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}

	if err := m.provider.awaitPrediction(ctx, prediction, requestOptions, m.modelID); err != nil {
		return provider.ImageModelV3Result{}, err
	}
	return provider.ImageModelV3Result{}, nil
}

func (m *imageModel) endpoint() string {
	model, version := splitModelID(m.modelID)
	if version != "" {
		return m.provider.endpoint("/predictions")
	}
	return m.provider.endpoint("/models/" + model + "/predictions")
}

func (m *imageModel) buildPayload(options provider.ImageModelV3CallOptions) (map[string]any, string) {
	input := map[string]any{}
	if options.Prompt != "" {
		input["prompt"] = options.Prompt
	}
	if options.AspectRatio != "" {
		input["aspect_ratio"] = options.AspectRatio
	}
	if options.Size != "" {
		input["size"] = options.Size
	}
	if options.Seed > 0 {
		input["seed"] = options.Seed
	}
	if options.N > 0 {
		input["num_outputs"] = options.N
	}

	replicateOpts := replicateOptions(options.RequestOptions.ProviderOptions, m.provider.providerID)
	for key, value := range replicateInputOverrides(replicateOpts) {
		input[key] = value
	}
	payload := map[string]any{"input": input}
	for key, value := range replicateRequestOverrides(replicateOpts) {
		payload[key] = value
	}

	_, version := splitModelID(m.modelID)
	if version != "" {
		payload["version"] = version
	}

	return payload, preferHeaderValue(replicateOpts)
}

func splitModelID(modelID provider.ModelID) (string, string) {
	parts := strings.SplitN(string(modelID), ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return parts[0], ""
}

type predictionResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Output any    `json:"output"`
	Error  any    `json:"error"`
	URLs   struct {
		Get string `json:"get"`
	} `json:"urls"`
}

func (p *Provider) awaitPrediction(ctx context.Context, prediction predictionResponse, requestOptions provider.RequestOptions, modelID provider.ModelID) error {
	pollURL := prediction.URLs.Get
	if pollURL == "" && prediction.ID != "" {
		pollURL = p.endpoint("/predictions/" + prediction.ID)
	}
	attempt := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := validatePrediction(prediction, p.providerID, modelID); err == nil {
			return nil
		} else if err != errPredictionPending {
			return err
		}
		if pollURL == "" {
			return provider.NewInvalidResponseDataError("replicate prediction missing poll URL", nil)
		}

		if attempt > 0 {
			if err := waitForPoll(ctx, defaultPollInterval); err != nil {
				return err
			}
		}
		attempt++

		next, err := p.fetchPrediction(ctx, pollURL, requestOptions, modelID)
		if err != nil {
			return err
		}
		prediction = next
	}
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

var errPredictionPending = fmt.Errorf("replicate prediction pending")

func validatePrediction(prediction predictionResponse, providerID provider.ProviderID, modelID provider.ModelID) error {
	status := strings.ToLower(prediction.Status)
	switch status {
	case "succeeded":
		if _, err := parsePredictionOutput(prediction.Output); err != nil {
			return err
		}
		return nil
	case "failed", "canceled", "cancelled":
		message := predictionErrorMessage(prediction.Error)
		if message == "" {
			message = "replicate prediction failed"
		}
		payload, _ := json.Marshal(prediction)
		return newReplicatePredictionError(message, payload, nil, providerID, modelID)
	case "starting", "processing":
		return errPredictionPending
	case "":
		return provider.NewInvalidResponseDataError("replicate prediction missing status", nil)
	default:
		return errPredictionPending
	}
}

func (p *Provider) fetchPrediction(ctx context.Context, url string, options provider.RequestOptions, modelID provider.ModelID) (predictionResponse, error) {
	req, cancel, err := providerutils.BuildRequest(ctx, http.MethodGet, url, nil, nil, options)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return predictionResponse{}, err
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
		return predictionResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return predictionResponse{}, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return predictionResponse{}, newReplicateAPIError(resp.StatusCode, body, resp.Header, p.providerID, modelID)
	}

	prediction, err := decodePrediction(body)
	if err != nil {
		return predictionResponse{}, err
	}
	return prediction, nil
}

func decodePrediction(body []byte) (predictionResponse, error) {
	if len(body) == 0 {
		return predictionResponse{}, provider.NewInvalidResponseDataError("replicate response empty", nil)
	}
	var prediction predictionResponse
	if err := json.Unmarshal(body, &prediction); err != nil {
		return predictionResponse{}, provider.NewInvalidResponseDataError("replicate response invalid", err)
	}
	return prediction, nil
}

func predictionErrorMessage(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]any:
		if message, ok := typed["message"].(string); ok {
			return message
		}
		if detail, ok := typed["detail"].(string); ok {
			return detail
		}
	case provider.JSONObject:
		if message, ok := typed["message"].(string); ok {
			return message
		}
		if detail, ok := typed["detail"].(string); ok {
			return detail
		}
	}
	return ""
}

func parsePredictionOutput(output any) ([]string, error) {
	if output == nil {
		return nil, provider.NewInvalidResponseDataError("replicate output missing", nil)
	}
	switch typed := output.(type) {
	case string:
		if typed == "" {
			return nil, provider.NewInvalidResponseDataError("replicate output empty", nil)
		}
		return []string{typed}, nil
	case []any:
		results := make([]string, 0, len(typed))
		for _, item := range typed {
			value, ok := item.(string)
			if !ok || value == "" {
				return nil, provider.NewInvalidResponseDataError("replicate output invalid", nil)
			}
			results = append(results, value)
		}
		if len(results) == 0 {
			return nil, provider.NewInvalidResponseDataError("replicate output empty", nil)
		}
		return results, nil
	default:
		return nil, provider.NewInvalidResponseDataError("replicate output invalid", nil)
	}
}

func replicateOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func replicateInputOverrides(options provider.JSONObject) map[string]any {
	overrides := map[string]any{}
	if options == nil {
		return overrides
	}
	for key, value := range options {
		switch key {
		case "maxWaitTimeInSeconds", "max_wait_time_in_seconds", "request":
			continue
		default:
			overrides[key] = value
		}
	}
	return overrides
}

func replicateRequestOverrides(options provider.JSONObject) map[string]any {
	overrides := map[string]any{}
	if options == nil {
		return overrides
	}
	if request, ok := options["request"]; ok {
		for key, value := range normalizeObject(request) {
			overrides[key] = value
		}
	}
	return overrides
}

func preferHeaderValue(options provider.JSONObject) string {
	if options == nil {
		return "wait"
	}
	if value, ok := options["maxWaitTimeInSeconds"]; ok {
		if wait, ok := parseWaitSeconds(value); ok {
			return fmt.Sprintf("wait=%d", wait)
		}
		return "wait"
	}
	if value, ok := options["max_wait_time_in_seconds"]; ok {
		if wait, ok := parseWaitSeconds(value); ok {
			return fmt.Sprintf("wait=%d", wait)
		}
	}
	return "wait"
}

func parseWaitSeconds(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed, true
		}
	case int64:
		if typed > 0 {
			return int(typed), true
		}
	case float64:
		if typed > 0 {
			return int(typed), true
		}
	case json.Number:
		if parsed, err := typed.Int64(); err == nil && parsed > 0 {
			return int(parsed), true
		}
	}
	return 0, false
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

func newReplicateAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("replicate api error (%d)", status)
	if len(body) > 0 {
		var payload struct {
			Detail string `json:"detail"`
			Error  string `json:"error"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			switch {
			case payload.Detail != "":
				message = payload.Detail
			case payload.Error != "":
				message = payload.Error
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

func newReplicatePredictionError(message string, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	base := provider.ApiCallError{
		AISDKError: provider.AISDKError{
			Category: provider.ErrorCategoryAPICall,
			Kind:     provider.ErrorKindAPICall,
			Message:  message,
		},
		StatusCode:      http.StatusOK,
		RequestID:       headers.Get("x-request-id"),
		ResponseHeaders: cloneHeaders(headers),
		ResponseBody:    string(body),
		ProviderID:      providerID,
		ModelID:         modelID,
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
