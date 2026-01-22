package elevenlabs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

const DefaultBaseURL = "https://api.elevenlabs.io"
const DefaultProviderName = "elevenlabs"
const DefaultSpeechVoiceID = "21m00Tcm4TlvDq8ikWAM"
const DefaultSpeechOutputFormat = "mp3_44100_128"

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

func CreateElevenLabs(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ELEVENLABS_API_KEY")
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
	return nil, provider.NewNoSuchModelError("elevenlabs does not support language models", nil, p.providerID, modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("elevenlabs does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return nil, provider.NewNoSuchModelError("elevenlabs does not support image models", nil, p.providerID, modelID)
}

func (p *Provider) SpeechModel(modelID provider.ModelID) (provider.SpeechModelV3, error) {
	return &speechModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) TranscriptionModel(modelID provider.ModelID) (provider.TranscriptionModelV3, error) {
	return &transcriptionModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("elevenlabs api key is required")
	}
	headers := map[string]string{
		"xi-api-key": p.apiKey,
	}
	for key, value := range p.headers {
		headers[key] = value
	}
	return headers, nil
}

func (p *Provider) endpoint(path string) string {
	return p.baseURL + path
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
		return provider.SpeechModelV3Result{}, provider.NewInvalidRequestError("elevenlabs speech text is required", nil)
	}
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.SpeechModelV3Result{}, err
	}
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)

	voice := options.Voice
	if voice == "" {
		voice = DefaultSpeechVoiceID
	}
	outputFormat := options.OutputFormat
	if outputFormat == "" {
		outputFormat = DefaultSpeechOutputFormat
	}
	payload := map[string]any{
		"text":     options.Text,
		"model_id": string(m.modelID),
	}
	if options.Language != "" {
		payload["language_code"] = options.Language
	}
	voiceSettings := map[string]any{}
	if options.Speed != 0 {
		voiceSettings["speed"] = options.Speed
	}
	query := url.Values{}
	query.Set("output_format", mapElevenLabsOutputFormat(outputFormat))

	if elevenlabsOpts := elevenlabsOptions(options.RequestOptions.ProviderOptions, m.provider.providerID); elevenlabsOpts != nil {
		applyElevenLabsSpeechOptions(payload, voiceSettings, query, elevenlabsOpts)
	}

	if len(voiceSettings) > 0 {
		payload["voice_settings"] = voiceSettings
	}

	url := m.provider.endpoint("/v1/text-to-speech/" + voice)
	if encoded := query.Encode(); encoded != "" {
		url += "?" + encoded
	}
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, url, payload, requestOptions, nil, nil)
	if err != nil {
		return provider.SpeechModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.SpeechModelV3Result{}, newElevenLabsAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	if len(response.Body) == 0 {
		return provider.SpeechModelV3Result{}, provider.NewInvalidResponseDataError("elevenlabs speech response empty", nil)
	}
	return provider.SpeechModelV3Result{}, nil
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
		return provider.TranscriptionModelV3Result{}, provider.NewInvalidRequestError("elevenlabs transcription audio is required", nil)
	}
	if options.MediaType == "" {
		return provider.TranscriptionModelV3Result{}, provider.NewInvalidRequestError("elevenlabs transcription media type is required", nil)
	}
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)

	fields := map[string]string{
		"model_id": string(m.modelID),
		"diarize":  "true",
	}
	if elevenlabsOpts := elevenlabsOptions(options.RequestOptions.ProviderOptions, m.provider.providerID); elevenlabsOpts != nil {
		applyElevenLabsTranscriptionOptions(fields, elevenlabsOpts)
	}
	fileName := "audio"
	if ext := mediaTypeToExtension(options.MediaType); ext != "" {
		fileName = fileName + "." + ext
	}
	payload := providerutils.MultipartPayload{
		Fields: fields,
		Files: []providerutils.MultipartFile{
			{
				FieldName:   "file",
				FileName:    fileName,
				ContentType: options.MediaType,
				Content:     options.Audio,
			},
		},
	}
	response, err := providerutils.PostMultipart(ctx, m.provider.httpClient, m.provider.endpoint("/v1/speech-to-text"), payload, requestOptions, nil, nil)
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.TranscriptionModelV3Result{}, newElevenLabsAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	if err := parseElevenLabsTranscriptionResponse(response.Body); err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	return provider.TranscriptionModelV3Result{}, nil
}

type elevenlabsTranscriptionResponse struct {
	LanguageCode string `json:"language_code"`
	Text         string `json:"text"`
	Words        []struct {
		Text string `json:"text"`
	} `json:"words"`
}

func parseElevenLabsTranscriptionResponse(body []byte) error {
	if len(body) == 0 {
		return provider.NewInvalidResponseDataError("elevenlabs transcription response empty", nil)
	}
	var response elevenlabsTranscriptionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return provider.NewInvalidResponseDataError("elevenlabs transcription response invalid", err)
	}
	if response.Text == "" {
		return provider.NewInvalidResponseDataError("elevenlabs transcription response missing text", nil)
	}
	return nil
}

