package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync/atomic"

	"github.com/bitop-dev/ai"
)

type Client struct {
	transport Transport
	nextID    atomic.Int64
}

type ClientOptions struct {
	Transport Transport
}

func NewClient(opts ClientOptions) (*Client, error) {
	if opts.Transport == nil {
		return nil, fmt.Errorf("mcp: transport is required")
	}
	c := &Client{transport: opts.Transport}
	c.nextID.Store(1)
	return c, nil
}

func (c *Client) Close() error {
	if c == nil || c.transport == nil {
		return nil
	}
	return c.transport.Close()
}

// OnElicitationRequest registers a handler for MCP elicitation requests.
//
// This is currently only supported for transports that can receive server->client
// JSON-RPC requests (e.g. stdio). For HTTP-based transports, this returns an error.
func (c *Client) OnElicitationRequest(handler func(ctx context.Context, req ElicitationRequest) (ElicitationResponse, error)) error {
	if c == nil || c.transport == nil {
		return fmt.Errorf("mcp: client is nil")
	}
	type elicitationCapable interface {
		SetElicitationHandler(func(ctx context.Context, req ElicitationRequest) (ElicitationResponse, error))
	}
	t, ok := c.transport.(elicitationCapable)
	if !ok {
		return fmt.Errorf("mcp: transport does not support elicitation")
	}
	t.SetElicitationHandler(handler)
	return nil
}

type ToolsOptions struct {
	// Prefix is prepended to returned tool names. The MCP server tool name is
	// preserved internally and used when calling tools/call.
	Prefix string

	// Allowlist/denylist apply to server tool names (before Prefix).
	// If AllowedTools is non-empty, only those tools are returned.
	AllowedTools []string
	DeniedTools  []string

	// Schemas optionally restricts which tools are returned and/or overrides the
	// server-provided schema for specific tools.
	//
	// When non-nil, only tools present in the map are returned.
	Schemas map[string]ai.Schema
}

func (c *Client) Tools(ctx context.Context, opts *ToolsOptions) ([]ai.Tool, error) {
	infos, err := c.listTools(ctx)
	if err != nil {
		return nil, err
	}

	byName := make(map[string]ToolInfo, len(infos))
	for _, t := range infos {
		byName[t.Name] = t
	}

	var names []string
	if opts != nil && opts.Schemas != nil {
		names = make([]string, 0, len(opts.Schemas))
		for n := range opts.Schemas {
			names = append(names, n)
		}
	} else {
		names = make([]string, 0, len(infos))
		for _, t := range infos {
			names = append(names, t.Name)
		}
	}

	allowed := map[string]bool{}
	denied := map[string]bool{}
	if opts != nil && len(opts.AllowedTools) > 0 {
		for _, n := range opts.AllowedTools {
			allowed[n] = true
		}
	}
	if opts != nil && len(opts.DeniedTools) > 0 {
		for _, n := range opts.DeniedTools {
			denied[n] = true
		}
	}

	// If Schemas is used, ensure deterministic ordering.
	if opts != nil && opts.Schemas != nil {
		sort.Strings(names)
	}

	out := make([]ai.Tool, 0, len(names))
	for _, name := range names {
		info, ok := byName[name]
		if !ok {
			// If caller restricted tools, surface a helpful error.
			if opts != nil && opts.Schemas != nil {
				return nil, fmt.Errorf("mcp: tool %q not found on server", name)
			}
			continue
		}

		if len(allowed) > 0 && !allowed[info.Name] {
			continue
		}
		if denied[info.Name] {
			continue
		}

		schema := info.InputSchema
		if opts != nil && opts.Schemas != nil {
			if s, ok := opts.Schemas[name]; ok && len(s.JSON) > 0 {
				schema = s.JSON
			}
		}

		serverToolName := info.Name
		publicToolName := serverToolName
		if opts != nil && opts.Prefix != "" {
			publicToolName = opts.Prefix + serverToolName
		}
		out = append(out, ai.Tool{
			Name:        publicToolName,
			Description: info.Description,
			InputSchema: ai.JSONSchema(schema),
			Handler: func(ctx context.Context, input json.RawMessage) (any, error) {
				return c.callTool(ctx, serverToolName, input)
			},
		})
	}

	return out, nil
}

func (c *Client) listTools(ctx context.Context) ([]ToolInfo, error) {
	var result toolListResult
	if err := c.rpc(ctx, "tools/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

func (c *Client) callTool(ctx context.Context, name string, input json.RawMessage) (any, error) {
	var args any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return nil, err
		}
	}

	var result CallToolResult
	if err := c.rpc(ctx, "tools/call", callToolParams{Name: name, Arguments: args}, &result); err != nil {
		return nil, err
	}

	// Common case: a single text content part -> return plain string for model consumption.
	if len(result.Content) == 1 && result.Content[0].Type == "text" {
		var t struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(result.Content[0].Raw, &t); err == nil && t.Text != "" {
			return t.Text, nil
		}
	}

	// Otherwise return the structured result; it will be JSON-encoded into the tool result message.
	return result, nil
}

func (c *Client) ListResources(ctx context.Context) ([]ResourceInfo, error) {
	var res ResourcesListResult
	if err := c.rpc(ctx, "resources/list", nil, &res); err != nil {
		return nil, err
	}
	return res.Resources, nil
}

func (c *Client) ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error) {
	var res ReadResourceResult
	if err := c.rpc(ctx, "resources/read", ReadResourceParams{URI: uri}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) ListPrompts(ctx context.Context) ([]PromptInfo, error) {
	var res PromptsListResult
	if err := c.rpc(ctx, "prompts/list", nil, &res); err != nil {
		return nil, err
	}
	return res.Prompts, nil
}

func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]string) (*GetPromptResult, error) {
	var res GetPromptResult
	if err := c.rpc(ctx, "prompts/get", GetPromptParams{Name: name, Arguments: args}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) rpc(ctx context.Context, method string, params any, out any) error {
	if c == nil || c.transport == nil {
		return fmt.Errorf("mcp: client is nil")
	}
	id := c.nextID.Add(1)
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	rawResp, err := c.transport.Call(ctx, b)
	if err != nil {
		return err
	}
	var resp rpcResponse
	if err := json.Unmarshal(rawResp, &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return &RPCError{Code: resp.Error.Code, Message: resp.Error.Message, Data: resp.Error.Data}
	}
	if out == nil {
		return nil
	}
	if len(resp.Result) == 0 {
		return fmt.Errorf("mcp: empty result for %s", method)
	}
	return json.Unmarshal(resp.Result, out)
}
