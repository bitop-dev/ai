# MCP

Use the MCP client to connect to Model Context Protocol tool servers over stdio or HTTP.

## Packages

- `pkg/mcp` provides the MCP client, JSON-RPC types, and transports.
- Tool bridging helpers convert MCP tools to `providerutils.ToolDefinition` values.

## Stdio Transport

```go
transport := mcp.NewStdioTransport(mcp.StdioConfig{
    Command: "mcp-server",
    Args:    []string{"--config", "server.json"},
})
client, err := mcp.CreateClient(ctx, mcp.ClientConfig{Transport: transport})
if err != nil {
    log.Fatal(err)
}
defer client.Close()

tools, err := client.ListTools(ctx, nil)
if err != nil {
    log.Fatal(err)
}
_ = tools
```

## HTTP Transport

```go
transport := mcp.NewHTTPTransport(mcp.HTTPConfig{
    URL: "https://mcp.example.com",
    Headers: map[string]string{
        "Authorization": "Bearer " + os.Getenv("MCP_TOKEN"),
    },
})
client, err := mcp.CreateClient(ctx, mcp.ClientConfig{Transport: transport})
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

## Tool Bridging

```go
toolSet, err := client.Tools(ctx, mcp.ToolSetOptions{
    ToolName: func(tool mcp.Tool) string {
        return "mcp." + tool.Name
    },
})
if err != nil {
    log.Fatal(err)
}

result, err := ai.GenerateTextWithTools(ctx, model, ai.ToolLoopOptions{
    TextOptions: ai.TextOptions{Prompt: prompt},
    Tools:       toolSet.Tools,
})
if err != nil {
    log.Fatal(err)
}
_ = result
```

## Examples

- `examples/mcp-stdio/main.go`
- `examples/mcp-http/main.go`
