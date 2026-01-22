package assemblyai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

const DefaultBaseURL = "https://api.assemblyai.com"
const DefaultProviderName = "assemblyai"

const defaultPollInterval = 3 * time.Second

// Settings configures the AssemblyAI provider.
type Settings struct {
	APIKey       string
	BaseURL      string
	Headers      map[string]string
	HTTPClient   *http.Client
	ProviderName string
	PollInterval time.Duration
}

// Provider connects to AssemblyAI APIs for transcription.
type Provider struct {
	apiKey       string
	baseURL      string
	headers      map[string]string
	httpClient   *http.Client
	providerID   provider.ProviderID
	pollInterval time.Duration
}

// CreateAssemblyAI constructs an AssemblyAI provider.
func CreateAssemblyAI(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ASSEMBLYAI_API_KEY")
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
	}
}

func (p *Provider) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (p *Provider) LanguageModel(modelID provider.ModelID) (provider.LanguageModelV3, error) {
	return nil, provider.NewNoSuchModelError("assemblyai does not support language models", nil, p.providerID, modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("assemblyai does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return nil, provider.NewNoSuchModelError("assemblyai does not support image models", nil, p.providerID, modelID)
}

func (p *Provider) SpeechModel(modelID provider.ModelID) (provider.SpeechModelV3, error) {
	return nil, provider.NewNoSuchModelError("assemblyai does not support speech models", nil, p.providerID, modelID)
}

func (p *Provider) TranscriptionModel(modelID provider.ModelID) (provider.TranscriptionModelV3, error) {
	return &transcriptionModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("assemblyai api key is required")
	}
	headers := map[string]string{
		"Authorization": p.apiKey,
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
		return provider.TranscriptionModelV3Result{}, provider.NewInvalidRequestError("assemblyai transcription audio is required", nil)
	}
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)

	uploadURL, err := m.provider.uploadAudio(ctx, options.Audio, requestOptions, m.modelID)
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	transcript, err := m.provider.submitTranscription(ctx, uploadURL, m.modelID, requestOptions, options.RequestOptions.ProviderOptions)
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	if transcript.Status == "completed" {
		return provider.TranscriptionModelV3Result{}, nil
	}
	if transcript.Status == "error" {
		return provider.TranscriptionModelV3Result{}, provider.NewInvalidResponseDataError(assemblyaiTranscriptionErrorMessage(transcript.Error), nil)
	}
	if err := m.provider.awaitTranscription(ctx, transcript.ID, requestOptions, m.modelID); err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	return provider.TranscriptionModelV3Result{}, nil
}

type assemblyaiUploadResponse struct {
	UploadURL string `json:"upload_url"`
}

type assemblyaiTranscriptResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

func (p *Provider) uploadAudio(ctx context.Context, audio []byte, options provider.RequestOptions, modelID provider.ModelID) (string, error) {
	url := p.endpoint("/v2/upload")
	headers := map[string]string{"Content-Type": "application/octet-stream"}
	request, cancel, err := providerutils.BuildRequest(ctx, http.MethodPost, url, bytes.NewReader(audio), headers, options)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return "", err
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
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return "", newAssemblyAIAPIError(resp.StatusCode, body, resp.Header, p.providerID, modelID)
	}
	upload, err := parseAssemblyAIUploadResponse(body)
	if err != nil {
		return "", err
	}
	if upload.UploadURL == "" {
		return "", provider.NewInvalidResponseDataError("assemblyai upload response missing upload_url", nil)
	}
	return upload.UploadURL, nil
}

func (p *Provider) submitTranscription(ctx context.Context, uploadURL string, modelID provider.ModelID, options provider.RequestOptions, providerOptions provider.ProviderOptions) (assemblyaiTranscriptResponse, error) {
	payload := map[string]any{
		"audio_url":    uploadURL,
		"speech_model": string(modelID),
	}
	if assemblyaiOpts := assemblyaiOptions(providerOptions, p.providerID); assemblyaiOpts != nil {
		applyAssemblyAITranscriptionOptions(payload, assemblyaiOpts)
	}
	response, err := providerutils.PostJSON(ctx, p.httpClient, p.endpoint("/v2/transcript"), payload, options, nil, nil)
	if err != nil {
		return assemblyaiTranscriptResponse{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return assemblyaiTranscriptResponse{}, newAssemblyAIAPIError(response.StatusCode, response.Body, response.Headers, p.providerID, modelID)
	}
	transcript, err := parseAssemblyAITranscriptResponse(response.Body)
	if err != nil {
		return assemblyaiTranscriptResponse{}, err
	}
	if transcript.ID == "" {
		return assemblyaiTranscriptResponse{}, provider.NewInvalidResponseDataError("assemblyai transcript response missing id", nil)
	}
	if transcript.Status == "" {
		return assemblyaiTranscriptResponse{}, provider.NewInvalidResponseDataError("assemblyai transcript response missing status", nil)
	}
	return transcript, nil
}

func (p *Provider) awaitTranscription(ctx context.Context, transcriptID string, options provider.RequestOptions, modelID provider.ModelID) error {
	attempt := 0
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt > 0 {
			if err := waitForPoll(ctx, p.pollIntervalValue()); err != nil {
				return err
			}
		}
		attempt++
		if err := p.fetchTranscription(ctx, transcriptID, options, modelID); err != nil {
			if errors.Is(err, errAssemblyAITranscriptPending) {
				continue
			}
			return err
		}
		return nil
	}
}

var errAssemblyAITranscriptPending = errors.New("assemblyai transcript pending")

