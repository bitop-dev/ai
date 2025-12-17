package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HTTPTransport struct {
	URL     string
	Headers map[string]string

	// Client defaults to a 60s timeout client when nil.
	Client *http.Client
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
	for k, v := range t.Headers {
		if v != "" {
			r.Header.Set(k, v)
		}
	}

	resp, err := client.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("mcp: http status %d: %s", resp.StatusCode, string(body))
	}

	// MCP servers return JSON-RPC responses.
	var out json.RawMessage
	out = append(out, body...)
	return out, nil
}

func (t *HTTPTransport) Close() error { return nil }
