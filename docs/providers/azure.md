# Azure OpenAI

Use this provider to connect to Azure OpenAI deployments via the OpenAI-compatible endpoints.

## Configuration

- Settings fields: `APIKey`, `ResourceName`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`, `APIVersion`, `UseDeploymentBasedURLs`

```go
client := azure.CreateAzure(azure.Settings{
    ResourceName: os.Getenv("AZURE_RESOURCE_NAME"),
    APIKey:       os.Getenv("AZURE_API_KEY"),
    APIVersion:   "2024-10-01",
})
```

Use `BaseURL` instead of `ResourceName` to point at proxies or custom domains. The base URL should omit the `/v1` suffix; the provider will append `/v1` automatically unless `UseDeploymentBasedURLs` is enabled.

## Language Models

```go
model, _ := client.LanguageModel("deployment-name")
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

Provider options use the provider name as the key (defaults to `azure`).

```go
options := provider.LanguageModelV3CallOptions{
    Prompt: prompt,
    ToolChoice: &provider.ToolChoice{Type: provider.ToolChoiceTypeRequired},
    ResponseFormat: &provider.ResponseFormat{
        Type:   provider.ResponseFormatTypeJSON,
        Schema: provider.JSONObject{"type": "object"},
    },
    ProviderOptions: provider.ProviderOptions{
        "azure": provider.JSONObject{
            "tools": []providerutils.ToolSpecification{
                {Name: "tool", Parameters: provider.JSONObject{"type": "object"}},
            },
        },
    },
}
```

## Streaming

- Streaming uses the OpenAI-compatible SSE format.
- Text deltas and finish reasons follow the OpenAI-compatible provider mappings.

## Request Overrides

```go
provider.ProviderOptions{
    "azure": provider.JSONObject{
        "request": provider.JSONObject{
            "user": "client-id",
        },
        "mode": "responses",
    },
}
```

## Limitations

- Set `UseDeploymentBasedURLs` to true for legacy deployment-based endpoints.
