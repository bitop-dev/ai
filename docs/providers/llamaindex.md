# LlamaIndex

Use this adapter to connect to LlamaIndex deployments that expose OpenAI-compatible chat endpoints.

## Configuration

- Environment variable: `LLAMAINDEX_API_KEY`
- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`

```go
client := llamaindex.CreateLlamaIndex(llamaindex.Settings{
    APIKey:  os.Getenv("LLAMAINDEX_API_KEY"),
    BaseURL: "http://localhost:8000/v1",
})
```

## Language Models

```go
model, _ := client.LanguageModel("llamaindex-chat")
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

Tool calls stream through OpenAI-compatible deltas and emit `tool-call` parts.

```go
options := provider.LanguageModelV3CallOptions{
    Prompt: provider.Prompt{Messages: []provider.ModelMessage{{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Use a tool"}}}}},
    ToolChoice: &provider.ToolChoice{Type: provider.ToolChoiceTypeRequired},
}
```

## Streaming

Streaming uses the OpenAI-compatible `chat/completions` SSE format and supports tool call deltas.

## Request Overrides

```go
provider.ProviderOptions{
    "llamaindex": provider.JSONObject{
        "request": provider.JSONObject{
            "metadata": "trace",
        },
    },
}
```

## Defaults

- Base URL: `http://localhost:8000/v1`
- Provider name: `llamaindex`

## Limitations

- Language model adapter only; embeddings and image generation are not supported.
- Requires a LlamaIndex deployment that exposes OpenAI-compatible endpoints.

## Checklist

- [x] Settings struct and defaults documented.
- [x] Language model request/response mapping documented.
- [x] Tool mapping documented.
- [x] Streaming mapping documented (if supported).
- [x] Errors and usage mapping documented.
- [x] Provider options documented.
