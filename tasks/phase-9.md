# Phase 9 - tests, examples, and docs

## Goals
- Provide comprehensive coverage and documentation for the Go port.
- Ensure parity with core features and provider behavior.
- Deliver runnable examples that mirror common AI SDK v6 use cases.

## Scope
### Unit Tests
- Core tool loop behavior (single tool, multiple tools, tool denial).
- Streaming part emission ordering and finish events.
- Structured output parsing and JSON schema validation.
- Provider registry and model resolution (providerId:modelId).
- Error classification and wrapping.
- `internal/testserver` fixtures and request/response capture helpers.

### Integration Tests
- Provider-specific integration tests behind env flags.
- Streaming tests for OpenAI and Anthropic.
- Embedding/image/speech/transcription tests for providers that support them.

### Examples
- Text generation (single prompt, multi-turn prompt).
- Streaming text generation with SSE output.
- Tool calling (manual approval, auto execution).
- Structured output with JSON schema.
- Embeddings and reranking.
- Image generation, speech generation, transcription.

### Documentation
- `README.md` with quickstart and installation.
- API reference for `ai` and provider packages.
- Provider-specific docs under `docs/providers`.
- Migration notes from TS AI SDK v6 to Go.
- Known limitations and parity table.

## Deliverables
- Test suite runnable via `go test ./...`.
- Example programs in `examples/`.
- Documentation set in `docs/` and README updates.
- `internal/testserver` helpers for provider tests.

## Implementation Notes
- Integration tests should skip when API keys are missing.
- Use golden files for stream part sequences where useful.
- Examples should avoid non-standard dependencies.

## Dependencies
- All previous phases.

## Open Decisions
- Whether to publish Go doc site or use README-only for v1.
- CI provider and baseline test matrix (Go versions, OSes).
