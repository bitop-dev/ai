# TogetherAI

Use this provider to connect to Together.ai models via the Together OpenAI-compatible API.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`, `ModelPrefix`
- Environment variable: `TOGETHER_AI_API_KEY`

```go
client := togetherai.CreateTogetherAI(togetherai.Settings{
    APIKey:  os.Getenv("TOGETHER_AI_API_KEY"),
    BaseURL: "https://api.together.xyz/v1",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Language Models

```go
model, _ := client.LanguageModel("meta-llama/Llama-3-70b-chat-hf")
result, _ := model.DoGenerate(ctx, provider.LanguageModelCallOptions{
    Prompt: provider.Prompt{
        Messages: []provider.ModelMessage{
            {Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
        },
    },
})
_ = result
```

## Model Prefix Mapping

- If a model ID starts with `togetherai/`, the prefix is stripped before sending the request.
- Customize the prefix with `Settings.ModelPrefix` when needed.

## Defaults

- Base URL: `https://api.together.xyz/v1`
- Provider name: `togetherai`
- Model prefix: `togetherai/`
