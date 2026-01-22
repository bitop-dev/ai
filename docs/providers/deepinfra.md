Use this provider to connect to DeepInfra models via the OpenAI-compatible API.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variable: `DEEPINFRA_API_KEY`

```go
client := deepinfra.CreateDeepInfra(deepinfra.Settings{
    APIKey:  os.Getenv("DEEPINFRA_API_KEY"),
    BaseURL: "https://api.deepinfra.com/v1",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Language Models

```go
model, _ := client.LanguageModel("meta-llama/Llama-3.1-8B-Instruct")
result, _ := model.DoStream(ctx, provider.LanguageModelCallOptions{
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
embedModel, _ := client.EmbeddingModel("BAAI/bge-large-en-v1.5")
result, _ := embedModel.DoEmbed(ctx, provider.EmbeddingModelCallOptions{
    Values: []string{"hello"},
})
_ = result
```

## Image Models

```go
imageModel, _ := client.ImageModel("stabilityai/sd3.5")
result, _ := imageModel.DoGenerate(ctx, provider.ImageModelCallOptions{
    Prompt:      "A futuristic cityscape",
    AspectRatio: "16:9",
    N:           1,
})
_ = result
```

## Streaming

DeepInfra uses OpenAI-compatible SSE streaming for language models. Token usage
from streamed responses is surfaced on the finish stream part.

## Image Options

DeepInfra image models accept provider-specific parameters via
`ProviderOptions["deepinfra"]`.

```go
options := provider.ImageModelCallOptions{
    Prompt: "A neon skyline",
    ProviderOptions: provider.ProviderOptions{
        "deepinfra": provider.JSONObject{
            "num_inference_steps": 30,
        },
    },
}
```

## Request Overrides

```go
provider.ProviderOptions{
    "deepinfra": provider.JSONObject{
        "request": provider.JSONObject{
            "metadata": "trace",
        },
    },
}
```

## Limitations

- Image editing and mask uploads are not mapped yet.

## Defaults

- Base URL: `https://api.deepinfra.com/v1`
- Provider name: `deepinfra`
- Language/embedding base URL: `{baseURL}/openai`
- Image base URL: `{baseURL}/inference`
