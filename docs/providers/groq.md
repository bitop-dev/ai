# Groq

Use this provider to connect to Groq language models via the Groq OpenAI-compatible API.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variable: `GROQ_API_KEY`

```go
client := groq.CreateGroq(groq.Settings{
    APIKey:  os.Getenv("GROQ_API_KEY"),
    BaseURL: "https://api.groq.com/openai/v1",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Language Models

```go
model, _ := client.LanguageModel("llama3-8b-8192")
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

- Base URL: `https://api.groq.com/openai/v1`
- Provider name: `groq`
