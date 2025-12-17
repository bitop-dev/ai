# Errors, Retries, Timeouts, Cancellation

This guide describes:

- Provider errors (`*ai.Error`)
- Tool errors (`NoSuchToolError`, `InvalidToolInputError`, `ToolExecutionError`)
- Feature-specific errors (e.g. `NoImageGeneratedError`)
- HTTP retries vs schema retries
- Timeouts and cancellation

## Provider errors (`*ai.Error`)

Most provider failures are returned as `*ai.Error`:

```go
resp, err := ai.GenerateText(ctx, req)
if err != nil {
  var e *ai.Error
  if errors.As(err, &e) {
    fmt.Println("provider:", e.Provider)
    fmt.Println("code:", e.Code)
    fmt.Println("status:", e.Status)
    fmt.Println("retryable:", e.Retryable)
  }
}
_ = resp
```

### Helpers

```go
if ai.IsRateLimited(err) { /* ... */ }
if ai.IsAuth(err) { /* ... */ }
if ai.IsTimeout(err) { /* ... */ }
if ai.IsCanceled(err) { /* ... */ }
```

## Tool errors

Tool-related errors are surfaced as typed errors:

- `*ai.NoSuchToolError` — model requested an unknown tool name
- `*ai.InvalidToolInputError` — tool arguments did not match the schema
- `*ai.ToolExecutionError` — your tool handler returned an error

```go
resp, err := ai.GenerateText(ctx, req)
if err != nil {
  switch {
  case ai.IsRateLimited(err):
    // backoff
  default:
    var noSuch *ai.NoSuchToolError
    var invalid *ai.InvalidToolInputError
    var exec *ai.ToolExecutionError
    switch {
    case errors.As(err, &noSuch):
      fmt.Println("unknown tool:", noSuch.ToolName)
    case errors.As(err, &invalid):
      fmt.Println("invalid tool input:", invalid.ToolName, invalid.ToolCallID)
    case errors.As(err, &exec):
      fmt.Println("tool execution failed:", exec.ToolName, exec.ToolCallID, exec.Cause)
    }
  }
}
_ = resp
```

## Feature-specific errors

Some APIs have specialized “no output produced” errors:

- `*ai.NoImageGeneratedError` (`ai.IsNoImageGenerated`)
- `*ai.NoTranscriptGeneratedError` (`ai.IsNoTranscriptGenerated`)
- `*ai.NoSpeechGeneratedError` (`ai.IsNoSpeechGenerated`)

These indicate a successful provider call that did not return usable output.

## Retries

There are two layers of “retry” in the library:

### 1) HTTP retries (provider transport)

These are provider-level retries for transient HTTP failures (timeouts/429/5xx, etc.).

Controls:

- Global default: `openai.Config.MaxRetries`
- Per-request override:
  - text/object: `BaseRequest.MaxRetries`
  - embeddings/images/audio: `MaxRetries` on their request structs

Example:

```go
retries := 0
resp, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    // ...
    MaxRetries: &retries,
  },
})
```

### 2) Schema/parse retries (GenerateObject only)

`GenerateObject` has an additional retry loop when model output does not validate against the schema.

Control:

- `GenerateObjectRequest.MaxRetries`

This is described in `docs/02-objects.md`.

## Timeouts

All public endpoints support per-request timeouts:

- text/object: `BaseRequest.Timeout`
- embeddings/images/audio: `Timeout` field on the request struct

```go
resp, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    // ...
    Timeout: 5 * time.Second,
  },
})
```

Timeouts are implemented with `context.WithTimeout`.

## Cancellation

Cancellation is always controlled by the context you pass in:

```go
ctx, cancel := context.WithCancel(context.Background())
cancel()

_, err := ai.GenerateText(ctx, req)
if ai.IsCanceled(err) {
  fmt.Println("canceled")
}
```

## Headers

Per-request headers:

- text/object: `BaseRequest.Headers`
- embeddings/images/audio: `Headers` fields on request structs

```go
resp, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    // ...
    Headers: map[string]string{"X-Request-ID": "abc123"},
  },
})
```

