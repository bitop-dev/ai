package mcp

import (
	"context"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

type testTransport struct {
	handlers TransportHandlers
	sent     chan Message
}

func newTestTransport() *testTransport {
	return &testTransport{sent: make(chan Message, 16)}
}

func (transport *testTransport) Start(ctx context.Context) error {
	return nil
}

func (transport *testTransport) Send(ctx context.Context, message Message) error {
	transport.sent <- message
	return nil
}

func (transport *testTransport) Close() error {
	if transport.handlers.OnClose != nil {
		transport.handlers.OnClose()
	}
	return nil
}

func (transport *testTransport) SetHandlers(handlers TransportHandlers) {
	transport.handlers = handlers
}

func respondInitialize(t *testing.T, transport *testTransport) {
	t.Helper()
	message := <-transport.sent
	request, ok := message.(JSONRPCRequest)
	if !ok {
		t.Fatalf("expected initialize request, got %T", message)
	}
	if request.Method != "initialize" {
		t.Fatalf("expected initialize method, got %q", request.Method)
	}
	transport.handlers.OnMessage(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result: InitializeResult{
			ProtocolVersion: LatestProtocolVersion,
			Capabilities:    ServerCapabilities{"tools": map[string]any{}},
			ServerInfo:      ImplementationInfo{Name: "server", Version: "1.0"},
		},
	})

	message = <-transport.sent
	notification, ok := message.(JSONRPCNotification)
	if !ok {
		t.Fatalf("expected initialized notification, got %T", message)
	}
	if notification.Method != "notifications/initialized" {
		t.Fatalf("expected initialized notification, got %q", notification.Method)
	}
}

func TestClientListTools(t *testing.T) {
	transport := newTestTransport()
	client, err := NewClient(ClientConfig{Transport: transport, Name: "test", Version: "0.1"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := context.Background()
	go respondInitialize(t, transport)
	if err := client.Init(ctx); err != nil {
		t.Fatalf("init client: %v", err)
	}

	go func() {
		message := <-transport.sent
		request := message.(JSONRPCRequest)
		if request.Method != "tools/list" {
			t.Errorf("expected tools/list, got %q", request.Method)
			return
		}
		transport.handlers.OnMessage(JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
			Result: ListToolsResult{
				Tools: []Tool{
					{
						Name:        "search",
						Description: "Search docs",
						InputSchema: provider.JSONObject{"type": "object"},
					},
				},
			},
		})
	}()

	result, err := client.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(result.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result.Tools))
	}
	if result.Tools[0].Name != "search" {
		t.Fatalf("expected tool name search, got %q", result.Tools[0].Name)
	}
}

func TestClientCallToolErrorMapping(t *testing.T) {
	transport := newTestTransport()
	client, err := NewClient(ClientConfig{Transport: transport})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx := context.Background()
	go respondInitialize(t, transport)
	if err := client.Init(ctx); err != nil {
		t.Fatalf("init client: %v", err)
	}

	go func() {
		message := <-transport.sent
		request := message.(JSONRPCRequest)
		if request.Method != "tools/call" {
			t.Errorf("expected tools/call, got %q", request.Method)
			return
		}
		transport.handlers.OnMessage(JSONRPCError{
			JSONRPC: "2.0",
			ID:      request.ID,
			Error: JSONRPCErrorObject{
				Code:    -32602,
				Message: "invalid params",
			},
		})
	}()

	_, err = client.CallTool(ctx, "search", map[string]any{"q": "mcp"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !provider.IsInvalidRequestError(err) {
		t.Fatalf("expected invalid request error, got %v", err)
	}
}
