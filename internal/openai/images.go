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

type imagesRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	N       int    `json:"n,omitempty"`
	Size    string `json:"size,omitempty"`
	Quality string `json:"quality,omitempty"`
	Style   string `json:"style,omitempty"`
	Seed    *int64 `json:"seed,omitempty"`

	// Prefer base64 so the SDK doesn't need to fetch URLs.
	ResponseFormat string `json:"response_format,omitempty"`
}

type imagesResponse struct {
	Created int64 `json:"created"`
	Data    []struct {
		B64JSON       string `json:"b64_json,omitempty"`
		URL           string `json:"url,omitempty"`
		RevisedPrompt string `json:"revised_prompt,omitempty"`
	} `json:"data"`
}

func (p *Provider) GenerateImage(ctx context.Context, req provider.GenerateImageRequest) (provider.GenerateImageResponse, error) {
	_, cfg, err := clientAndConfig(req.ProviderData)
	if err != nil {
		return provider.GenerateImageResponse{}, &provider.Error{Provider: "openai", Code: "config_error", Message: err.Error(), Retryable: false, Cause: err}
	}
	if req.Model == "" {
		return provider.GenerateImageResponse{}, &provider.Error{Provider: "openai", Code: "invalid_request", Message: "model is required", Retryable: false}
	}
	if req.Prompt == "" {
		return provider.GenerateImageResponse{}, &provider.Error{Provider: "openai", Code: "invalid_request", Message: "prompt is required", Retryable: false}
	}
	if req.AspectRatio != "" {
		// OpenAI images API uses size, not aspect ratio.
		return provider.GenerateImageResponse{}, &provider.Error{Provider: "openai", Code: "invalid_request", Message: "aspectRatio is not supported for OpenAI images; use size", Retryable: false}
	}

	var opts publicopenai.ImageOptions
	if v, ok := req.ProviderOptions.(map[string]any); ok {
		if raw, ok := v["openai"]; ok {
			switch o := raw.(type) {
			case publicopenai.ImageOptions:
				opts = o
			case *publicopenai.ImageOptions:
				if o != nil {
					opts = *o
				}
			}
		}
	}

	payload := imagesRequest{
		Model:          req.Model,
		Prompt:         req.Prompt,
		N:              req.N,
		Size:           req.Size,
		Quality:        opts.Quality,
		Style:          opts.Style,
		Seed:           req.Seed,
		ResponseFormat: "b64_json",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return provider.GenerateImageResponse{}, &provider.Error{Provider: "openai", Code: "marshal_error", Message: err.Error(), Retryable: false, Cause: err}
	}

	u, err := imagesURL(cfg)
	if err != nil {
		return provider.GenerateImageResponse{}, &provider.Error{Provider: "openai", Code: "url_error", Message: err.Error(), Retryable: false, Cause: err}
	}

	h := make(http.Header)
	h.Set("Authorization", "Bearer "+cfg.APIKey)
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

	resp, err := httpx.DoJSON(ctx, cfg.HTTPClient, http.MethodPost, u, body, h, httpx.RetryPolicy{
		MaxRetries: maxRetries,
		MinBackoff: cfg.MinBackoff,
		MaxBackoff: cfg.MaxBackoff,
	})
	if err != nil {
		code, retryable := classifyNetworkErr(err)
		return provider.GenerateImageResponse{}, &provider.Error{Provider: "openai", Code: code, Message: err.Error(), Retryable: retryable, Cause: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var er errorResponse
		if json.Unmarshal(b, &er) == nil && er.Error.Message != "" {
			return provider.GenerateImageResponse{}, &provider.Error{
				Provider:  "openai",
				Code:      stringifyCode(er.Error.Code, er.Error.Type),
				Status:    resp.StatusCode,
				Message:   er.Error.Message,
				Retryable: shouldRetryStatus(resp.StatusCode),
			}
		}
		return provider.GenerateImageResponse{}, &provider.Error{
			Provider:  "openai",
			Code:      "http_error",
			Status:    resp.StatusCode,
			Message:   strings.TrimSpace(string(b)),
			Retryable: shouldRetryStatus(resp.StatusCode),
		}
	}

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return provider.GenerateImageResponse{}, &provider.Error{Provider: "openai", Code: "read_error", Message: err.Error(), Retryable: true, Cause: err}
	}
	var out imagesResponse
	if err := json.Unmarshal(rawBody, &out); err != nil {
		return provider.GenerateImageResponse{}, &provider.Error{Provider: "openai", Code: "decode_error", Message: err.Error(), Retryable: false, Cause: err}
	}

	images := make([]provider.Image, 0, len(out.Data))
	openaiImagesMeta := make([]map[string]any, 0, len(out.Data))
	for _, d := range out.Data {
		if d.B64JSON == "" {
			continue
		}
		images = append(images, provider.Image{Base64: d.B64JSON, MediaType: "image/png"})
		openaiImagesMeta = append(openaiImagesMeta, map[string]any{"revisedPrompt": d.RevisedPrompt})
	}

	var md map[string]any
	if len(openaiImagesMeta) > 0 {
		md = map[string]any{
			"openai": map[string]any{
				"images": openaiImagesMeta,
			},
		}
	}

	return provider.GenerateImageResponse{
		N:                req.N,
		Images:           images,
		ProviderMetadata: md,
		RawResponse:      rawBody,
	}, nil
}

func imagesURL(cfg publicopenai.Config) (string, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	prefix := strings.TrimRight(cfg.APIPrefix, "/")
	u, err := url.Parse(base + prefix + "/images/generations")
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

var _ provider.ImageProvider = (*Provider)(nil)
