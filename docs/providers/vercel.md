# Vercel

Use this provider to connect to Vercel v0 models via the v0 API.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variable: `VERCEL_API_KEY`

```go
client := vercel.CreateVercel(vercel.Settings{
    APIKey:  os.Getenv("VERCEL_API_KEY"),
    BaseURL: "https://api.v0.dev/v1",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Language Models

```go
model, _ := client.LanguageModel("v0-1.5-md")
result, _ := model.DoGenerate(ctx, provider.LanguageModelV3CallOptions{
    Prompt: provider.Prompt{
        Messages: []provider.ModelMessage{
            {Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
        },
    },
})
_ = result
```

## Defaults

- Base URL: `https://api.v0.dev/v1`
- Provider name: `vercel`
