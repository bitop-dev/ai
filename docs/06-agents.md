# Agents

This library supports agents in two ways:

1. Use the built-in `ai.Agent` wrapper (recommended for most use cases).
2. Compose directly with `GenerateText` / `StreamText` for maximum control.

This doc shows both approaches:

1. **“Agent by configuration”** using `GenerateText`/`StreamText` directly
2. The built-in `ai.Agent` wrapper (similar to other SDKs)

## Key Concepts

### Step

A **step** is one model generation. A multi-step agent loop happens when the model calls tools and the SDK continues the conversation.

Each step captures:

- final assistant message for that generation
- tool calls requested
- tool results appended
- usage + finish reason

### Tool Loop

Tool loops are controlled via `ToolLoopOptions`:

- `MaxIterations` (default `5`)
- `StopWhen` (optional)

## 1) Agent by Configuration (no wrapper)

### Minimal tool-using agent

```go
add := ai.NewTool("add", ai.ToolSpec[struct {
  A int `json:"a"`
  B int `json:"b"`
}, map[string]int]{
  Description: "Add two integers.",
  InputSchema: ai.JSONSchema([]byte(`{
    "type":"object",
    "properties":{"a":{"type":"integer"},"b":{"type":"integer"}},
    "required":["a","b"],
    "additionalProperties":false
  }`)),
  Execute: func(ctx context.Context, in struct {
    A int `json:"a"`
    B int `json:"b"`
  }, meta ai.ToolExecutionMeta) (map[string]int, error) {
    _ = ctx
    return map[string]int{"result": in.A + in.B}, nil
  },
})

resp, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    Model: openai.Chat("gpt-4o-mini"),
    Messages: []ai.Message{
      ai.User("Use the add tool to add 1 and 2 and reply with the result."),
    },
    Tools: []ai.Tool{add},
    ToolLoop: &ai.ToolLoopOptions{
      MaxIterations: 10,
    },
  },
})
```

### Persisting chat history (conversation continuation)

If you are building a chat app, keep a `history []ai.Message` and append the messages produced by each call:

```go
history := []ai.Message{ai.User("Hi! Use tools if needed.")}

resp, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    Model:    openai.Chat("gpt-4o-mini"),
    Messages: history,
    Tools:    []ai.Tool{add},
  },
})
if err != nil { /* ... */ }

history = append(history, resp.Response.Messages...)
```

For streaming:

```go
stream, err := ai.StreamText(ctx, ai.StreamTextRequest{
  BaseRequest: ai.BaseRequest{
    Model:    openai.Chat("gpt-4o-mini"),
    Messages: history,
    Tools:    []ai.Tool{add},
  },
})
if err != nil { /* ... */ }
defer stream.Close()

for stream.Next() {
  fmt.Print(stream.Delta())
}
if err := stream.Err(); err != nil { /* ... */ }

history = append(history, stream.Response().Messages...)
```

## 2) Optional Agent Wrapper (recommended for app code)

If you want a reusable “agent object” (similar to other SDKs), use `ai.Agent`.

### Example `ai.Agent`

```go
agent := ai.Agent{
  Model: openai.Chat("gpt-4o-mini"),
  System: "You are a helpful assistant.",
  Tools: []ai.Tool{add},
  MaxIterations: 20,
}

resp, err := agent.Generate(ctx, ai.AgentGenerateRequest{
  Prompt: "Use add to add 40 and 2, then reply with the result.",
})
```

Usage:

```go
stream, err := agent.Stream(ctx, ai.AgentStreamRequest{
  Prompt: "Tell me a short story about a gopher.",
})
```

Notes:

- If you do not set `MaxIterations` or `StopWhen`, `ai.Agent` defaults to **1 step** (no multi-step loop).
- To enable multi-step behavior, set `MaxIterations` (or `StopWhen`).

## Stop Conditions (`StopWhen`)

Stop conditions are evaluated after a step that produced tool results.

### Stop after N steps

