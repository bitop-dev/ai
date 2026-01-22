# Phase 3 - providerutils package

## Goals
- Provide shared utilities used by both core APIs and provider adapters.
- Centralize schema handling, tool definitions, JSON parsing, and HTTP helpers.
- Deliver robust streaming and SSE parsing primitives.

## Scope
### Tool System
- Tool definition types (input schema, execute function, approval callback).
- Tool call/result structs compatible with `provider` stream parts.
- Tool result output types (text, json, error, content).
- Dynamic tool definition for runtime-provided tool metadata.
- Tool name mapping helpers for providers with internal tool IDs.
- Tool-call argument accumulator for streaming deltas.

### Schema Helpers
- JSON Schema wrapper types for validation and introspection.
- Schema validation interface to allow swapping libraries.
- Helpers for strict vs non-strict schema generation.

### JSON Parsing
- Safe JSON parsing with informative error types.
- Helpers to check parsability and convert to `JSONValue`.
- Option to preserve raw JSON on validation failures.

### HTTP Utilities
- Header combination, request building, and response handling helpers.
- `PostJSON` and `PostMultipart` utilities for providers.
- Retry helpers for transient errors and rate limits.
- Standard API call tracing hooks (request/response capture).
- Shared request options application (headers, timeout, idempotency key).

### Streaming and SSE
- SSE parser compatible with OpenAI and Anthropic event streams.
- Chunk decoding with parse/validation hooks.
- Helpers to map provider chunks to shared stream parts.
- SSE encoder/pipe helper for `http.ResponseWriter`.

### Data Content Helpers
- Data content types: raw bytes, base64, URLs.
- Media type inference from data and filename.
- URL support checks and normalization.

### Misc Utilities
- ID generation (UUID/ULID) with deterministic test helpers.
- Time helpers for timestamps in metadata.
- Minimal logging interfaces for diagnostics.
- Backoff and retry policy helpers for providers.

## Deliverables
- Go package `providerutils` with tool and schema helpers, HTTP + SSE utilities.
- Shared interfaces for schema validation and streaming.
- Tests for JSON parsing, header merging, SSE parsing, and tool handling.
- Tool call accumulator and SSE response writer helper.

## Implementation Notes
- Keep dependencies minimal; only add schema validation and SSE parsing libs if needed.
- Prefer Go stdlib `net/http` with `io.Reader` streaming support.
- Use `context.Context` in all HTTP utilities.
- Ensure consistent error wrapping for API failures.

## Dependencies
- Phase 2.

## Open Decisions
- JSON Schema validation library choice (recommended: `github.com/santhosh-tekuri/jsonschema/v5`).
- Streaming abstraction (channels vs iterator pattern).
- ID generation choice (UUID vs ULID).
