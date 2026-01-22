# Provider Template

Use this outline when porting a provider in Phase 8.
Replace the placeholder names with the provider-specific values.

## Configuration

- Environment variable: `PROVIDER_API_KEY`
- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`

```go
client := providername.CreateProvider(providername.Settings{
    APIKey: "...",
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

```go
options := provider.LanguageModelCallOptions{
    Prompt: prompt,
    ToolChoice: &provider.ToolChoice{Type: provider.ToolChoiceTypeRequired},
    ResponseFormat: &provider.ResponseFormat{
        Type:   provider.ResponseFormatTypeJSON,
        Schema: provider.JSONObject{"type": "object"},
    },
    ProviderOptions: provider.ProviderOptions{
        "providername": provider.JSONObject{
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

Document streaming support, any provider-specific events, and how finish reasons map.

## Request Overrides

```go
provider.ProviderOptions{
    "providername": provider.JSONObject{
        "request": provider.JSONObject{
            "metadata": "trace",
        },
    },
}
```

## Limitations

- List known gaps or API constraints.

## Checklist

- [ ] Settings struct and defaults documented.
- [ ] Language model request/response mapping documented.
- [ ] Tool mapping documented.
- [ ] Streaming mapping documented (if supported).
- [ ] Errors and usage mapping documented.
- [ ] Provider options documented.
