package amazonbedrock

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providerutils"
)

const (
	DefaultProviderName    = "amazon-bedrock"
	DefaultService         = "bedrock"
	DefaultBaseURLTemplate = "https://bedrock-runtime.%s.amazonaws.com"
)

type Settings struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Region          string
	BaseURL         string
	Headers         map[string]string
	HTTPClient      *http.Client
	ProviderName    string
	Profile         string
	CredentialsFile string
	Service         string
	Now             func() time.Time
}

type Provider struct {
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	region          string
	baseURL         string
	headers         map[string]string
	httpClient      *http.Client
	providerID      provider.ProviderID
	profile         string
	credentialsFile string
	service         string
	now             func() time.Time
}

func CreateAmazonBedrock(settings Settings) *Provider {
	region := settings.Region
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}

	baseURL := strings.TrimRight(settings.BaseURL, "/")
	providerName := settings.ProviderName
	if providerName == "" {
		providerName = DefaultProviderName
	}
	profile := settings.Profile
	if profile == "" {
		profile = os.Getenv("AWS_PROFILE")
	}
	credentialsFile := settings.CredentialsFile
	if credentialsFile == "" {
		credentialsFile = os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	}
	service := settings.Service
	if service == "" {
		service = DefaultService
	}
	now := settings.Now
	if now == nil {
		now = time.Now
	}

	return &Provider{
		accessKeyID:     settings.AccessKeyID,
		secretAccessKey: settings.SecretAccessKey,
		sessionToken:    settings.SessionToken,
		region:          region,
		baseURL:         baseURL,
		headers:         settings.Headers,
		httpClient:      settings.HTTPClient,
		providerID:      provider.ProviderID(providerName),
		profile:         profile,
		credentialsFile: credentialsFile,
		service:         service,
		now:             now,
	}
}

func (p *Provider) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (p *Provider) LanguageModel(modelID provider.ModelID) (provider.LanguageModelV3, error) {
	return &languageModel{provider: p, modelID: modelID}, nil
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewUnsupportedFunctionalityError("amazon bedrock does not support embedding models", nil, "embedding")
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return nil, provider.NewUnsupportedFunctionalityError("amazon bedrock does not support image models", nil, "image")
}

func (p *Provider) endpoint(path string) (string, error) {
	baseURL := p.baseURL
	if baseURL == "" {
		region, err := p.regionForRequest()
		if err != nil {
			return "", err
		}
		baseURL = fmt.Sprintf(DefaultBaseURLTemplate, region)
	}
	return baseURL + path, nil
}

func (p *Provider) regionForRequest() (string, error) {
	if p.region != "" {
		return p.region, nil
	}
	if region := os.Getenv("AWS_REGION"); region != "" {
		return region, nil
	}
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		return region, nil
	}
	return "", fmt.Errorf("amazon bedrock region is required")
}

func (p *Provider) credentials() (awsCredentials, error) {
	if p.accessKeyID != "" || p.secretAccessKey != "" {
		if p.accessKeyID == "" || p.secretAccessKey == "" {
			return awsCredentials{}, fmt.Errorf("amazon bedrock access key id and secret access key are required")
		}
		return awsCredentials{
			AccessKeyID:     p.accessKeyID,
			SecretAccessKey: p.secretAccessKey,
			SessionToken:    p.sessionToken,
		}, nil
	}

	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	if accessKey == "" {
		accessKey = os.Getenv("AWS_ACCESS_KEY")
	}
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if secretKey == "" {
		secretKey = os.Getenv("AWS_SECRET_KEY")
	}
	if accessKey != "" || secretKey != "" {
		if accessKey == "" || secretKey == "" {
			return awsCredentials{}, fmt.Errorf("amazon bedrock access key id and secret access key are required")
		}
		return awsCredentials{
			AccessKeyID:     accessKey,
			SecretAccessKey: secretKey,
			SessionToken:    os.Getenv("AWS_SESSION_TOKEN"),
		}, nil
	}

	return loadSharedCredentials(p.profile, p.credentialsFile)
}

func (p *Provider) signedRequest(ctx context.Context, method, url string, body []byte, options provider.RequestOptions) (*http.Request, context.CancelFunc, error) {
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}
	for key, value := range p.headers {
		headers[key] = value
	}
	req, cancel, err := providerutils.BuildRequest(ctx, method, url, bytes.NewReader(body), headers, options)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, nil, err
	}
	creds, err := p.credentials()
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, nil, err
	}
	region, err := p.regionForRequest()
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, nil, err
	}
	if err := signRequest(req, body, creds, region, p.service, p.now()); err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, nil, err
	}
	return req, cancel, nil
}

