package fal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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

const DefaultBaseURL = "https://fal.run"
const DefaultQueueURL = "https://queue.fal.run"
const DefaultProviderName = "fal"

const defaultPollInterval = time.Second
const defaultPollTimeout = 60 * time.Second

type Settings struct {
	APIKey       string
	BaseURL      string
	QueueURL     string
	Headers      map[string]string
	HTTPClient   *http.Client
	ProviderName string
}

type Provider struct {
	apiKey     string
	baseURL    string
	queueURL   string
	headers    map[string]string
	httpClient *http.Client
	providerID provider.ProviderID
}

func CreateFal(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("FAL_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("FAL_KEY")
		}
	}
	baseURL := strings.TrimRight(settings.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	queueURL := strings.TrimRight(settings.QueueURL, "/")
	if queueURL == "" {
		queueURL = DefaultQueueURL
	}
	providerName := settings.ProviderName
	if providerName == "" {
		providerName = DefaultProviderName
	}
	return &Provider{
		apiKey:     apiKey,
		baseURL:    baseURL,
		queueURL:   queueURL,
		headers:    settings.Headers,
		httpClient: settings.HTTPClient,
		providerID: provider.ProviderID(providerName),
	}
}

func (p *Provider) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (p *Provider) LanguageModel(modelID provider.ModelID) (provider.LanguageModelV3, error) {
	return nil, provider.NewNoSuchModelError("fal does not support language models", nil, p.providerID, modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("fal does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return &imageModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) SpeechModel(modelID provider.ModelID) (provider.SpeechModelV3, error) {
	return &speechModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) TranscriptionModel(modelID provider.ModelID) (provider.TranscriptionModelV3, error) {
	return &transcriptionModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("fal api key is required")
	}
	headers := map[string]string{
		"Authorization": "Key " + p.apiKey,
	}
	for key, value := range p.headers {
		headers[key] = value
	}
	return headers, nil
}

func (p *Provider) endpoint(path string) string {
	return p.baseURL + path
}

func (p *Provider) queueEndpoint(path string) string {
	return p.queueURL + path
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
	payload, err := m.buildPayload(options)
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.endpoint(), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.ImageModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.ImageModelV3Result{}, newFalAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	if err := parseFalImageResponse(response.Body); err != nil {
		return provider.ImageModelV3Result{}, err
	}
	return provider.ImageModelV3Result{}, nil
}

func (m *imageModel) endpoint() string {
	return m.provider.endpoint("/" + string(m.modelID))
}

func (m *imageModel) buildPayload(options provider.ImageModelV3CallOptions) (map[string]any, error) {
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
	if options.Size != "" {
		imageSize, err := parseFalSize(options.Size)
		if err != nil {
			return nil, err
		}
		payload["image_size"] = imageSize
	} else if options.AspectRatio != "" {
		if imageSize := falAspectRatioSize(options.AspectRatio); imageSize != nil {
			payload["image_size"] = imageSize
		}
	}
	falOpts := falOptions(options.RequestOptions.ProviderOptions, m.provider.providerID)
	if err := applyFalImageOptions(payload, falOpts); err != nil {
		return nil, err
	}
	return payload, nil
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
		return provider.SpeechModelV3Result{}, provider.NewInvalidRequestError("fal speech text is required", nil)
	}
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.SpeechModelV3Result{}, err
	}
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)
	payload := map[string]any{
		"text":          options.Text,
		"output_format": normalizeFalSpeechOutputFormat(options.OutputFormat),
	}
	if options.Voice != "" {
		payload["voice"] = options.Voice
	}
	if options.Speed != 0 {
		payload["speed"] = options.Speed
	}
	if falOpts := falOptions(options.RequestOptions.ProviderOptions, m.provider.providerID); falOpts != nil {
		applyFalSpeechOptions(payload, falOpts)
	}
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.endpoint(), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.SpeechModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.SpeechModelV3Result{}, newFalAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	if err := parseFalSpeechResponse(response.Body); err != nil {
		return provider.SpeechModelV3Result{}, err
	}
	return provider.SpeechModelV3Result{}, nil
}

func (m *speechModel) endpoint() string {
	return m.provider.endpoint("/" + string(m.modelID))
}

