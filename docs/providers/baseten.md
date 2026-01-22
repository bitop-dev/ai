Use this provider to generate images with Baseten model APIs.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variable: `BASETEN_API_KEY`

```go
client := baseten.CreateBaseten(baseten.Settings{
    APIKey:  os.Getenv("BASETEN_API_KEY"),
    BaseURL: "https://inference.baseten.co/v1",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Image Models

```go
imageModel, _ := client.ImageModel("flux-1")
result, _ := imageModel.DoGenerate(ctx, provider.ImageModelV3CallOptions{
    Prompt:      "A neon skyline",
    Size:        "1024x768",
    AspectRatio: "16:9",
    N:           1,
})
_ = result
```

## Image Options

Baseten image models accept extra configuration via `ProviderOptions["baseten"]`.

```go
options := provider.ImageModelV3CallOptions{
    Prompt: "A luminous forest",
    RequestOptions: provider.RequestOptions{
        ProviderOptions: provider.ProviderOptions{
            "baseten": provider.JSONObject{
                "guidanceScale":  7.5,
                "negativePrompt": "fog",
                "request": provider.JSONObject{
                    "metadata": "trace",
                },
            },
        },
    },
}
```

## Limitations

- Text and embedding models are not supported.
- Image editing and masks are not mapped yet.

## Defaults

- Base URL: `https://inference.baseten.co/v1`
- Provider name: `baseten`
