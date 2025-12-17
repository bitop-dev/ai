package mcp

import "fmt"

type RPCError struct {
	Code    int64
	Message string
	Data    []byte
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return fmt.Sprintf("mcp rpc error %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("mcp rpc error %d", e.Code)
}

// HTTPStatusError is returned by HTTP-based transports when the server returns a
// non-2xx response.
type HTTPStatusError struct {
	Method     string
	URL        string
	StatusCode int
	Body       []byte

	Headers         map[string][]string
	SessionID       string
	ProtocolVersion string
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("mcp http %s %s: status %d: %s", e.Method, e.URL, e.StatusCode, string(e.Body))
}

// ClientError wraps client-side failures (transport, parsing, lifecycle).
type ClientError struct {
	Op     string // e.g. "initialize", "request", "notify", "listen"
	Method string // JSON-RPC method if applicable
	Cause  error
}

func (e *ClientError) Error() string {
	if e == nil {
		return ""
	}
	if e.Method != "" {
		return fmt.Sprintf("mcp %s (%s): %v", e.Op, e.Method, e.Cause)
	}
	return fmt.Sprintf("mcp %s: %v", e.Op, e.Cause)
}

func (e *ClientError) Unwrap() error { return e.Cause }

// CallToolError wraps failures returned while calling an MCP tool.
type CallToolError struct {
	ToolName string
	Cause    error
}

func (e *CallToolError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("mcp call tool %q: %v", e.ToolName, e.Cause)
	}
	return fmt.Sprintf("mcp call tool %q", e.ToolName)
}

func (e *CallToolError) Unwrap() error { return e.Cause }
