Use this provider to generate images with Prodia models.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variable: `PRODIA_TOKEN`

```go
client := prodia.CreateProdia(prodia.Settings{
    APIKey:  os.Getenv("PRODIA_TOKEN"),
    BaseURL: "https://inference.prodia.com/v2",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Image Models

```go
imageModel, _ := client.ImageModel("inference.flux-fast.schnell.txt2img.v2")
result, _ := imageModel.DoGenerate(ctx, provider.ImageModelV3CallOptions{
    Prompt: "A neon skyline",
    Seed:   123,
    Size:   "1024x1024",
})
_ = result
```

## Image Options

Prodia accepts extra configuration via `ProviderOptions["prodia"]`.

```go
options := provider.ImageModelV3CallOptions{
    Prompt: "A luminous forest",
    RequestOptions: provider.RequestOptions{
        ProviderOptions: provider.ProviderOptions{
            "prodia": provider.JSONObject{
                "width":       1024,
                "height":      1024,
                "steps":       4,
                "stylePreset": "anime",
                "loras":       []string{"prodia/lora/flux/anime@v1"},
                "progressive": true,
            },
        },
    },
}
```

## Limitations

- Text and embedding models are not supported.
- Image masks and edits are not mapped yet.

## Defaults

- Base URL: `https://inference.prodia.com/v2`
- Provider name: `prodia`
