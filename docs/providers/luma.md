Use this provider to generate images with Luma Dream Machine models.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`, `PollInterval`, `MaxPollAttempts`
- Environment variable: `LUMA_API_KEY`

```go
client := luma.CreateLuma(luma.Settings{
    APIKey:  os.Getenv("LUMA_API_KEY"),
    BaseURL: "https://api.lumalabs.ai",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Image Models

```go
imageModel, _ := client.ImageModel("dream-machine")
result, _ := imageModel.DoGenerate(ctx, provider.ImageModelCallOptions{
    Prompt:      "A neon skyline",
    AspectRatio: "16:9",
})
_ = result
```

## Image Options

Luma accepts provider-specific options via `ProviderOptions["luma"]`.

```go
options := provider.ImageModelCallOptions{
    Prompt: "Stylized portrait",
    RequestOptions: provider.RequestOptions{
        ProviderOptions: provider.ProviderOptions{
            "luma": provider.JSONObject{
                "files": []any{
                    "https://example.com/reference.png",
                },
                "referenceType": "style",
                "images": []any{
                    provider.JSONObject{"weight": 0.8},
                },
                "pollIntervalMillis": 500,
                "maxPollAttempts":    120,
                "quality":            "hd",
            },
        },
    },
}
```

## Limitations

- Luma only supports URL-based reference images.
- `Size` and `Seed` are ignored; use `AspectRatio` for sizing.
- Text and embedding models are not supported.

## Defaults

- Base URL: `https://api.lumalabs.ai`
- Provider name: `luma`
- Poll interval: `500ms`
- Max poll attempts: `120`
