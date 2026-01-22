## Configuration

- Settings fields: `APIKey`, `BaseURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variables: `ELEVENLABS_API_KEY`

```go
client := elevenlabs.CreateElevenLabs(elevenlabs.Settings{
    APIKey:  os.Getenv("ELEVENLABS_API_KEY"),
    BaseURL: "https://api.elevenlabs.io",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Speech Models

ElevenLabs speech models use the model ID (for example, `eleven_multilingual_v2`).
Voices are selected via the `Voice` option and default to the ElevenLabs public voice.

```go
speechModel, _ := client.SpeechModel("eleven_multilingual_v2")
result, _ := speechModel.DoGenerate(ctx, provider.SpeechModelCallOptions{
    Text:         "Hello from ElevenLabs",
    Voice:        "21m00Tcm4TlvDq8ikWAM",
    OutputFormat: "mp3",
})
_ = result
```

Speech provider options can be passed via `ProviderOptions["elevenlabs"]`:
`languageCode`, `voiceSettings`, `pronunciationDictionaryLocators`, `seed`,
`previousText`, `nextText`, `previousRequestIds`, `nextRequestIds`,
`applyTextNormalization`, `applyLanguageTextNormalization`, `enableLogging`.

## Transcription Models

```go
transcriptionModel, _ := client.TranscriptionModel("scribe_v1")
result, _ := transcriptionModel.DoGenerate(ctx, provider.TranscriptionModelCallOptions{
    Audio:     audioBytes,
    MediaType: "audio/wav",
})
_ = result
```

Transcription provider options can be passed via `ProviderOptions["elevenlabs"]`:
`languageCode`, `tagAudioEvents`, `numSpeakers`, `timestampsGranularity`,
`diarize`, `fileFormat`.

## Defaults

- Base URL: `https://api.elevenlabs.io`
- Provider name: `elevenlabs`
