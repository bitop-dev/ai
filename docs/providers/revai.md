## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`, `PollInterval`, `PollTimeout`
- Environment variables: `REVAI_API_KEY`

```go
client := revai.CreateRevAI(revai.Settings{
    APIKey:  os.Getenv("REVAI_API_KEY"),
    BaseURL: "https://api.rev.ai",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Transcription Models

```go
transcriptionModel, _ := client.TranscriptionModel("machine")
result, _ := transcriptionModel.DoGenerate(ctx, provider.TranscriptionModelCallOptions{
    Audio:     audioBytes,
    MediaType: "audio/wav",
})
_ = result
```

## Transcription Options

Rev.ai transcription options can be passed via `ProviderOptions["revai"]`, including
`metadata`, `notificationConfig`, `deleteAfterSeconds`, `verbatim`, `rush`, `segmentsToTranscribe`,
`speakerNames`, `skipDiarization`, `skipPunctuation`, `removeDisfluencies`,
`filterProfanity`, `customVocabularyId`, `summarizationConfig`, `translationConfig`,
and `language`.

```go
options := provider.TranscriptionModelCallOptions{
    Audio:     audioBytes,
    MediaType: "audio/wav",
    RequestOptions: provider.RequestOptions{
        ProviderOptions: provider.ProviderOptions{
            "revai": provider.JSONObject{
                "metadata": "job-123",
                "notificationConfig": provider.JSONObject{
                    "url": "https://example.com/hook",
                },
                "skipPunctuation": true,
            },
        },
    },
}
```

## Defaults

- Base URL: `https://api.rev.ai`
- Provider name: `revai`
