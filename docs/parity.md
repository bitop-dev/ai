# Parity and coverage

This repository is a Go reimplementation of AI SDK v6. It focuses on a Go-first
API surface; migration guides from the TypeScript SDK are not included.

## Core coverage

| Area | Status | Notes |
| --- | --- | --- |
| Text generation | Implemented | `GenerateText`, `StreamText` over `LanguageModel`. |
| Structured output | Implemented | `GenerateObject` + `StreamObject`, parsing on collect. |
| Tool loop | Implemented | Tool approvals and max-step limit. |
| Streaming helpers | Implemented | `Stream` iterator and `PipeStream` SSE helper. |
| Telemetry hooks | Implemented | `Telemetry` interface and no-op default. |
| Other modalities | Implemented | Embed, image, speech, transcription, rerank wrappers. |
| Provider registry | Implemented | `providerId:modelId` resolution and middleware. |
| MCP client | Implemented | Stdio/HTTP transports and tool bridging. |
| Gateway provider | Partial | Language model streaming only. |

## Providers

Provider packages included in this repo:

- Amazon Bedrock, Anthropic, AssemblyAI, Azure, Baseten, Black Forest Labs,
  Cerebras, Cohere, Deepgram, DeepInfra, DeepSeek, ElevenLabs, Fal, Fireworks,
  Gladia, Google, Google Vertex, Groq, Hugging Face, Hume, LangChain,
  LlamaIndex, LMNT, Luma, Mistral, OpenAI, OpenAI Compatible, Perplexity,
  Prodia, Replicate, Rev AI, Together AI, Vercel, xAI.

See `docs/providers/` for provider-specific coverage notes.
