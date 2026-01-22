Use this provider to generate images with Replicate models.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variable: `REPLICATE_API_TOKEN`

```go
client := replicate.CreateReplicate(replicate.Settings{
    APIKey:  os.Getenv("REPLICATE_API_TOKEN"),
    BaseURL: "https://api.replicate.com/v1",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Image Models

```go
imageModel, _ := client.ImageModel("black-forest-labs/flux-1.1-pro")
result, _ := imageModel.DoGenerate(ctx, provider.ImageModelV3CallOptions{
    Prompt:      "A futuristic cityscape",
    AspectRatio: "16:9",
    N:           1,
})
_ = result
```

## Image Options

Replicate image models accept provider-specific parameters via
`ProviderOptions["replicate"]` and use the `Prefer: wait` header for sync
predictions. Use `maxWaitTimeInSeconds` to set a custom wait time.

```go
options := provider.ImageModelV3CallOptions{
    Prompt: "A neon skyline",
    RequestOptions: provider.RequestOptions{
        ProviderOptions: provider.ProviderOptions{
            "replicate": provider.JSONObject{
                "maxWaitTimeInSeconds": 120,
                "num_inference_steps": 30,
                "request": provider.JSONObject{
                    "webhook": "https://example.com/replicate",
                },
            },
        },
    },
}
```

## Async Predictions

If a prediction remains in `starting` or `processing` after the initial request,
the provider polls the prediction URL until it completes or fails.

## Limitations

- Text/embedding models are not supported.
- Image editing uploads and masks are not mapped yet.

## Defaults

- Base URL: `https://api.replicate.com/v1`
- Provider name: `replicate`
