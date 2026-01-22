# providerutils package

The `providerutils` package bundles helper utilities for provider
implementations, tool execution, schema validation, JSON parsing, HTTP retries,
and SSE streaming.

## Tools

Tool helpers map tool definitions to provider tool calls and results.

```go
tool := providerutils.ToolDefinition{
    Name:       "search",
    Parameters: provider.JSONObject{"type": "object"},
    Execute: func(ctx context.Context, call providerutils.ToolCall) (providerutils.ToolOutput, error) {
        return providerutils.ToolTextOutput{Text: "ok"}, nil
    },
}

result, err := providerutils.ExecuteTool(ctx, tool, providerutils.ToolCall{ID: "1", Name: "search"})
```

`ToolArgumentAccumulator` helps assemble tool call arguments from streaming
deltas.

## Schema helpers

- `Schema` wraps a JSON schema with an optional `SchemaValidator`.
- `StrictJSONSchema` and `LenientJSONSchema` adjust additional properties.

## JSON parsing

- `ParseJSON` and `SafeParseJSON` parse and validate JSON payloads.
- `JSONParseError` and `JSONValidationError` surface parse/validation failures.
- `ToJSONValue` converts arbitrary values into JSON-compatible types.

## HTTP helpers

Helpers for building requests and handling retries:

- `BuildRequest`, `PostJSON`, `PostMultipart`
- `RetryPolicy`, `DefaultRetryPolicy`, `APICallHook`
- `MergeHeaders`, `IsRetryableError`, `BackoffDelay`

## SSE helpers

- `ParseSSE` parses Server-Sent Events from an `io.Reader`.
- `WriteSSE` and `PipeSSE` emit SSE events.

## Misc utilities

- `GenerateID`, `IDGenerator` for UUID-style IDs.
- `NowUTC`, `FormatTimestamp`, `ParseTimestamp` time helpers.
- `Logger` and `NopLogger` logging interface.
