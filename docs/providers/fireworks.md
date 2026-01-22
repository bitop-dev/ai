# Fireworks

Use this provider to connect to Fireworks models via the OpenAI-compatible API.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variable: `FIREWORKS_API_KEY`

```go
client := fireworks.CreateFireworks(fireworks.Settings{
    APIKey:  os.Getenv("FIREWORKS_API_KEY"),
    BaseURL: "https://api.fireworks.ai/inference/v1",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Language Models

```go
model, _ := client.LanguageModel("accounts/fireworks/models/firefunction-v1")
result, _ := model.DoStream(ctx, provider.LanguageModelV3CallOptions{
    Prompt: provider.Prompt{
        Messages: []provider.ModelMessage{
            {Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
        },
    },
})
_ = result
```

## Embedding Models

```go
embedModel, _ := client.EmbeddingModel("nomic-ai/nomic-embed-text-v1.5")
result, _ := embedModel.DoEmbed(ctx, provider.EmbeddingModelV3CallOptions{
    Values: []string{"hello"},
})
_ = result
```

## Streaming Metadata

Fireworks streaming responses can include provider-specific payloads such as
`safety` and `response_metadata`. These are surfaced in `ProviderMetadata` on
the final stream part.

## Safety Settings

Use provider options to pass Fireworks safety settings into requests:

```go
options := provider.LanguageModelV3CallOptions{
    Prompt: prompt,
    ProviderOptions: provider.ProviderOptions{
        "fireworks": provider.JSONObject{
            "safety_settings": provider.JSONObject{
                "policy": "strict",
            },
        },
    },
}
```

## Request Overrides

```go
provider.ProviderOptions{
    "fireworks": provider.JSONObject{
        "request": provider.JSONObject{
            "metadata": "trace",
        },
    },
}
```

## Limitations

- Image generation endpoints are not mapped yet.

## Defaults

- Base URL: `https://api.fireworks.ai/inference/v1`
- Provider name: `fireworks`
