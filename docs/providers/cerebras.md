# Cerebras

Use this provider to connect to Cerebras language models via the Cerebras OpenAI-compatible API.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variable: `CEREBRAS_API_KEY`

```go
client := cerebras.CreateCerebras(cerebras.Settings{
    APIKey:  os.Getenv("CEREBRAS_API_KEY"),
    BaseURL: "https://api.cerebras.ai/v1",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Language Models

```go
model, _ := client.LanguageModel("llama-3.3-70b")
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

- Base URL: `https://api.cerebras.ai/v1`
- Provider name: `cerebras`
