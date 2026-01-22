# Phase 6 - providers/openai

## Goals
- Implement OpenAI provider adapters for all supported modalities.
- Match AI SDK v6 request/response mapping, streaming parts, and tool handling.
- Provide a clean, configurable factory with sane defaults.

## Scope
### Provider Factory
- `CreateOpenAI` with settings:
  - API key (env fallback `OPENAI_API_KEY`).
  - Base URL override (default `https://api.openai.com/v1`).
  - Organization and project headers.
  - Custom headers and HTTP client override.
  - Provider name override (for 3rd party proxies).

### Language Models
- Chat models: `/chat/completions`.
- Responses models: `/responses`.
- Completion models: `/completions` (legacy).
- Model capabilities map for reasoning models and parameter restrictions.

### Other Modalities
- Embeddings: `/embeddings`.
- Images: `/images/generations`.
- Speech: `/audio/speech`.
- Transcriptions: `/audio/transcriptions`.

### Tool and Output Mapping
- Map SDK tools to OpenAI tool schemas (function tools, strict JSON schema).
- Translate tool call deltas and accumulate tool arguments in streaming.
- Support tool choice modes (`auto`, `none`, `required`, specific tool).

### Response Parsing
- Usage conversion (prompt/completion tokens, logprobs).
- Provider metadata extraction (logprobs, accepted/rejected prediction tokens).
- Finish reason mapping (stop/length/tool-calls/content-filter/other).

### Streaming
- SSE parsing for chunked deltas.
- Emit `text-start`, `text-delta`, `tool-input-*`, `tool-call`, and `finish` parts.
- Support raw chunk forwarding for debugging.

## Deliverables
- Go package `providers/openai` implementing `ProviderV3` and factory helpers.
- Model-specific options structs for chat/responses/completions.
- Tests for request serialization, response parsing, and streaming.

## Implementation Notes
- Keep provider options in a dedicated file, parsed via `providerutils`.
- Use a shared request builder for headers and URL composition.
- Ensure reasoning model parameter validation mirrors TS warnings.

## Dependencies
- Phase 2, Phase 3.
- Phase 5 for integration tests and examples.

## Open Decisions
- How strictly to validate OpenAI options vs pass-through.
- Whether to include file upload helpers or keep them out of scope.
