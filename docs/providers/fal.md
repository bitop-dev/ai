Use this provider to access fal.ai image, speech, and transcription models.

## Configuration

- Settings fields: `APIKey`, `BaseURL`, `QueueURL`, `Headers`, `HTTPClient`, `ProviderName`
- Environment variables: `FAL_API_KEY`, `FAL_KEY`

```go
client := fal.CreateFal(fal.Settings{
    APIKey:   os.Getenv("FAL_API_KEY"),
    BaseURL:  "https://fal.run",
    QueueURL: "https://queue.fal.run",
    Headers: map[string]string{
        "X-Custom": "value",
    },
})
```

## Image Models

```go
imageModel, _ := client.ImageModel("fal-ai/flux")
result, _ := imageModel.DoGenerate(ctx, provider.ImageModelV3CallOptions{
    Prompt: "A neon skyline",
    Size:   "1024x768",
    N:      1,
})
_ = result
```

## Image Options

Fal image models accept provider-specific parameters via
`ProviderOptions["fal"]`. Use these for image URLs, masks, or
model-specific options.

```go
options := provider.ImageModelV3CallOptions{
    Prompt: "An in-painted scene",
    RequestOptions: provider.RequestOptions{
        ProviderOptions: provider.ProviderOptions{
            "fal": provider.JSONObject{
                "imageUrl": provider.ImageContent{
                    MediaType: "image/png",
                    Data:      []byte("..."),
                },
                "maskUrl": "https://example.com/mask.png",
                "numInferenceSteps": 30,
            },
        },
    },
}
```

## Speech Models

```go
speechModel, _ := client.SpeechModel("fal-ai/chatterbox")
result, _ := speechModel.DoGenerate(ctx, provider.SpeechModelV3CallOptions{
    Text:         "Hello from fal",
    Voice:        "samantha",
    OutputFormat: "url",
})
_ = result
```

## Transcription Models

Transcription requests are submitted to the fal.ai queue service and polled
until completion.

```go
transcriptionModel, _ := client.TranscriptionModel("wizper")
result, _ := transcriptionModel.DoGenerate(ctx, provider.TranscriptionModelV3CallOptions{
    Audio:     audioBytes,
    MediaType: "audio/wav",
})
_ = result
```

## Defaults

- Base URL: `https://fal.run`
- Queue URL: `https://queue.fal.run`
- Provider name: `fal`
