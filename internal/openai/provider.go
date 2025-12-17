package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/bitop-dev/ai/internal/httpx"
	"github.com/bitop-dev/ai/internal/provider"
	"github.com/bitop-dev/ai/internal/sse"
	publicopenai "github.com/bitop-dev/ai/openai"
)

type Provider struct{}

func (p *Provider) Generate(ctx context.Context, req provider.Request) (provider.Response, error) {
	_, cfg, err := clientAndConfig(req.ProviderData)
	if err != nil {
		return provider.Response{}, &provider.Error{Provider: "openai", Code: "config_error", Message: err.Error(), Retryable: false, Cause: err}
	}

	payload, err := buildRequest(req, false)
	if err != nil {
		return provider.Response{}, &provider.Error{Provider: "openai", Code: "request_error", Message: err.Error(), Retryable: false, Cause: err}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return provider.Response{}, &provider.Error{Provider: "openai", Code: "marshal_error", Message: err.Error(), Retryable: false, Cause: err}
	}

	u, err := endpointURL(cfg)
	if err != nil {
		return provider.Response{}, &provider.Error{Provider: "openai", Code: "url_error", Message: err.Error(), Retryable: false, Cause: err}
	}

	h := make(http.Header)
	h.Set("Authorization", "Bearer "+cfg.APIKey)
	for k, v := range cfg.Headers {
		h.Set(k, v)
	}

	resp, err := httpx.DoJSON(ctx, cfg.HTTPClient, http.MethodPost, u, body, h, httpx.RetryPolicy{
		MaxRetries: cfg.MaxRetries,
		MinBackoff: cfg.MinBackoff,
		MaxBackoff: cfg.MaxBackoff,
	})
	if err != nil {
		code, retryable := classifyNetworkErr(err)
		return provider.Response{}, &provider.Error{Provider: "openai", Code: code, Message: err.Error(), Retryable: retryable, Cause: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var er errorResponse
		if json.Unmarshal(b, &er) == nil && er.Error.Message != "" {
			return provider.Response{}, &provider.Error{
				Provider:  "openai",
				Code:      stringifyCode(er.Error.Code, er.Error.Type),
				Status:    resp.StatusCode,
				Message:   er.Error.Message,
				Retryable: shouldRetryStatus(resp.StatusCode),
			}
		}
		return provider.Response{}, &provider.Error{
			Provider:  "openai",
			Code:      "http_error",
			Status:    resp.StatusCode,
			Message:   strings.TrimSpace(string(b)),
			Retryable: shouldRetryStatus(resp.StatusCode),
		}
	}

	var out chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return provider.Response{}, &provider.Error{Provider: "openai", Code: "decode_error", Message: err.Error(), Retryable: false, Cause: err}
	}
	if len(out.Choices) == 0 {
		return provider.Response{}, &provider.Error{Provider: "openai", Code: "invalid_response", Message: "response has no choices", Retryable: false}
	}
	c := out.Choices[0]

	msg, err := fromChatMessage(c.Message)
	if err != nil {
		return provider.Response{}, &provider.Error{Provider: "openai", Code: "invalid_response", Message: err.Error(), Retryable: false, Cause: err}
	}

	return provider.Response{
		Message: msg,
		Usage: provider.Usage{
			PromptTokens:     out.Usage.PromptTokens,
			CompletionTokens: out.Usage.CompletionTokens,
			TotalTokens:      out.Usage.TotalTokens,
		},
		FinishReason: provider.FinishReason(c.FinishReason),
	}, nil
}

func (p *Provider) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	_, cfg, err := clientAndConfig(req.ProviderData)
	if err != nil {
		return nil, &provider.Error{Provider: "openai", Code: "config_error", Message: err.Error(), Retryable: false, Cause: err}
	}

	payload, err := buildRequest(req, true)
	if err != nil {
		return nil, &provider.Error{Provider: "openai", Code: "request_error", Message: err.Error(), Retryable: false, Cause: err}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &provider.Error{Provider: "openai", Code: "marshal_error", Message: err.Error(), Retryable: false, Cause: err}
	}

	u, err := endpointURL(cfg)
	if err != nil {
		return nil, &provider.Error{Provider: "openai", Code: "url_error", Message: err.Error(), Retryable: false, Cause: err}
	}

	h := make(http.Header)
	h.Set("Authorization", "Bearer "+cfg.APIKey)
	h.Set("Accept", "text/event-stream")
	for k, v := range cfg.Headers {
		h.Set(k, v)
	}

	httpResp, err := httpx.DoJSON(ctx, cfg.HTTPClient, http.MethodPost, u, body, h, httpx.RetryPolicy{
		MaxRetries: cfg.MaxRetries,
		MinBackoff: cfg.MinBackoff,
		MaxBackoff: cfg.MaxBackoff,
	})
	if err != nil {
		code, retryable := classifyNetworkErr(err)
		return nil, &provider.Error{Provider: "openai", Code: code, Message: err.Error(), Retryable: retryable, Cause: err}
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode > 299 {
		defer httpResp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
		var er errorResponse
		if json.Unmarshal(b, &er) == nil && er.Error.Message != "" {
			return nil, &provider.Error{
				Provider:  "openai",
				Code:      stringifyCode(er.Error.Code, er.Error.Type),
				Status:    httpResp.StatusCode,
				Message:   er.Error.Message,
				Retryable: shouldRetryStatus(httpResp.StatusCode),
			}
		}
		return nil, &provider.Error{
			Provider:  "openai",
			Code:      "http_error",
			Status:    httpResp.StatusCode,
			Message:   strings.TrimSpace(string(b)),
			Retryable: shouldRetryStatus(httpResp.StatusCode),
		}
	}

	return newStream(httpResp, sse.NewDecoder(httpResp.Body)), nil
}