```go
req := ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    // ...
    ToolLoop: &ai.ToolLoopOptions{
      MaxIterations: 50,
      StopWhen: ai.StepCountIs(10),
    },
  },
}
```

### Stop when a specific tool was called

```go
ToolLoop: &ai.ToolLoopOptions{
  MaxIterations: 50,
  StopWhen: ai.HasToolCall("search"),
},
```

### Custom stop condition

```go
ToolLoop: &ai.ToolLoopOptions{
  MaxIterations: 50,
  StopWhen: func(e ai.StopConditionEvent) bool {
    // Example: stop once any step produced non-empty text
    for _, s := range e.Steps {
      if s.Text != "" {
        return true
      }
    }
    return false
  },
},
```

## Per-step Control (`PrepareStep`)

`PrepareStep` runs before each model generation in a multi-step loop.
You can:

- trim/transform messages (context management)
- restrict tools for that step (`ActiveTools`)
- (optionally) switch models **within the same provider**

### Tool routing by phase

```go
PrepareStep: func(e ai.PrepareStepEvent) (ai.PrepareStepResult, error) {
  if e.StepNumber == 0 {
    return ai.PrepareStepResult{
      ActiveTools: []string{"search"},
    }, nil
  }
  return ai.PrepareStepResult{
    ActiveTools: []string{"summarize"},
  }, nil
},
```

### Message trimming (simple context management)

```go
PrepareStep: func(e ai.PrepareStepEvent) (ai.PrepareStepResult, error) {
  msgs := e.Messages
  if len(msgs) > 20 {
    msgs = msgs[len(msgs)-10:]
  }
  return ai.PrepareStepResult{Messages: msgs}, nil
},
```

### Switching models (same provider only)

```go
PrepareStep: func(e ai.PrepareStepEvent) (ai.PrepareStepResult, error) {
  if e.StepNumber >= 3 {
    return ai.PrepareStepResult{
      Model: openai.Chat("gpt-4o"), // same provider as original
    }, nil
  }
  return ai.PrepareStepResult{}, nil
},
```

## Observability: `OnStepFinish` + `Steps`

### On-step callback

```go
OnStepFinish: func(e ai.StepFinishEvent) {
  log.Printf("step=%d finish=%s tokens=%d toolCalls=%d",
    e.Step.StepNumber,
    e.Step.FinishReason,
    e.Step.Usage.TotalTokens,
    len(e.Step.ToolCalls),
  )
},
```

### Full transcript after completion

```go
resp, _ := ai.GenerateText(ctx, req)
for _, s := range resp.Steps {
  fmt.Printf("step %d text=%q toolCalls=%d toolResults=%d\n",
    s.StepNumber, s.Text, len(s.ToolCalls), len(s.ToolResults))
}
```

For streaming:

```go
stream, _ := ai.StreamText(ctx, req)
for stream.Next() { /* ... */ }
steps := stream.Steps()
```

## Tool lifecycle hooks + tool progress

See `docs/01-getting-started.md` for:

- `Tool.OnInputStart`, `Tool.OnInputDelta`, `Tool.OnInputAvailable` (streaming tool input lifecycle)
- `BaseRequest.OnToolProgress` + `ToolExecutionMeta.Report(...)` (tool progress)

## Error handling

Common classes of errors:

- Provider/network errors: `*ai.Error` (use `ai.IsAuth`, `ai.IsRateLimited`, etc.)
- Tool schema errors: `*ai.NoSuchToolError`, `*ai.InvalidToolInputError`
- Tool execution errors: `*ai.ToolExecutionError`
- Loop limit: `"tool loop exceeded max iterations (...)"`

```go
resp, err := ai.GenerateText(ctx, req)
if err != nil {
  switch {
  case ai.IsRateLimited(err):
    // backoff
  case ai.IsAuth(err):
    // check key
  default:
    // log + handle
  }
}
_ = resp
```

## Example in this repo

- `go run ./examples/agent_steps`