type languageModel struct {
	provider *Provider
	modelID  provider.ModelID
}

func (m *languageModel) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (m *languageModel) ProviderID() provider.ProviderID {
	return m.provider.providerID
}

func (m *languageModel) ModelID() provider.ModelID {
	return m.modelID
}

func (m *languageModel) SupportedURLs() provider.SupportedURLPatterns {
	return nil
}

func (m *languageModel) DoGenerate(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3GenerateResult, error) {
	payload, err := m.buildPayload(options)
	if err != nil {
		return provider.LanguageModelV3GenerateResult{}, err
	}
	endpoint, err := m.provider.endpoint(fmt.Sprintf("/model/%s/invoke", url.PathEscape(string(m.modelID))))
	if err != nil {
		return provider.LanguageModelV3GenerateResult{}, err
	}
	response, err := m.doRequest(ctx, endpoint, payload, options.RequestOptions)
	if err != nil {
		return provider.LanguageModelV3GenerateResult{}, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return provider.LanguageModelV3GenerateResult{}, newBedrockAPIError(response.StatusCode, response.Body, response.Headers, m.provider.providerID, m.modelID)
	}
	return provider.LanguageModelV3GenerateResult{}, nil
}

func (m *languageModel) DoStream(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3StreamResult, error) {
	payload, err := m.buildPayload(options)
	if err != nil {
		return provider.LanguageModelV3StreamResult{}, err
	}
	endpoint, err := m.provider.endpoint(fmt.Sprintf("/model/%s/invoke-with-response-stream", url.PathEscape(string(m.modelID))))
	if err != nil {
		return provider.LanguageModelV3StreamResult{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return provider.LanguageModelV3StreamResult{}, err
	}
	req, cancel, err := m.provider.signedRequest(ctx, http.MethodPost, endpoint, body, options.RequestOptions)
	if err != nil {
		return provider.LanguageModelV3StreamResult{}, err
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
		return provider.LanguageModelV3StreamResult{}, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return provider.LanguageModelV3StreamResult{}, newBedrockAPIError(resp.StatusCode, body, resp.Header, m.provider.providerID, m.modelID)
	}

	responseHeaders := cloneHeaders(resp.Header)
	responseMetadata := &provider.ResponseMetadata{
		RequestID:  responseRequestID(resp.Header),
		HTTPStatus: resp.StatusCode,
		Headers:    responseHeaders,
	}

	stream := make(chan provider.StreamPart)
	result := provider.LanguageModelV3StreamResult{
		Stream: stream,
		Request: &provider.LanguageModelV3Request{
			Body: payload,
		},
		Response: &provider.LanguageModelV3Response{Headers: responseHeaders},
	}

	state := &bedrockStreamState{includeRaw: options.IncludeRawChunks}

	go func() {
		defer close(stream)
		defer resp.Body.Close()
		stream <- provider.StreamPart{
			Type:        provider.StreamPartTypeStreamStart,
			StreamStart: &provider.StreamStart{ProviderID: m.provider.providerID, ModelID: m.modelID},
		}
		parseErr := parseBedrockStream(ctx, resp.Body, func(data string) error {
			if state.includeRaw {
				stream <- rawStreamPart(data)
			}
			return handleBedrockEvent(stream, state, data, responseMetadata)
		})
		if parseErr != nil && !errors.Is(parseErr, context.Canceled) && !errors.Is(parseErr, io.EOF) {
			stream <- provider.StreamPart{Type: provider.StreamPartTypeError, Error: &provider.StreamError{Err: parseErr}}
		}
	}()

	return result, nil
}

func (m *languageModel) doRequest(ctx context.Context, endpoint string, payload map[string]any, options provider.RequestOptions) (providerutils.HTTPResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return providerutils.HTTPResponse{}, err
	}
	req, cancel, err := m.provider.signedRequest(ctx, http.MethodPost, endpoint, body, options)
	if err != nil {
		return providerutils.HTTPResponse{}, err
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
		return providerutils.HTTPResponse{}, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return providerutils.HTTPResponse{}, err
	}
	return providerutils.HTTPResponse{StatusCode: resp.StatusCode, Headers: resp.Header.Clone(), Body: responseBody}, nil
}