func applyElevenLabsSpeechOptions(payload map[string]any, voiceSettings map[string]any, query url.Values, options provider.JSONObject) {
	if options == nil {
		return
	}
	if payload["language_code"] == nil {
		if value, ok := lookupOption(options, "languageCode", "language_code"); ok {
			payload["language_code"] = value
		}
	}
	if value, ok := lookupOption(options, "seed"); ok {
		payload["seed"] = value
	}
	if value, ok := lookupOption(options, "previousText", "previous_text"); ok {
		payload["previous_text"] = value
	}
	if value, ok := lookupOption(options, "nextText", "next_text"); ok {
		payload["next_text"] = value
	}
	if value, ok := lookupOption(options, "previousRequestIds", "previous_request_ids"); ok {
		if slice := normalizeStringSlice(value); len(slice) > 0 {
			payload["previous_request_ids"] = slice
		}
	}
	if value, ok := lookupOption(options, "nextRequestIds", "next_request_ids"); ok {
		if slice := normalizeStringSlice(value); len(slice) > 0 {
			payload["next_request_ids"] = slice
		}
	}
	if value, ok := lookupOption(options, "applyTextNormalization", "apply_text_normalization"); ok {
		payload["apply_text_normalization"] = value
	}
	if value, ok := lookupOption(options, "applyLanguageTextNormalization", "apply_language_text_normalization"); ok {
		payload["apply_language_text_normalization"] = value
	}
	if value, ok := lookupOption(options, "pronunciationDictionaryLocators", "pronunciation_dictionary_locators"); ok {
		if locators := normalizePronunciationLocators(value); len(locators) > 0 {
			payload["pronunciation_dictionary_locators"] = locators
		}
	}
	if value, ok := lookupOption(options, "enableLogging", "enable_logging"); ok {
		query.Set("enable_logging", fmt.Sprint(value))
	}
	if value, ok := lookupOption(options, "voiceSettings", "voice_settings"); ok {
		if settings := normalizeVoiceSettings(value); len(settings) > 0 {
			for key, value := range settings {
				voiceSettings[key] = value
			}
		}
	}
}

func applyElevenLabsTranscriptionOptions(fields map[string]string, options provider.JSONObject) {
	if options == nil {
		return
	}
	if value, ok := lookupOption(options, "languageCode", "language_code"); ok {
		fields["language_code"] = fmt.Sprint(value)
	}
	if value, ok := lookupOption(options, "tagAudioEvents", "tag_audio_events"); ok {
		fields["tag_audio_events"] = fmt.Sprint(value)
	}
	if value, ok := lookupOption(options, "numSpeakers", "num_speakers"); ok {
		fields["num_speakers"] = fmt.Sprint(value)
	}
	if value, ok := lookupOption(options, "timestampsGranularity", "timestamps_granularity"); ok {
		fields["timestamps_granularity"] = fmt.Sprint(value)
	}
	if value, ok := lookupOption(options, "fileFormat", "file_format"); ok {
		fields["file_format"] = fmt.Sprint(value)
	}
	if value, ok := lookupOption(options, "diarize"); ok {
		fields["diarize"] = fmt.Sprint(value)
	}
}

func normalizeVoiceSettings(value any) map[string]any {
	settings := map[string]any{}
	options := normalizeJSONObject(value)
	if options == nil {
		return settings
	}
	if value, ok := lookupOption(options, "stability"); ok {
		settings["stability"] = value
	}
	if value, ok := lookupOption(options, "similarityBoost", "similarity_boost"); ok {
		settings["similarity_boost"] = value
	}
	if value, ok := lookupOption(options, "style"); ok {
		settings["style"] = value
	}
	if value, ok := lookupOption(options, "useSpeakerBoost", "use_speaker_boost"); ok {
		settings["use_speaker_boost"] = value
	}
	if value, ok := lookupOption(options, "speed"); ok {
		settings["speed"] = value
	}
	return settings
}

func normalizePronunciationLocators(value any) []map[string]any {
	options := normalizeJSONArray(value)
	if len(options) == 0 {
		return nil
	}
	locators := make([]map[string]any, 0, len(options))
	for _, item := range options {
		obj := normalizeJSONObject(item)
		if obj == nil {
			continue
		}
		locator := map[string]any{}
		if value, ok := lookupOption(obj, "pronunciationDictionaryId", "pronunciation_dictionary_id"); ok {
			locator["pronunciation_dictionary_id"] = value
		}
		if value, ok := lookupOption(obj, "versionId", "version_id"); ok {
			locator["version_id"] = value
		}
		if len(locator) > 0 {
			locators = append(locators, locator)
		}
	}
	return locators
}

func normalizeStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			values = append(values, fmt.Sprint(item))
		}
		return values
	default:
		return nil
	}
}

func normalizeJSONArray(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []provider.JSONObject:
		values := make([]any, 0, len(typed))
		for _, item := range typed {
			values = append(values, item)
		}
		return values
	default:
		return nil
	}
}

func normalizeJSONObject(value any) provider.JSONObject {
	switch typed := value.(type) {
	case provider.JSONObject:
		return typed
	case map[string]any:
		converted := make(provider.JSONObject, len(typed))
		for key, value := range typed {
			converted[key] = value
		}
		return converted
	default:
		return nil
	}
}

func lookupOption(options provider.JSONObject, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := options[key]; ok {
			return value, true
		}
	}
	return nil, false
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

func mapElevenLabsOutputFormat(format string) string {
	if format == "" {
		return ""
	}
	mapping := map[string]string{
		"mp3":       "mp3_44100_128",
		"mp3_32":    "mp3_44100_32",
		"mp3_64":    "mp3_44100_64",
		"mp3_96":    "mp3_44100_96",
		"mp3_128":   "mp3_44100_128",
		"mp3_192":   "mp3_44100_192",
		"pcm":       "pcm_44100",
		"pcm_16000": "pcm_16000",
		"pcm_22050": "pcm_22050",
		"pcm_24000": "pcm_24000",
		"pcm_44100": "pcm_44100",
		"ulaw":      "ulaw_8000",
	}
	if mapped, ok := mapping[format]; ok {
		return mapped
	}
	return format
}

func elevenlabsOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func newElevenLabsAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("elevenlabs api error (%d)", status)
	if parsed := parseElevenLabsErrorMessage(body); parsed != "" {
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

func parseElevenLabsErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if payload.Error.Message != "" {
		return payload.Error.Message
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
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}
