## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variables: `HUME_API_KEY`

```go
client := hume.CreateHume(hume.Settings{
    APIKey:  os.Getenv("HUME_API_KEY"),
    BaseURL: "https://api.hume.ai",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Speech Models

```go
speechModel, _ := client.SpeechModel("default")
result, _ := speechModel.DoGenerate(ctx, provider.SpeechModelCallOptions{
    Text:         "Hello from Hume",
    OutputFormat: "mp3",
})
_ = result
```

Hume accepts provider-specific options via `ProviderOptions["hume"]`:

```go
result, _ := speechModel.DoGenerate(ctx, provider.SpeechModelCallOptions{
    Text: "Hello",
    RequestOptions: provider.RequestOptions{
        ProviderOptions: provider.ProviderOptions{
            "hume": provider.JSONObject{
                "context": provider.JSONObject{
                    "generationId": "gen-123",
                },
            },
        },
    },
})
_ = result
```

## Limitations

- Only speech models are supported.
- `Language` is ignored by the Hume API.
- Output formats are limited to `mp3`, `pcm`, and `wav`.

## Defaults

- Base URL: `https://api.hume.ai`
- Provider name: `hume`
- Default voice ID: `d8ab67c6-953d-4bd8-9370-8fa53a0f1453`
- Default output format: `mp3`
