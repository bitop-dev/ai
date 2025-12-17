package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/bitop-dev/ai/internal/httpx"
	"github.com/bitop-dev/ai/internal/provider"
	publicopenai "github.com/bitop-dev/ai/openai"
)

type speechRequest struct {
	Model  string   `json:"model"`
	Input  string   `json:"input"`
	Voice  string   `json:"voice"`
	Format string   `json:"format,omitempty"`
	Speed  *float32 `json:"speed,omitempty"`
}

func (p *Provider) GenerateSpeech(ctx context.Context, req provider.SpeechRequest) (provider.SpeechResponse, error) {
	_, cfg, err := clientAndConfig(req.ProviderData)
	if err != nil {
		return provider.SpeechResponse{}, &provider.Error{Provider: "openai", Code: "config_error", Message: err.Error(), Retryable: false, Cause: err}
	}
	if req.Model == "" {
		return provider.SpeechResponse{}, &provider.Error{Provider: "openai", Code: "invalid_request", Message: "model is required", Retryable: false}
	}
	if req.Text == "" {
		return provider.SpeechResponse{}, &provider.Error{Provider: "openai", Code: "invalid_request", Message: "text is required", Retryable: false}
	}
	if req.Voice == "" {
		return provider.SpeechResponse{}, &provider.Error{Provider: "openai", Code: "invalid_request", Message: "voice is required", Retryable: false}
	}

	var opts publicopenai.SpeechOptions
	if v, ok := req.ProviderOptions.(map[string]any); ok {
		if raw, ok := v["openai"]; ok {
			switch o := raw.(type) {
			case publicopenai.SpeechOptions:
				opts = o
			case *publicopenai.SpeechOptions:
				if o != nil {
					opts = *o
				}
			}
		}
	}
	if opts.Format == "" {
		opts.Format = "mp3"
	}

	body, err := json.Marshal(speechRequest{
		Model:  req.Model,
		Input:  req.Text,
		Voice:  req.Voice,
		Format: opts.Format,
		Speed:  opts.Speed,
	})
	if err != nil {
		return provider.SpeechResponse{}, &provider.Error{Provider: "openai", Code: "marshal_error", Message: err.Error(), Retryable: false, Cause: err}
	}

	u, err := speechURL(cfg)
	if err != nil {
		return provider.SpeechResponse{}, &provider.Error{Provider: "openai", Code: "url_error", Message: err.Error(), Retryable: false, Cause: err}
	}

	h := make(http.Header)
	h.Set("Authorization", "Bearer "+cfg.APIKey)
	h.Set("Content-Type", "application/json")
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

	resp, err := httpx.Do(ctx, cfg.HTTPClient, http.MethodPost, u, body, h, httpx.RetryPolicy{
		MaxRetries: maxRetries,
		MinBackoff: cfg.MinBackoff,
		MaxBackoff: cfg.MaxBackoff,
	})
	if err != nil {
		code, retryable := classifyNetworkErr(err)
		return provider.SpeechResponse{}, &provider.Error{Provider: "openai", Code: code, Message: err.Error(), Retryable: retryable, Cause: err}
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return provider.SpeechResponse{}, &provider.Error{Provider: "openai", Code: "read_error", Message: err.Error(), Retryable: true, Cause: err}
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var er errorResponse
		if json.Unmarshal(rawBody, &er) == nil && er.Error.Message != "" {
			return provider.SpeechResponse{}, &provider.Error{
				Provider:  "openai",
				Code:      stringifyCode(er.Error.Code, er.Error.Type),
				Status:    resp.StatusCode,
				Message:   er.Error.Message,
				Retryable: shouldRetryStatus(resp.StatusCode),
			}
		}
		return provider.SpeechResponse{}, &provider.Error{
			Provider:  "openai",
			Code:      "http_error",
			Status:    resp.StatusCode,
			Message:   strings.TrimSpace(string(rawBody)),
			Retryable: shouldRetryStatus(resp.StatusCode),
		}
	}

	mt := resp.Header.Get("Content-Type")
	return provider.SpeechResponse{
		AudioBytes:  rawBody,
		MediaType:   mt,
		RawResponse: rawBody,
	}, nil
}

func speechURL(cfg publicopenai.Config) (string, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	prefix := strings.TrimRight(cfg.APIPrefix, "/")
	u, err := url.Parse(base + prefix + "/audio/speech")
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

var _ provider.SpeechProvider = (*Provider)(nil)
