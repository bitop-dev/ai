# Gateway package

The `gateway` package implements the Vercel AI Gateway provider. It supports
language model streaming and exposes gateway-specific model settings and error
mapping.

## Configuration

- Environment variable: `AI_GATEWAY_API_KEY`
- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderID`
- Default base URL: `https://gateway.ai.vercel.com`

```go
gatewayProvider := gateway.CreateGateway(gateway.GatewaySettings{
    APIKey: os.Getenv("AI_GATEWAY_API_KEY"),
})
```

`ProviderID` defaults to `gateway` when unset.

## Usage

```go
model, err := gatewayProvider.LanguageModel("openai/gpt-4o")
if err != nil {
    return err
}

prompt := provider.Prompt{
    Messages: []provider.ModelMessage{
        {
            Role: provider.RoleUser,
            Content: []provider.ContentPart{
                provider.TextContent{Text: "Write a short haiku."},
            },
        },
    },
}

result, err := ai.StreamText(ctx, model, ai.StreamTextOptions{Prompt: prompt})
if err != nil {
    return err
}
defer result.Stream.Close()
for result.Stream.Next() {
    _ = result.Stream.Value()
}
```

See `examples/gateway_streaming/main.go` for a runnable streaming example.

## Streaming behavior

Gateway language models only implement streaming (`DoStream`). `DoGenerate`
returns an `UnsupportedFunctionalityError`. Use `ai.GenerateText` if you need
to collect a full response from the stream.

## Model settings

Gateway models expose settings metadata on the model struct:

- `GatewayLanguageModel.Settings` (tokens, tool support, JSON support)
- `GatewayEmbeddingModel.Settings` (dimensions, max tokens)
- `GatewayImageModel.Settings` (size constraints, aspect ratios)

Use these fields to render UI or validate options.

## Errors and metadata

Gateway errors map to provider errors via `MapGatewayErrorToProvider`. The
mapping preserves request IDs, response headers, and response body text.

## Limitations

- Language model `DoGenerate` is not implemented (streaming only).
- Embedding and image models are not implemented yet and return
  `UnsupportedFunctionalityError`.
