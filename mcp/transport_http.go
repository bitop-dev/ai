package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bitop-dev/ai/internal/sse"
)

type HTTPTransport struct {
	URL     string
	Headers map[string]string

	// AuthProvider can provide (and refresh) an authorization header value
	// (e.g. OAuth bearer token). If set and the request does not already specify
	// Authorization, it will be added.
	AuthProvider interface {
		AuthorizationHeader(ctx context.Context) (string, error)
	}

	// HeaderProvider can add dynamic headers (e.g. refreshed auth tokens) per request.
	HeaderProvider func(ctx context.Context) (map[string]string, error)

	// Client defaults to a 60s timeout client when nil.
	Client *http.Client

	mu sync.Mutex

	// protocolVersion is sent via MCP-Protocol-Version header after initialization.
	protocolVersion string
	// sessionID is sent via Mcp-Session-Id header after initialization when provided by server.
	sessionID string
}

func (t *HTTPTransport) Call(ctx context.Context, req json.RawMessage) (json.RawMessage, error) {
	if t == nil || t.URL == "" {
		return nil, fmt.Errorf("mcp: http transport url is required")
	}
	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, t.URL, bytes.NewReader(req))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	// Streamable HTTP requires clients advertise both response types.
	r.Header.Set("Accept", "application/json, text/event-stream")

	t.mu.Lock()
	if t.protocolVersion != "" {
		r.Header.Set("MCP-Protocol-Version", t.protocolVersion)
	}
	if t.sessionID != "" {
		r.Header.Set("Mcp-Session-Id", t.sessionID)
	}
	t.mu.Unlock()

	for k, v := range t.Headers {
		if v != "" {
			r.Header.Set(k, v)
		}
	}
	if t.AuthProvider != nil && r.Header.Get("Authorization") == "" {
		v, err := t.AuthProvider.AuthorizationHeader(ctx)
		if err != nil {
			return nil, err
		}
		if v != "" {
			r.Header.Set("Authorization", v)
		}
	}
	if t.HeaderProvider != nil {
		h, err := t.HeaderProvider(ctx)
		if err != nil {
			return nil, err
		}
		for k, v := range h {
			if v != "" {
				r.Header.Set(k, v)
			}
		}
	}

	resp, err := client.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Capture session ID header if present.
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		t.mu.Lock()
		t.sessionID = sid
		t.mu.Unlock()
	}

	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "text/event-stream") {
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			b, _ := io.ReadAll(resp.Body)
			t.mu.Lock()
			sid := t.sessionID
			pv := t.protocolVersion
			t.mu.Unlock()
			return nil, &HTTPStatusError{
				Method:          http.MethodPost,
				URL:             t.URL,
				StatusCode:      resp.StatusCode,
				Body:            b,
				Headers:         resp.Header.Clone(),
				SessionID:       sid,
				ProtocolVersion: pv,
			}
		}
		return t.readSSEResponse(resp.Body, req)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 202 Accepted is valid for notifications/responses.
	if resp.StatusCode == http.StatusAccepted && len(body) == 0 {
		return json.RawMessage(`{"jsonrpc":"2.0","id":0,"result":{}}`), nil
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		t.mu.Lock()
		sid := t.sessionID
		pv := t.protocolVersion
		t.mu.Unlock()
		return nil, &HTTPStatusError{
			Method:          http.MethodPost,
			URL:             t.URL,
			StatusCode:      resp.StatusCode,
			Body:            body,
			Headers:         resp.Header.Clone(),
			SessionID:       sid,
			ProtocolVersion: pv,
		}
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("mcp: empty response body")
	}
	out := append(json.RawMessage(nil), body...)
	return out, nil
}

