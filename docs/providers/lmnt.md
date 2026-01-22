## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variables: `LMNT_API_KEY`

```go
client := lmnt.CreateLMNT(lmnt.Settings{
    APIKey:  os.Getenv("LMNT_API_KEY"),
    BaseURL: "https://api.lmnt.com",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Speech Models

```go
speechModel, _ := client.SpeechModel("aurora")
result, _ := speechModel.DoGenerate(ctx, provider.SpeechModelV3CallOptions{
    Text:         "Hello from LMNT",
    Voice:        "ava",
    OutputFormat: "mp3",
    Language:     "en",
})
_ = result
```

LMNT accepts provider-specific options via `ProviderOptions["lmnt"]`:
`conversational`, `length`, `seed`, `speed`, `temperature`, `topP`, `sampleRate`.

## Limitations

- Only speech models are supported.
- Output formats are limited to `aac`, `mp3`, `mulaw`, `raw`, and `wav`.

## Defaults

- Base URL: `https://api.lmnt.com`
- Provider name: `lmnt`
- Default voice ID: `ava`
- Default output format: `mp3`
