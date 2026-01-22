package gladia

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

const DefaultBaseURL = "https://api.gladia.io"
const DefaultProviderName = "gladia"

const defaultPollInterval = time.Second
const defaultPollTimeout = 60 * time.Second

// Settings configures the Gladia provider.
type Settings struct {
	APIKey       string
	BaseURL      string
	Headers      map[string]string
	HTTPClient   *http.Client
	ProviderName string
	PollInterval time.Duration
	PollTimeout  time.Duration
}

// Provider connects to Gladia APIs for transcription.
type Provider struct {
	apiKey       string
	baseURL      string
	headers      map[string]string
	httpClient   *http.Client
	providerID   provider.ProviderID
	pollInterval time.Duration
	pollTimeout  time.Duration
}

// CreateGladia constructs a Gladia provider.
func CreateGladia(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("GLADIA_API_KEY")
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
	return nil, provider.NewNoSuchModelError("gladia does not support language models", nil, p.providerID, modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("gladia does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return nil, provider.NewNoSuchModelError("gladia does not support image models", nil, p.providerID, modelID)
}

func (p *Provider) SpeechModel(modelID provider.ModelID) (provider.SpeechModelV3, error) {
	return nil, provider.NewNoSuchModelError("gladia does not support speech models", nil, p.providerID, modelID)
}

func (p *Provider) TranscriptionModel(modelID provider.ModelID) (provider.TranscriptionModelV3, error) {
	return &transcriptionModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("gladia api key is required")
	}
	headers := map[string]string{
		"x-gladia-key": p.apiKey,
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
		return provider.TranscriptionModelV3Result{}, provider.NewInvalidRequestError("gladia transcription audio is required", nil)
	}
	if options.MediaType == "" {
		return provider.TranscriptionModelV3Result{}, provider.NewInvalidRequestError("gladia transcription media type is required", nil)
	}
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)

	uploadURL, err := m.provider.uploadAudio(ctx, options.Audio, options.MediaType, requestOptions, m.modelID)
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	init, err := m.provider.submitTranscription(ctx, uploadURL, requestOptions, options.RequestOptions.ProviderOptions, m.modelID)
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	result, err := m.provider.awaitTranscription(ctx, init.ResultURL, requestOptions, m.modelID)
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	if err := validateGladiaResult(result); err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	return provider.TranscriptionModelV3Result{}, nil
}

type gladiaUploadResponse struct {
	AudioURL string `json:"audio_url"`
}

type gladiaInitResponse struct {
	ResultURL string `json:"result_url"`
}

type gladiaResultResponse struct {
	Status string        `json:"status"`
	Result *gladiaResult `json:"result"`
}

type gladiaResult struct {
	Metadata struct {
		AudioDuration float64 `json:"audio_duration"`
	} `json:"metadata"`
	Transcription struct {
		FullTranscript string            `json:"full_transcript"`
		Languages      []string          `json:"languages"`
		Utterances     []gladiaUtterance `json:"utterances"`
	} `json:"transcription"`
}

type gladiaUtterance struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

func (p *Provider) uploadAudio(ctx context.Context, audio []byte, mediaType string, options provider.RequestOptions, modelID provider.ModelID) (string, error) {
	fileName := "audio"
	if ext := mediaTypeToExtension(mediaType); ext != "" {
		fileName = fileName + "." + ext
	}
	payload := providerutils.MultipartPayload{
		Fields: nil,
		Files: []providerutils.MultipartFile{
			{
				FieldName:   "audio",
				FileName:    fileName,
				ContentType: mediaType,
				Content:     audio,
			},
		},
	}
	response, err := providerutils.PostMultipart(ctx, p.httpClient, p.endpoint("/v2/upload"), payload, options, nil, nil)
	if err != nil {
		return "", err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return "", newGladiaAPIError(response.StatusCode, response.Body, response.Headers, p.providerID, modelID)
	}
	upload, err := parseGladiaUploadResponse(response.Body)
	if err != nil {
		return "", err
	}
	if upload.AudioURL == "" {
		return "", provider.NewInvalidResponseDataError("gladia upload response missing audio_url", nil)
	}
	return upload.AudioURL, nil
}

