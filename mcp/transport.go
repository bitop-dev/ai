package mcp

import (
	"context"
	"encoding/json"
)

// Transport sends JSON-RPC requests to an MCP server.
//
// Implementations must be safe for concurrent use unless documented otherwise.
type Transport interface {
	Call(ctx context.Context, req json.RawMessage) (json.RawMessage, error)
	Close() error
}
