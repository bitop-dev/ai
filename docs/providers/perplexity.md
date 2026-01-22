# Perplexity

Use this provider to connect to Perplexity Sonar models via the OpenAI-compatible API.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variable: `PERPLEXITY_API_KEY`

```go
client := perplexity.CreatePerplexity(perplexity.Settings{
    APIKey:  os.Getenv("PERPLEXITY_API_KEY"),
    BaseURL: "https://api.perplexity.ai",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Language Models

```go
model, _ := client.LanguageModel("sonar-pro")
result, _ := model.DoStream(ctx, provider.LanguageModelCallOptions{
    Prompt: provider.Prompt{
        Messages: []provider.ModelMessage{
            {Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Search"}}},
        },
    },
})
_ = result
```

## Defaults

- Base URL: `https://api.perplexity.ai`
- Provider name: `perplexity`
