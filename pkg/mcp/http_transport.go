package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/vercel/ai-sdk-go/pkg/providerutils"
)

type HTTPConfig struct {
	URL               string
	Headers           map[string]string
	Client            *http.Client
	DisableInboundSSE bool
}

// HTTPTransport implements the MCP streamable HTTP transport using POST for
// requests and SSE for streaming responses when supported.
type HTTPTransport struct {
	config        HTTPConfig
	handlers      TransportHandlers
	client        *http.Client
	url           *url.URL
	sessionID     string
	ctx           context.Context
	cancel        context.CancelFunc
	inboundActive bool
	started       bool
	closing       bool
	mu            sync.Mutex
}

func NewHTTPTransport(config HTTPConfig) *HTTPTransport {
	return &HTTPTransport{config: config}
}

func (transport *HTTPTransport) Start(ctx context.Context) error {
	transport.mu.Lock()
	if transport.started {
		transport.mu.Unlock()
		return fmt.Errorf("mcp http transport already started")
	}
	if transport.config.URL == "" {
		transport.mu.Unlock()
		return fmt.Errorf("mcp http transport requires url")
	}
	parsedURL, err := url.Parse(transport.config.URL)
	if err != nil {
		transport.mu.Unlock()
		return err
	}
	client := transport.config.Client
	if client == nil {
		client = http.DefaultClient
	}
	startCtx, cancel := context.WithCancel(ctx)
	transport.client = client
	transport.url = parsedURL
	transport.ctx = startCtx
	transport.cancel = cancel
	transport.started = true
	transport.closing = false
	transport.mu.Unlock()

	if !transport.config.DisableInboundSSE {
		go transport.openInboundSSE(startCtx)
	}
	return nil
}

func (transport *HTTPTransport) Send(ctx context.Context, message Message) error {
	transport.mu.Lock()
	if !transport.started || transport.closing {
		transport.mu.Unlock()
		return fmt.Errorf("mcp http transport not connected")
	}
	client := transport.client
	targetURL := transport.url
	transport.mu.Unlock()

	payload, err := json.Marshal(message)
	if err != nil {
		return newResponseError("mcp http failed to encode message", err)
	}
	headers := transport.commonHeaders(map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json, text/event-stream",
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL.String(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	transport.captureSessionID(response)

	if response.StatusCode == http.StatusAccepted {
		transport.maybeStartInboundSSE()
		return nil
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(response.Body)
		message := fmt.Sprintf("mcp http request failed with status %d", response.StatusCode)
		if len(body) > 0 {
			message = fmt.Sprintf("mcp http request failed with status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
		}
		return newResponseError(message, nil)
	}

	contentType := response.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		messages, err := parseJSONMessages(body)
		if err != nil {
			return err
		}
		for _, parsed := range messages {
			transport.dispatchMessage(parsed)
		}
		return nil
	}

	if strings.Contains(contentType, "text/event-stream") {
		return transport.handleSSE(ctx, response.Body)
	}

	return newResponseError(fmt.Sprintf("mcp http unexpected content type: %s", contentType), nil)
}

func (transport *HTTPTransport) Close() error {
	transport.mu.Lock()
	if !transport.started || transport.closing {
		transport.mu.Unlock()
		return nil
	}
	transport.closing = true
	cancel := transport.cancel
	sessionID := transport.sessionID
	client := transport.client
	targetURL := transport.url
	transport.started = false
	transport.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if sessionID != "" && client != nil && targetURL != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		request, err := http.NewRequestWithContext(ctx, http.MethodDelete, targetURL.String(), nil)
		if err == nil {
			headers := transport.commonHeaders(map[string]string{})
			for key, value := range headers {
				request.Header.Set(key, value)
			}
			_, _ = client.Do(request)
		}
		cancel()
	}
	if transport.handlers.OnClose != nil {
		transport.handlers.OnClose()
	}
	return nil
}

func (transport *HTTPTransport) SetHandlers(handlers TransportHandlers) {
	transport.handlers = handlers
}

func (transport *HTTPTransport) openInboundSSE(ctx context.Context) {
	transport.mu.Lock()
	if transport.closing || transport.inboundActive {
		transport.mu.Unlock()
		return
	}
	client := transport.client
	targetURL := transport.url
	transport.inboundActive = true
	transport.mu.Unlock()
	defer func() {
		transport.mu.Lock()
		transport.inboundActive = false
		transport.mu.Unlock()
	}()

	headers := transport.commonHeaders(map[string]string{"Accept": "text/event-stream"})
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL.String(), nil)
	if err != nil {
		transport.dispatchError(err)
		return
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		transport.dispatchError(err)
		return
	}
	defer response.Body.Close()
	transport.captureSessionID(response)

	if response.StatusCode == http.StatusMethodNotAllowed || response.StatusCode == http.StatusNotFound {
		return
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		transport.dispatchError(fmt.Errorf("mcp http sse failed with status %d", response.StatusCode))
		return
	}
	if !strings.Contains(response.Header.Get("Content-Type"), "text/event-stream") {
		transport.dispatchError(newResponseError("mcp http sse unexpected content type", nil))
		return
	}

	if err := transport.handleSSE(ctx, response.Body); err != nil {
		if ctx.Err() != nil {
			return
		}
		transport.dispatchError(err)
	}
}

