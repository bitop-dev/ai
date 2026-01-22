# DeepSeek

Use this provider to connect to DeepSeek language models via the DeepSeek OpenAI-compatible API.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variable: `DEEPSEEK_API_KEY`

```go
client := deepseek.CreateDeepSeek(deepseek.Settings{
    APIKey:  os.Getenv("DEEPSEEK_API_KEY"),
    BaseURL: "https://api.deepseek.com",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Language Models

```go
model, _ := client.LanguageModel("deepseek-chat")
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

- Base URL: `https://api.deepseek.com`
- Provider name: `deepseek`