func (m *languageModel) buildPayload(options provider.LanguageModelV3CallOptions) (map[string]any, error) {
	payload := map[string]any{
		"inputText": promptToText(options.Prompt),
	}
	config := map[string]any{}
	if options.MaxOutputTokens > 0 {
		config["maxTokenCount"] = options.MaxOutputTokens
	}
	if options.Temperature != 0 {
		config["temperature"] = options.Temperature
	}
	if options.TopP != 0 {
		config["topP"] = options.TopP
	}
	if options.TopK != 0 {
		config["topK"] = options.TopK
	}
	if len(options.StopSequences) > 0 {
		config["stopSequences"] = options.StopSequences
	}
	if len(config) > 0 {
		payload["textGenerationConfig"] = config
	}

	bedrockOpts := bedrockOptions(resolveProviderOptions(options.ProviderOptions, options.RequestOptions), m.provider.providerID)
	for key, value := range bedrockRequestOverrides(bedrockOpts) {
		payload[key] = value
	}

	return payload, nil
}

type bedrockStreamState struct {
	includeRaw  bool
	textStarted bool
	finishSent  bool
}

type bedrockStreamEvent struct {
	Chunk                     *bedrockStreamChunk     `json:"chunk,omitempty"`
	InternalServerException   *bedrockStreamException `json:"internalServerException,omitempty"`
	ModelStreamErrorException *bedrockStreamException `json:"modelStreamErrorException,omitempty"`
	ThrottlingException       *bedrockStreamException `json:"throttlingException,omitempty"`
	ValidationException       *bedrockStreamException `json:"validationException,omitempty"`
	AccessDeniedException     *bedrockStreamException `json:"accessDeniedException,omitempty"`
	ResourceNotFoundException *bedrockStreamException `json:"resourceNotFoundException,omitempty"`
}

type bedrockStreamChunk struct {
	Bytes string `json:"bytes"`
}

type bedrockStreamException struct {
	Message string `json:"message"`
}

type bedrockStreamPayload struct {
	OutputText       string        `json:"outputText,omitempty"`
	CompletionReason string        `json:"completionReason,omitempty"`
	StopReason       string        `json:"stop_reason,omitempty"`
	Usage            *bedrockUsage `json:"usage,omitempty"`
}

type bedrockUsage struct {
	InputTokens  int `json:"inputTokenCount,omitempty"`
	OutputTokens int `json:"outputTokenCount,omitempty"`
	TotalTokens  int `json:"totalTokenCount,omitempty"`
}

func parseBedrockStream(ctx context.Context, reader io.Reader, handle func(string) error) error {
	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
		if line == "" {
			continue
		}
		if err := handle(line); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func handleBedrockEvent(stream chan<- provider.StreamPart, state *bedrockStreamState, data string, metadata *provider.ResponseMetadata) error {
	var event bedrockStreamEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return err
	}
	if err := eventError(event); err != nil {
		return err
	}
	if event.Chunk == nil || event.Chunk.Bytes == "" {
		return nil
	}
	payloadBytes, err := base64.StdEncoding.DecodeString(event.Chunk.Bytes)
	if err != nil {
		return err
	}
	var payload bedrockStreamPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return err
	}
	if payload.OutputText != "" {
		emitText(stream, state, payload.OutputText)
	}
	reason := resolveFinishReason(payload.CompletionReason, payload.StopReason)
	if reason != "" && !state.finishSent {
		state.finishSent = true
		finish := provider.Finish{Reason: reason, Usage: usageFromBedrock(payload.Usage)}
		stream <- provider.StreamPart{Type: provider.StreamPartTypeFinish, Finish: &finish, ResponseMetadata: metadata}
	}
	return nil
}

func eventError(event bedrockStreamEvent) error {
	exceptions := []*bedrockStreamException{
		event.InternalServerException,
		event.ModelStreamErrorException,
		event.ThrottlingException,
		event.ValidationException,
		event.AccessDeniedException,
		event.ResourceNotFoundException,
	}
	for _, exception := range exceptions {
		if exception == nil {
			continue
		}
		if exception.Message != "" {
			return fmt.Errorf("amazon bedrock stream error: %s", exception.Message)
		}
		return fmt.Errorf("amazon bedrock stream error")
	}
	return nil
}

