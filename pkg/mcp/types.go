package mcp

import "github.com/bitop-dev/ai/pkg/provider"

const LatestProtocolVersion = "2025-06-18"

var SupportedProtocolVersions = []string{
	LatestProtocolVersion,
	"2025-03-26",
	"2024-11-05",
}

type Capabilities map[string]any

type ClientCapabilities = Capabilities
type ServerCapabilities = Capabilities

type ImplementationInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities,omitempty"`
	ClientInfo      ImplementationInfo `json:"clientInfo"`
}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ImplementationInfo `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"`
}

type PaginatedParams struct {
	Cursor string `json:"cursor,omitempty"`
	Meta   any    `json:"_meta,omitempty"`
}

type ToolMeta map[string]any

type Tool struct {
	Name         string              `json:"name"`
	Title        string              `json:"title,omitempty"`
	Description  string              `json:"description,omitempty"`
	InputSchema  provider.JSONSchema `json:"inputSchema"`
	OutputSchema provider.JSONSchema `json:"outputSchema,omitempty"`
	Annotations  map[string]any      `json:"annotations,omitempty"`
	Meta         ToolMeta            `json:"_meta,omitempty"`
}

type ListToolsResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
	Meta       any    `json:"_meta,omitempty"`
}

type CallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type ToolContent struct {
	Type     string           `json:"type"`
	Text     string           `json:"text,omitempty"`
	Data     string           `json:"data,omitempty"`
	MimeType string           `json:"mimeType,omitempty"`
	Resource *ResourceContent `json:"resource,omitempty"`
}

type ResourceContent struct {
	URI      string `json:"uri"`
	Name     string `json:"name,omitempty"`
	Title    string `json:"title,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

type CallToolResult struct {
	Content           []ToolContent      `json:"content,omitempty"`
	StructuredContent provider.JSONValue `json:"structuredContent,omitempty"`
	IsError           bool               `json:"isError,omitempty"`
	ToolResult        provider.JSONValue `json:"toolResult,omitempty"`
	Meta              any                `json:"_meta,omitempty"`
}

func ToolResultFromCall(call provider.ToolCall, result CallToolResult) provider.ToolResult {
	toolResult := provider.ToolResult{ID: call.ID, Name: call.Name}
	if result.ToolResult != nil {
		toolResult.Result = result.ToolResult
	} else if result.StructuredContent != nil {
		toolResult.Result = result.StructuredContent
	} else if len(result.Content) > 0 {
		toolResult.Result = result.Content
	}
	toolResult.IsError = result.IsError
	return toolResult
}

func (capabilities ServerCapabilities) SupportsTools() bool {
	if capabilities == nil {
		return false
	}
	_, ok := capabilities["tools"]
	return ok
}
