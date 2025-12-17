package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"sync/atomic"
	"time"

	"github.com/bitop-dev/ai"
	"github.com/bitop-dev/ai/internal/sse"
)

type Client struct {
	transport Transport
	nextID    atomic.Int64

	protocolVersion string
	clientInfo      ClientInfo
	capabilities    map[string]any

	initialized atomic.Bool

	elicitationHandler  atomic.Value // func(context.Context, ElicitationRequest) (ElicitationResponse, error)
	notificationHandler atomic.Value // func(context.Context, string, json.RawMessage)

	toolCache              atomic.Value // toolCacheEntry
	resourcesCache         atomic.Value // []ResourceInfo
	resourceTemplatesCache atomic.Value // []ResourceTemplateInfo
	promptsCache           atomic.Value // []PromptInfo
}

type toolCacheEntry struct {
	key   string
	tools []ai.Tool
}

func (c *Client) invalidateCaches(method string) {
	switch method {
	case "notifications/tools/list_changed":
		c.toolCache.Store(toolCacheEntry{})
	case "notifications/resources/list_changed":
		c.resourcesCache.Store([]ResourceInfo(nil))
		c.resourceTemplatesCache.Store([]ResourceTemplateInfo(nil))
	case "notifications/prompts/list_changed":
		c.promptsCache.Store([]PromptInfo(nil))
	}
}

type AutoRefreshOptions struct {
	ToolsOptions *ToolsOptions

	OnTools             func(ctx context.Context, tools []ai.Tool)
	OnResources         func(ctx context.Context, resources []ResourceInfo)
	OnResourceTemplates func(ctx context.Context, templates []ResourceTemplateInfo)
	OnPrompts           func(ctx context.Context, prompts []PromptInfo)

	OnError func(ctx context.Context, err error)
}

// ListenAndAutoRefresh runs Listen() and, on list-changed notifications, re-fetches
// the relevant lists and calls the corresponding callbacks.
//
// This is additive sugar over Listen() + manual refetching.
func (c *Client) ListenAndAutoRefresh(ctx context.Context, opts AutoRefreshOptions) error {
	c.OnNotification(func(nctx context.Context, method string, _ json.RawMessage) {
		switch method {
		case "notifications/tools/list_changed":
			if opts.OnTools == nil {
				return
			}
			tools, err := c.Tools(nctx, opts.ToolsOptions)
			if err != nil {
				if opts.OnError != nil {
					opts.OnError(nctx, err)
				}
				return
			}
			opts.OnTools(nctx, tools)
		case "notifications/resources/list_changed":
			if opts.OnResources != nil {
				res, err := c.ListResources(nctx)
				if err != nil {
					if opts.OnError != nil {
						opts.OnError(nctx, err)
					}
				} else {
					opts.OnResources(nctx, res)
				}
			}
			if opts.OnResourceTemplates != nil {
				templates, err := c.ListResourceTemplates(nctx)
				if err != nil {
					if opts.OnError != nil {
						opts.OnError(nctx, err)
					}
				} else {
					opts.OnResourceTemplates(nctx, templates)
				}
			}
		case "notifications/prompts/list_changed":
			if opts.OnPrompts == nil {
				return
			}
			prompts, err := c.ListPrompts(nctx)
			if err != nil {
				if opts.OnError != nil {
					opts.OnError(nctx, err)
				}
				return
			}
			opts.OnPrompts(nctx, prompts)
		}
	})

	return c.Listen(ctx)
}

type ClientOptions struct {
	Transport Transport

	// ProtocolVersion is sent in the initialize request. Defaults to "2025-06-18".
	ProtocolVersion string

	// ClientInfo is sent in the initialize request. Name defaults to "ai-go-mcp-client".
	ClientInfo ClientInfo

	// Capabilities is sent in the initialize request (e.g. {"elicitation":{}}).
	Capabilities map[string]any
}

func NewClient(opts ClientOptions) (*Client, error) {
	if opts.Transport == nil {
		return nil, fmt.Errorf("mcp: transport is required")
	}
	c := &Client{transport: opts.Transport}
	c.nextID.Store(1)
	c.protocolVersion = opts.ProtocolVersion
	if c.protocolVersion == "" {
		c.protocolVersion = "2025-06-18"
	}
	c.clientInfo = opts.ClientInfo
	if c.clientInfo.Name == "" {
		c.clientInfo.Name = "ai-go-mcp-client"
	}
	c.capabilities = opts.Capabilities
	return c, nil
}

