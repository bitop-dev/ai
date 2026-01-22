# Google Vertex Provider

The Google Vertex provider supports Gemini language models and embeddings via Vertex AI.

## Configuration

- Environment variables: `GOOGLE_VERTEX_PROJECT`, `GOOGLE_VERTEX_LOCATION`, `GOOGLE_VERTEX_API_KEY`
- Service account ADC: `GOOGLE_APPLICATION_CREDENTIALS`
- Settings fields: `APIKey`, `Project`, `Location`, `BaseURL`, `Headers`, `ProviderName`, `AccessToken`, `CredentialsJSON`, `CredentialsFile`

Create the provider with:

```go
client := googlevertex.CreateGoogleVertex(googlevertex.Settings{
    Project:  "my-project",
    Location: "us-central1",
})
```

### Authentication

- Use `CredentialsJSON` or `CredentialsFile` for service account credentials.
- Leave credentials unset to use Application Default Credentials (metadata server).
- Use `APIKey` to enable express mode with API key authentication.

## Language Models

```go
model, _ := client.LanguageModel("gemini-2.5-flash")
_, _ = model.DoGenerate(ctx, provider.LanguageModelV3CallOptions{
    Prompt: provider.Prompt{
        Messages: []provider.ModelMessage{
            {Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
        },
    },
})
```

### Tools and Structured Output

```go
options := provider.LanguageModelV3CallOptions{
    Prompt: prompt,
    ToolChoice: &provider.ToolChoice{Type: provider.ToolChoiceTypeRequired},
    ResponseFormat: &provider.ResponseFormat{
        Type:   provider.ResponseFormatTypeJSON,
        Schema: provider.JSONObject{"type": "object"},
    },
    ProviderOptions: provider.ProviderOptions{
        "google-vertex": provider.JSONObject{
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
model, _ := client.EmbeddingModel("text-embedding-004")
_, _ = model.DoEmbed(ctx, provider.EmbeddingModelV3CallOptions{
    Values: []string{"hello", "world"},
    RequestOptions: provider.RequestOptions{ProviderOptions: provider.ProviderOptions{
        "google-vertex": provider.JSONObject{
            "outputDimensionality": 512,
        },
    }},
})
```
