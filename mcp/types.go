package mcp

import "encoding/json"

// JSON-RPC 2.0 envelope types (subset used by MCP).

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *int64      `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int64           `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// MCP server types (subset).

type ToolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

type toolListResult struct {
	Tools []ToolInfo `json:"tools"`
}

type callToolParams struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content []ToolContentPart `json:"content,omitempty"`
	IsError bool              `json:"isError,omitempty"`
}

// ToolContentPart is a generic representation of MCP tool results.
// The spec defines multiple content part shapes; we preserve the raw payload.
type ToolContentPart struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (p *ToolContentPart) UnmarshalJSON(b []byte) error {
	p.Raw = append(p.Raw[:0], b...)
	var tmp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}
	p.Type = tmp.Type
	return nil
}

type ResourcesListResult struct {
	Resources []ResourceInfo `json:"resources"`
}

type ResourceTemplatesListResult struct {
	ResourceTemplates []ResourceTemplateInfo `json:"resourceTemplates"`
}

type ResourceTemplateInfo struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	MediaType   string `json:"mimeType,omitempty"`
}

type ResourceInfo struct {
	URI         string `json:"uri"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	MediaType   string `json:"mimeType,omitempty"`
}

type ReadResourceParams struct {
	URI string `json:"uri"`
}

type ReadResourceResult struct {
	Contents []ResourceContent `json:"contents"`
}

type ResourceContent struct {
	URI        string `json:"uri,omitempty"`
	Text       string `json:"text,omitempty"`
	BlobBase64 string `json:"blob,omitempty"`
	MediaType  string `json:"mimeType,omitempty"`
}

type PromptsListResult struct {
	Prompts []PromptInfo `json:"prompts"`
}

type PromptInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type GetPromptParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

type GetPromptResult struct {
	Messages []PromptMessage `json:"messages"`
}

type PromptMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Initialize / lifecycle.

type ClientInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

type InitializeRequest struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
	ClientInfo      ClientInfo     `json:"clientInfo"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
	ServerInfo      ServerInfo     `json:"serverInfo"`
	Instructions    string         `json:"instructions,omitempty"`
}

// Elicitation (server -> client request).

type elicitationCreateParams struct {
	Message         string          `json:"message"`
	RequestedSchema json.RawMessage `json:"requestedSchema"`
}

type ElicitationRequest struct {
	Message         string
	RequestedSchema json.RawMessage
}

type ElicitationAction string

const (
	ElicitationAccept  ElicitationAction = "accept"
	ElicitationDecline ElicitationAction = "decline"
	ElicitationCancel  ElicitationAction = "cancel"
)

type ElicitationResponse struct {
	Action  ElicitationAction `json:"action"`
	Content any               `json:"content,omitempty"`
}