func (p *Provider) submitTranscription(ctx context.Context, uploadURL string, options provider.RequestOptions, providerOptions provider.ProviderOptions, modelID provider.ModelID) (gladiaInitResponse, error) {
	payload := map[string]any{
		"audio_url": uploadURL,
	}
	if gladiaOpts := gladiaOptions(providerOptions, p.providerID); gladiaOpts != nil {
		applyGladiaOptions(payload, gladiaOpts)
	}
	response, err := providerutils.PostJSON(ctx, p.httpClient, p.endpoint("/v2/pre-recorded"), payload, options, nil, nil)
	if err != nil {
		return gladiaInitResponse{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return gladiaInitResponse{}, newGladiaAPIError(response.StatusCode, response.Body, response.Headers, p.providerID, modelID)
	}
	initResponse, err := parseGladiaInitResponse(response.Body)
	if err != nil {
		return gladiaInitResponse{}, err
	}
	if initResponse.ResultURL == "" {
		return gladiaInitResponse{}, provider.NewInvalidResponseDataError("gladia transcription response missing result_url", nil)
	}
	return initResponse, nil
}

func (p *Provider) awaitTranscription(ctx context.Context, resultURL string, options provider.RequestOptions, modelID provider.ModelID) (gladiaResultResponse, error) {
	if resultURL == "" {
		return gladiaResultResponse{}, provider.NewInvalidResponseDataError("gladia transcription response missing result_url", nil)
	}
	start := time.Now()
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return gladiaResultResponse{}, err
		}
		if time.Since(start) > p.pollTimeoutValue() {
			return gladiaResultResponse{}, provider.NewInvalidResponseDataError("gladia transcription timed out", nil)
		}
		if attempt > 0 {
			if err := waitForPoll(ctx, p.pollIntervalValue()); err != nil {
				return gladiaResultResponse{}, err
			}
		}
		result, err := p.fetchTranscriptionResult(ctx, resultURL, options, modelID)
		if err != nil {
			return gladiaResultResponse{}, err
		}
		switch strings.ToLower(result.Status) {
		case "done":
			return result, nil
		case "error":
			return gladiaResultResponse{}, provider.NewInvalidResponseDataError("gladia transcription failed", nil)
		case "queued", "processing":
			continue
		case "":
			return gladiaResultResponse{}, provider.NewInvalidResponseDataError("gladia transcription response missing status", nil)
		default:
			continue
		}
	}
}

func (p *Provider) fetchTranscriptionResult(ctx context.Context, url string, options provider.RequestOptions, modelID provider.ModelID) (gladiaResultResponse, error) {
	req, cancel, err := providerutils.BuildRequest(ctx, http.MethodGet, url, nil, nil, options)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return gladiaResultResponse{}, err
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
		return gladiaResultResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return gladiaResultResponse{}, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return gladiaResultResponse{}, newGladiaAPIError(resp.StatusCode, body, resp.Header, p.providerID, modelID)
	}
	result, err := parseGladiaResultResponse(body)
	if err != nil {
		return gladiaResultResponse{}, err
	}
	return result, nil
}

func parseGladiaUploadResponse(body []byte) (gladiaUploadResponse, error) {
	if len(body) == 0 {
		return gladiaUploadResponse{}, provider.NewInvalidResponseDataError("gladia upload response empty", nil)
	}
	var payload gladiaUploadResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return gladiaUploadResponse{}, provider.NewInvalidResponseDataError("gladia upload response invalid", err)
	}
	return payload, nil
}

func parseGladiaInitResponse(body []byte) (gladiaInitResponse, error) {
	if len(body) == 0 {
		return gladiaInitResponse{}, provider.NewInvalidResponseDataError("gladia transcription response empty", nil)
	}
	var payload gladiaInitResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return gladiaInitResponse{}, provider.NewInvalidResponseDataError("gladia transcription response invalid", err)
	}
	return payload, nil
}

