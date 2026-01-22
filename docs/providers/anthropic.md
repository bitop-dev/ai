# Anthropic Provider

The Anthropic provider supports Claude language models over the Messages API.
It includes tool calling, structured output, and streaming support.

## Configuration

- Environment variable: `ANTHROPIC_API_KEY`
- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`

Create the provider with:

```go
client := anthropic.CreateAnthropic(anthropic.Settings{
    APIKey: "...",
})
```

## Language Models

```go
model, _ := client.LanguageModel("claude-3-5-sonnet-20240620")
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

```go
options := provider.LanguageModelV3CallOptions{
    Prompt: prompt,
    ToolChoice: &provider.ToolChoice{Type: provider.ToolChoiceTypeRequired},
    ResponseFormat: &provider.ResponseFormat{
        Type:   provider.ResponseFormatTypeJSON,
        Schema: provider.JSONObject{"type": "object"},
    },
    ProviderOptions: provider.ProviderOptions{
        "anthropic": provider.JSONObject{
            "tools": []providerutils.ToolSpecification{
                {
                    Name: "weather",
                    Parameters: provider.JSONObject{"type": "object"},
                },
            },
        },
    },
}
```

## Request Overrides

Pass Anthropic-specific fields via provider options:

```go
provider.ProviderOptions{
    "anthropic": provider.JSONObject{
        "request": provider.JSONObject{
            "metadata": "trace",
        },
    },
}
```
