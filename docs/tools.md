# Tools and dynamic tools

The Go SDK models tools with `providerutils.ToolDefinition`. A tool definition
describes the JSON schema, an optional approval hook, and the execute function.
You can create tools at startup or on demand per request.

## Define a tool

```go
tool := providerutils.ToolDefinition{
    Name:        "current_time",
    Description: "Return the current time in RFC3339 format.",
    Parameters: provider.JSONObject{
        "type": "object",
        "properties": map[string]any{
            "timezone": map[string]any{
                "type":        "string",
                "description": "IANA time zone name, e.g. America/Los_Angeles",
            },
        },
    },
    Execute: func(ctx context.Context, call providerutils.ToolCall) (providerutils.ToolOutput, error) {
        locationName, _ := call.Arguments["timezone"].(string)
        if locationName == "" {
            locationName = "UTC"
        }
        location, err := time.LoadLocation(locationName)
        if err != nil {
            return providerutils.ToolErrorOutput{Err: err}, err
        }
        return providerutils.ToolTextOutput{Text: time.Now().In(location).Format(time.RFC3339)}, nil
    },
}
```

## Use tools with GenerateTextWithTools

```go
result, err := ai.GenerateTextWithTools(ctx, model, ai.ToolLoopOptions{
    TextOptions: ai.TextOptions{Prompt: prompt},
    Tools:       []providerutils.ToolDefinition{tool},
})
```

## Use tools with ToolLoopAgent

```go
agent := ai.NewToolLoopAgent(ai.ToolLoopAgentSettings[any]{
    Model: model,
    Tools: []providerutils.ToolDefinition{tool},
})

result, err := agent.Generate(ctx, ai.AgentCallOptions[any]{
    Prompt: "What time is it in Tokyo?",
})
```

`ToolLoopAgent` automatically injects tool specifications into provider
options for providers that read `ProviderOptions[providerID]["tools"]`.

## Dynamic tools

Dynamic tools are simply tools built per request. For example, you can
construct tool definitions based on user permissions or runtime data and pass
them into `GenerateTextWithTools` or a per-request `ToolLoopAgent`.

```go
tools := []providerutils.ToolDefinition{buildToolForUser(user)}
result, err := ai.GenerateTextWithTools(ctx, model, ai.ToolLoopOptions{
    TextOptions: ai.TextOptions{Prompt: prompt},
    Tools:       tools,
})
```

## Approvals

Tools can require approval at two levels:

- `ToolLoopOptions.Approve` or `ToolLoopAgentSettings.Approve` for global checks.
- `ToolDefinition.Approve` for per-tool checks.

Approval hooks return `(bool, error)`; returning false rejects the tool call.

## Related docs and examples

- `docs/agents.md`
- `docs/providerutils.md`
- `examples/tool_calling_auto/main.go`
- `examples/tool_calling_manual/main.go`
- `examples/agent_tool_loop/main.go`