func (t *HTTPTransport) Close() error {
	// Attempt to terminate the session if supported by the server.
	t.mu.Lock()
	sid := t.sessionID
	pv := t.protocolVersion
	url := t.URL
	headers := cloneStringMap(t.Headers)
	headerProvider := t.HeaderProvider
	client := t.Client
	t.mu.Unlock()

	if sid == "" || url == "" {
		return nil
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	r, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil
	}
	r.Header.Set("Accept", "application/json")
	if pv != "" {
		r.Header.Set("MCP-Protocol-Version", pv)
	}
	r.Header.Set("Mcp-Session-Id", sid)
	for k, v := range headers {
		if v != "" {
			r.Header.Set(k, v)
		}
	}
	if t.AuthProvider != nil && r.Header.Get("Authorization") == "" {
		v, err := t.AuthProvider.AuthorizationHeader(ctx)
		if err == nil && v != "" {
			r.Header.Set("Authorization", v)
		}
	}
	if headerProvider != nil {
		h, err := headerProvider(ctx)
		if err == nil {
			for k, v := range h {
				if v != "" {
					r.Header.Set(k, v)
				}
			}
		}
	}

	resp, err := client.Do(r)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	// 405 is allowed by spec (server may not support explicit termination).
	if resp.StatusCode == http.StatusMethodNotAllowed {
		return nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return nil
	}
	return nil
}

func (t *HTTPTransport) SetProtocolVersion(v string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.protocolVersion = v
}

// OpenSSEStream opens a server-to-client event stream using HTTP GET.
// This is part of MCP Streamable HTTP transport.
func (t *HTTPTransport) OpenSSEStream(ctx context.Context) (io.ReadCloser, error) {
	if t == nil || t.URL == "" {
		return nil, fmt.Errorf("mcp: http transport url is required")
	}
	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 0} // let ctx control lifetime
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Accept", "text/event-stream")

	t.mu.Lock()
	if t.protocolVersion != "" {
		r.Header.Set("MCP-Protocol-Version", t.protocolVersion)
	}
	if t.sessionID != "" {
		r.Header.Set("Mcp-Session-Id", t.sessionID)
	}
	t.mu.Unlock()

	for k, v := range t.Headers {
		if v != "" {
			r.Header.Set(k, v)
		}
	}
	if t.AuthProvider != nil && r.Header.Get("Authorization") == "" {
		v, err := t.AuthProvider.AuthorizationHeader(ctx)
		if err != nil {
			return nil, err
		}
		if v != "" {
			r.Header.Set("Authorization", v)
		}
	}
	if t.HeaderProvider != nil {
		h, err := t.HeaderProvider(ctx)
		if err != nil {
			return nil, err
		}
		for k, v := range h {
			if v != "" {
				r.Header.Set(k, v)
			}
		}
	}

	resp, err := client.Do(r)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.mu.Lock()
		sid := t.sessionID
		pv := t.protocolVersion
		t.mu.Unlock()
		return nil, &HTTPStatusError{
			Method:          http.MethodGet,
			URL:             t.URL,
			StatusCode:      resp.StatusCode,
			Body:            b,
			Headers:         resp.Header.Clone(),
			SessionID:       sid,
			ProtocolVersion: pv,
		}
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.mu.Lock()
		sid := t.sessionID
		pv := t.protocolVersion
		t.mu.Unlock()
		return nil, &HTTPStatusError{
			Method:          http.MethodGet,
			URL:             t.URL,
			StatusCode:      resp.StatusCode,
			Body:            b,
			Headers:         resp.Header.Clone(),
			SessionID:       sid,
			ProtocolVersion: pv,
		}
	}

	// Capture session ID header if present.
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		t.mu.Lock()
		t.sessionID = sid
		t.mu.Unlock()
	}

	return resp.Body, nil
}

func (t *HTTPTransport) SessionID() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sessionID
}

func (t *HTTPTransport) ProtocolVersion() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.protocolVersion
}

func cloneStringMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (t *HTTPTransport) readSSEResponse(r io.Reader, req json.RawMessage) (json.RawMessage, error) {
	// Determine expected response id from request.
	var probe struct {
		ID *int64 `json:"id"`
	}
	_ = json.Unmarshal(req, &probe)

	dec := sse.NewDecoder(r)
	for dec.Next() {
		data := dec.Data()
		if len(data) == 0 {
			continue
		}
		// data payload is JSON-RPC message.
		var msg struct {
			ID *int64 `json:"id,omitempty"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if probe.ID != nil && msg.ID != nil && *msg.ID == *probe.ID {
			return append(json.RawMessage(nil), data...), nil
		}
		// Ignore other messages (requests/notifications) for now.
	}
	if err := dec.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("mcp: sse stream ended without response")
}