func (c *Client) Close() error {
	if c == nil || c.transport == nil {
		return nil
	}
	return c.transport.Close()
}

// Initialize performs the MCP lifecycle initialization handshake.
//
// It is called automatically by higher-level methods like Tools, but can be
// invoked explicitly to validate connectivity up front.
func (c *Client) Initialize(ctx context.Context) (*InitializeResult, error) {
	if c == nil || c.transport == nil {
		return nil, fmt.Errorf("mcp: client is nil")
	}
	if c.initialized.Load() {
		return &InitializeResult{ProtocolVersion: c.protocolVersion}, nil
	}

	// Use a short timeout by default for init if caller didn't provide one.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	var res InitializeResult
	if err := c.rpcRaw(ctx, "initialize", InitializeRequest{
		ProtocolVersion: c.protocolVersion,
		Capabilities:    c.capabilities,
		ClientInfo:      c.clientInfo,
	}, &res); err != nil {
		return nil, &ClientError{Op: "initialize", Method: "initialize", Cause: err}
	}

	// Version negotiation: if server responded with a different version and we
	// don't support it, callers should treat it as an error. For now, accept any
	// non-empty string and update the transport header.
	if res.ProtocolVersion != "" {
		c.protocolVersion = res.ProtocolVersion
		if pv, ok := c.transport.(interface{ SetProtocolVersion(string) }); ok {
			pv.SetProtocolVersion(c.protocolVersion)
		}
	}

	// Must send initialized notification after initialize.
	if err := c.notify(ctx, "notifications/initialized", nil); err != nil {
		return nil, &ClientError{Op: "initialize", Method: "notifications/initialized", Cause: err}
	}

	c.initialized.Store(true)
	return &res, nil
}

func (c *Client) ensureInitialized(ctx context.Context) error {
	if c.initialized.Load() {
		return nil
	}
	_, err := c.Initialize(ctx)
	return err
}

// OnElicitationRequest registers a handler for MCP elicitation requests.
//
// This is currently only supported for transports that can receive server->client
// JSON-RPC requests (e.g. stdio). For HTTP-based transports, this returns an error.
func (c *Client) OnElicitationRequest(handler func(ctx context.Context, req ElicitationRequest) (ElicitationResponse, error)) error {
	if c == nil || c.transport == nil {
		return fmt.Errorf("mcp: client is nil")
	}
	c.elicitationHandler.Store(handler)
	type elicitationCapable interface {
		SetElicitationHandler(func(ctx context.Context, req ElicitationRequest) (ElicitationResponse, error))
	}
	t, ok := c.transport.(elicitationCapable)
	if ok {
		t.SetElicitationHandler(handler)
	}
	return nil
}

// OnNotification registers a handler for server->client JSON-RPC notifications.
// It is invoked from Listen().
func (c *Client) OnNotification(handler func(ctx context.Context, method string, params json.RawMessage)) {
	if c == nil {
		return
	}
	c.notificationHandler.Store(handler)
}

// Listen opens a server-to-client event stream (when supported by the transport)
// and handles incoming notifications and requests.
//
// This blocks until the stream ends or ctx is cancelled.
func (c *Client) Listen(ctx context.Context) error {
	if err := c.ensureInitialized(ctx); err != nil {
		return err
	}
	type streamOpener interface {
		OpenSSEStream(ctx context.Context) (io.ReadCloser, error)
	}
	so, ok := c.transport.(streamOpener)
	if !ok {
		return fmt.Errorf("mcp: transport does not support server event streams")
	}
	rc, err := so.OpenSSEStream(ctx)
	if err != nil {
		return &ClientError{Op: "listen", Method: "GET", Cause: err}
	}
	defer rc.Close()

	dec := sse.NewDecoder(rc)
	for dec.Next() {
		data := dec.Data()
		if len(data) == 0 {
			continue
		}

		// Determine if it's a request/notification (method present).
		var probe struct {
			Method string          `json:"method"`
			ID     *int64          `json:"id,omitempty"`
			Params json.RawMessage `json:"params,omitempty"`
		}
		if err := json.Unmarshal(data, &probe); err != nil {
			continue
		}
		if probe.Method == "" {
			// It's likely a response (shouldn't happen on GET stream); ignore.
			continue
		}

		if probe.Method == "elicitation/create" && probe.ID != nil {
			hAny := c.elicitationHandler.Load()
			if hAny == nil {
				continue
			}
			h := hAny.(func(context.Context, ElicitationRequest) (ElicitationResponse, error))

			var p elicitationCreateParams
			if err := json.Unmarshal(probe.Params, &p); err != nil {
				_ = c.sendRPCResponse(ctx, *probe.ID, nil, &rpcError{Code: -32602, Message: "invalid params"})
				continue
			}
			res, err := h(ctx, ElicitationRequest{Message: p.Message, RequestedSchema: p.RequestedSchema})
			if err != nil {
				_ = c.sendRPCResponse(ctx, *probe.ID, nil, &rpcError{Code: -32000, Message: err.Error()})
				continue
			}
			_ = c.sendRPCResponse(ctx, *probe.ID, res, nil)
			continue
		}

		c.invalidateCaches(probe.Method)

		if probe.ID == nil {
			hAny := c.notificationHandler.Load()
			if hAny != nil {
				h := hAny.(func(context.Context, string, json.RawMessage))
				h(ctx, probe.Method, probe.Params)
			}
		}
	}

	if err := dec.Err(); err != nil {
		return &ClientError{Op: "listen", Cause: err}
	}
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
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}
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

