package revai

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

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providerutils"
)

const DefaultBaseURL = "https://api.rev.ai"
const DefaultProviderName = "revai"

const defaultPollInterval = time.Second
const defaultPollTimeout = 60 * time.Second

// Settings configures the Rev.ai provider.
type Settings struct {
	APIKey       string
	BaseURL      string
	Headers      map[string]string
	HTTPClient   *http.Client
	ProviderName string
	PollInterval time.Duration
	PollTimeout  time.Duration
}

// Provider connects to Rev.ai APIs for transcription.
type Provider struct {
	apiKey       string
	baseURL      string
	headers      map[string]string
	httpClient   *http.Client
	providerID   provider.ProviderID
	pollInterval time.Duration
	pollTimeout  time.Duration
}

// CreateRevAI constructs a Rev.ai provider.
func CreateRevAI(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("REVAI_API_KEY")
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
	return nil, provider.NewNoSuchModelError("revai does not support language models", nil, p.providerID, modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("revai does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return nil, provider.NewNoSuchModelError("revai does not support image models", nil, p.providerID, modelID)
}

func (p *Provider) SpeechModel(modelID provider.ModelID) (provider.SpeechModelV3, error) {
	return nil, provider.NewNoSuchModelError("revai does not support speech models", nil, p.providerID, modelID)
}

func (p *Provider) TranscriptionModel(modelID provider.ModelID) (provider.TranscriptionModelV3, error) {
	return &transcriptionModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("revai api key is required")
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

func (p *Provider) pollIntervalValue() time.Duration {
	if p.pollInterval > 0 {
		return p.pollInterval
	}
	return defaultPollInterval
}

func (p *Provider) pollTimeoutValue() time.Duration {
	if p.pollTimeout > 0 {
		return p.pollTimeout
	}
	return defaultPollTimeout
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
		return provider.TranscriptionModelV3Result{}, provider.NewInvalidRequestError("revai transcription audio is required", nil)
	}
	if options.MediaType == "" {
		return provider.TranscriptionModelV3Result{}, provider.NewInvalidRequestError("revai transcription media type is required", nil)
	}
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)

	config := map[string]any{
		"transcriber": string(m.modelID),
	}
	if revaiOpts := revaiOptions(options.RequestOptions.ProviderOptions, m.provider.providerID); revaiOpts != nil {
		applyRevaiOptions(config, revaiOpts)
	}
	configBytes, err := json.Marshal(config)
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	fileName := "audio"
	if ext := mediaTypeToExtension(options.MediaType); ext != "" {
		fileName = fileName + "." + ext
	}
	payload := providerutils.MultipartPayload{
		Fields: map[string]string{"config": string(configBytes)},
		Files: []providerutils.MultipartFile{
			{
				FieldName:   "media",
				FileName:    fileName,
				ContentType: options.MediaType,
				Content:     options.Audio,
			},
		},
	}
	response, err := providerutils.PostMultipart(ctx, m.provider.httpClient, m.provider.endpoint("/speechtotext/v1/jobs"), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.TranscriptionModelV3Result{}, newRevAIAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	job, err := parseRevaiJobResponse(response.Body)
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	if job.ID == "" {
		return provider.TranscriptionModelV3Result{}, provider.NewInvalidResponseDataError("revai transcription response missing id", nil)
	}
	if job.Status == "failed" {
		return provider.TranscriptionModelV3Result{}, provider.NewInvalidResponseDataError("revai transcription failed", nil)
	}
	if job.Status != "transcribed" {
		job, err = m.provider.awaitTranscription(ctx, job.ID, requestOptions, m.modelID)
		if err != nil {
			return provider.TranscriptionModelV3Result{}, err
		}
	}
	if _, err := m.provider.fetchTranscript(ctx, job.ID, requestOptions, m.modelID); err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	return provider.TranscriptionModelV3Result{}, nil
}

type revaiJobResponse struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Language string `json:"language"`
}

type revaiTranscriptResponse struct {
	Monologues []revaiMonologue `json:"monologues"`
}

type revaiMonologue struct {
	Elements []revaiElement `json:"elements"`
}

type revaiElement struct {
	Type  string  `json:"type"`
	Value string  `json:"value"`
	Start float64 `json:"ts"`
	End   float64 `json:"end_ts"`
}

func (p *Provider) awaitTranscription(ctx context.Context, jobID string, options provider.RequestOptions, modelID provider.ModelID) (revaiJobResponse, error) {
	start := time.Now()
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return revaiJobResponse{}, err
		}
		if time.Since(start) > p.pollTimeoutValue() {
			return revaiJobResponse{}, provider.NewInvalidResponseDataError("revai transcription timed out", nil)
		}
		if attempt > 0 {
			if err := waitForPoll(ctx, p.pollIntervalValue()); err != nil {
				return revaiJobResponse{}, err
			}
		}
		job, err := p.fetchTranscriptionJob(ctx, jobID, options, modelID)
		if err != nil {
			return revaiJobResponse{}, err
		}
		switch strings.ToLower(job.Status) {
		case "transcribed":
			return job, nil
		case "failed":
			return revaiJobResponse{}, provider.NewInvalidResponseDataError("revai transcription failed", nil)
		case "queued", "in_progress", "processing":
			continue
		case "":
			return revaiJobResponse{}, provider.NewInvalidResponseDataError("revai transcription status missing", nil)
		default:
			continue
		}
	}
}

