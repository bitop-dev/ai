package openai

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/bitop-dev/ai/internal/httpx"
	"github.com/bitop-dev/ai/internal/provider"
	publicopenai "github.com/bitop-dev/ai/openai"
)

type embeddingsRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	Metadata       any      `json:"metadata,omitempty"`
	Dimensions     *int     `json:"dimensions,omitempty"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
}

type embeddingsResponse struct {
	Data []struct {
		Embedding json.RawMessage `json:"embedding"`
		Index     int             `json:"index"`
		Object    string          `json:"object"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func (p *Provider) Embed(ctx context.Context, req provider.EmbeddingRequest) (provider.EmbeddingResponse, error) {
	_, cfg, err := clientAndConfig(req.ProviderData)
	if err != nil {
		return provider.EmbeddingResponse{}, &provider.Error{Provider: "openai", Code: "config_error", Message: err.Error(), Retryable: false, Cause: err}
	}
	if req.Model == "" {
		return provider.EmbeddingResponse{}, &provider.Error{Provider: "openai", Code: "invalid_request", Message: "model is required", Retryable: false}
	}
	if len(req.Inputs) == 0 {
		return provider.EmbeddingResponse{}, &provider.Error{Provider: "openai", Code: "invalid_request", Message: "inputs are required", Retryable: false}
	}

	var opts publicopenai.EmbeddingOptions
	if v, ok := req.ProviderOptions.(map[string]any); ok {
		if raw, ok := v["openai"]; ok {
			switch o := raw.(type) {
			case publicopenai.EmbeddingOptions:
				opts = o
			case *publicopenai.EmbeddingOptions:
				if o != nil {
					opts = *o
				}
			}
		}
	}
	if opts.EncodingFormat == "" {
		opts.EncodingFormat = "float"
	}

	body, err := json.Marshal(embeddingsRequest{
		Model:          req.Model,
		Input:          req.Inputs,
		Metadata:       req.Metadata,
		Dimensions:     opts.Dimensions,
		EncodingFormat: opts.EncodingFormat,
	})
	if err != nil {
		return provider.EmbeddingResponse{}, &provider.Error{Provider: "openai", Code: "marshal_error", Message: err.Error(), Retryable: false, Cause: err}
	}

	u, err := embeddingsURL(cfg)
	if err != nil {
		return provider.EmbeddingResponse{}, &provider.Error{Provider: "openai", Code: "url_error", Message: err.Error(), Retryable: false, Cause: err}
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
		return provider.EmbeddingResponse{}, &provider.Error{Provider: "openai", Code: code, Message: err.Error(), Retryable: retryable, Cause: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var er errorResponse
		if json.Unmarshal(b, &er) == nil && er.Error.Message != "" {
			return provider.EmbeddingResponse{}, &provider.Error{
				Provider:  "openai",
				Code:      stringifyCode(er.Error.Code, er.Error.Type),
				Status:    resp.StatusCode,
				Message:   er.Error.Message,
				Retryable: shouldRetryStatus(resp.StatusCode),
			}
		}
		return provider.EmbeddingResponse{}, &provider.Error{
			Provider:  "openai",
			Code:      "http_error",
			Status:    resp.StatusCode,
			Message:   strings.TrimSpace(string(b)),
			Retryable: shouldRetryStatus(resp.StatusCode),
		}
	}

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return provider.EmbeddingResponse{}, &provider.Error{Provider: "openai", Code: "read_error", Message: err.Error(), Retryable: true, Cause: err}
	}
	var out embeddingsResponse
	if err := json.Unmarshal(rawBody, &out); err != nil {
		return provider.EmbeddingResponse{}, &provider.Error{Provider: "openai", Code: "decode_error", Message: err.Error(), Retryable: false, Cause: err}
	}
	if len(out.Data) == 0 {
		return provider.EmbeddingResponse{}, &provider.Error{Provider: "openai", Code: "invalid_response", Message: "response has no embeddings", Retryable: false}
	}

	vectors := make([][]float32, 0, len(out.Data))
	for _, d := range out.Data {
		vec, err := parseEmbedding(d.Embedding)
		if err != nil {
			return provider.EmbeddingResponse{}, &provider.Error{Provider: "openai", Code: "decode_error", Message: err.Error(), Retryable: false, Cause: err}
		}
		vectors = append(vectors, vec)
	}

	return provider.EmbeddingResponse{
		Vectors: vectors,
		Usage: provider.Usage{
			PromptTokens: out.Usage.PromptTokens,
			TotalTokens:  out.Usage.TotalTokens,
		},
		RawResponse: rawBody,
	}, nil
}

func embeddingsURL(cfg publicopenai.Config) (string, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	prefix := strings.TrimRight(cfg.APIPrefix, "/")
	u, err := url.Parse(base + prefix + "/embeddings")
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

var _ provider.EmbeddingProvider = (*Provider)(nil)

func parseEmbedding(raw json.RawMessage) ([]float32, error) {
	// Float array form
	var floats []float64
	if err := json.Unmarshal(raw, &floats); err == nil {
		vec := make([]float32, len(floats))
		for i, x := range floats {
			vec[i] = float32(x)
		}
		return vec, nil
	}

	// Base64 form
	var b64 string
	if err := json.Unmarshal(raw, &b64); err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	if len(data)%4 != 0 {
		return nil, io.ErrUnexpectedEOF
	}
	vec := make([]float32, len(data)/4)
	for i := 0; i < len(vec); i++ {
		u := binary.LittleEndian.Uint32(data[i*4 : i*4+4])
		vec[i] = math.Float32frombits(u)
	}
	return vec, nil
}