type transcriptionModel struct {
	provider *Provider
	modelID  provider.ModelID
}

func (m *transcriptionModel) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (m *transcriptionModel) ProviderID() provider.ProviderID {
	return m.provider.providerID
}

func (m *transcriptionModel) ModelID() provider.ModelID {
	return m.modelID
}

func (m *transcriptionModel) DoGenerate(ctx context.Context, options provider.TranscriptionModelV3CallOptions) (provider.TranscriptionModelV3Result, error) {
	if len(options.Audio) == 0 {
		return provider.TranscriptionModelV3Result{}, provider.NewInvalidRequestError("fal transcription audio is required", nil)
	}
	if options.MediaType == "" {
		return provider.TranscriptionModelV3Result{}, provider.NewInvalidRequestError("fal transcription media type is required", nil)
	}
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)
	encodedAudio := base64.StdEncoding.EncodeToString(options.Audio)
	audioURL := fmt.Sprintf("data:%s;base64,%s", options.MediaType, encodedAudio)
	payload := map[string]any{
		"task":        "transcribe",
		"diarize":     true,
		"chunk_level": "word",
		"audio_url":   audioURL,
	}
	if falOpts := falOptions(options.RequestOptions.ProviderOptions, m.provider.providerID); falOpts != nil {
		applyFalTranscriptionOptions(payload, falOpts)
	}
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, m.endpoint(), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.TranscriptionModelV3Result{}, newFalAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	requestID, err := parseFalQueueResponse(response.Body)
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	if err := m.provider.awaitTranscription(ctx, requestID, requestOptions, m.modelID); err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	return provider.TranscriptionModelV3Result{}, nil
}

func (m *transcriptionModel) endpoint() string {
	return m.provider.queueEndpoint("/fal-ai/" + string(m.modelID))
}

func (p *Provider) awaitTranscription(ctx context.Context, requestID string, options provider.RequestOptions, modelID provider.ModelID) error {
	start := time.Now()
	attempt := 0
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt > 0 {
			if err := waitForPoll(ctx, defaultPollInterval); err != nil {
				return err
			}
		}
		attempt++
		if err := p.fetchTranscription(ctx, requestID, options, modelID); err != nil {
			if errors.Is(err, errFalRequestPending) {
				if time.Since(start) > defaultPollTimeout {
					return provider.NewInvalidResponseDataError("fal transcription polling timed out", nil)
				}
				continue
			}
			return err
		}
		return nil
	}
}

func (p *Provider) fetchTranscription(ctx context.Context, requestID string, options provider.RequestOptions, modelID provider.ModelID) error {
	url := p.queueEndpoint("/fal-ai/" + string(modelID) + "/requests/" + requestID)
	req, cancel, err := providerutils.BuildRequest(ctx, http.MethodGet, url, nil, nil, options)
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
		if isFalRequestPending(body) {
			return errFalRequestPending
		}
		return newFalAPIError(resp.StatusCode, body, resp.Header, p.providerID, modelID)
	}
	return parseFalTranscriptionResponse(body)
}

func falOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func applyFalImageOptions(payload map[string]any, options provider.JSONObject) error {
	if options == nil {
		return nil
	}
	fieldMapping := map[string]string{
		"imageUrl":             "image_url",
		"imageURL":             "image_url",
		"imageUrls":            "image_urls",
		"imageURLs":            "image_urls",
		"maskUrl":              "mask_url",
		"maskURL":              "mask_url",
		"guidanceScale":        "guidance_scale",
		"numInferenceSteps":    "num_inference_steps",
		"enableSafetyChecker":  "enable_safety_checker",
		"outputFormat":         "output_format",
		"syncMode":             "sync_mode",
		"safetyTolerance":      "safety_tolerance",
		"useMultipleImages":    "",
		"__deprecatedKeys":     "",
		"deprecatedKeys":       "",
		"deprecated_keys":      "",
		"deprecatedkeys":       "",
		"deprecatedKeysIgnore": "",
	}
	for key, value := range options {
		mapped, ok := fieldMapping[key]
		if ok && mapped == "" {
			continue
		}
		if ok && mapped != "" {
			key = mapped
		}
		switch key {
		case "image_url", "mask_url":
			normalized, err := normalizeFalFileValue(value)
			if err != nil {
				return err
			}
			if normalized != nil {
				payload[key] = normalized
			}
		case "image_urls":
			normalized, err := normalizeFalFileArray(value)
			if err != nil {
				return err
			}
			if normalized != nil {
				payload[key] = normalized
			}
		default:
			payload[key] = value
		}
	}
	return nil
}