func (p *Provider) fetchTranscriptionJob(ctx context.Context, jobID string, options provider.RequestOptions, modelID provider.ModelID) (revaiJobResponse, error) {
	url := p.endpoint("/speechtotext/v1/jobs/" + jobID)
	request, cancel, err := providerutils.BuildRequest(ctx, http.MethodGet, url, nil, nil, options)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return revaiJobResponse{}, err
	}
	if cancel != nil {
		defer cancel()
	}
	client := p.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(request)
	if err != nil {
		return revaiJobResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return revaiJobResponse{}, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return revaiJobResponse{}, newRevAIAPIError(resp.StatusCode, body, resp.Header, p.providerID, modelID)
	}
	job, err := parseRevaiJobResponse(body)
	if err != nil {
		return revaiJobResponse{}, err
	}
	if job.ID == "" {
		job.ID = jobID
	}
	return job, nil
}

func (p *Provider) fetchTranscript(ctx context.Context, jobID string, options provider.RequestOptions, modelID provider.ModelID) (revaiTranscriptResponse, error) {
	url := p.endpoint("/speechtotext/v1/jobs/" + jobID + "/transcript")
	request, cancel, err := providerutils.BuildRequest(ctx, http.MethodGet, url, nil, nil, options)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return revaiTranscriptResponse{}, err
	}
	if cancel != nil {
		defer cancel()
	}
	client := p.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(request)
	if err != nil {
		return revaiTranscriptResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return revaiTranscriptResponse{}, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return revaiTranscriptResponse{}, newRevAIAPIError(resp.StatusCode, body, resp.Header, p.providerID, modelID)
	}
	transcript, err := parseRevaiTranscriptResponse(body)
	if err != nil {
		return revaiTranscriptResponse{}, err
	}
	return transcript, nil
}

func parseRevaiJobResponse(body []byte) (revaiJobResponse, error) {
	if len(body) == 0 {
		return revaiJobResponse{}, provider.NewInvalidResponseDataError("revai transcription response empty", nil)
	}
	var payload revaiJobResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return revaiJobResponse{}, provider.NewInvalidResponseDataError("revai transcription response invalid", err)
	}
	return payload, nil
}

func parseRevaiTranscriptResponse(body []byte) (revaiTranscriptResponse, error) {
	if len(body) == 0 {
		return revaiTranscriptResponse{}, provider.NewInvalidResponseDataError("revai transcription transcript response empty", nil)
	}
	var payload revaiTranscriptResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return revaiTranscriptResponse{}, provider.NewInvalidResponseDataError("revai transcription transcript response invalid", err)
	}
	return payload, nil
}

