# OpenAI Provider

The OpenAI provider supports the Responses API, Chat Completions, and legacy Completions modes.
It also includes embeddings, image generation, speech synthesis, and transcription models.

## Configuration

- Environment variable: `OPENAI_API_KEY`
- Settings fields: `APIKey`, `BaseURL`, `Organization`, `Project`, `Headers`, `ProviderName`

Create the provider with:

```go
client := openai.CreateOpenAI(openai.Settings{
    APIKey: "...",
})
```

## Language Models

```go
model, _ := client.LanguageModel("gpt-4o")
result, _ := model.DoGenerate(ctx, provider.LanguageModelV3CallOptions{
    Prompt: provider.Prompt{
        Messages: []provider.ModelMessage{
            {Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
        },
    },
})
_ = result
```

Use provider options to select the API mode:

- `mode: "responses"` for the Responses API
- `mode: "completions"` for legacy text completions

```go
options := provider.LanguageModelV3CallOptions{
    Prompt: prompt,
    ProviderOptions: provider.ProviderOptions{
        "openai": provider.JSONObject{
            "mode": "responses",
        },
    },
}
```

## Tools and Structured Output

```go
options := provider.LanguageModelV3CallOptions{
    Prompt: prompt,
    ToolChoice: &provider.ToolChoice{Type: provider.ToolChoiceTypeRequired},
    ResponseFormat: &provider.ResponseFormat{
        Type:   provider.ResponseFormatTypeJSON,
        Schema: provider.JSONObject{"type": "object"},
    },
    ProviderOptions: provider.ProviderOptions{
        "openai": provider.JSONObject{
            "tools": []providerutils.ToolSpecification{
                {
                    Name: "search",
                    Parameters: provider.JSONObject{"type": "object"},
                },
            },
        },
    },
}
```

## Request Overrides

Pass OpenAI-specific fields via provider options:

```go
provider.ProviderOptions{
    "openai": provider.JSONObject{
        "user": "beta",
        "request": provider.JSONObject{
            "metadata": "trace",
        },
    },
}
```

## Embeddings, Images, Speech, Transcription

```go
embeddingModel, _ := client.EmbeddingModel("text-embedding-3-large")
imageModel, _ := client.ImageModel("gpt-image-1")
speechModel, _ := client.SpeechModel("gpt-4o-mini-tts")
transcriptionModel, _ := client.TranscriptionModel("whisper-1")
```
