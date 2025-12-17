# Reference & FAQ

This is a quick reference to the public API and common questions.

## Packages

- `github.com/bitop-dev/ai` — public API + types
- `github.com/bitop-dev/ai/openai` — OpenAI config + model refs
- `github.com/bitop-dev/ai/mcp` — MCP client + tool adapter

## Model references (OpenAI)

These helpers create `ai.ModelRef` values:

```go
openai.Chat("gpt-4o-mini")                     // chat completions
openai.Image("gpt-image-1")                    // image generation
openai.TextEmbedding("text-embedding-3-small") // embeddings
openai.Transcription("whisper-1")              // transcription
openai.Speech("tts-1")                         // text-to-speech
```

## What providers are supported?

Currently: OpenAI and OpenAI-compatible providers.

The public API is provider-agnostic; you can add more providers later.

## Why does the root package have many files?

`package ai` (root) is intentionally the primary public surface:

- `core_*.go`: public types/primitives
- `api_*.go`: public entrypoints
- glue code that maps public types to internal provider types

Implementation details live in `internal/...`.

See `AGENTS.md` for layout conventions.

## Do I need to import the OpenAI package?

Yes, for configuration and model refs:

```go
import "github.com/bitop-dev/ai/openai"
```

The main API is always:

```go
import "github.com/bitop-dev/ai"
```

## How do I use an OpenAI-compatible server?

```go
openai.Configure(openai.Config{
  APIKey:    os.Getenv("OPENAI_API_KEY"),
  BaseURL:   "http://localhost:8080",
  APIPrefix: "/v1",
})
```

## How do I keep chat history?

Append `Response.Messages` after each call:

```go
history = append(history, resp.Response.Messages...)
```

For streaming:

```go
history = append(history, stream.Response().Messages...)
```

## Why is chat audio input not supported?

OpenAI chat completions does not accept the audio content part format used by some other providers/SDKs in a consistent way.
Use `Transcribe` and `GenerateSpeech` instead.

## Where are examples?

See the `examples/` directory and `README.md`.

