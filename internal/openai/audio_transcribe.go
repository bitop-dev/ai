package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/bitop-dev/ai/internal/httpx"
	"github.com/bitop-dev/ai/internal/provider"
	publicopenai "github.com/bitop-dev/ai/openai"
)

type transcriptionVerboseJSON struct {
	Text     string   `json:"text"`
	Language string   `json:"language,omitempty"`
	Duration *float64 `json:"duration,omitempty"`
	Segments []struct {
		ID    int     `json:"id"`
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Text  string  `json:"text"`
	} `json:"segments,omitempty"`
}

func (p *Provider) Transcribe(ctx context.Context, req provider.TranscriptionRequest) (provider.TranscriptionResponse, error) {
	_, cfg, err := clientAndConfig(req.ProviderData)
	if err != nil {
		return provider.TranscriptionResponse{}, &provider.Error{Provider: "openai", Code: "config_error", Message: err.Error(), Retryable: false, Cause: err}
	}
	if req.Model == "" {
		return provider.TranscriptionResponse{}, &provider.Error{Provider: "openai", Code: "invalid_request", Message: "model is required", Retryable: false}
	}
	if len(req.AudioBytes) == 0 {
		return provider.TranscriptionResponse{}, &provider.Error{Provider: "openai", Code: "invalid_request", Message: "audio bytes are required", Retryable: false}
	}
	filename := req.Filename
	if filename == "" {
		filename = "audio"
	}

	var opts publicopenai.TranscriptionOptions
	if v, ok := req.ProviderOptions.(map[string]any); ok {
		if raw, ok := v["openai"]; ok {
			switch o := raw.(type) {
			case publicopenai.TranscriptionOptions:
				opts = o
			case *publicopenai.TranscriptionOptions:
				if o != nil {
					opts = *o
				}
			}
		}
	}
	if opts.ResponseFormat == "" {
		opts.ResponseFormat = "verbose_json"
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	_ = w.WriteField("model", req.Model)
	if opts.Prompt != "" {
		_ = w.WriteField("prompt", opts.Prompt)
	}
	if opts.Language != "" {
		_ = w.WriteField("language", opts.Language)
	}
	if opts.Temperature != nil {
		_ = w.WriteField("temperature", fmt.Sprintf("%g", *opts.Temperature))
	}
	if opts.ResponseFormat != "" {
		_ = w.WriteField("response_format", opts.ResponseFormat)
	}
	for _, g := range opts.TimestampGranularities {
		_ = w.WriteField("timestamp_granularities[]", g)
	}

	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return provider.TranscriptionResponse{}, &provider.Error{Provider: "openai", Code: "request_error", Message: err.Error(), Retryable: false, Cause: err}
	}
	if _, err := io.Copy(part, bytes.NewReader(req.AudioBytes)); err != nil {
		return provider.TranscriptionResponse{}, &provider.Error{Provider: "openai", Code: "request_error", Message: err.Error(), Retryable: false, Cause: err}
	}
	_ = w.Close()

	u, err := transcriptionsURL(cfg)
	if err != nil {
		return provider.TranscriptionResponse{}, &provider.Error{Provider: "openai", Code: "url_error", Message: err.Error(), Retryable: false, Cause: err}
	}

	h := make(http.Header)
	h.Set("Authorization", "Bearer "+cfg.APIKey)
	h.Set("Content-Type", w.FormDataContentType())
	for k, v := range cfg.Headers {
		h.Set(k, v)
	}
	for k, v := range req.Headers {
		h.Set(k, v)
	}

	maxRetries := cfg.MaxRetries
	if req.MaxRetries != nil {
		maxRetries = *req.MaxRetries
	}

	resp, err := httpx.Do(ctx, cfg.HTTPClient, http.MethodPost, u, body.Bytes(), h, httpx.RetryPolicy{
		MaxRetries: maxRetries,
		MinBackoff: cfg.MinBackoff,
		MaxBackoff: cfg.MaxBackoff,
	})
	if err != nil {
		code, retryable := classifyNetworkErr(err)
		return provider.TranscriptionResponse{}, &provider.Error{Provider: "openai", Code: code, Message: err.Error(), Retryable: retryable, Cause: err}
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return provider.TranscriptionResponse{}, &provider.Error{Provider: "openai", Code: "read_error", Message: err.Error(), Retryable: true, Cause: err}
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var er errorResponse
		if json.Unmarshal(rawBody, &er) == nil && er.Error.Message != "" {
			return provider.TranscriptionResponse{}, &provider.Error{
				Provider:  "openai",
				Code:      stringifyCode(er.Error.Code, er.Error.Type),
				Status:    resp.StatusCode,
				Message:   er.Error.Message,
				Retryable: shouldRetryStatus(resp.StatusCode),
			}
		}
		return provider.TranscriptionResponse{}, &provider.Error{
			Provider:  "openai",
			Code:      "http_error",
			Status:    resp.StatusCode,
			Message:   strings.TrimSpace(string(rawBody)),
			Retryable: shouldRetryStatus(resp.StatusCode),
		}
	}

	out := provider.TranscriptionResponse{RawResponse: rawBody}

	switch opts.ResponseFormat {
	case "text":
		out.Text = strings.TrimSpace(string(rawBody))
		return out, nil
	default:
		var v transcriptionVerboseJSON
		if err := json.Unmarshal(rawBody, &v); err != nil {
			return provider.TranscriptionResponse{}, &provider.Error{Provider: "openai", Code: "decode_error", Message: err.Error(), Retryable: false, Cause: err}
		}
		out.Text = v.Text
		out.Language = v.Language
		out.DurationInSeconds = v.Duration
		if len(v.Segments) > 0 {
			out.Segments = make([]provider.TranscriptSegment, len(v.Segments))
			for i, s := range v.Segments {
				out.Segments[i] = provider.TranscriptSegment{ID: s.ID, Start: s.Start, End: s.End, Text: s.Text}
			}
		}
		return out, nil
	}
}

func transcriptionsURL(cfg publicopenai.Config) (string, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	prefix := strings.TrimRight(cfg.APIPrefix, "/")
	u, err := url.Parse(base + prefix + "/audio/transcriptions")
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

var _ provider.TranscriptionProvider = (*Provider)(nil)