func (transport *HTTPTransport) handleSSE(ctx context.Context, body io.Reader) error {
	return providerutils.ParseSSE(ctx, body, providerutils.SSEParseOptions{
		OnEvent: func(event providerutils.SSEEvent) error {
			if event.Event != "" && event.Event != "message" {
				return nil
			}
			if event.Data == "" {
				return nil
			}
			message, err := deserializeMessage([]byte(event.Data))
			if err != nil {
				transport.dispatchError(err)
				return nil
			}
			transport.dispatchMessage(message)
			return nil
		},
	})
}

func (transport *HTTPTransport) commonHeaders(base map[string]string) map[string]string {
	headers := map[string]string{}
	for key, value := range transport.config.Headers {
		headers[key] = value
	}
	for key, value := range base {
		headers[key] = value
	}
	headers["mcp-protocol-version"] = LatestProtocolVersion
	if transport.sessionID != "" {
		headers["mcp-session-id"] = transport.sessionID
	}
	return headers
}

func (transport *HTTPTransport) captureSessionID(response *http.Response) {
	sessionID := response.Header.Get("mcp-session-id")
	if sessionID == "" {
		return
	}
	transport.mu.Lock()
	transport.sessionID = sessionID
	transport.mu.Unlock()
}

func (transport *HTTPTransport) dispatchMessage(message Message) {
	if transport.handlers.OnMessage != nil {
		transport.handlers.OnMessage(message)
	}
}

func (transport *HTTPTransport) dispatchError(err error) {
	if transport.handlers.OnError != nil {
		transport.handlers.OnError(err)
	}
}

func (transport *HTTPTransport) maybeStartInboundSSE() {
	transport.mu.Lock()
	if transport.config.DisableInboundSSE || transport.inboundActive || transport.ctx == nil || transport.closing {
		transport.mu.Unlock()
		return
	}
	startCtx := transport.ctx
	transport.mu.Unlock()
	go transport.openInboundSSE(startCtx)
}

func parseJSONMessages(payload []byte) ([]Message, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, newResponseError("mcp http empty response", nil)
	}
	if trimmed[0] == '[' {
		var messages []json.RawMessage
		if err := json.Unmarshal(trimmed, &messages); err != nil {
			return nil, newResponseError("mcp http failed to decode response", err)
		}
		results := make([]Message, 0, len(messages))
		for _, raw := range messages {
			parsed, err := deserializeMessage(raw)
			if err != nil {
				return nil, err
			}
			results = append(results, parsed)
		}
		return results, nil
	}

	var raw json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return nil, newResponseError("mcp http failed to decode response", err)
	}
	parsed, err := deserializeMessage(raw)
	if err != nil {
		return nil, err
	}
	return []Message{parsed}, nil
}
