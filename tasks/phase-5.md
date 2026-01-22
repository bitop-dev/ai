# Phase 5 - ai core package

## Goals
- Port the high-level APIs and orchestration layer.
- Provide streaming and tool-loop behavior equivalent to AI SDK v6.
- Deliver a stable, idiomatic Go entrypoint for applications.

## Scope
### Core APIs
- `GenerateText` (non-streaming language model calls).
- `StreamText` (streaming language model calls).
- `GenerateObject` and `StreamObject` (structured output helpers or wrappers).
- `Embed` (single and batch embeddings).
- `GenerateImage` (multi-image generation).
- `GenerateSpeech` and `Transcribe`.
- `Rerank` (ranking over text + metadata).

### Tool Loop
- Parse model tool calls from content parts or streaming deltas.
- Tool approval flow (auto approval, manual approval, deny).
- Execute tool calls in sequence or parallel (depending on provider support).
- Merge tool results into new prompt turns.
- Retry policy and termination conditions (max steps, finish reason, errors).

### Prompt and Message Types
- `Prompt` struct with `System`, `User`, `Assistant`, `Tool` messages.
- Content parts: text, image, file, tool-call, tool-result, reasoning, source.
- Provider prompt conversion utilities for V3 models.

### Output Parsing
- Parse structured JSON when `ResponseFormat` is set.
- Support JSON schema validation and best-effort parsing on malformed JSON.
- Emit partial deltas for streaming structured output when available.

### Model Resolution
- `ResolveModel` helper for `providerId:modelId` strings.
- Registry support for default provider (gateway) and custom providers.
- Provider middleware chain (pre/post hooks, logging, retries).
- `pkg/registry` package with a reusable provider registry.

### Streaming Helpers
- Stream part iterator or channel wrapper type.
- Helpers to pipe to `http.ResponseWriter` with SSE framing.
- Optional UI message stream helpers (if feasible in Go).

### Telemetry
- Internal hooks for timing, token usage, warnings, and errors.
- Optional OpenTelemetry adapters in a separate package or build tag.

## Deliverables
- Go package `ai` with high-level entrypoints mirroring AI SDK v6 behavior.
- Stream result types with `Next`/`Value` or channel-based APIs.
- Tool loop tests (single tool, multi tool, tool denial).
- Streaming tests (text deltas, tool deltas, finish events).
- `pkg/registry` with registry and middleware wiring.

## Implementation Notes
- Keep `GenerateObject`/`StreamObject` as wrappers around `GenerateText`/`StreamText` with JSON output format.
- Provide small helpers for constructing prompts/messages (fluent builders optional).
- Use context cancellation to stop tool execution and provider calls.

## Dependencies
- Phase 2, Phase 3, Phase 4.

## Open Decisions
- Exact streaming API shape (channels vs iterator types).
- Whether to keep `GenerateObject`/`StreamObject` as first-class APIs or deprecated wrappers.
- How to expose tool approval hooks to callers.
