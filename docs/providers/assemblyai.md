## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`, `PollInterval`
- Environment variables: `ASSEMBLYAI_API_KEY`

```go
client := assemblyai.CreateAssemblyAI(assemblyai.Settings{
    APIKey:  os.Getenv("ASSEMBLYAI_API_KEY"),
    BaseURL: "https://api.assemblyai.com",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Transcription Models

```go
transcriptionModel, _ := client.TranscriptionModel("best")
result, _ := transcriptionModel.DoGenerate(ctx, provider.TranscriptionModelCallOptions{
    Audio: audioBytes,
})
_ = result
```

## Transcription Options

AssemblyAI transcription options can be passed via `ProviderOptions["assemblyai"]`,
including `contentSafety`, `wordBoost`, `languageCode`, `speakerLabels`,
`summarization`, `summaryType`, `webhookUrl`, and `webhookAuthHeaderName`.

```go
options := provider.TranscriptionModelCallOptions{
    Audio: audioBytes,
    RequestOptions: provider.RequestOptions{
        ProviderOptions: provider.ProviderOptions{
            "assemblyai": provider.JSONObject{
                "contentSafety": true,
                "summaryType":   "bullets",
                "webhookUrl":    "https://example.com/transcripts",
            },
        },
    },
}
```

## Defaults

- Base URL: `https://api.assemblyai.com`
- Provider name: `assemblyai`