// ToolsCached returns tools like Tools(), but caches the result for identical
// options. Cache is invalidated automatically when Listen() receives
// `notifications/tools/list_changed`.
func (c *Client) ToolsCached(ctx context.Context, opts *ToolsOptions) ([]ai.Tool, error) {
	if c == nil {
		return nil, fmt.Errorf("mcp: client is nil")
	}
	key, err := toolsOptionsKey(opts)
	if err != nil {
		return nil, err
	}
	if v := c.toolCache.Load(); v != nil {
		e := v.(toolCacheEntry)
		if e.key == key && len(e.tools) > 0 {
			out := make([]ai.Tool, len(e.tools))
			copy(out, e.tools)
			return out, nil
		}
	}
	tools, err := c.Tools(ctx, opts)
	if err != nil {
		return nil, err
	}
	c.toolCache.Store(toolCacheEntry{key: key, tools: tools})
	return tools, nil
}

func toolsOptionsKey(opts *ToolsOptions) (string, error) {
	if opts == nil {
		return "nil", nil
	}
	type schemaEntry struct {
		Name   string          `json:"name"`
		Schema json.RawMessage `json:"schema"`
	}
	var schemas []schemaEntry
	if opts.Schemas != nil {
		names := make([]string, 0, len(opts.Schemas))
		for n := range opts.Schemas {
			names = append(names, n)
		}
		sort.Strings(names)
		schemas = make([]schemaEntry, 0, len(names))
		for _, n := range names {
			schemas = append(schemas, schemaEntry{Name: n, Schema: opts.Schemas[n].JSON})
		}
	}

	allowed := append([]string(nil), opts.AllowedTools...)
	denied := append([]string(nil), opts.DeniedTools...)
	sort.Strings(allowed)
	sort.Strings(denied)

	keyObj := struct {
		Prefix  string        `json:"prefix"`
		Allowed []string      `json:"allowed,omitempty"`
		Denied  []string      `json:"denied,omitempty"`
		Schemas []schemaEntry `json:"schemas,omitempty"`
	}{
		Prefix:  opts.Prefix,
		Allowed: allowed,
		Denied:  denied,
		Schemas: schemas,
	}
	b, err := json.Marshal(keyObj)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Client) listTools(ctx context.Context) ([]ToolInfo, error) {
	var result toolListResult
	if err := c.rpcRaw(ctx, "tools/list", nil, &result); err != nil {
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
	if err := c.rpcRaw(ctx, "tools/call", callToolParams{Name: name, Arguments: args}, &result); err != nil {
		return nil, &CallToolError{ToolName: name, Cause: err}
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
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	var res ResourcesListResult
	if err := c.rpcRaw(ctx, "resources/list", nil, &res); err != nil {
		return nil, err
	}
	return res.Resources, nil
}

func (c *Client) ListResourcesCached(ctx context.Context) ([]ResourceInfo, error) {
	if c == nil {
		return nil, fmt.Errorf("mcp: client is nil")
	}
	if v := c.resourcesCache.Load(); v != nil {
		if cached := v.([]ResourceInfo); len(cached) > 0 {
			out := make([]ResourceInfo, len(cached))
			copy(out, cached)
			return out, nil
		}
	}
	res, err := c.ListResources(ctx)
	if err != nil {
		return nil, err
	}
	c.resourcesCache.Store(res)
	return res, nil
}

func (c *Client) ListResourceTemplates(ctx context.Context) ([]ResourceTemplateInfo, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	var res ResourceTemplatesListResult
	if err := c.rpcRaw(ctx, "resources/templates/list", nil, &res); err != nil {
		return nil, err
	}
	return res.ResourceTemplates, nil
}

func (c *Client) ListResourceTemplatesCached(ctx context.Context) ([]ResourceTemplateInfo, error) {
	if c == nil {
		return nil, fmt.Errorf("mcp: client is nil")
	}
	if v := c.resourceTemplatesCache.Load(); v != nil {
		if cached := v.([]ResourceTemplateInfo); len(cached) > 0 {
			out := make([]ResourceTemplateInfo, len(cached))
			copy(out, cached)
			return out, nil
		}
	}
	res, err := c.ListResourceTemplates(ctx)
	if err != nil {
		return nil, err
	}
	c.resourceTemplatesCache.Store(res)
	return res, nil
}

func (c *Client) ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	var res ReadResourceResult
	if err := c.rpcRaw(ctx, "resources/read", ReadResourceParams{URI: uri}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) ListPrompts(ctx context.Context) ([]PromptInfo, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	var res PromptsListResult
	if err := c.rpcRaw(ctx, "prompts/list", nil, &res); err != nil {
		return nil, err
	}
	return res.Prompts, nil
}

func (c *Client) ListPromptsCached(ctx context.Context) ([]PromptInfo, error) {
	if c == nil {
		return nil, fmt.Errorf("mcp: client is nil")
	}
	if v := c.promptsCache.Load(); v != nil {
		if cached := v.([]PromptInfo); len(cached) > 0 {
			out := make([]PromptInfo, len(cached))
			copy(out, cached)
			return out, nil
		}
	}
	res, err := c.ListPrompts(ctx)
	if err != nil {
		return nil, err
	}
	c.promptsCache.Store(res)
	return res, nil
}

func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]string) (*GetPromptResult, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	var res GetPromptResult
	if err := c.rpcRaw(ctx, "prompts/get", GetPromptParams{Name: name, Arguments: args}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) rpcRaw(ctx context.Context, method string, params any, out any) error {
	if c == nil || c.transport == nil {
		return &ClientError{Op: "request", Method: method, Cause: fmt.Errorf("client is nil")}
	}
	id := c.nextID.Add(1)
	idPtr := &id
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      idPtr,
		Method:  method,
		Params:  params,
	}
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	rawResp, err := c.transport.Call(ctx, b)
	if err != nil {
		return &ClientError{Op: "request", Method: method, Cause: err}
	}
	return parseRPCResult(rawResp, out, method)
}

