## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variables: `DEEPGRAM_API_KEY`

```go
client := deepgram.CreateDeepgram(deepgram.Settings{
    APIKey:  os.Getenv("DEEPGRAM_API_KEY"),
    BaseURL: "https://api.deepgram.com",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Speech Models

Deepgram voices are selected via the model ID (for example, `aura-2-helena-en`).

```go
speechModel, _ := client.SpeechModel("aura-2-helena-en")
result, _ := speechModel.DoGenerate(ctx, provider.SpeechModelCallOptions{
    Text:         "Hello from Deepgram",
    OutputFormat: "wav",
})
_ = result
```

Speech provider options can be passed via `ProviderOptions["deepgram"]`:
`bitRate`, `container`, `encoding`, `sampleRate`, `callback`, `callbackMethod`,
`mipOptOut`, `tag`.

## Transcription Models

```go
transcriptionModel, _ := client.TranscriptionModel("nova-3")
result, _ := transcriptionModel.DoGenerate(ctx, provider.TranscriptionModelCallOptions{
    Audio:     audioBytes,
    MediaType: "audio/wav",
    RequestOptions: provider.RequestOptions{
        ProviderOptions: provider.ProviderOptions{
            "deepgram": provider.JSONObject{
                "detectLanguage": true,
                "utterances":     true,
            },
        },
    },
})
_ = result
```

Transcription provider options can be passed via `ProviderOptions["deepgram"]`:
`language`, `detectLanguage`, `smartFormat`, `punctuate`, `paragraphs`,
`summarize`, `topics`, `intents`, `sentiment`, `detectEntities`, `redact`,
`replace`, `search`, `keyterm`, `diarize`, `utterances`, `uttSplit`,
`fillerWords`.

## Defaults

- Base URL: `https://api.deepgram.com`
- Provider name: `deepgram`
