# AI SDK Go

AI SDK Go is a Go reimplementation of AI SDK v6. It provides provider-agnostic
interfaces, streaming helpers, tool orchestration, and utilities for embeddings,
images, speech, transcription, and reranking.

## Install

```bash
go get github.com/bitop-dev/ai
```

## Quickstart

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/bitop-dev/ai/pkg/ai"
    "github.com/bitop-dev/ai/pkg/provider"
    "github.com/bitop-dev/ai/pkg/providers/openai"
)

func main() {
    ctx := context.Background()
    openaiProvider := openai.CreateOpenAI(openai.Settings{})
    model, err := openaiProvider.LanguageModel("gpt-4o-mini")
    if err != nil {
        log.Fatal(err)
    }

    prompt := provider.Prompt{
        Messages: []provider.ModelMessage{
            {
                Role: provider.RoleUser,
                Content: []provider.ContentPart{
                    provider.TextContent{Text: "Summarize the Pacific Ocean in one sentence."},
                },
            },
        },
    }

    result, err := ai.GenerateText(ctx, model, ai.GenerateTextOptions{Prompt: prompt})
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.Text)
}
```

Set `OPENAI_API_KEY` to authenticate with OpenAI.

## Streaming

Use `StreamText` and pipe parts as SSE when building HTTP handlers:

```go
result, err := ai.StreamText(ctx, model, ai.StreamTextOptions{Prompt: prompt})
if err != nil {
    return err
}
if err := ai.PipeStream(ctx, w, result.Stream); err != nil {
    return err
}
```

## Documentation

- `docs/README.md`
- `docs/ai.md`
- `docs/gateway.md`
- `docs/provider.md`
- `docs/providerutils.md`
- `docs/registry.md`
- `docs/parity.md`
- `docs/limitations.md`
- `docs/providers/*.md`

## Examples

- `examples/text_generation/main.go`
- `examples/multi_turn_chat/main.go`
- `examples/streaming_sse/main.go`
- `examples/gateway_streaming/main.go`
- `examples/tool_calling_auto/main.go`
- `examples/tool_calling_manual/main.go`
- `examples/structured_output/main.go`
- `examples/embeddings_rerank/main.go`
- `examples/image_generation/main.go`
- `examples/speech_generation/main.go`
- `examples/transcription/main.go`
- `examples/mcp-stdio/main.go`
- `examples/mcp-http/main.go`