func clientAndConfig(providerData any) (*publicopenai.Client, publicopenai.Config, error) {
	c, ok := providerData.(*publicopenai.Client)
	if !ok || c == nil {
		return nil, publicopenai.Config{}, fmt.Errorf("openai provider requires a client-bound model ref")
	}
	cfg := c.Config()
	if cfg.APIKey == "" {
		return nil, publicopenai.Config{}, fmt.Errorf("openai API key is required")
	}
	return c, cfg, nil
}

func endpointURL(cfg publicopenai.Config) (string, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	prefix := strings.TrimRight(cfg.APIPrefix, "/")
	u, err := url.Parse(base + prefix + "/chat/completions")
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func buildRequest(req provider.Request, stream bool) (chatCompletionRequest, error) {
	msgs := make([]chatMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		cm, err := toChatMessage(m)
		if err != nil {
			return chatCompletionRequest{}, err
		}
		msgs = append(msgs, cm)
	}

	var tools []tool
	if len(req.Tools) > 0 {
		tools = make([]tool, 0, len(req.Tools))
		for _, t := range req.Tools {
			if t.Name == "" {
				return chatCompletionRequest{}, fmt.Errorf("tool name is required")
			}
			tools = append(tools, tool{
				Type: "function",
				Function: toolFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			})
		}
	}

	out := chatCompletionRequest{
		Model:       req.Model,
		Messages:    msgs,
		Tools:       tools,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        append([]string(nil), req.Stop...),
		Metadata:    req.Metadata,
		Stream:      stream,
	}
	if stream {
		out.StreamOptions = &streamOptions{IncludeUsage: true}
	}
	return out, nil
}

func toChatMessage(m provider.Message) (chatMessage, error) {
	role := string(m.Role)
	if role == "" {
		return chatMessage{}, fmt.Errorf("message role is required")
	}
	content, toolCalls, err := splitContentParts(m.Content)
	if err != nil {
		return chatMessage{}, err
	}

	var contentPtr *string
	if content != "" || len(toolCalls) == 0 {
		contentPtr = &content
	}

	cm := chatMessage{
		Role:      role,
		Content:   contentPtr,
		ToolCalls: toolCalls,
	}

	if m.Role == provider.RoleTool {
		if m.ToolCallID == "" {
			return chatMessage{}, fmt.Errorf("tool message missing ToolCallID")
		}
		cm.ToolCallID = m.ToolCallID
	}

	if m.Role == provider.RoleUser || m.Role == provider.RoleAssistant || m.Role == provider.RoleSystem {
		if m.Name != "" {
			cm.Name = m.Name
		}
	}
	return cm, nil
}

func splitContentParts(parts []provider.ContentPart) (string, []toolCall, error) {
	var b strings.Builder
	var toolCalls []toolCall

	for _, p := range parts {
		switch v := p.(type) {
		case provider.TextPart:
			b.WriteString(v.Text)
		case provider.ToolCallPart:
			toolCalls = append(toolCalls, toolCall{
				ID:   v.ID,
				Type: "function",
				Function: toolCallFn{
					Name:      v.Name,
					Arguments: string(v.Args),
				},
			})
		default:
			return "", nil, fmt.Errorf("unsupported content part %T", p)
		}
	}
	return b.String(), toolCalls, nil
}

func fromChatMessage(m chatMessage) (provider.Message, error) {
	role := provider.Role(m.Role)
	if role == "" {
		return provider.Message{}, fmt.Errorf("missing role")
	}
	var parts []provider.ContentPart
	if m.Content != nil && *m.Content != "" {
		parts = append(parts, provider.TextPart{Text: *m.Content})
	}
	for _, tc := range m.ToolCalls {
		if tc.Function.Name == "" {
			return provider.Message{}, fmt.Errorf("tool call missing name")
		}
		raw := json.RawMessage(tc.Function.Arguments)
		parts = append(parts, provider.ToolCallPart{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: raw,
		})
	}
	return provider.Message{
		Role:    role,
		Content: parts,
		Name:    m.Name,
	}, nil
}

var _ provider.Provider = (*Provider)(nil)

type stream struct {
	httpResp *http.Response
	dec      *sse.Decoder

	curDelta provider.Delta
	final    *provider.Response
	err      error

	// Aggregate final assistant message.
	textBuilder strings.Builder

	toolCallsByIndex map[int]*toolCallAgg
	finishReason     provider.FinishReason
	usage            provider.Usage
}

type toolCallAgg struct {
	id   string
	name string
	args strings.Builder
}

func newStream(httpResp *http.Response, dec *sse.Decoder) *stream {
	return &stream{
		httpResp:         httpResp,
		dec:              dec,
		toolCallsByIndex: map[int]*toolCallAgg{},
	}
}

func (s *stream) Next() bool {
	if s.err != nil || s.final != nil {
		return false
	}
	s.curDelta = provider.Delta{}

	for s.dec.Next() {
		data := s.dec.Data()
		data = bytes.TrimSpace(data)
		if len(data) == 0 {
			continue
		}
		if string(data) == "[DONE]" {
			s.finalize()
			return false
		}

		var chunk chatCompletionChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			var er errorResponse
			if json.Unmarshal(data, &er) == nil && er.Error.Message != "" {
				s.err = &provider.Error{
					Provider:  "openai",
					Code:      stringifyCode(er.Error.Code, er.Error.Type),
					Message:   er.Error.Message,
					Retryable: false,
				}
				return false
			}
			s.err = &provider.Error{Provider: "openai", Code: "decode_error", Message: err.Error(), Retryable: false, Cause: err}
			return false
		}

		if len(chunk.Choices) == 0 {
			continue
		}
		c := chunk.Choices[0]

		if c.Delta.Content != nil {
			s.textBuilder.WriteString(*c.Delta.Content)
			s.curDelta.Text = *c.Delta.Content
		}

		if len(c.Delta.ToolCalls) > 0 {
			for _, tc := range c.Delta.ToolCalls {
				agg, ok := s.toolCallsByIndex[tc.Index]
				if !ok {
					agg = &toolCallAgg{}
					s.toolCallsByIndex[tc.Index] = agg
				}
				if tc.ID != "" {
					agg.id = tc.ID
				}
				if tc.Function.Name != "" {
					agg.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					agg.args.WriteString(tc.Function.Arguments)
					s.curDelta.ToolCalls = append(s.curDelta.ToolCalls, provider.ToolCallDelta{
						Index:          tc.Index,
						ID:             tc.ID,
						Name:           tc.Function.Name,
						ArgumentsDelta: tc.Function.Arguments,
					})
				}
			}
		}

		if c.FinishReason != nil && *c.FinishReason != "" {
			s.finishReason = provider.FinishReason(*c.FinishReason)
		}
		if chunk.Usage != nil {
			s.usage = provider.Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}

		if s.curDelta.Text != "" || len(s.curDelta.ToolCalls) > 0 {
			return true
		}
	}

	if err := s.dec.Err(); err != nil {
		code, retryable := classifyNetworkErr(err)
		s.err = &provider.Error{Provider: "openai", Code: code, Message: err.Error(), Retryable: retryable, Cause: err}
	}
	s.finalize()
	return false
}