func parseGladiaResultResponse(body []byte) (gladiaResultResponse, error) {
	if len(body) == 0 {
		return gladiaResultResponse{}, provider.NewInvalidResponseDataError("gladia transcription result response empty", nil)
	}
	var payload gladiaResultResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return gladiaResultResponse{}, provider.NewInvalidResponseDataError("gladia transcription result response invalid", err)
	}
	return payload, nil
}

func validateGladiaResult(response gladiaResultResponse) error {
	if response.Result == nil {
		return provider.NewInvalidResponseDataError("gladia transcription result missing result", nil)
	}
	if response.Result.Transcription.FullTranscript == "" {
		return provider.NewInvalidResponseDataError("gladia transcription result missing transcript", nil)
	}
	return nil
}

func gladiaOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func applyGladiaOptions(payload map[string]any, options provider.JSONObject) {
	if options == nil {
		return
	}
	setOption := func(target string, keys ...string) {
		if value, ok := lookupOption(options, keys...); ok {
			payload[target] = value
		}
	}
	setOption("context_prompt", "contextPrompt", "context_prompt")
	setOption("custom_vocabulary", "customVocabulary", "custom_vocabulary")
	setOption("detect_language", "detectLanguage", "detect_language")
	setOption("enable_code_switching", "enableCodeSwitching", "enable_code_switching")
	setOption("language", "language")
	setOption("callback", "callback")
	setOption("subtitles", "subtitles")
	setOption("diarization", "diarization")
	setOption("translation", "translation")
	setOption("summarization", "summarization")
	setOption("moderation", "moderation")
	setOption("named_entity_recognition", "namedEntityRecognition", "named_entity_recognition")
	setOption("chapterization", "chapterization")
	setOption("name_consistency", "nameConsistency", "name_consistency")
	setOption("custom_spelling", "customSpelling", "custom_spelling")
	setOption("structured_data_extraction", "structuredDataExtraction", "structured_data_extraction")
	setOption("sentiment_analysis", "sentimentAnalysis", "sentiment_analysis")
	setOption("audio_to_llm", "audioToLlm", "audio_to_llm")
	setOption("custom_metadata", "customMetadata", "custom_metadata")
	setOption("sentences", "sentences")
	setOption("display_mode", "displayMode", "display_mode")
	setOption("punctuation_enhanced", "punctuationEnhanced", "punctuation_enhanced")

	if value, ok := lookupOption(options, "customVocabularyConfig", "custom_vocabulary_config"); ok {
		if converted, ok := convertCustomVocabularyConfig(value); ok {
			payload["custom_vocabulary_config"] = converted
		} else {
			payload["custom_vocabulary_config"] = value
		}
	}
	if value, ok := lookupOption(options, "codeSwitchingConfig", "code_switching_config"); ok {
		if converted, ok := convertCodeSwitchingConfig(value); ok {
			payload["code_switching_config"] = converted
		} else {
			payload["code_switching_config"] = value
		}
	}
	if value, ok := lookupOption(options, "callbackConfig", "callback_config"); ok {
		if converted, ok := convertCallbackConfig(value); ok {
			payload["callback_config"] = converted
		} else {
			payload["callback_config"] = value
		}
	}
	if value, ok := lookupOption(options, "subtitlesConfig", "subtitles_config"); ok {
		if converted, ok := convertSubtitlesConfig(value); ok {
			payload["subtitles_config"] = converted
		} else {
			payload["subtitles_config"] = value
		}
	}
	if value, ok := lookupOption(options, "diarizationConfig", "diarization_config"); ok {
		if converted, ok := convertDiarizationConfig(value); ok {
			payload["diarization_config"] = converted
		} else {
			payload["diarization_config"] = value
		}
	}
	if value, ok := lookupOption(options, "translationConfig", "translation_config"); ok {
		if converted, ok := convertTranslationConfig(value); ok {
			payload["translation_config"] = converted
		} else {
			payload["translation_config"] = value
		}
	}
	if value, ok := lookupOption(options, "summarizationConfig", "summarization_config"); ok {
		if converted, ok := convertSummarizationConfig(value); ok {
			payload["summarization_config"] = converted
		} else {
			payload["summarization_config"] = value
		}
	}
	if value, ok := lookupOption(options, "customSpellingConfig", "custom_spelling_config"); ok {
		if converted, ok := convertCustomSpellingConfig(value); ok {
			payload["custom_spelling_config"] = converted
		} else {
			payload["custom_spelling_config"] = value
		}
	}
	if value, ok := lookupOption(options, "structuredDataExtractionConfig", "structured_data_extraction_config"); ok {
		if converted, ok := convertStructuredDataExtractionConfig(value); ok {
			payload["structured_data_extraction_config"] = converted
		} else {
			payload["structured_data_extraction_config"] = value
		}
	}
	if value, ok := lookupOption(options, "audioToLlmConfig", "audio_to_llm_config"); ok {
		if converted, ok := convertAudioToLlmConfig(value); ok {
			payload["audio_to_llm_config"] = converted
		} else {
			payload["audio_to_llm_config"] = value
		}
	}
}

