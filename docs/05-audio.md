# Audio

This guide covers:

- `ai.Transcribe` — speech-to-text (transcription)
- `ai.GenerateSpeech` — text-to-speech (TTS)

The current focus is OpenAI and OpenAI-compatible providers.

## Transcription (`Transcribe`)

### Quick Start (file on disk)

The `TranscribeRequest` accepts audio as bytes/base64/URL. A common pattern is reading a file.

```go
package main

import (
  "context"
  "fmt"
  "os"

  "github.com/bitop-dev/ai"
  "github.com/bitop-dev/ai/openai"
)

func main() {
  openai.Configure(openai.Config{APIKey: os.Getenv("OPENAI_API_KEY")})

  audioPath := os.Getenv("AUDIO_PATH")
  if audioPath == "" {
    fmt.Fprintln(os.Stderr, "AUDIO_PATH is required")
    os.Exit(1)
  }

  b, err := os.ReadFile(audioPath)
  if err != nil {
    panic(err)
  }

  tr, err := ai.Transcribe(context.Background(), ai.TranscribeRequest{
    Model:      openai.Transcription("whisper-1"),
    AudioBytes: b,
    Filename:   "audio.mp3", // optional but recommended for multipart uploads
    MediaType:  "audio/mpeg",
  })
  if err != nil {
    panic(err)
  }

  fmt.Println(tr.Text)
}
```

### Inputs

`TranscribeRequest` supports:

- `AudioBytes []byte`
- `AudioBase64 string`
- `AudioURL string`

You can also set:

- `Filename` (recommended when uploading bytes)
- `MediaType` (e.g. `audio/mpeg`, `audio/wav`)

### Response shape

`Transcript` includes:

- `Text`
- `Segments` (if available)
- `Language` (if available)
- `DurationInSeconds` (if available)
- `Warnings`, `ProviderMetadata`, `RawResponse`

### Segments

```go
for _, seg := range tr.Segments {
  fmt.Printf("[%0.2f-%0.2f] %s\n", seg.Start, seg.End, seg.Text)
}
```

### Provider options

Pass provider-specific parameters via `ProviderOptions`:

```go
tr, err := ai.Transcribe(ctx, ai.TranscribeRequest{
  Model: openai.Transcription("whisper-1"),
  AudioURL: "https://example.com/audio.mp3",
  ProviderOptions: map[string]any{
    "openai": map[string]any{
      "timestamp_granularities": []string{"word"},
    },
  },
})
```

OpenAI-compatible servers may differ in supported fields.

### Request controls (Headers / Retries / Timeout)

```go
retries := 0
tr, err := ai.Transcribe(ctx, ai.TranscribeRequest{
  Model: openai.Transcription("whisper-1"),
  AudioURL: "https://example.com/audio.mp3",
  Headers: map[string]string{"X-Request-ID": "abc123"},
  MaxRetries: &retries,
  Timeout: 30 * time.Second,
})
```

### Errors

When the provider returns no transcript text, `Transcribe` returns `*ai.NoTranscriptGeneratedError`.

```go
tr, err := ai.Transcribe(ctx, req)
if err != nil {
  if ai.IsNoTranscriptGenerated(err) {
    fmt.Println("no transcript generated")
    return
  }
  panic(err)
}
_ = tr
```

Provider/network errors are returned as `*ai.Error` (see `docs/01-getting-started.md`).

## Speech (Text-to-Speech) (`GenerateSpeech`)

### Quick Start

```go
audio, err := ai.GenerateSpeech(ctx, ai.GenerateSpeechRequest{
  Model: openai.Speech("tts-1"),
  Text:  "Hello, world!",
  Voice: "alloy",
})
if err != nil {
  panic(err)
}

fmt.Println("mediaType:", audio.MediaType)
fmt.Println("bytes:", len(audio.AudioData))
```

### Save to disk

```go
audio, _ := ai.GenerateSpeech(ctx, ai.GenerateSpeechRequest{
  Model: openai.Speech("tts-1"),
  Text:  "Hello, world!",
  Voice: "alloy",
})

os.WriteFile("out.mp3", audio.AudioData, 0o644)
```

### Language (if supported)

```go
audio, err := ai.GenerateSpeech(ctx, ai.GenerateSpeechRequest{
  Model:    openai.Speech("tts-1"),
  Text:     "Hola, mundo!",
  Voice:    "alloy",
  Language: "es",
})
```

Support varies by provider/model.

### Provider options

```go
audio, err := ai.GenerateSpeech(ctx, ai.GenerateSpeechRequest{
  Model: openai.Speech("tts-1"),
  Text:  "Hello, world!",
  Voice: "alloy",
  ProviderOptions: map[string]any{
    "openai": map[string]any{
      // provider/model-specific knobs
    },
  },
})
```

### Request controls (Headers / Retries / Timeout)

```go
retries := 1
audio, err := ai.GenerateSpeech(ctx, ai.GenerateSpeechRequest{
  Model: openai.Speech("tts-1"),
  Text:  "Hello, world!",
  Voice: "alloy",
  Headers: map[string]string{"X-Request-ID": "abc123"},
  MaxRetries: &retries,
  Timeout: 30 * time.Second,
})
```

### Errors

When the provider returns no audio bytes, `GenerateSpeech` returns `*ai.NoSpeechGeneratedError`.

```go
audio, err := ai.GenerateSpeech(ctx, req)
if err != nil {
  if ai.IsNoSpeechGenerated(err) {
    fmt.Println("no speech generated")
    return
  }
  panic(err)
}
_ = audio
```

## Examples in this repo

- `go run ./examples/transcribe` (requires `AUDIO_PATH`)
- `go run ./examples/generate_speech`

