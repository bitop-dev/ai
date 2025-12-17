# Getting Started

This guide shows how to install and use `github.com/bitop-dev/ai` with OpenAI (and OpenAI-compatible) providers.

## Install

```bash
go get github.com/bitop-dev/ai
go get github.com/bitop-dev/ai/openai
```

## Configuration

The library keeps the public DX simple:

- Core API + types: `github.com/bitop-dev/ai`
- OpenAI helper package: `github.com/bitop-dev/ai/openai`

Set your API key:

```bash
export OPENAI_API_KEY="..."
```

Optional (OpenAI-compatible servers):

```bash
export OPENAI_BASE_URL="http://localhost:8080"
export OPENAI_API_PREFIX="/v1"
```

Configure OpenAI once at startup:

```go
package main

import (
  "github.com/bitop-dev/ai/openai"
)

func main() {
  openai.Configure(openai.Config{
    APIKey:    os.Getenv("OPENAI_API_KEY"),
    BaseURL:   os.Getenv("OPENAI_BASE_URL"),  // optional
    APIPrefix: os.Getenv("OPENAI_API_PREFIX"), // optional
  })

  // ...
}
```

## Generate Text

```go
resp, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    Model: openai.Chat("gpt-4o-mini"),
    Messages: []ai.Message{
      ai.User("Invent a new holiday and describe its traditions."),
    },
  },
})
if err != nil {
  log.Fatal(err)
}

fmt.Println(resp.Text)
```

## Stream Text

`StreamText` returns a `*ai.TextStream`. You can iterate with `Next()` or use helpers like `Iter()` / `Reader()`.

### `Next()` loop

```go
stream, err := ai.StreamText(ctx, ai.StreamTextRequest{
  BaseRequest: ai.BaseRequest{
    Model: openai.Chat("gpt-4o-mini"),
    Messages: []ai.Message{
      ai.User("Write a haiku about Go."),
    },
  },
})
if err != nil {
  log.Fatal(err)
}
defer stream.Close()

for stream.Next() {
  fmt.Print(stream.Delta())
}
if err := stream.Err(); err != nil {
  log.Fatal(err)
}
```

### `Iter()` helper

```go
for delta := range stream.Iter() {
  fmt.Print(delta)
}
if err := stream.Err(); err != nil {
  log.Fatal(err)
}
```

### `Reader()` helper

```go
io.Copy(os.Stdout, stream.Reader())
if err := stream.Err(); err != nil {
  log.Fatal(err)
}
```

## Tools (Tool Calling)

Tools are provided as `[]ai.Tool`. The model can call tools; the library executes them and continues the loop.

### Typed tool (`ai.NewTool`)

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
    "additionalProperties": false
  }`)),
  Execute: func(ctx context.Context, in struct {
    A int `json:"a"`
    B int `json:"b"`
  }, meta ai.ToolExecutionMeta) (map[string]int, error) {
    _ = ctx
    return map[string]int{"result": in.A + in.B}, nil
  },
})
```

Use it in `GenerateText`:

```go
resp, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    Model: openai.Chat("gpt-4o-mini"),
    Messages: []ai.Message{
      ai.User("Use the add tool to add 1 and 2, then reply with the result."),
    },
    Tools: []ai.Tool{add},
    ToolLoop: &ai.ToolLoopOptions{
      MaxIterations: 5, // default is 5
    },
  },
})
```

### Dynamic tool (`ai.NewDynamicTool`)

Dynamic tools receive `json.RawMessage` so you can validate/cast at runtime:

```go
dyn := ai.NewDynamicTool("echo", ai.DynamicToolSpec{
  InputSchema: ai.JSONSchema([]byte(`{"type":"object"}`)),
  Execute: func(ctx context.Context, input json.RawMessage, meta ai.ToolExecutionMeta) (any, error) {
    _ = ctx
    _ = meta
    return map[string]any{"you_sent": json.RawMessage(input)}, nil
  },
})
```

## Agent (Optional Wrapper)

If you prefer an “agent object” that holds model/tools/defaults, use `ai.Agent`:

```go
agent := ai.Agent{
  Model: openai.Chat("gpt-4o-mini"),
  Tools: []ai.Tool{add},
  MaxIterations: 10,
}

resp, err := agent.Generate(ctx, ai.AgentGenerateRequest{
  Prompt: "Use the add tool to add 1 and 2, then reply with the result.",
})
```

## Tool Lifecycle Hooks (Streaming)

When using `StreamText`, you can observe tool argument streaming:

