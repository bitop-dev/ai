# Cohere

Use this provider to connect to the Cohere API for chat and embedding models.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`

```go
client := cohere.CreateCohere(cohere.Settings{
    APIKey:  os.Getenv("COHERE_API_KEY"),
    BaseURL: "https://api.cohere.com/v2",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Language Models

```go
model, _ := client.LanguageModel("command")
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

Provider options use the provider name as the key (defaults to `cohere`).

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
        "cohere": provider.JSONObject{
            "tools": []providerutils.ToolSpecification{
                {
                    Name:       "tool",
                    Parameters: provider.JSONObject{"type": "object"},
                },
            },
            "thinking": provider.JSONObject{
                "type":        "enabled",
                "tokenBudget": 120,
            },
        },
    },
}
```

## Embeddings

```go
embeddingModel, _ := client.EmbeddingModel("embed-english-v3.0")
result, _ := embeddingModel.DoEmbed(ctx, provider.EmbeddingModelV3CallOptions{
    Values: []string{"hello"},
    RequestOptions: provider.RequestOptions{
        ProviderOptions: provider.ProviderOptions{
            "cohere": provider.JSONObject{
                "inputType": "search_query",
            },
        },
    },
})
_ = result
```

## Streaming

- Streaming uses `/chat` SSE responses.
- Text and reasoning deltas map to text/reasoning stream parts.
- Tool-call deltas emit tool input and tool call stream parts.

## Reranking

Reranking models are not yet available in the Go SDK because the reranking call
options are still being defined. This provider returns an unsupported
functionality error for `RerankingModel` for now.

## Request Overrides

```go
provider.ProviderOptions{
    "cohere": provider.JSONObject{
        "request": provider.JSONObject{
            "preamble": "system",
        },
    },
}
```