func parseRPCResult(rawResp json.RawMessage, out any, method string) error {
	var resp rpcResponse
	if err := json.Unmarshal(rawResp, &resp); err != nil {
		return &ClientError{Op: "parse", Method: method, Cause: err}
	}
	if resp.Error != nil {
		return &RPCError{Code: resp.Error.Code, Message: resp.Error.Message, Data: resp.Error.Data}
	}
	if out == nil {
		return nil
	}
	if len(resp.Result) == 0 {
		return &ClientError{Op: "parse", Method: method, Cause: fmt.Errorf("empty result")}
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		return &ClientError{Op: "parse", Method: method, Cause: err}
	}
	return nil
}

func (c *Client) notify(ctx context.Context, method string, params any) error {
	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = c.transport.Call(ctx, b)
	if err != nil {
		return &ClientError{Op: "notify", Method: method, Cause: err}
	}
	return nil
}

func (c *Client) sendRPCResponse(ctx context.Context, id int64, result any, rpcErr *rpcError) error {
	msg := rpcResponse{JSONRPC: "2.0", ID: id}
	if rpcErr != nil {
		msg.Error = rpcErr
	} else {
		b, err := json.Marshal(result)
		if err != nil {
			return &ClientError{Op: "response", Cause: err}
		}
		msg.Result = b
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return &ClientError{Op: "response", Cause: err}
	}
	_, err = c.transport.Call(ctx, b)
	if err != nil {
		return &ClientError{Op: "response", Cause: err}
	}
	return nil
}