```go
add.OnInputStart = func(e ai.ToolInputStartEvent) {
  log.Printf("tool %s call starting (id=%s index=%d)", e.ToolName, e.ToolCallID, e.ToolCallIndex)
}
add.OnInputDelta = func(e ai.ToolInputDeltaEvent) {
  log.Printf("tool %s args delta: %q", e.ToolName, e.InputTextDelta)
}
add.OnInputAvailable = func(e ai.ToolInputAvailableEvent) {
  log.Printf("tool %s args complete: %s", e.ToolName, string(e.Input))
}
```

## Tool Progress Events

To stream tool “status/progress” during execution:

1. Set `BaseRequest.OnToolProgress`.
2. Call `meta.Report(...)` inside your tool.

```go
req := ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    // ...
    OnToolProgress: func(e ai.ToolProgressEvent) {
      log.Printf("tool progress %s(%s): %#v", e.ToolName, e.ToolCallID, e.Data)
    },
  },
}
```

Inside the tool:

```go
Execute: func(ctx context.Context, in Input, meta ai.ToolExecutionMeta) (Output, error) {
  if meta.Report != nil {
    meta.Report(map[string]any{"status": "starting"})
  }
  // ...
  if meta.Report != nil {
    meta.Report(map[string]any{"status": "done"})
  }
  return out, nil
},
```

## Steps (Agentic Loop Ergonomics)

When tools are used, calls can span multiple **steps** (one model generation per step).

### Per-step callback: `OnStepFinish`

```go
req := ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    // ...
    OnStepFinish: func(e ai.StepFinishEvent) {
      log.Printf("step %d finish=%s toolCalls=%d toolResults=%d tokens=%d",
        e.Step.StepNumber,
        e.Step.FinishReason,
        len(e.Step.ToolCalls),
        len(e.Step.ToolResults),
        e.Step.Usage.TotalTokens,
      )
    },
  },
}
```

### Get the full step transcript

```go
resp, _ := ai.GenerateText(ctx, req)
for _, step := range resp.Steps {
  fmt.Printf("step %d: %s\n", step.StepNumber, step.Text)
}
```

### Per-step control: `PrepareStep`

Use `PrepareStep` to modify messages and/or active tools per step:

```go
req := ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    // ...
    PrepareStep: func(e ai.PrepareStepEvent) (ai.PrepareStepResult, error) {
      // Example: keep the last 10 messages
      msgs := e.Messages
      if len(msgs) > 10 {
        msgs = msgs[len(msgs)-10:]
      }
      return ai.PrepareStepResult{
        Messages: msgs,
        ActiveTools: []string{
          "add", // restrict tools for this step
        },
      }, nil
    },
  },
}
```

### Stop conditions

`ToolLoopOptions.StopWhen` lets you stop the loop once your condition is met.

```go
req := ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    // ...
    ToolLoop: &ai.ToolLoopOptions{
      MaxIterations: 20,
      StopWhen: ai.StepCountIs(5),
    },
  },
}
```

## Conversation Continuation (`response.messages`)

When you want to keep a chat history, append the call’s produced assistant/tool messages:

```go
history := []ai.Message{
  ai.User("Use tools if you need to."),
}

resp, _ := ai.GenerateText(ctx, ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    Model:    openai.Chat("gpt-4o-mini"),
    Messages: history,
    Tools:    []ai.Tool{add},
  },
})

history = append(history, resp.Response.Messages...)
```

For streaming:

```go
stream, _ := ai.StreamText(ctx, ai.StreamTextRequest{BaseRequest: ai.BaseRequest{ /* ... */ }})
for stream.Next() { fmt.Print(stream.Delta()) }
history = append(history, stream.Response().Messages...)
```

## Request Controls (Headers / Retries / Timeout)

### Per-request headers

```go
resp, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    // ...
    Headers: map[string]string{
      "X-Request-ID": "abc123",
    },
  },
})
```

### Per-request retries

```go
retries := 0
resp, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    // ...
    MaxRetries: &retries, // disable retries for this call
  },
})
```

### Per-request timeout

```go
resp, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    // ...
    Timeout: 5 * time.Second,
  },
})
```

## What’s next?

If you want, the next doc can cover:

- Structured output (`GenerateObject` / `StreamObject`)
- Embeddings (`Embed` / `EmbedMany` + cosine similarity)
- Image generation (`GenerateImage`)
- Audio (`Transcribe`, `GenerateSpeech`)
- MCP (`mcp.NewClient`, tools/resources/prompts, auth/OAuth, notifications)