func applyFalSpeechOptions(payload map[string]any, options provider.JSONObject) {
	if options == nil {
		return
	}
	fieldMapping := map[string]string{
		"voiceSetting":       "voice_setting",
		"audioSetting":       "audio_setting",
		"languageBoost":      "language_boost",
		"pronunciationDict":  "pronunciation_dict",
		"pronunciation_dict": "pronunciation_dict",
	}
	for key, value := range options {
		if mapped, ok := fieldMapping[key]; ok {
			key = mapped
		}
		payload[key] = value
	}
}

func applyFalTranscriptionOptions(payload map[string]any, options provider.JSONObject) {
	if options == nil {
		return
	}
	fieldMapping := map[string]string{
		"chunkLevel":  "chunk_level",
		"batchSize":   "batch_size",
		"numSpeakers": "num_speakers",
	}
	for key, value := range options {
		if mapped, ok := fieldMapping[key]; ok {
			key = mapped
		}
		payload[key] = value
	}
}

func normalizeFalSpeechOutputFormat(format string) string {
	if format == "hex" {
		return "hex"
	}
	return "url"
}

func parseFalSize(size string) (map[string]int, error) {
	parts := strings.SplitN(size, "x", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("fal image size must be <width>x<height>")
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil || width <= 0 {
		return nil, fmt.Errorf("fal image size width invalid")
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil || height <= 0 {
		return nil, fmt.Errorf("fal image size height invalid")
	}
	return map[string]int{"width": width, "height": height}, nil
}

func falAspectRatioSize(aspectRatio string) any {
	switch aspectRatio {
	case "1:1":
		return "square_hd"
	case "16:9":
		return "landscape_16_9"
	case "9:16":
		return "portrait_16_9"
	case "4:3":
		return "landscape_4_3"
	case "3:4":
		return "portrait_4_3"
	case "16:10":
		return map[string]int{"width": 1280, "height": 800}
	case "10:16":
		return map[string]int{"width": 800, "height": 1280}
	case "21:9":
		return map[string]int{"width": 2560, "height": 1080}
	case "9:21":
		return map[string]int{"width": 1080, "height": 2560}
	}
	return nil
}

type falImageResponse struct {
	Images []falImage `json:"images"`
	Image  *falImage  `json:"image"`
}

type falImage struct {
	URL       string  `json:"url"`
	FileData  string  `json:"file_data"`
	FileName  *string `json:"file_name"`
	FileSize  *int64  `json:"file_size"`
	Width     *int    `json:"width"`
	Height    *int    `json:"height"`
	MediaType string  `json:"content_type"`
}

func parseFalImageResponse(body []byte) error {
	if len(body) == 0 {
		return provider.NewInvalidResponseDataError("fal image response empty", nil)
	}
	var response falImageResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return provider.NewInvalidResponseDataError("fal image response invalid", err)
	}
	images := response.Images
	if len(images) == 0 && response.Image != nil {
		images = []falImage{*response.Image}
	}
	if len(images) == 0 {
		return provider.NewInvalidResponseDataError("fal image response missing images", nil)
	}
	for _, image := range images {
		if image.URL == "" && image.FileData == "" {
			return provider.NewInvalidResponseDataError("fal image response missing image url", nil)
		}
	}
	return nil
}

type falSpeechResponse struct {
	Audio struct {
		URL string `json:"url"`
	} `json:"audio"`
}

func parseFalSpeechResponse(body []byte) error {
	if len(body) == 0 {
		return provider.NewInvalidResponseDataError("fal speech response empty", nil)
	}
	var response falSpeechResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return provider.NewInvalidResponseDataError("fal speech response invalid", err)
	}
	if response.Audio.URL == "" {
		return provider.NewInvalidResponseDataError("fal speech response missing audio url", nil)
	}
	return nil
}

type falQueueResponse struct {
	RequestID string `json:"request_id"`
}

