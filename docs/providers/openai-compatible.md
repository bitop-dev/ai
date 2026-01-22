# OpenAI Compatible

Use this provider to connect to OpenAI-compatible endpoints (chat, completions, responses, embeddings, images).

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `QueryParams`, `HTTPClient`, `ProviderName`

```go
client := openaicompatible.CreateOpenAICompatible(openaicompatible.Settings{
    APIKey:  os.Getenv("PROVIDER_API_KEY"),
    BaseURL: "https://api.provider.com/v1",
    Headers: map[string]string{
        "X-Provider": "example",
    },
    QueryParams: map[string]string{
        "api-version": "2024-10-01",
    },
})
```

## Language Models

```go
model, _ := client.LanguageModel("model-id")
result, _ := model.DoGenerate(ctx, provider.LanguageModelCallOptions{
    Prompt: provider.Prompt{
        Messages: []provider.ModelMessage{
            {Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
        },
    },
})
_ = result
```

## Tools and Structured Output

Provider options use the provider name as the key (defaults to `openai-compatible`).

```go
options := provider.LanguageModelCallOptions{
    Prompt: prompt,
    ToolChoice: &provider.ToolChoice{Type: provider.ToolChoiceTypeRequired},
    ResponseFormat: &provider.ResponseFormat{
        Type:   provider.ResponseFormatTypeJSON,
        Schema: provider.JSONObject{"type": "object"},
    },
    ProviderOptions: provider.ProviderOptions{
        "openai-compatible": provider.JSONObject{
            "tools": []providerutils.ToolSpecification{
                {
                    Name: "tool",
                    Parameters: provider.JSONObject{"type": "object"},
                },
            },
        },
    },
}
```

## Streaming

- Chat and completions streaming map text deltas to text stream parts.
- Responses streaming maps `response.output_text.delta` to text parts.
- Finish reasons map OpenAI-compatible reasons to AI SDK finish reasons.

## Request Overrides

```go
provider.ProviderOptions{
    "openai-compatible": provider.JSONObject{
        "request": provider.JSONObject{
            "user": "client-id",
        },
        "mode": "responses",
    },
}
```

## Limitations

- The base URL must be provided for non-OpenAI endpoints.
