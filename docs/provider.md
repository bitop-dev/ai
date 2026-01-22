# provider package

The `provider` package defines shared interfaces, types, and error taxonomy for
model providers. Provider implementations and adapters should align with these
interfaces.

Unversioned aliases (`Provider`, `LanguageModel`, etc.) map to the current
provider specification (v3).

## Provider interfaces

`Provider` exposes language, embedding, and image models:

```go
type Provider interface {
    SpecificationVersion() SpecificationVersion
    LanguageModel(modelID ModelID) (LanguageModel, error)
    EmbeddingModel(modelID ModelID) (EmbeddingModel, error)
    ImageModel(modelID ModelID) (ImageModel, error)
}
```

Optional capability interfaces:

- `TextEmbeddingProvider` (legacy text embeddings)
- `TranscriptionProvider`
- `SpeechProvider`
- `RerankingProvider`

## Model interfaces

Each model interface includes IDs and call methods:

- `LanguageModel` (`DoGenerate`, `DoStream`)
- `EmbeddingModel` (`DoEmbed`)
- `ImageModel` (`DoGenerate`)
- `SpeechModel` (`DoGenerate`)
- `TranscriptionModel` (`DoGenerate`)
- `RerankingModel` (`DoRerank`)

## Prompts and content

Prompts are structured as messages with content parts:

```go
prompt := provider.Prompt{
    Messages: []provider.ModelMessage{
        {
            Role: provider.RoleUser,
            Content: []provider.ContentPart{
                provider.TextContent{Text: "Hello"},
            },
        },
    },
}
```

Content part variants include text, tool calls/results, sources, reasoning,
images, and files.

## Call options and overrides

`LanguageModelCallOptions` contains generation parameters, response format,
tool choice, and request overrides. `RequestOptions` carries headers, timeout,
idempotency keys, and provider-specific options:

```go
options := provider.LanguageModelCallOptions{
    Prompt: prompt,
    RequestOptions: provider.RequestOptions{
        Headers: map[string]string{"X-Trace": "1"},
        ProviderOptions: provider.ProviderOptions{
            "openai": provider.JSONObject{"mode": "responses"},
        },
    },
}
```

`ResponseFormat` controls JSON vs text output, and `ToolChoice` controls tool
selection behavior.

## Streaming parts

`StreamPart` represents streaming deltas and metadata. Core parts include:

- Text start/delta/end
- Tool call and tool input parts
- Reasoning and source parts
- Response metadata and finish parts
- Error parts and raw payloads

`Finish` includes the finish reason and usage metrics.

## Errors

Errors are modeled with `AISDKError` and specialized types:

- `ApiCallError`, `AuthenticationError`, `RateLimitError`
- `InvalidRequestError`, `InvalidPromptError`, `InvalidResponseDataError`
- `NoSuchModelError`, `UnsupportedFunctionalityError`

Helper predicates such as `IsRateLimitError` and `IsNoSuchModelError` simplify
classification.

## JSON types

JSON helpers include `JSONValue`, `JSONObject`, `JSONArray`, and `JSONSchema`
(an alias of `JSONValue`). Use these to express response formats and schemas.