func convertCustomVocabularyConfig(value any) (map[string]any, bool) {
	cfg, ok := asMap(value)
	if !ok {
		return nil, false
	}
	converted := map[string]any{}
	if vocab, ok := lookupMapValue(cfg, "vocabulary"); ok {
		converted["vocabulary"] = convertVocabularyItems(vocab)
	}
	if val, ok := lookupMapValue(cfg, "defaultIntensity", "default_intensity"); ok {
		converted["default_intensity"] = val
	}
	if len(converted) == 0 {
		return nil, false
	}
	return converted, true
}

func convertCodeSwitchingConfig(value any) (map[string]any, bool) {
	cfg, ok := asMap(value)
	if !ok {
		return nil, false
	}
	converted := map[string]any{}
	if val, ok := lookupMapValue(cfg, "languages"); ok {
		converted["languages"] = val
	}
	if len(converted) == 0 {
		return nil, false
	}
	return converted, true
}

func convertCallbackConfig(value any) (map[string]any, bool) {
	cfg, ok := asMap(value)
	if !ok {
		return nil, false
	}
	converted := map[string]any{}
	if val, ok := lookupMapValue(cfg, "url"); ok {
		converted["url"] = val
	}
	if val, ok := lookupMapValue(cfg, "method"); ok {
		converted["method"] = val
	}
	if len(converted) == 0 {
		return nil, false
	}
	return converted, true
}

func convertSubtitlesConfig(value any) (map[string]any, bool) {
	cfg, ok := asMap(value)
	if !ok {
		return nil, false
	}
	converted := map[string]any{}
	if val, ok := lookupMapValue(cfg, "formats"); ok {
		converted["formats"] = val
	}
	if val, ok := lookupMapValue(cfg, "minimumDuration", "minimum_duration"); ok {
		converted["minimum_duration"] = val
	}
	if val, ok := lookupMapValue(cfg, "maximumDuration", "maximum_duration"); ok {
		converted["maximum_duration"] = val
	}
	if val, ok := lookupMapValue(cfg, "maximumCharactersPerRow", "maximum_characters_per_row"); ok {
		converted["maximum_characters_per_row"] = val
	}
	if val, ok := lookupMapValue(cfg, "maximumRowsPerCaption", "maximum_rows_per_caption"); ok {
		converted["maximum_rows_per_caption"] = val
	}
	if val, ok := lookupMapValue(cfg, "style"); ok {
		converted["style"] = val
	}
	if len(converted) == 0 {
		return nil, false
	}
	return converted, true
}

