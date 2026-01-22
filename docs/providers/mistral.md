# Mistral

Use this provider to connect to the Mistral API for chat and embedding models.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`

```go
client := mistral.CreateMistral(mistral.Settings{
    APIKey:  os.Getenv("MISTRAL_API_KEY"),
    BaseURL: "https://api.mistral.ai/v1",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Language Models

```go
model, _ := client.LanguageModel("mistral-large-latest")
result, _ := model.DoGenerate(ctx, provider.LanguageModelV3CallOptions{
    Prompt: provider.Prompt{
        Messages: []provider.ModelMessage{
            {Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
        },
    },
})
_ = result
```

## Tools and Structured Output

Provider options use the provider name as the key (defaults to `mistral`).

```go
options := provider.LanguageModelV3CallOptions{
    Prompt: prompt,
    ToolChoice: &provider.ToolChoice{Type: provider.ToolChoiceTypeRequired},
    ResponseFormat: &provider.ResponseFormat{
        Type:        provider.ResponseFormatTypeJSON,
        Schema:      provider.JSONObject{"type": "object"},
        Name:        "payload",
        Description: "payload schema",
    },
    ProviderOptions: provider.ProviderOptions{
        "mistral": provider.JSONObject{
            "tools": []providerutils.ToolSpecification{
                {
                    Name:       "tool",
                    Parameters: provider.JSONObject{"type": "object"},
                },
            },
            "strictJsonSchema": true,
            "parallelToolCalls": false,
        },
    },
}
```

## Embeddings

```go
embeddingModel, _ := client.EmbeddingModel("mistral-embed")
result, _ := embeddingModel.DoEmbed(ctx, provider.EmbeddingModelV3CallOptions{
    Values: []string{"hello"},
})
_ = result
```

## Streaming

- Streaming uses `/chat/completions` SSE responses.
- Text deltas are mapped to text stream parts.
- Tool-call deltas emit tool input and tool call stream parts.

## Request Overrides

```go
provider.ProviderOptions{
    "mistral": provider.JSONObject{
        "request": provider.JSONObject{
            "user": "client-id",
        },
    },
}
```
