# Embeddings

This guide covers:

- `ai.Embed` — embed a single string into a vector
- `ai.EmbedMany` — batch embedding
- `ai.CosineSimilarity` — compare embedding vectors

The current focus is OpenAI and OpenAI-compatible providers.

## Quick Start: `Embed`

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

  resp, err := ai.Embed(context.Background(), ai.EmbedRequest{
    Model: openai.TextEmbedding("text-embedding-3-small"),
    Input: "sunny day at the beach",
  })
  if err != nil {
    panic(err)
  }

  fmt.Println("dims:", len(resp.Vector))
  fmt.Println("tokens:", resp.Usage.TotalTokens)
}
```

The response includes:

- `Vector []float32`
- `Usage` (tokens)
- `RawResponse []byte` (provider response payload)

## Batch Embedding: `EmbedMany`

```go
resp, err := ai.EmbedMany(ctx, ai.EmbedManyRequest{
  Model: openai.TextEmbedding("text-embedding-3-small"),
  Input: []string{
    "sunny day at the beach",
    "rainy afternoon in the city",
    "snowy night in the mountains",
  },
})
if err != nil {
  log.Fatal(err)
}

fmt.Println("count:", len(resp.Vectors))
fmt.Println("dims:", len(resp.Vectors[0]))
```

The vectors are returned in the same order as the inputs.

## Similarity: `CosineSimilarity`

```go
resp, _ := ai.EmbedMany(ctx, ai.EmbedManyRequest{
  Model: openai.TextEmbedding("text-embedding-3-small"),
  Input: []string{
    "sunny day at the beach",
    "rainy afternoon in the city",
  },
})

sim := ai.CosineSimilarity(resp.Vectors[0], resp.Vectors[1])
fmt.Println("cosine similarity:", sim)
```

Notes:

- `CosineSimilarity` expects equal-length vectors.
- The value is in `[-1, 1]` (higher is “more similar”).

## Provider Options

OpenAI embedding endpoints support provider-specific parameters (e.g. `dimensions`, `encoding_format`).
Pass them via `ProviderOptions`:

```go
resp, err := ai.Embed(ctx, ai.EmbedRequest{
  Model: openai.TextEmbedding("text-embedding-3-small"),
  Input: "hello",
  ProviderOptions: map[string]any{
    "openai": map[string]any{
      "dimensions": 512,
      // "encoding_format": "float",
    },
  },
})
```

For OpenAI-compatible servers, the supported set may vary.

## Parallelization: `MaxParallelCalls` (EmbedMany)

For larger batches, `EmbedMany` can split requests and run them in parallel:

```go
resp, err := ai.EmbedMany(ctx, ai.EmbedManyRequest{
  Model: openai.TextEmbedding("text-embedding-3-small"),
  Input: values,
  MaxParallelCalls: 4,
})
```

Guidance:

- Start small (`2` or `4`).
- Too much parallelism can lead to rate-limiting.

## Request Controls (Headers / Retries / Timeout)

### Headers

```go
resp, err := ai.Embed(ctx, ai.EmbedRequest{
  Model: openai.TextEmbedding("text-embedding-3-small"),
  Input: "hello",
  Headers: map[string]string{
    "X-Request-ID": "abc123",
  },
})
```

### Retries

```go
retries := 0
resp, err := ai.EmbedMany(ctx, ai.EmbedManyRequest{
  Model: openai.TextEmbedding("text-embedding-3-small"),
  Input: []string{"a", "b"},
  MaxRetries: &retries,
})
```

### Timeout

```go
resp, err := ai.EmbedMany(ctx, ai.EmbedManyRequest{
  Model: openai.TextEmbedding("text-embedding-3-small"),
  Input: []string{"a", "b"},
  Timeout: 5 * time.Second,
})
```

## Common Pitfalls

### 1) Using a chat model for embeddings

Use an embedding model ref:

```go
openai.TextEmbedding("text-embedding-3-small")
```

### 2) Mixed dimensions

If you override dimensions via provider options, all vectors you compare must share the same length.

### 3) Large batches and rate limits

If you hit rate limits:

- lower `MaxParallelCalls`,
- reduce batch size,
- or increase provider-side limits/quotas.

## Examples in this repo

- `go run ./examples/embed`
- `go run ./examples/embed_many`

