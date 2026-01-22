# ai package

The `ai` package provides high-level helpers for invoking models, collecting
streaming responses, and orchestrating tool calls. It wraps provider interfaces
so callers can focus on prompts and results.

## Text generation

```go
prompt := provider.Prompt{
    Messages: []provider.ModelMessage{
        {
            Role: provider.RoleUser,
            Content: []provider.ContentPart{
                provider.TextContent{Text: "Summarize the Pacific Ocean in one sentence."},
            },
        },
    },
}

result, err := ai.GenerateText(ctx, model, ai.GenerateTextOptions{Prompt: prompt})
if err != nil {
    return err
}
fmt.Println(result.Text)
```

Common `TextOptions` fields:

- `Prompt`: chat prompt with messages and content parts.
- `MaxOutputTokens`, `Temperature`, `TopP`, `TopK`.
- `StopSequences`, `PresencePenalty`, `FrequencyPenalty`, `Seed`.
- `ResponseFormat` and `ToolChoice` for structured output and tool use.
- `RequestOptions` for headers, timeouts, idempotency keys, provider overrides.
- `SchemaValidator` for structured output validation.
- `Telemetry` to capture request/usage events.

## Streaming text

```go
streamResult, err := ai.StreamText(ctx, model, ai.StreamTextOptions{Prompt: prompt})
if err != nil {
    return err
}
defer streamResult.Stream.Close()

for streamResult.Stream.Next() {
    part := streamResult.Stream.Value()
    _ = part
}
if err := streamResult.Stream.Err(); err != nil {
    return err
}
```

Use `ai.PipeStream` to emit stream parts as SSE from an HTTP handler.

## Structured output

`GenerateObject` and `StreamObject` enforce JSON responses. The helpers set
`ResponseFormat.Type` to `json` automatically and can validate against a schema.

```go
options := ai.GenerateObjectOptions{
    Prompt: prompt,
    ResponseFormat: &provider.ResponseFormat{
        Schema: provider.JSONObject{"type": "object"},
    },
}
result, err := ai.GenerateObject(ctx, model, options)
```

`StreamObjectResult.Collect` provides a convenience helper for collecting the
stream and parsing the final object.

## Tool loop

`GenerateTextWithTools` handles multi-step tool calling. Provide tool
definitions, optional approval hooks, and a max step count.

```go
result, err := ai.GenerateTextWithTools(ctx, model, ai.ToolLoopOptions{
    TextOptions: ai.TextOptions{Prompt: prompt},
    Tools: []providerutils.ToolDefinition{
        {
            Name:       "search",
            Parameters: provider.JSONObject{"type": "object"},
            Execute: func(ctx context.Context, call providerutils.ToolCall) (providerutils.ToolOutput, error) {
                return providerutils.ToolTextOutput{Text: "ok"}, nil
            },
        },
    },
})
```

## Other modalities

Convenience helpers call the provider interfaces directly:

- `Embed` for embeddings.
- `GenerateImage` for images.
- `GenerateSpeech` for TTS.
- `Transcribe` for audio transcription.
- `Rerank` for reranking models.

## Registry integration

Use `ResolveModel` to fetch a language model from a provider registry:

```go
model, err := ai.ResolveModel(registry, "openai:gpt-4o-mini")
```

## Telemetry

Provide a `Telemetry` implementation in `TextOptions` to capture latency,
usage, warnings, and metadata. The package ships with `NoopTelemetry` and
`NoopSpan` defaults.