func applyRevaiOptions(payload map[string]any, options provider.JSONObject) {
	if options == nil {
		return
	}
	setOption := func(target string, keys ...string) {
		if value, ok := lookupOption(options, keys...); ok {
			payload[target] = value
		}
	}
	setOption("metadata", "metadata")
	if value, ok := lookupOption(options, "notificationConfig", "notification_config"); ok {
		payload["notification_config"] = convertNotificationConfig(value)
	}
	setOption("delete_after_seconds", "deleteAfterSeconds", "delete_after_seconds")
	setOption("verbatim", "verbatim")
	setOption("rush", "rush")
	setOption("test_mode", "testMode", "test_mode")
	if value, ok := lookupOption(options, "segmentsToTranscribe", "segments_to_transcribe"); ok {
		payload["segments_to_transcribe"] = value
	}
	if value, ok := lookupOption(options, "speakerNames", "speaker_names"); ok {
		payload["speaker_names"] = convertSpeakerNames(value)
	}
	setOption("skip_diarization", "skipDiarization", "skip_diarization")
	setOption("skip_postprocessing", "skipPostprocessing", "skip_postprocessing")
	setOption("skip_punctuation", "skipPunctuation", "skip_punctuation")
	setOption("remove_disfluencies", "removeDisfluencies", "remove_disfluencies")
	setOption("remove_atmospherics", "removeAtmospherics", "remove_atmospherics")
	setOption("filter_profanity", "filterProfanity", "filter_profanity")
	setOption("speaker_channels_count", "speakerChannelsCount", "speaker_channels_count")
	setOption("speakers_count", "speakersCount", "speakers_count")
	setOption("diarization_type", "diarizationType", "diarization_type")
	setOption("custom_vocabulary_id", "customVocabularyId", "custom_vocabulary_id")
	setOption("custom_vocabularies", "customVocabularies", "custom_vocabularies")
	setOption("strict_custom_vocabulary", "strictCustomVocabulary", "strict_custom_vocabulary")
	setOption("summarization_config", "summarizationConfig", "summarization_config")
	setOption("translation_config", "translationConfig", "translation_config")
	setOption("language", "language")
	setOption("forced_alignment", "forcedAlignment", "forced_alignment")
}

func convertNotificationConfig(value any) any {
	cfg, ok := asMap(value)
	if !ok {
		return value
	}
	converted := map[string]any{}
	if val, ok := lookupMapValue(cfg, "url"); ok {
		converted["url"] = val
	}
	if val, ok := lookupMapValue(cfg, "authHeaders", "auth_headers"); ok {
		converted["auth_headers"] = convertAuthHeaders(val)
	}
	if len(converted) == 0 {
		return value
	}
	return converted
}

func convertAuthHeaders(value any) any {
	cfg, ok := asMap(value)
	if !ok {
		return value
	}
	if val, ok := lookupMapValue(cfg, "Authorization"); ok {
		return map[string]any{"Authorization": val}
	}
	return value
}

func convertSpeakerNames(value any) any {
	switch typed := value.(type) {
	case []map[string]any:
		converted := make([]any, 0, len(typed))
		for _, item := range typed {
			converted = append(converted, convertSpeakerNameItem(item))
		}
		return converted
	case []provider.JSONObject:
		converted := make([]any, 0, len(typed))
		for _, item := range typed {
			converted = append(converted, convertSpeakerNameItem(item))
		}
		return converted
	case []any:
		converted := make([]any, 0, len(typed))
		for _, item := range typed {
			converted = append(converted, convertSpeakerNameItem(item))
		}
		return converted
	default:
		return value
	}
}

func convertSpeakerNameItem(value any) any {
	cfg, ok := asMap(value)
	if !ok {
		return value
	}
	converted := map[string]any{}
	if val, ok := lookupMapValue(cfg, "displayName", "display_name"); ok {
		converted["display_name"] = val
	}
	if len(converted) == 0 {
		return value
	}
	return converted
}

func revaiOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func asMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case provider.JSONObject:
		converted := make(map[string]any, len(typed))
		for key, val := range typed {
			converted[key] = val
		}
		return converted, true
	default:
		return nil, false
	}
}

func lookupMapValue(options map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := options[key]; ok {
			return value, true
		}
	}
	return nil, false
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

type revaiErrorResponse struct {
	Error revaiErrorDetail `json:"error"`
}

type revaiErrorDetail struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func parseRevaiErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload revaiErrorResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if payload.Error.Message != "" {
		return payload.Error.Message
	}
	return ""
}

func newRevAIAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("revai api error (%d)", status)
	if parsed := parseRevaiErrorMessage(body); parsed != "" {
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

func mediaTypeToExtension(mediaType string) string {
	_, subtype, ok := strings.Cut(strings.ToLower(mediaType), "/")
	if !ok {
		return ""
	}
	switch subtype {
	case "mpeg":
		return "mp3"
	case "x-wav":
		return "wav"
	case "opus":
		return "ogg"
	case "mp4", "x-m4a":
		return "m4a"
	default:
		return subtype
	}
}

var _ provider.ProviderV3 = (*Provider)(nil)
var _ provider.TranscriptionModelV3 = (*transcriptionModel)(nil)
