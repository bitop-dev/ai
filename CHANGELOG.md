# Changelog

All notable changes to this project will be documented in this file.

## v0.1.0 - 2025-12-17

### Added

- Core text APIs: `GenerateText`, `StreamText` (stream helpers: `Iter`, `Reader`).
- Tool calling: typed tools (`NewTool`), dynamic tools (`NewDynamicTool`), tool-loop limit (default 5), tool lifecycle hooks for streaming inputs, and tool progress events.
- Agent ergonomics: `Steps`, `OnStepFinish`, `PrepareStep`, `StopWhen`, conversation continuation via `Response.Messages`.
- `Agent` wrapper with internal implementation in `internal/agents`.
- Structured outputs: `GenerateObject[T]`, `StreamObject[T]` with JSON Schema enforcement and validation.
- Embeddings: `Embed`, `EmbedMany`, `CosineSimilarity`, batching + parallelism.
- Images: `GenerateImage` with batching (`N`, `MaxImagesPerCall`) + parallelism and provider metadata.
- Audio: `Transcribe` and `GenerateSpeech`.
- Multimodal chat inputs: image parts (`ImageURL`, `ImageBytes`, `ImageBase64`).
- MCP support: `mcp` client with HTTP/SSE/stdio transports, tool adapter, resources/prompts helpers, notifications/listening, caching/auto-refresh, and OAuth hooks.
- Developer documentation under `docs/` and runnable examples under `examples/`.

### Notes

- Currently targets OpenAI and OpenAI-compatible providers.
- Chat audio content parts are not supported; use `Transcribe` / `GenerateSpeech` instead.

