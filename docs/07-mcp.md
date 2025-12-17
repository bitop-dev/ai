# MCP (Model Context Protocol)

This library includes a first-class MCP client in `github.com/bitop-dev/ai/mcp`.

It supports:

- HTTP / “Streamable HTTP” style servers (including SSE responses)
- SSE server event streams (where the server pushes notifications/requests)
- stdio transport (local servers)
- Adapting MCP tools to `[]ai.Tool`
- Resources and prompts helpers
- Notifications, caching + auto-refresh, OAuth hooks

## Install

```bash
go get github.com/bitop-dev/ai/mcp
```

## Mental model

An MCP client is a JSON-RPC client with:

- **tools**: model-controlled functions the model can call
- **resources**: app-controlled context you fetch and inject
- **prompts**: server-defined templates you can fetch and apply

## 1) Create an MCP client

### HTTP transport

```go
client := mcp.NewClient(mcp.HTTPTransport{
  URL: "http://localhost:8080/mcp",
})
defer client.Close()
```

You can also set headers:

```go
client := mcp.NewClient(mcp.HTTPTransport{
  URL: "http://localhost:8080/mcp",
  Headers: map[string]string{
    "Authorization": "Bearer ...",
  },
})
```

### Stdio transport (local servers)

```go
client := mcp.NewClient(mcp.StdioTransport{
  Command: "node",
  Args:    []string{"./server.js"},
})
defer client.Close()
```

## 2) Handshake behavior

The MCP lifecycle handshake (`initialize` + `notifications/initialized`) is performed automatically on first use.
You can trigger it explicitly by calling any method, e.g. `client.Tools(...)` or `client.ListResources(...)`.

## 3) Use MCP tools with `ai.GenerateText` / `ai.StreamText`

### Discover tools and run a tool-capable request

```go
tools, err := client.Tools(ctx, mcp.ToolsOptions{
  Prefix: "mcp_", // optional: avoid name collisions
})
if err != nil {
  log.Fatal(err)
}

resp, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    Model: openai.Chat("gpt-4o-mini"),
    Messages: []ai.Message{
      ai.User("What is the weather in Brooklyn, New York?"),
    },
    Tools: tools,
    ToolLoop: &ai.ToolLoopOptions{MaxIterations: 10},
  },
})
```

### Close on finish (common pattern)

For short-lived usage, close the client when you’re done:

```go
defer client.Close()
```

For streaming, you can close after the stream finishes:

```go
stream, err := ai.StreamText(ctx, req)
// ...
for stream.Next() { fmt.Print(stream.Delta()) }
_ = stream.Close()
_ = client.Close()
```

## 4) Notifications + Listen loop

Some transports allow the server to send notifications (or requests) to the client.
To receive them, register handlers and call `Listen(ctx)`.

```go
client.OnNotification("notifications/tools/list_changed", func(data []byte) {
  log.Printf("tools list changed: %s", string(data))
})

go func() {
  if err := client.Listen(ctx); err != nil {
    log.Printf("listen error: %v", err)
  }
}()
```

## 5) Cached discovery + auto refresh

### Cached tools

```go
tools, err := client.ToolsCached(ctx, mcp.ToolsOptions{})
```

The cache is invalidated when `Listen(ctx)` receives a `*_list_changed` notification.

### Auto refresh helper

```go
err := client.ListenAndAutoRefresh(ctx, mcp.AutoRefreshOptions{
  OnToolsChanged: func(tools []ai.Tool) {
    log.Printf("tools refreshed: %d", len(tools))
  },
})
```

## 6) Schema overrides for type safety

You can restrict tool discovery and provide explicit JSON schemas:

```go
tools, err := client.Tools(ctx, mcp.ToolsOptions{
  Schemas: map[string]mcp.ToolSchema{
    "get-data": {
      InputSchema: []byte(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"],"additionalProperties":false}`),
    },
  },
})
```

This reduces drift when servers change tool definitions.

## 7) Resources

Resources are **application-driven**. You decide when to fetch and inject them as context.

```go
resources, err := client.ListResources(ctx)
```

Read a resource:

```go
data, err := client.ReadResource(ctx, mcp.ReadResourceRequest{
  URI: "file:///example/document.txt",
})
```

Resource templates:

```go
templates, err := client.ListResourceTemplates(ctx)
```

## 8) Prompts

List prompts:

```go
prompts, err := client.ListPrompts(ctx)
```

Get a prompt (with arguments):

```go
prompt, err := client.GetPrompt(ctx, mcp.GetPromptRequest{
  Name: "code_review",
  Arguments: map[string]string{
    "code": "function add(a, b) { return a + b; }",
  },
})
```

## 9) Elicitation

Some MCP servers can request additional user input during tool execution (elicitation).
The client supports handling elicitation requests in the `Listen(ctx)` loop.

```go
client.OnElicitationRequest(func(req mcp.ElicitationRequest) (mcp.ElicitationResponse, error) {
  // You decide how to surface this to a user; this example always declines.
  return mcp.ElicitationResponse{Action: "decline"}, nil
})
```

## 10) Auth + OAuth hooks

### Static headers

```go
client := mcp.NewClient(mcp.HTTPTransport{
  URL: "https://server/mcp",
  Headers: map[string]string{"Authorization": "Bearer ..."},
})
```

### Dynamic headers (e.g. token refresh)

Use `HeaderProvider` to compute headers at request time.

```go
tr := mcp.HTTPTransport{
  URL: "https://server/mcp",
  HeaderProvider: func(ctx context.Context) (map[string]string, error) {
    return map[string]string{"Authorization": "Bearer ..."}, nil
  },
}
client := mcp.NewClient(tr)
```

### OAuth client credentials helper

```go
provider := mcp.OAuthClientCredentialsProvider{
  TokenURL:     "https://auth.example.com/oauth/token",
  ClientID:     "id",
  ClientSecret: "secret",
  Audience:     "mcp",
}

client := mcp.NewClient(mcp.HTTPTransport{
  URL:          "https://server/mcp",
  AuthProvider: provider,
})
```

## 11) Errors

The MCP package exposes typed errors:

- `*mcp.RPCError` — server returned a JSON-RPC error
- `*mcp.HTTPStatusError` — HTTP transport returned non-2xx
- `*mcp.ClientError` — client-side failure (transport/parsing/lifecycle), with `Op`/`Method`
- `*mcp.CallToolError` — `tools/call` failed

Helpers:

```go
if mcp.IsRPCError(err) { /* ... */ }
if mcp.IsHTTPStatusError(err) { /* ... */ }
if mcp.IsInitError(err) { /* ... */ }
if mcp.IsCallToolError(err) { /* ... */ }
if mcp.IsAuthError(err) { /* ... */ }
if mcp.IsRateLimited(err) { /* ... */ }
if mcp.IsServerError(err) { /* ... */ }
```

## Examples in this repo

- `go run ./examples/mcp_tools`
- `go run ./examples/mcp_stream_text`