func shouldRetryStatus(status int) bool {
	return status == http.StatusRequestTimeout ||
		status == http.StatusConflict ||
		status == http.StatusTooManyRequests ||
		(status >= 500 && status <= 599)
}

func stringifyCode(code any, fallback string) string {
	switch v := code.(type) {
	case string:
		if v != "" {
			return v
		}
	}
	if fallback != "" {
		return fallback
	}
	return "unknown"
}

func classifyNetworkErr(err error) (code string, retryable bool) {
	if err == nil {
		return "network_error", false
	}
	if errors.Is(err, context.Canceled) {
		return "canceled", false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "timeout", true
	}
	return "network_error", true
}

func (s *stream) Delta() provider.Delta {
	return s.curDelta
}

func (s *stream) Final() *provider.Response {
	return s.final
}

func (s *stream) Err() error {
	return s.err
}

func (s *stream) Close() error {
	if s.httpResp != nil && s.httpResp.Body != nil {
		return s.httpResp.Body.Close()
	}
	return nil
}

func (s *stream) finalize() {
	if s.final != nil {
		return
	}

	var parts []provider.ContentPart
	if txt := s.textBuilder.String(); txt != "" {
		parts = append(parts, provider.TextPart{Text: txt})
	}

	// Add completed tool calls (if any) to the final message; args should be full JSON.
	if len(s.toolCallsByIndex) > 0 {
		indices := make([]int, 0, len(s.toolCallsByIndex))
		for i := range s.toolCallsByIndex {
			indices = append(indices, i)
		}
		sort.Ints(indices)
		for _, i := range indices {
			agg := s.toolCallsByIndex[i]
			if agg == nil || agg.name == "" {
				continue
			}
			raw := json.RawMessage(agg.args.String())
			parts = append(parts, provider.ToolCallPart{
				ID:   agg.id,
				Name: agg.name,
				Args: raw,
			})
		}
	}

	s.final = &provider.Response{
		Message: provider.Message{
			Role:    provider.RoleAssistant,
			Content: parts,
		},
		FinishReason: s.finishReason,
		Usage:        s.usage,
	}
}

var _ provider.Stream = (*stream)(nil)