func parseFalQueueResponse(body []byte) (string, error) {
	if len(body) == 0 {
		return "", provider.NewInvalidResponseDataError("fal queue response empty", nil)
	}
	var response falQueueResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", provider.NewInvalidResponseDataError("fal queue response invalid", err)
	}
	if response.RequestID == "" {
		return "", provider.NewInvalidResponseDataError("fal queue response missing request_id", nil)
	}
	return response.RequestID, nil
}

type falTranscriptionResponse struct {
	Text string `json:"text"`
}

func parseFalTranscriptionResponse(body []byte) error {
	if len(body) == 0 {
		return provider.NewInvalidResponseDataError("fal transcription response empty", nil)
	}
	var response falTranscriptionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return provider.NewInvalidResponseDataError("fal transcription response invalid", err)
	}
	if response.Text == "" {
		return provider.NewInvalidResponseDataError("fal transcription response missing text", nil)
	}
	return nil
}

type falProgressResponse struct {
	Detail string `json:"detail"`
}

var errFalRequestPending = errors.New("fal request pending")

func isFalRequestPending(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var payload falProgressResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	return strings.EqualFold(payload.Detail, "Request is still in progress")
}

func normalizeFalFileArray(value any) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case []string:
		return typed, nil
	case []provider.ImageContent:
		results := make([]string, 0, len(typed))
		for _, item := range typed {
			value, err := normalizeFalFileValue(item)
			if err != nil {
				return nil, err
			}
			if str, ok := value.(string); ok && str != "" {
				results = append(results, str)
			}
		}
		return results, nil
	case []provider.FileContent:
		results := make([]string, 0, len(typed))
		for _, item := range typed {
			value, err := normalizeFalFileValue(item)
			if err != nil {
				return nil, err
			}
			if str, ok := value.(string); ok && str != "" {
				results = append(results, str)
			}
		}
		return results, nil
	case []any:
		results := make([]string, 0, len(typed))
		for _, item := range typed {
			value, err := normalizeFalFileValue(item)
			if err != nil {
				return nil, err
			}
			if str, ok := value.(string); ok && str != "" {
				results = append(results, str)
			}
		}
		return results, nil
	default:
		value, err := normalizeFalFileValue(typed)
		if err != nil {
			return nil, err
		}
		if str, ok := value.(string); ok && str != "" {
			return []string{str}, nil
		}
		return nil, nil
	}
}

func normalizeFalFileValue(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case string:
		return typed, nil
	case provider.ImageContent:
		return encodeFalContent(typed.URL, typed.Data, typed.MediaType)
	case *provider.ImageContent:
		if typed == nil {
			return nil, nil
		}
		return encodeFalContent(typed.URL, typed.Data, typed.MediaType)
	case provider.FileContent:
		return encodeFalContent(typed.URL, typed.Data, typed.MediaType)
	case *provider.FileContent:
		if typed == nil {
			return nil, nil
		}
		return encodeFalContent(typed.URL, typed.Data, typed.MediaType)
	default:
		return typed, nil
	}
}

func encodeFalContent(url string, data []byte, mediaType string) (string, error) {
	if url != "" {
		return url, nil
	}
	if len(data) == 0 {
		return "", provider.NewInvalidRequestError("fal file content missing data", nil)
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mediaType, encoded), nil
}

func newFalAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("fal api error (%d)", status)
	if parsed := parseFalErrorMessage(body); parsed != "" {
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

func parseFalErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
		Detail  []struct {
			Loc []string `json:"loc"`
			Msg string   `json:"msg"`
		} `json:"detail"`
		DetailMessage string `json:"detail"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if payload.Error != nil && payload.Error.Message != "" {
		return payload.Error.Message
	}
	if len(payload.Detail) > 0 {
		lines := make([]string, 0, len(payload.Detail))
		for _, detail := range payload.Detail {
			if detail.Msg == "" {
				continue
			}
			if len(detail.Loc) > 0 {
				lines = append(lines, strings.Join(detail.Loc, ".")+": "+detail.Msg)
			} else {
				lines = append(lines, detail.Msg)
			}
		}
		if len(lines) > 0 {
			return strings.Join(lines, "\n")
		}
	}
	if payload.DetailMessage != "" {
		return payload.DetailMessage
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
