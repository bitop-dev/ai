## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`, `PollInterval`, `PollTimeout`
- Environment variables: `GLADIA_API_KEY`

```go
client := gladia.CreateGladia(gladia.Settings{
    APIKey:  os.Getenv("GLADIA_API_KEY"),
    BaseURL: "https://api.gladia.io",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Transcription Models

```go
transcriptionModel, _ := client.TranscriptionModel("default")
result, _ := transcriptionModel.DoGenerate(ctx, provider.TranscriptionModelV3CallOptions{
    Audio:     audioBytes,
    MediaType: "audio/mpeg",
})
_ = result
```

## Transcription Options

Gladia transcription options can be passed via `ProviderOptions["gladia"]`, including
`contextPrompt`, `customVocabulary`, `detectLanguage`, `codeSwitchingConfig`,
`customVocabularyConfig`, `callbackConfig`, `subtitlesConfig`, `diarizationConfig`,
`translationConfig`, `summarizationConfig`, `customSpellingConfig`,
`structuredDataExtractionConfig`, `audioToLlmConfig`.

```go
options := provider.TranscriptionModelV3CallOptions{
    Audio:     audioBytes,
    MediaType: "audio/mpeg",
    RequestOptions: provider.RequestOptions{
        ProviderOptions: provider.ProviderOptions{
            "gladia": provider.JSONObject{
                "contextPrompt": "meeting notes",
                "translationConfig": provider.JSONObject{
                    "targetLanguages": []string{"es"},
                },
            },
        },
    },
}
```

## Defaults

- Base URL: `https://api.gladia.io`
- Provider name: `gladia`
