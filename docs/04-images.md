# Image Generation

This guide covers `ai.GenerateImage` and the data returned by image models.

The current focus is OpenAI and OpenAI-compatible providers.

## Quick Start

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

  resp, err := ai.GenerateImage(context.Background(), ai.GenerateImageRequest{
    Model:  openai.Image("gpt-image-1"),
    Prompt: "Santa Claus driving a Cadillac",
  })
  if err != nil {
    panic(err)
  }

  fmt.Println("mediaType:", resp.Image.MediaType)
  fmt.Println("bytes:", len(resp.Image.Uint8Array))
}
```

## Return Shape

`GenerateImageResponse` includes:

- `Image` — convenience alias for the first image (same as `Images[0]`)
- `Images` — all generated images
- `Warnings` — provider warnings (if any)
- `ProviderMetadata` — provider-specific metadata (if any)
- `RawResponse` — raw provider response payload

Each `Image` contains:

- `Base64` — base64-encoded image bytes (when provided)
- `Uint8Array` — decoded bytes
- `MediaType` — e.g. `image/png`

## Sizes / Aspect Ratio

Depending on the model, you can set either `Size` (e.g. `1024x1024`) or `AspectRatio` (e.g. `16:9`).

```go
resp, err := ai.GenerateImage(ctx, ai.GenerateImageRequest{
  Model:  openai.Image("dall-e-3"),
  Prompt: "Santa Claus driving a Cadillac",
  Size:   "1024x1024",
})
```

## Multiple Images (`N`) + Batching

Request more than one image with `N`.

```go
resp, err := ai.GenerateImage(ctx, ai.GenerateImageRequest{
  Model:  openai.Image("dall-e-2"),
  Prompt: "Santa Claus driving a Cadillac",
  N:      4,
})
fmt.Println("images:", len(resp.Images))
```

Internally, the SDK will batch requests to respect model/provider limits.

### Control batching (`MaxImagesPerCall`)

```go
resp, err := ai.GenerateImage(ctx, ai.GenerateImageRequest{
  Model:           openai.Image("dall-e-2"),
  Prompt:          "Santa Claus driving a Cadillac",
  N:               10,
  MaxImagesPerCall: 5, // 2 calls of 5 images each
})
```

### Control parallelism (`MaxParallelCalls`)

When batching requires multiple requests, they can be performed in parallel:

```go
resp, err := ai.GenerateImage(ctx, ai.GenerateImageRequest{
  Model:            openai.Image("dall-e-2"),
  Prompt:           "Santa Claus driving a Cadillac",
  N:                10,
  MaxImagesPerCall: 5,
  MaxParallelCalls: 2,
})
```

## Seed (if supported)

```go
seed := int64(1234567890)
resp, err := ai.GenerateImage(ctx, ai.GenerateImageRequest{
  Model:  openai.Image("gpt-image-1"),
  Prompt: "A cute robot holding a coffee",
  Seed:   &seed,
})
```

Support varies by provider/model.

## Provider Options

Pass provider-specific settings via `ProviderOptions`:

```go
resp, err := ai.GenerateImage(ctx, ai.GenerateImageRequest{
  Model:  openai.Image("dall-e-3"),
  Prompt: "Santa Claus driving a Cadillac",
  Size:   "1024x1024",
  ProviderOptions: map[string]any{
    "openai": map[string]any{
      "quality": "hd",
      "style":   "vivid",
    },
  },
})
```

## Request Controls (Headers / Retries / Timeout)

### Headers

```go
resp, err := ai.GenerateImage(ctx, ai.GenerateImageRequest{
  Model:  openai.Image("gpt-image-1"),
  Prompt: "A logo for a Go conference",
  Headers: map[string]string{
    "X-Request-ID": "abc123",
  },
})
```

### Retries

```go
retries := 0
resp, err := ai.GenerateImage(ctx, ai.GenerateImageRequest{
  Model:      openai.Image("gpt-image-1"),
  Prompt:     "A logo for a Go conference",
  MaxRetries: &retries,
})
```

### Timeout

```go
resp, err := ai.GenerateImage(ctx, ai.GenerateImageRequest{
  Model:   openai.Image("gpt-image-1"),
  Prompt:  "A logo for a Go conference",
  Timeout: 10 * time.Second,
})
```

## Errors

When no images are returned, `GenerateImage` returns a `*ai.NoImageGeneratedError`.

```go
resp, err := ai.GenerateImage(ctx, req)
if err != nil {
  if ai.IsNoImageGenerated(err) {
    fmt.Println("no image generated")
    return
  }
  panic(err)
}
_ = resp
```

Provider/network errors are returned as `*ai.Error` (see `docs/01-getting-started.md` for error helpers).

## Saving Images to Disk

```go
resp, _ := ai.GenerateImage(ctx, ai.GenerateImageRequest{
  Model:  openai.Image("gpt-image-1"),
  Prompt: "A pixel-art gopher",
})

os.WriteFile("out.png", resp.Image.Uint8Array, 0o644)
```

## Examples in this repo

- `go run ./examples/generate_image`

