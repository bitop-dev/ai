# Multimodal Chat Inputs

This library currently supports **image inputs** for chat messages when using OpenAI Chat Completions (and compatible providers).

Audio as a chat message content part is **not** supported for OpenAI chat completions in this library.
Use `Transcribe` / `GenerateSpeech` for audio workflows.

## Image Parts

An image input is represented as `ai.ImagePart` (a `ContentPart`).

You can create image parts using helpers:

- `ai.ImageURL(url)`
- `ai.ImageBytes(mediaType, bytes)`
- `ai.ImageBase64(mediaType, b64)`

## Send a message with text + image

```go
msg := ai.Message{
  Role: ai.RoleUser,
  Content: []ai.ContentPart{
    ai.TextPart{Text: "Describe the image in one sentence."},
    ai.ImageURL("https://example.com/cat.png"),
  },
}

resp, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
  BaseRequest: ai.BaseRequest{
    Model: openai.Chat("gpt-4o-mini"),
    Messages: []ai.Message{msg},
  },
})
```

## Data URLs (bytes/base64)

If you provide bytes/base64, the OpenAI chat provider will encode a `data:` URL internally.

```go
b, _ := os.ReadFile("cat.png")

msg := ai.Message{
  Role: ai.RoleUser,
  Content: []ai.ContentPart{
    ai.TextPart{Text: "Describe the image."},
    ai.ImageBytes("image/png", b),
  },
}
```

## Streaming

Multimodal inputs work with `StreamText` the same way as `GenerateText`:

```go
stream, err := ai.StreamText(ctx, ai.StreamTextRequest{
  BaseRequest: ai.BaseRequest{
    Model: openai.Chat("gpt-4o-mini"),
    Messages: []ai.Message{msg},
  },
})
```

## Limitations / Notes

- Only OpenAI chat `image_url` style content parts are supported right now.
- `ai.AudioPart` as a chat content part is rejected by the OpenAI chat provider in this library.

## Example in this repo

- `go run ./examples/multimodal_image`

