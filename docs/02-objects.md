# Structured Outputs (Objects)

This guide covers:

- `ai.GenerateObject[T]` — produce a JSON object that matches a JSON Schema and decode it into `T`
- `ai.StreamObject[T]` — stream partial JSON arguments and get a final decoded `T`

These APIs are designed to feel similar to “structured output” patterns in other SDKs, while staying provider-agnostic in the public surface.

## Quick Start: `GenerateObject`

Define a Go type and a JSON Schema that describes it, then call `GenerateObject[T]`:

```go
package main

import (
  "context"
  "fmt"
  "os"

  "github.com/bitop-dev/ai"
  "github.com/bitop-dev/ai/openai"
)

type Recipe struct {
  Name        string   `json:"name"`
  Ingredients []string `json:"ingredients"`
  Steps       []string `json:"steps"`
}

func main() {
  openai.Configure(openai.Config{APIKey: os.Getenv("OPENAI_API_KEY")})

  schema := ai.JSONSchema([]byte(`{
    "type": "object",
    "properties": {
      "name": {"type":"string"},
      "ingredients": {"type":"array","items":{"type":"string"}},
      "steps": {"type":"array","items":{"type":"string"}}
    },
    "required": ["name","ingredients","steps"],
    "additionalProperties": false
  }`))

  resp, err := ai.GenerateObject[Recipe](context.Background(), ai.GenerateObjectRequest[Recipe]{
    BaseRequest: ai.BaseRequest{
      Model: openai.Chat("gpt-4o-mini"),
      Messages: []ai.Message{
        ai.User("Generate a lasagna recipe."),
      },
    },
    Schema: schema,
  })
  if err != nil {
    panic(err)
  }

  fmt.Println(resp.Object.Name)
  fmt.Println("ingredients:", len(resp.Object.Ingredients))
  fmt.Println("steps:", len(resp.Object.Steps))
}
```

The response includes:

- `resp.Object` — decoded Go value of type `T`
- `resp.RawJSON` — the raw JSON that was validated/decoded
- `resp.Usage` — token usage (aggregated across internal retries/loops)
- `resp.Message` — the final assistant message
- `resp.FinishReason` — provider finish reason

## What the library does internally

To enforce “object output”, the library injects a synthetic tool named `__ai_return_json` and asks the model to call it with valid JSON arguments.

- This works with providers that support tool calling.
- Tool name collisions are checked (you can’t define a tool with the same name).
- Schema validation uses `santhosh-tekuri/jsonschema/v5`.

You don’t need to call `__ai_return_json` yourself; it’s internal plumbing.

## Strict vs Non-Strict Mode

`GenerateObject` can fail for two broad reasons:

1. The provider call fails (network/auth/rate limit/etc).
2. The model returns output that cannot be validated/decoded into your schema.

Use `Strict` to choose behavior when (2) happens.

### Strict (default)

In strict mode, invalid output returns an error.

```go
strict := true
resp, err := ai.GenerateObject[Recipe](ctx, ai.GenerateObjectRequest[Recipe]{
  BaseRequest: ai.BaseRequest{ /* ... */ },
  Schema: schema,
  Strict: &strict,
})
```

### Non-strict

In non-strict mode, the call returns:

- best-effort `Object` (may be zero value if decoding failed),
- `RawJSON` (what the model returned),
- and `ValidationError` populated.

```go
strict := false
resp, err := ai.GenerateObject[Recipe](ctx, ai.GenerateObjectRequest[Recipe]{
  BaseRequest: ai.BaseRequest{ /* ... */ },
  Schema: schema,
  Strict: &strict,
})
if err != nil {
  // Provider failures still return an error
  panic(err)
}
if resp.ValidationError != nil {
  fmt.Println("validation failed:", resp.ValidationError)
  fmt.Println("raw:", string(resp.RawJSON))
}
```

## Retrying invalid outputs (`MaxRetries`)

You can allow the library to retry when the model produces invalid JSON / schema violations.

