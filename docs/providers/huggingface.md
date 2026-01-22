# Hugging Face

Use this provider to connect to Hugging Face Inference Providers via the OpenAI-compatible API.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variable: `HUGGINGFACE_API_KEY`

```go
client := huggingface.CreateHuggingFace(huggingface.Settings{
    APIKey:  os.Getenv("HUGGINGFACE_API_KEY"),
    BaseURL: "https://router.huggingface.co/v1",
    Headers: map[string]string{
        "X-Region": "global",
    },
})
```

## Language Models

```go
model, _ := client.LanguageModel("meta-llama/Llama-3.1-8B-Instruct")
result, _ := model.DoGenerate(ctx, provider.LanguageModelV3CallOptions{
    Prompt: provider.Prompt{
        Messages: []provider.ModelMessage{
            {Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
        },
    },
})
_ = result
```

## Provider Options

Provider options use the provider name as the key (defaults to `huggingface`).

```go
options := provider.LanguageModelV3CallOptions{
    Prompt: prompt,
    ProviderOptions: provider.ProviderOptions{
        "huggingface": provider.JSONObject{
            "request": provider.JSONObject{
                "user": "client-id",
            },
        },
    },
}
```

## Defaults

- Base URL: `https://router.huggingface.co/v1`
- Provider name: `huggingface`

## Notes

- Model IDs map to Hugging Face model identifiers (for example, `org/model`).
- This adapter targets OpenAI-compatible text generation endpoints.