func convertDiarizationConfig(value any) (map[string]any, bool) {
	cfg, ok := asMap(value)
	if !ok {
		return nil, false
	}
	converted := map[string]any{}
	if val, ok := lookupMapValue(cfg, "numberOfSpeakers", "number_of_speakers"); ok {
		converted["number_of_speakers"] = val
	}
	if val, ok := lookupMapValue(cfg, "minSpeakers", "min_speakers"); ok {
		converted["min_speakers"] = val
	}
	if val, ok := lookupMapValue(cfg, "maxSpeakers", "max_speakers"); ok {
		converted["max_speakers"] = val
	}
	if val, ok := lookupMapValue(cfg, "enhanced"); ok {
		converted["enhanced"] = val
	}
	if len(converted) == 0 {
		return nil, false
	}
	return converted, true
}

func convertTranslationConfig(value any) (map[string]any, bool) {
	cfg, ok := asMap(value)
	if !ok {
		return nil, false
	}
	converted := map[string]any{}
	if val, ok := lookupMapValue(cfg, "targetLanguages", "target_languages"); ok {
		converted["target_languages"] = val
	}
	if val, ok := lookupMapValue(cfg, "model"); ok {
		converted["model"] = val
	}
	if val, ok := lookupMapValue(cfg, "matchOriginalUtterances", "match_original_utterances"); ok {
		converted["match_original_utterances"] = val
	}
	if len(converted) == 0 {
		return nil, false
	}
	return converted, true
}

func convertSummarizationConfig(value any) (map[string]any, bool) {
	cfg, ok := asMap(value)
	if !ok {
		return nil, false
	}
	converted := map[string]any{}
	if val, ok := lookupMapValue(cfg, "type"); ok {
		converted["type"] = val
	}
	if len(converted) == 0 {
		return nil, false
	}
	return converted, true
}

func convertCustomSpellingConfig(value any) (map[string]any, bool) {
	cfg, ok := asMap(value)
	if !ok {
		return nil, false
	}
	converted := map[string]any{}
	if val, ok := lookupMapValue(cfg, "spellingDictionary", "spelling_dictionary"); ok {
		converted["spelling_dictionary"] = val
	}
	if len(converted) == 0 {
		return nil, false
	}
	return converted, true
}

func convertStructuredDataExtractionConfig(value any) (map[string]any, bool) {
	cfg, ok := asMap(value)
	if !ok {
		return nil, false
	}
	converted := map[string]any{}
	if val, ok := lookupMapValue(cfg, "classes"); ok {
		converted["classes"] = val
	}
	if len(converted) == 0 {
		return nil, false
	}
	return converted, true
}

func convertAudioToLlmConfig(value any) (map[string]any, bool) {
	cfg, ok := asMap(value)
	if !ok {
		return nil, false
	}
	converted := map[string]any{}
	if val, ok := lookupMapValue(cfg, "prompts"); ok {
		converted["prompts"] = val
	}
	if len(converted) == 0 {
		return nil, false
	}
	return converted, true
}

func convertVocabularyItems(value any) any {
	switch typed := value.(type) {
	case []string:
		items := make([]any, len(typed))
		for i, item := range typed {
			items[i] = item
		}
		return items
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, convertVocabularyItem(item))
		}
		return items
	case provider.JSONArray:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, convertVocabularyItem(item))
		}
		return items
	default:
		return value
	}
}

func convertVocabularyItem(value any) any {
	if cfg, ok := asMap(value); ok {
		converted := map[string]any{}
		if val, ok := lookupMapValue(cfg, "value"); ok {
			converted["value"] = val
		}
		if val, ok := lookupMapValue(cfg, "intensity"); ok {
			converted["intensity"] = val
		}
		if val, ok := lookupMapValue(cfg, "pronunciations"); ok {
			converted["pronunciations"] = val
		}
		if val, ok := lookupMapValue(cfg, "language"); ok {
			converted["language"] = val
		}
		if len(converted) > 0 {
			return converted
		}
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

type gladiaErrorResponse struct {
	Error gladiaErrorDetail `json:"error"`
}

type gladiaErrorDetail struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func parseGladiaErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload gladiaErrorResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if payload.Error.Message != "" {
		return payload.Error.Message
	}
	return ""
}

func newGladiaAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("gladia api error (%d)", status)
	if parsed := parseGladiaErrorMessage(body); parsed != "" {
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
