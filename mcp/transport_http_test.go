package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPTransport_HTTPStatusErrorIncludesHeadersAndSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Mcp-Session-Id", "sess_123")
		w.Header().Set("X-Test", "1")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	tr := &HTTPTransport{URL: srv.URL}
	tr.SetProtocolVersion("2025-06-18")

	_, err := tr.Call(context.Background(), json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err == nil {
		t.Fatalf("expected error")
	}
	var se *HTTPStatusError
	if !IsHTTPStatusError(err) || !errors.As(err, &se) {
		t.Fatalf("expected HTTPStatusError, got %T", err)
	}
	if se.SessionID != "sess_123" {
		t.Fatalf("SessionID=%q", se.SessionID)
	}
	if se.ProtocolVersion != "2025-06-18" {
		t.Fatalf("ProtocolVersion=%q", se.ProtocolVersion)
	}
	if se.Headers == nil || http.Header(se.Headers).Get("X-Test") != "1" {
		t.Fatalf("headers missing")
	}
}