```go
retries := 2 // 2 retries = 3 attempts total
resp, err := ai.GenerateObject[Recipe](ctx, ai.GenerateObjectRequest[Recipe]{
  BaseRequest: ai.BaseRequest{ /* ... */ },
  Schema:     schema,
  MaxRetries: &retries,
})
```

Notes:

- This is *schema/parse retry* inside `GenerateObject`, not HTTP retry.
- HTTP retry is controlled separately (see `BaseRequest.MaxRetries` in `docs/01-getting-started.md`).

## Streaming: `StreamObject`

`StreamObject[T]` streams *partial tool-call argument JSON* (the model is streaming the JSON it will eventually submit to `__ai_return_json`).

This is similar to “partial object stream” patterns in other SDKs.

### Example

```go
stream, err := ai.StreamObject[Recipe](ctx, ai.StreamObjectRequest[Recipe]{
  BaseRequest: ai.BaseRequest{
    Model: openai.Chat("gpt-4o-mini"),
    Messages: []ai.Message{
      ai.User("Generate a lasagna recipe."),
    },
  },
  Schema: schema,
})
if err != nil {
  log.Fatal(err)
}
defer stream.Close()

for stream.Next() {
  // Raw is the accumulated JSON args so far; it may be invalid mid-stream.
  raw := stream.Raw()

  // Partial is best-effort parsed data when raw is currently valid JSON.
  // It will return nil until it can be parsed.
  partial := stream.Partial()

  _ = raw
  _ = partial
}

if err := stream.Err(); err != nil {
  log.Fatal(err)
}

obj := stream.Object()
fmt.Printf("final: %#v\n", obj)
```

### How `Partial()` works

`Partial()` returns `map[string]any` and is updated whenever the currently accumulated JSON becomes parseable.

For example, you can “render progress” as the object fills in:

```go
for stream.Next() {
  if partial := stream.Partial(); partial != nil {
    fmt.Printf("partial keys: %v\n", maps.Keys(partial))
  }
}
```

## Using tools together with objects

`GenerateObject` and `StreamObject` can run tool loops *in addition* to the internal `__ai_return_json` enforcement tool.

Example: call a tool to fetch data, then return an object:

```go
type Result struct {
  Answer string `json:"answer"`
}

schema := ai.JSONSchema([]byte(`{
  "type":"object",
  "properties":{"answer":{"type":"string"}},
  "required":["answer"],
  "additionalProperties":false
}`))

lookup := ai.NewTool("lookup", ai.ToolSpec[struct {
  Query string `json:"query"`
}, map[string]any]{
  Execute: func(ctx context.Context, in struct{ Query string `json:"query"` }, meta ai.ToolExecutionMeta) (map[string]any, error) {
    _ = ctx
    return map[string]any{"result": "some data for " + in.Query}, nil
  },
})

resp, err := ai.GenerateObject[Result](ctx, ai.GenerateObjectRequest[Result]{
  BaseRequest: ai.BaseRequest{
    Model: openai.Chat("gpt-4o-mini"),
    Messages: []ai.Message{
      ai.User("Lookup the latest value for X, then respond with {\"answer\": ...}."),
    },
    Tools: []ai.Tool{lookup},
    ToolLoop: &ai.ToolLoopOptions{MaxIterations: 5},
  },
  Schema: schema,
})
```

## Common pitfalls

### 1) Schema and struct tags must match

If your schema expects `snake_case` but your struct tags use `camelCase`, decoding will fail.
Prefer explicit JSON tags and keep schema aligned.

### 2) Use `additionalProperties: false` when you want tight output

If you allow additional properties, models may add fields you don’t expect.

### 3) Arrays and required fields

Models often forget required fields unless the schema is explicit and strict.

### 4) Streaming: `Raw()` is often invalid JSON mid-stream

That’s expected; only the final value must be valid.

## Next doc

If you want, the next doc can cover embeddings or MCP.

