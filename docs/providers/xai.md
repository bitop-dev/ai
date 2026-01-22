# xAI

Use this provider to connect to xAI language models via the xAI OpenAI-compatible API.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variable: `XAI_API_KEY`

```go
client := xai.CreateXAI(xai.Settings{
    APIKey:  os.Getenv("XAI_API_KEY"),
    BaseURL: "https://api.x.ai/v1",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Language Models

```go
model, _ := client.LanguageModel("grok-2")
result, _ := model.DoGenerate(ctx, provider.LanguageModelCallOptions{
    Prompt: provider.Prompt{
        Messages: []provider.ModelMessage{
            {Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
        },
    },
})
_ = result
```

## Defaults

- Base URL: `https://api.x.ai/v1`
- Provider name: `xai`