func emitText(stream chan<- provider.StreamPart, state *bedrockStreamState, text string) {
	if !state.textStarted {
		state.textStarted = true
		stream <- provider.StreamPart{Type: provider.StreamPartTypeTextStart, TextStart: &provider.TextStart{Text: text}}
		return
	}
	stream <- provider.StreamPart{Type: provider.StreamPartTypeTextDelta, TextDelta: &provider.TextDelta{Delta: text}}
}

func resolveFinishReason(primary, secondary string) provider.FinishReason {
	reason := strings.TrimSpace(primary)
	if reason == "" {
		reason = strings.TrimSpace(secondary)
	}
	if reason == "" {
		return ""
	}
	switch strings.ToUpper(reason) {
	case "STOP", "FINISH", "END_TURN":
		return provider.FinishReasonStop
	case "MAX_TOKENS", "LENGTH":
		return provider.FinishReasonLength
	case "CONTENT_FILTERED", "CONTENT_FILTER":
		return provider.FinishReasonContentFilter
	default:
		return provider.FinishReasonOther
	}
}

func usageFromBedrock(usage *bedrockUsage) *provider.LanguageModelUsage {
	if usage == nil {
		return nil
	}
	total := usage.TotalTokens
	if total == 0 {
		total = usage.InputTokens + usage.OutputTokens
	}
	if total == 0 {
		return nil
	}
	return &provider.LanguageModelUsage{
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      total,
	}
}

func promptToText(prompt provider.Prompt) string {
	var builder strings.Builder
	for _, message := range prompt.Messages {
		text := messageContentText(message)
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(text)
	}
	return builder.String()
}

func messageContentText(message provider.ModelMessage) string {
	var builder strings.Builder
	for _, part := range message.Content {
		text := contentPartText(part)
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(text)
	}
	return builder.String()
}

func contentPartText(part provider.ContentPart) string {
	switch typed := part.(type) {
	case provider.TextContent:
		return typed.Text
	case *provider.TextContent:
		if typed == nil {
			return ""
		}
		return typed.Text
	case provider.ToolResultContent:
		return stringifyJSONValue(typed.ToolResult.Result)
	case *provider.ToolResultContent:
		if typed == nil {
			return ""
		}
		return stringifyJSONValue(typed.ToolResult.Result)
	case provider.ReasoningContent:
		return typed.Text
	case *provider.ReasoningContent:
		if typed == nil {
			return ""
		}
		return typed.Text
	default:
		return ""
	}
}

func stringifyJSONValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		payload, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(payload)
	}
}

func resolveProviderOptions(explicit provider.ProviderOptions, requestOptions provider.RequestOptions) provider.ProviderOptions {
	if explicit != nil {
		return explicit
	}
	if requestOptions.ProviderOptions != nil {
		return requestOptions.ProviderOptions
	}
	return nil
}

func bedrockOptions(options provider.ProviderOptions, providerID provider.ProviderID) provider.JSONObject {
	if options == nil {
		return nil
	}
	if providerID == "" {
		providerID = DefaultProviderName
	}
	if value, ok := options[string(providerID)]; ok {
		return value
	}
	if value, ok := options["bedrock"]; ok {
		return value
	}
	if value, ok := options["amazon-bedrock"]; ok {
		return value
	}
	return nil
}

func bedrockRequestOverrides(options provider.JSONObject) map[string]any {
	overrides := map[string]any{}
	if options == nil {
		return overrides
	}
	for key, value := range options {
		if key == "request" {
			for requestKey, requestValue := range normalizeObject(value) {
				overrides[requestKey] = requestValue
			}
			continue
		}
		overrides[key] = value
	}
	return overrides
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

type awsCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

func loadSharedCredentials(profile, credentialsFile string) (awsCredentials, error) {
	path := credentialsFile
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return awsCredentials{}, fmt.Errorf("amazon bedrock credentials file is required")
		}
		path = filepathJoin(home, ".aws", "credentials")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return awsCredentials{}, fmt.Errorf("amazon bedrock credentials file is required")
	}
	if profile == "" {
		profile = "default"
	}
	creds := parseSharedCredentials(string(data), profile)
	if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
		return awsCredentials{}, fmt.Errorf("amazon bedrock credentials for profile %q are required", profile)
	}
	return creds, nil
}

