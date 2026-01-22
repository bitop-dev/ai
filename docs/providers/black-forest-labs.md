Use this provider to generate images with Black Forest Labs model APIs.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`, `PollInterval`, `PollTimeout`
- Environment variable: `BFL_API_KEY`

```go
client := blackforestlabs.CreateBlackForestLabs(blackforestlabs.Settings{
    APIKey:      os.Getenv("BFL_API_KEY"),
    BaseURL:     "https://api.bfl.ai/v1",
    PollInterval: 500 * time.Millisecond,
    PollTimeout:  60 * time.Second,
})
```

## Image Models

```go
imageModel, _ := client.ImageModel("flux-pro-1.1")
result, _ := imageModel.DoGenerate(ctx, provider.ImageModelCallOptions{
    Prompt:      "A neon skyline",
    Size:        "1024x768",
    AspectRatio: "4:3",
})
_ = result
```

## Image Options

Black Forest Labs image models accept extra configuration via `ProviderOptions["black-forest-labs"]`.

```go
options := provider.ImageModelCallOptions{
    Prompt: "A luminous forest",
    RequestOptions: provider.RequestOptions{
        ProviderOptions: provider.ProviderOptions{
            "black-forest-labs": provider.JSONObject{
                "guidance":         3.5,
                "steps":            30,
                "imagePrompt":      "sketch",
                "safetyTolerance":  2,
                "outputFormat":     "png",
                "inputImage": provider.ImageContent{
                    Data: []byte("reference"),
                },
                "mask": provider.FileContent{
                    Data: []byte("mask"),
                },
                "pollIntervalMillis": 200,
                "pollTimeoutMillis":  30000,
            },
        },
    },
}
```

## Limitations

- Text and embedding models are not supported.

## Defaults

- Base URL: `https://api.bfl.ai/v1`
- Provider name: `black-forest-labs`
- Poll interval: 500ms
- Poll timeout: 60s
