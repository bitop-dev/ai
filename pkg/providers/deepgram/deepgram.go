package deepgram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providerutils"
)

const DefaultBaseURL = "https://api.deepgram.com"
const DefaultProviderName = "deepgram"

// Settings configures the Deepgram provider.
type Settings struct {
	APIKey       string
	BaseURL      string
	Headers      map[string]string
	HTTPClient   *http.Client
	ProviderName string
}

// Provider connects to Deepgram APIs for speech and transcription.
type Provider struct {
	apiKey     string
	baseURL    string
	headers    map[string]string
	httpClient *http.Client
	providerID provider.ProviderID
}

// CreateDeepgram constructs a Deepgram provider.
func CreateDeepgram(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("DEEPGRAM_API_KEY")
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
	return nil, provider.NewNoSuchModelError("deepgram does not support language models", nil, p.providerID, modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("deepgram does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return nil, provider.NewNoSuchModelError("deepgram does not support image models", nil, p.providerID, modelID)
}

func (p *Provider) SpeechModel(modelID provider.ModelID) (provider.SpeechModelV3, error) {
	return &speechModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) TranscriptionModel(modelID provider.ModelID) (provider.TranscriptionModelV3, error) {
	return &transcriptionModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("deepgram api key is required")
	}
	headers := map[string]string{
		"Authorization": "Token " + p.apiKey,
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
		return provider.SpeechModelV3Result{}, provider.NewInvalidRequestError("deepgram speech text is required", nil)
	}
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.SpeechModelV3Result{}, err
	}
	requestOptions := mergeRequestOptions(options.RequestOptions, headers)
	payload := map[string]any{"text": options.Text}
	query := url.Values{}
	query.Set("model", string(m.modelID))
	if options.OutputFormat != "" {
		applyDeepgramSpeechOutputFormat(query, options.OutputFormat)
	}
	if deepgramOpts := deepgramOptions(options.RequestOptions.ProviderOptions, m.provider.providerID); deepgramOpts != nil {
		applyDeepgramSpeechOptions(query, deepgramOpts)
	}
	url := m.provider.endpoint("/v1/speak")
	if encoded := query.Encode(); encoded != "" {
		url += "?" + encoded
	}
	response, err := providerutils.PostJSON(ctx, m.provider.httpClient, url, payload, requestOptions, nil, nil)
	if err != nil {
		return provider.SpeechModelV3Result{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.SpeechModelV3Result{}, newDeepgramAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
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
		return provider.TranscriptionModelV3Result{}, provider.NewInvalidRequestError("deepgram transcription audio is required", nil)
	}
	if options.MediaType == "" {
		return provider.TranscriptionModelV3Result{}, provider.NewInvalidRequestError("deepgram transcription media type is required", nil)
	}
	headers, err := m.provider.requestHeaders()
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	query := url.Values{}
	query.Set("model", string(m.modelID))
	query.Set("diarize", "true")
	if deepgramOpts := deepgramOptions(options.RequestOptions.ProviderOptions, m.provider.providerID); deepgramOpts != nil {
		applyDeepgramTranscriptionOptions(query, deepgramOpts)
	}
	endpoint := m.provider.endpoint("/v1/listen")
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	baseHeaders := map[string]string{"Content-Type": options.MediaType}
	for key, value := range headers {
		baseHeaders[key] = value
	}
	req, cancel, err := providerutils.BuildRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(options.Audio), baseHeaders, options.RequestOptions)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return provider.TranscriptionModelV3Result{}, err
	}
	if cancel != nil {
		defer cancel()
	}

	client := m.provider.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return provider.TranscriptionModelV3Result{}, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return provider.TranscriptionModelV3Result{}, newDeepgramAPIError(resp.StatusCode, body, resp.Header, m.provider.providerID, m.modelID)
	}
	return provider.TranscriptionModelV3Result{}, nil
}