func (p *Provider) fetchTranscription(ctx context.Context, transcriptID string, options provider.RequestOptions, modelID provider.ModelID) error {
	url := p.endpoint("/v2/transcript/" + transcriptID)
	request, cancel, err := providerutils.BuildRequest(ctx, http.MethodGet, url, nil, nil, options)
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
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return newAssemblyAIAPIError(resp.StatusCode, body, resp.Header, p.providerID, modelID)
	}
	transcript, err := parseAssemblyAITranscriptResponse(body)
	if err != nil {
		return err
	}
	if transcript.Status == "completed" {
		return nil
	}
	if transcript.Status == "error" {
		return provider.NewInvalidResponseDataError(assemblyaiTranscriptionErrorMessage(transcript.Error), nil)
	}
	if transcript.Status == "" {
		return provider.NewInvalidResponseDataError("assemblyai transcript response missing status", nil)
	}
	return errAssemblyAITranscriptPending
}

func parseAssemblyAIUploadResponse(body []byte) (assemblyaiUploadResponse, error) {
	if len(body) == 0 {
		return assemblyaiUploadResponse{}, provider.NewInvalidResponseDataError("assemblyai upload response empty", nil)
	}
	var payload assemblyaiUploadResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return assemblyaiUploadResponse{}, provider.NewInvalidResponseDataError("assemblyai upload response invalid", err)
	}
	return payload, nil
}

func parseAssemblyAITranscriptResponse(body []byte) (assemblyaiTranscriptResponse, error) {
	if len(body) == 0 {
		return assemblyaiTranscriptResponse{}, provider.NewInvalidResponseDataError("assemblyai transcript response empty", nil)
	}
	var payload assemblyaiTranscriptResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return assemblyaiTranscriptResponse{}, provider.NewInvalidResponseDataError("assemblyai transcript response invalid", err)
	}
	return payload, nil
}

func assemblyaiTranscriptionErrorMessage(message string) string {
	if message != "" {
		return "assemblyai transcription failed: " + message
	}
	return "assemblyai transcription failed"
}

func applyAssemblyAITranscriptionOptions(payload map[string]any, options provider.JSONObject) {
	for key, value := range options {
		switch key {
		case "audioEndAt", "audio_end_at":
			payload["audio_end_at"] = value
		case "audioStartFrom", "audio_start_from":
			payload["audio_start_from"] = value
		case "autoChapters", "auto_chapters":
			payload["auto_chapters"] = value
		case "autoHighlights", "auto_highlights":
			payload["auto_highlights"] = value
		case "boostParam", "boost_param":
			payload["boost_param"] = value
		case "contentSafety", "content_safety":
			payload["content_safety"] = value
		case "contentSafetyConfidence", "content_safety_confidence":
			payload["content_safety_confidence"] = value
		case "customSpelling", "custom_spelling":
			payload["custom_spelling"] = value
		case "disfluencies":
			payload["disfluencies"] = value
		case "entityDetection", "entity_detection":
			payload["entity_detection"] = value
		case "filterProfanity", "filter_profanity":
			payload["filter_profanity"] = value
		case "formatText", "format_text":
			payload["format_text"] = value
		case "iabCategories", "iab_categories":
			payload["iab_categories"] = value
		case "languageCode", "language_code":
			payload["language_code"] = value
		case "languageConfidenceThreshold", "language_confidence_threshold":
			payload["language_confidence_threshold"] = value
		case "languageDetection", "language_detection":
			payload["language_detection"] = value
		case "multichannel":
			payload["multichannel"] = value
		case "punctuate":
			payload["punctuate"] = value
		case "redactPii", "redact_pii":
			payload["redact_pii"] = value
		case "redactPiiAudio", "redact_pii_audio":
			payload["redact_pii_audio"] = value
		case "redactPiiAudioQuality", "redact_pii_audio_quality":
			payload["redact_pii_audio_quality"] = value
		case "redactPiiPolicies", "redact_pii_policies":
			payload["redact_pii_policies"] = value
		case "redactPiiSub", "redact_pii_sub":
			payload["redact_pii_sub"] = value
		case "sentimentAnalysis", "sentiment_analysis":
			payload["sentiment_analysis"] = value
		case "speakerLabels", "speaker_labels":
			payload["speaker_labels"] = value
		case "speakersExpected", "speakers_expected":
			payload["speakers_expected"] = value
		case "speechThreshold", "speech_threshold":
			payload["speech_threshold"] = value
		case "summarization":
			payload["summarization"] = value
		case "summaryModel", "summary_model":
			payload["summary_model"] = value
		case "summaryType", "summary_type":
			payload["summary_type"] = value
		case "webhookAuthHeaderName", "webhook_auth_header_name":
			payload["webhook_auth_header_name"] = value
		case "webhookAuthHeaderValue", "webhook_auth_header_value":
			payload["webhook_auth_header_value"] = value
		case "webhookUrl", "webhook_url":
			payload["webhook_url"] = value
		case "wordBoost", "word_boost":
			payload["word_boost"] = value
		}
	}
}

func assemblyaiOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

type assemblyaiErrorResponse struct {
	Error assemblyaiErrorDetail `json:"error"`
}

type assemblyaiErrorDetail struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func parseAssemblyAIErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload assemblyaiErrorResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if payload.Error.Message != "" {
		return payload.Error.Message
	}
	return ""
}

func newAssemblyAIAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("assemblyai api error (%d)", status)
	if parsed := parseAssemblyAIErrorMessage(body); parsed != "" {
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
