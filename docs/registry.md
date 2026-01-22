# registry package

The `registry` package manages providers and resolves models by a composite
identifier (for example, `openai:gpt-4o-mini`).

## Create a registry

```go
providers := map[provider.ProviderID]provider.Provider{
    "openai": openai.CreateOpenAI(openai.Settings{}),
    "anthropic": anthropic.CreateAnthropic(anthropic.Settings{}),
}

reg := registry.CreateProviderRegistry(providers, registry.Options{})
```

## Resolve models

```go
model, err := reg.LanguageModel("openai:gpt-4o-mini")
```

Registry methods exist for embeddings, images, transcription, speech, and
reranking models. Errors surface when a provider is missing or does not support
the requested model type.

## Options

- `Separator` controls the `providerId:modelId` delimiter (default `:`).
- `LanguageModelMiddleware` and `ImageModelMiddleware` wrap resolved models.

## Errors

- `NoSuchProviderError` indicates a missing provider entry.
- `provider.NoSuchModelError` indicates the model ID is invalid.
- `provider.UnsupportedFunctionalityError` indicates unsupported model types.
