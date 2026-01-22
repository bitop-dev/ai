package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

type ClientConfig struct {
	Transport       Transport
	Name            string
	Version         string
	Capabilities    ClientCapabilities
	OnUncaughtError func(error)
}

type Client struct {
	transport          Transport
	onUncaughtError    func(error)
	clientInfo         ImplementationInfo
	clientCapabilities ClientCapabilities
	serverCapabilities ServerCapabilities
	requestID          int64
	mu                 sync.Mutex
	pending            map[int64]chan responseEnvelope
	closed             bool
}

type responseEnvelope struct {
	result any
	err    error
}

func NewClient(config ClientConfig) (*Client, error) {
	if config.Transport == nil {
		return nil, fmt.Errorf("mcp client requires a transport")
	}
	name := config.Name
	if name == "" {
		name = "ai-sdk-mcp-client"
	}
	version := config.Version
	if version == "" {
		version = "1.0.0"
	}
	client := &Client{
		transport:          config.Transport,
		onUncaughtError:    config.OnUncaughtError,
		clientInfo:         ImplementationInfo{Name: name, Version: version},
		clientCapabilities: config.Capabilities,
		pending:            map[int64]chan responseEnvelope{},
		closed:             true,
	}
	client.transport.SetHandlers(TransportHandlers{
		OnMessage: client.onMessage,
		OnError:   client.onError,
		OnClose:   client.onClose,
	})
	return client, nil
}

func CreateClient(ctx context.Context, config ClientConfig) (*Client, error) {
	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}
	if err := client.Init(ctx); err != nil {
		return nil, err
	}
	return client, nil
}

func (client *Client) Init(ctx context.Context) error {
	if err := client.transport.Start(ctx); err != nil {
		return err
	}
	client.mu.Lock()
	client.closed = false
	client.mu.Unlock()

	result, err := requestResult[InitializeResult](client, ctx, "initialize", InitializeParams{
		ProtocolVersion: LatestProtocolVersion,
		Capabilities:    client.clientCapabilities,
		ClientInfo:      client.clientInfo,
	})
	if err != nil {
		client.Close()
		return err
	}
	if !isSupportedProtocol(result.ProtocolVersion) {
		client.Close()
		return newResponseError("mcp protocol version not supported", nil)
	}
	client.serverCapabilities = result.Capabilities

	if err := client.notification(ctx, "notifications/initialized", nil); err != nil {
		client.Close()
		return err
	}
	return nil
}

func (client *Client) Close() error {
	client.mu.Lock()
	if client.closed {
		client.mu.Unlock()
		return nil
	}
	client.closed = true
	client.mu.Unlock()

	return client.transport.Close()
}

func (client *Client) ListTools(ctx context.Context, params *PaginatedParams) (ListToolsResult, error) {
	if !client.serverCapabilities.SupportsTools() {
		return ListToolsResult{}, newCapabilityError("tools")
	}
	return requestResult[ListToolsResult](client, ctx, "tools/list", params)
}

func (client *Client) CallTool(ctx context.Context, name string, args map[string]any) (CallToolResult, error) {
	if !client.serverCapabilities.SupportsTools() {
		return CallToolResult{}, newCapabilityError("tools")
	}
	params := CallToolParams{Name: name, Arguments: args}
	return requestResult[CallToolResult](client, ctx, "tools/call", params)
}

func (client *Client) request(ctx context.Context, method string, params any) (any, error) {
	client.mu.Lock()
	if client.closed {
		client.mu.Unlock()
		return nil, fmt.Errorf("mcp client is closed")
	}
	client.mu.Unlock()

	messageID := atomic.AddInt64(&client.requestID, 1)
	responseCh := make(chan responseEnvelope, 1)
	client.mu.Lock()
	client.pending[messageID] = responseCh
	client.mu.Unlock()

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      messageID,
		Method:  method,
		Params:  params,
	}
	if err := client.transport.Send(ctx, request); err != nil {
		client.cleanupPending(messageID)
		return nil, err
	}

	select {
	case <-ctx.Done():
		client.cleanupPending(messageID)
		return nil, ctx.Err()
	case response := <-responseCh:
		if response.err != nil {
			return nil, response.err
		}
		return response.result, nil
	}
}

func (client *Client) notification(ctx context.Context, method string, params any) error {
	client.mu.Lock()
	if client.closed {
		client.mu.Unlock()
		return fmt.Errorf("mcp client is closed")
	}
	client.mu.Unlock()

	message := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return client.transport.Send(ctx, message)
}

func (client *Client) onMessage(message Message) {
	switch typed := message.(type) {
	case JSONRPCResponse:
		client.handleResponse(typed.ID, responseEnvelope{result: typed.Result})
	case *JSONRPCResponse:
		client.handleResponse(typed.ID, responseEnvelope{result: typed.Result})
	case JSONRPCError:
		client.handleResponse(typed.ID, responseEnvelope{err: MapRPCError(RPCError{Code: typed.Error.Code, Message: typed.Error.Message, Data: typed.Error.Data})})
	case *JSONRPCError:
		client.handleResponse(typed.ID, responseEnvelope{err: MapRPCError(RPCError{Code: typed.Error.Code, Message: typed.Error.Message, Data: typed.Error.Data})})
	default:
		client.onError(newResponseError("mcp client received unsupported message", nil))
	}
}

func (client *Client) handleResponse(messageID int64, response responseEnvelope) {
	client.mu.Lock()
	responseCh, ok := client.pending[messageID]
	if ok {
		delete(client.pending, messageID)
	}
	client.mu.Unlock()

	if !ok {
		client.onError(newResponseError("mcp response for unknown request", nil))
		return
	}
	responseCh <- response
}

func (client *Client) cleanupPending(messageID int64) {
	client.mu.Lock()
	delete(client.pending, messageID)
	client.mu.Unlock()
}

func (client *Client) onClose() {
	client.mu.Lock()
	if client.closed {
		client.mu.Unlock()
		return
	}
	client.closed = true
	pending := client.pending
	client.pending = map[int64]chan responseEnvelope{}
	client.mu.Unlock()

	for _, responseCh := range pending {
		responseCh <- responseEnvelope{err: fmt.Errorf("mcp transport closed")}
	}
}

func (client *Client) onError(err error) {
	if client.onUncaughtError != nil {
		client.onUncaughtError(err)
	}
}

func decodeResult[T any](payload any) (T, error) {
	var zero T
	encoded, err := json.Marshal(payload)
	if err != nil {
		return zero, newResponseError("mcp response encoding failed", err)
	}
	if err := json.Unmarshal(encoded, &zero); err != nil {
		return zero, newResponseError("mcp response parsing failed", err)
	}
	return zero, nil
}

func requestResult[T any](client *Client, ctx context.Context, method string, params any) (T, error) {
	var zero T
	result, err := client.request(ctx, method, params)
	if err != nil {
		return zero, err
	}
	return decodeResult[T](result)
}

func isSupportedProtocol(version string) bool {
	for _, supported := range SupportedProtocolVersions {
		if supported == version {
			return true
		}
	}
	return false
}