func parseSharedCredentials(payload, profile string) awsCredentials {
	profile = strings.TrimSpace(profile)
	scanner := bufio.NewScanner(strings.NewReader(payload))
	current := ""
	creds := awsCredentials{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = strings.TrimSpace(line[1 : len(line)-1])
			if strings.HasPrefix(current, "profile ") {
				current = strings.TrimSpace(strings.TrimPrefix(current, "profile "))
			}
			continue
		}
		if current != profile {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		switch key {
		case "aws_access_key_id":
			creds.AccessKeyID = value
		case "aws_secret_access_key":
			creds.SecretAccessKey = value
		case "aws_session_token":
			creds.SessionToken = value
		}
	}
	return creds
}

func filepathJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	joined := path.Clean(strings.Join(parts, "/"))
	if strings.HasPrefix(parts[0], "/") {
		return "/" + strings.TrimPrefix(joined, "/")
	}
	return joined
}

func signRequest(req *http.Request, payload []byte, creds awsCredentials, region, service string, now time.Time) error {
	if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
		return fmt.Errorf("amazon bedrock credentials are required")
	}
	if region == "" {
		return fmt.Errorf("amazon bedrock region is required")
	}
	if service == "" {
		service = DefaultService
	}
	timestamp := now.UTC()
	amzDate := timestamp.Format("20060102T150405Z")
	date := timestamp.Format("20060102")

	if req.Header.Get("X-Amz-Date") == "" {
		req.Header.Set("X-Amz-Date", amzDate)
	} else {
		amzDate = req.Header.Get("X-Amz-Date")
		if len(amzDate) >= 8 {
			date = amzDate[:8]
		}
	}
	payloadHash := sha256Hex(payload)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	if creds.SessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", creds.SessionToken)
	}

	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := canonicalQueryString(req.URL)
	canonicalHeaders, signedHeaders := canonicalHeaders(req)
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", date, region, service)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := awsSigningKey(creds.SecretAccessKey, date, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	authorization := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		creds.AccessKeyID,
		credentialScope,
		signedHeaders,
		signature,
	)
	req.Header.Set("Authorization", authorization)
	return nil
}

func canonicalQueryString(u *url.URL) string {
	if u == nil {
		return ""
	}
	values := u.Query()
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var parts []string
	for _, key := range keys {
		vals := values[key]
		sort.Strings(vals)
		for _, value := range vals {
			parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(value))
		}
	}
	return strings.Join(parts, "&")
}

func canonicalHeaders(req *http.Request) (string, string) {
	headers := map[string][]string{}
	for key, values := range req.Header {
		if strings.EqualFold(key, "Authorization") {
			continue
		}
		lower := strings.ToLower(strings.TrimSpace(key))
		if lower == "" {
			continue
		}
		headers[lower] = append(headers[lower], values...)
	}
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	if host != "" {
		headers["host"] = []string{host}
	}

	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var canonical strings.Builder
	var signed []string
	for _, key := range keys {
		values := headers[key]
		normalized := make([]string, 0, len(values))
		for _, value := range values {
			normalized = append(normalized, normalizeHeaderValue(value))
		}
		canonical.WriteString(key)
		canonical.WriteString(":")
		canonical.WriteString(strings.Join(normalized, ","))
		canonical.WriteString("\n")
		signed = append(signed, key)
	}
	return canonical.String(), strings.Join(signed, ";")
}

func normalizeHeaderValue(value string) string {
	fields := strings.Fields(value)
	return strings.Join(fields, " ")
}

func awsSigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func responseRequestID(headers http.Header) string {
	if headers == nil {
		return ""
	}
	if requestID := headers.Get("x-amzn-requestid"); requestID != "" {
		return requestID
	}
	return headers.Get("x-amz-request-id")
}

func newBedrockAPIError(status int, body []byte, headers http.Header, providerID provider.ProviderID, modelID provider.ModelID) error {
	message := fmt.Sprintf("amazon bedrock api error (%d)", status)
	if len(body) > 0 {
		var payload struct {
			Message      string `json:"message"`
			ErrorMessage string `json:"errorMessage"`
			Error        struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			if payload.Message != "" {
				message = payload.Message
			} else if payload.ErrorMessage != "" {
				message = payload.ErrorMessage
			} else if payload.Error.Message != "" {
				message = payload.Error.Message
			}
		}
	}
	requestID := responseRequestID(headers)
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

func rawStreamPart(data string) provider.StreamPart {
	var payload any
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		payload = data
	}
	return provider.StreamPart{Type: provider.StreamPartTypeRaw, Raw: payload}
}
