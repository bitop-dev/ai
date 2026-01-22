# Google Generative AI Provider

The Google provider supports Gemini language models and Google Generative AI embeddings.

## Configuration

- Environment variable: `GOOGLE_GENERATIVE_AI_API_KEY`
- Settings fields: `APIKey`, `BaseURL`, `Headers`, `ProviderName`

Create the provider with:

```go
client := google.CreateGoogle(google.Settings{
    APIKey: "...",
})
```

## Language Models

```go
model, _ := client.LanguageModel("gemini-2.5-flash")
result, _ := model.DoGenerate(ctx, provider.LanguageModelCallOptions{
    Prompt: provider.Prompt{
        Messages: []provider.ModelMessage{
            {Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
        },
    },
})
_ = result
```

### Tools and Structured Output

```go
options := provider.LanguageModelCallOptions{
    Prompt: prompt,
    ToolChoice: &provider.ToolChoice{Type: provider.ToolChoiceTypeRequired},
    ResponseFormat: &provider.ResponseFormat{
        Type:   provider.ResponseFormatTypeJSON,
        Schema: provider.JSONObject{"type": "object"},
    },
    ProviderOptions: provider.ProviderOptions{
        "google": provider.JSONObject{
            "tools": []providerutils.ToolSpecification{
                {Name: "search", Parameters: provider.JSONObject{"type": "object"}},
            },
            "cachedContent": "cachedContents/123",
        },
    },
}
```

## Embeddings

```go
model, _ := client.EmbeddingModel("gemini-embedding-001")
_, _ = model.DoEmbed(ctx, provider.EmbeddingModelCallOptions{
    Values: []string{"hello", "world"},
    RequestOptions: provider.RequestOptions{ProviderOptions: provider.ProviderOptions{
        "google": provider.JSONObject{
            "outputDimensionality": 512,
        },
    }},
})
```
