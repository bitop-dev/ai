package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHTTPTransportStartAlreadyStarted(t *testing.T) {
	transport := NewHTTPTransport(HTTPConfig{URL: "http://example.test", DisableInboundSSE: true})
	if err := transport.Start(context.Background()); err != nil {
		t.Fatalf("start transport: %v", err)
	}
	if err := transport.Start(context.Background()); err == nil {
		t.Fatalf("expected error on second start")
	}
	_ = transport.Close()
}

func TestHTTPTransportSendJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", request.Method)
		}
		var payload JSONRPCRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		response := JSONRPCResponse{JSONRPC: "2.0", ID: payload.ID, Result: map[string]any{"ok": true}}
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(response); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	transport := NewHTTPTransport(HTTPConfig{URL: server.URL, DisableInboundSSE: true})
	messageCh := make(chan Message, 1)
	transport.SetHandlers(TransportHandlers{OnMessage: func(message Message) { messageCh <- message }})

	if err := transport.Start(context.Background()); err != nil {
		t.Fatalf("start transport: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := transport.Send(ctx, JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "ping"}); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case message := <-messageCh:
		response, ok := message.(JSONRPCResponse)
		if !ok {
			t.Fatalf("expected JSONRPCResponse, got %T", message)
		}
		if response.ID != 1 {
			t.Fatalf("expected id 1, got %d", response.ID)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for response")
	}

	_ = transport.Close()
}

func TestHTTPTransportSendSSEResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", request.Method)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("Cache-Control", "no-cache")
		flusher, ok := writer.(http.Flusher)
		if !ok {
			t.Fatalf("expected flusher")
		}
		payload := `{"jsonrpc":"2.0","id":42,"result":{"pong":true}}`
		_, _ = writer.Write([]byte("event: message\n"))
		_, _ = writer.Write([]byte("data: "))
		_, _ = writer.Write([]byte(payload))
		_, _ = writer.Write([]byte("\n\n"))
		flusher.Flush()
	}))
	defer server.Close()

	transport := NewHTTPTransport(HTTPConfig{URL: server.URL, DisableInboundSSE: true})
	messageCh := make(chan Message, 1)
	transport.SetHandlers(TransportHandlers{OnMessage: func(message Message) { messageCh <- message }})

	if err := transport.Start(context.Background()); err != nil {
		t.Fatalf("start transport: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := transport.Send(ctx, JSONRPCRequest{JSONRPC: "2.0", ID: 42, Method: "ping"}); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case message := <-messageCh:
		response, ok := message.(JSONRPCResponse)
		if !ok {
			t.Fatalf("expected JSONRPCResponse, got %T", message)
		}
		if response.ID != 42 {
			t.Fatalf("expected id 42, got %d", response.ID)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for response")
	}
	_ = transport.Close()
}

func TestHTTPTransportInboundSSE(t *testing.T) {
	var mu sync.Mutex
	var received bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		flusher := writer.(http.Flusher)
		payload := `{"jsonrpc":"2.0","id":7,"result":{"ok":true}}`
		_, _ = writer.Write([]byte("event: message\n"))
		_, _ = writer.Write([]byte("data: "))
		_, _ = writer.Write([]byte(payload))
		_, _ = writer.Write([]byte("\n\n"))
		flusher.Flush()
		mu.Lock()
		received = true
		mu.Unlock()
	}))
	defer server.Close()

	transport := NewHTTPTransport(HTTPConfig{URL: server.URL})
	messageCh := make(chan Message, 1)
	errCh := make(chan error, 1)
	transport.SetHandlers(TransportHandlers{
		OnMessage: func(message Message) {
			messageCh <- message
		},
		OnError: func(err error) {
			errCh <- err
		},
	})

	if err := transport.Start(context.Background()); err != nil {
		t.Fatalf("start transport: %v", err)
	}

	select {
	case message := <-messageCh:
		response, ok := message.(JSONRPCResponse)
		if !ok {
			t.Fatalf("expected JSONRPCResponse, got %T", message)
		}
		if response.ID != 7 {
			t.Fatalf("expected id 7, got %d", response.ID)
		}
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for inbound message")
	}

	mu.Lock()
	if !received {
		t.Fatalf("expected inbound request")
	}
	mu.Unlock()

	_ = transport.Close()
}