func applyDeepgramSpeechOutputFormat(query url.Values, format string) {
	format = strings.ToLower(format)
	switch format {
	case "mp3":
		query.Set("encoding", "mp3")
	case "wav":
		query.Set("encoding", "linear16")
		query.Set("container", "wav")
	case "linear16":
		query.Set("encoding", "linear16")
		query.Set("container", "wav")
	case "mulaw":
		query.Set("encoding", "mulaw")
		query.Set("container", "wav")
	case "alaw":
		query.Set("encoding", "alaw")
		query.Set("container", "wav")
	case "opus":
		query.Set("encoding", "opus")
		query.Set("container", "ogg")
	case "ogg":
		query.Set("encoding", "opus")
		query.Set("container", "ogg")
	case "flac":
		query.Set("encoding", "flac")
	case "aac":
		query.Set("encoding", "aac")
	case "pcm":
		query.Set("encoding", "linear16")
		query.Set("container", "none")
	}
}

func applyDeepgramSpeechOptions(query url.Values, options provider.JSONObject) {
	for key, value := range options {
		switch key {
		case "bitRate", "bit_rate":
			query.Set("bit_rate", stringifyValue(value))
		case "container":
			query.Set("container", stringifyValue(value))
		case "encoding":
			query.Set("encoding", stringifyValue(value))
		case "sampleRate", "sample_rate":
			query.Set("sample_rate", stringifyValue(value))
		case "callback":
			query.Set("callback", stringifyValue(value))
		case "callbackMethod", "callback_method":
			query.Set("callback_method", stringifyValue(value))
		case "mipOptOut", "mip_opt_out":
			query.Set("mip_opt_out", stringifyValue(value))
		case "tag":
			query.Set("tag", stringifySliceValue(value))
		}
	}
}

func applyDeepgramTranscriptionOptions(query url.Values, options provider.JSONObject) {
	for key, value := range options {
		switch key {
		case "language":
			query.Set("language", stringifyValue(value))
		case "detectLanguage", "detect_language":
			query.Set("detect_language", stringifyValue(value))
		case "smartFormat", "smart_format":
			query.Set("smart_format", stringifyValue(value))
		case "punctuate":
			query.Set("punctuate", stringifyValue(value))
		case "paragraphs":
			query.Set("paragraphs", stringifyValue(value))
		case "summarize":
			query.Set("summarize", stringifyValue(value))
		case "topics":
			query.Set("topics", stringifyValue(value))
		case "intents":
			query.Set("intents", stringifyValue(value))
		case "sentiment":
			query.Set("sentiment", stringifyValue(value))
		case "detectEntities", "detect_entities":
			query.Set("detect_entities", stringifyValue(value))
		case "redact":
			query.Set("redact", stringifySliceValue(value))
		case "replace":
			query.Set("replace", stringifyValue(value))
		case "search":
			query.Set("search", stringifyValue(value))
		case "keyterm":
			query.Set("keyterm", stringifyValue(value))
		case "diarize":
			query.Set("diarize", stringifyValue(value))
		case "utterances":
			query.Set("utterances", stringifyValue(value))
		case "uttSplit", "utt_split":
			query.Set("utt_split", stringifyValue(value))
		case "fillerWords", "filler_words":
			query.Set("filler_words", stringifyValue(value))
		}
	}
}

func deepgramOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
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

func stringifyValue(value any) string {
	return fmt.Sprint(value)
}

func stringifySliceValue(value any) string {
	switch typed := value.(type) {
	case []string:
		return strings.Join(typed, ",")
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			items = append(items, fmt.Sprint(item))
		}
		return strings.Join(items, ",")
	default:
		return fmt.Sprint(value)
	}
}

func newDeepgramAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("deepgram api error (%d)", status)
	if parsed := parseDeepgramErrorMessage(body); parsed != "" {
		message = parsed
	}
	requestID := headers.Get("dg-request-id")
	if requestID == "" {
		requestID = headers.Get("x-request-id")
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

type deepgramErrorResponse struct {
	Error deepgramErrorDetail `json:"error"`
}

type deepgramErrorDetail struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func parseDeepgramErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload deepgramErrorResponse
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
