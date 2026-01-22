# Amazon Bedrock Provider

The Amazon Bedrock provider maps prompts to the Bedrock Runtime InvokeModel APIs.
It signs requests with AWS SigV4 credentials and supports streaming responses.

## Configuration

- Environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_REGION`, `AWS_PROFILE`, `AWS_SHARED_CREDENTIALS_FILE`
- Settings fields: `AccessKeyID`, `SecretAccessKey`, `SessionToken`, `Region`, `BaseURL`, `Headers`, `ProviderName`, `Profile`, `CredentialsFile`, `Service`

Create the provider with:

```go
client := amazonbedrock.CreateAmazonBedrock(amazonbedrock.Settings{
    AccessKeyID:     "...",
    SecretAccessKey: "...",
    Region:          "us-east-1",
})
```

## Language Models

```go
model, _ := client.LanguageModel("amazon.titan-text-lite-v1")
_, _ = model.DoGenerate(ctx, provider.LanguageModelV3CallOptions{
    Prompt: provider.Prompt{
        Messages: []provider.ModelMessage{
            {Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
        },
    },
    MaxOutputTokens: 120,
})
```

The default request mapping uses `inputText` and `textGenerationConfig` fields. For model families
that require custom payloads, pass request overrides in provider options.

## Request Overrides

```go
options := provider.LanguageModelV3CallOptions{
    Prompt: prompt,
    ProviderOptions: provider.ProviderOptions{
        "amazon-bedrock": provider.JSONObject{
            "request": provider.JSONObject{
                "inputText": "override",
            },
        },
    },
}
```

## Streaming

```go
result, _ := model.DoStream(ctx, provider.LanguageModelV3CallOptions{Prompt: prompt})
for part := range result.Stream {
    _ = part
}
```

## Limitations

- Only language model endpoints are wired; embeddings and images are unsupported.
- Titan-style request defaults are provided. Use provider options to supply model-specific payloads
  for Claude, Mistral, or other Bedrock models.
