# Plan: Go AI SDK (v0)

## Goals (v0)

- Provider-agnostic public API in `ai`: `GenerateText`, `StreamText`, `GenerateObject[T]`
- OpenAI Chat Completions + OpenAI-compatible servers (`BaseURL`, `APIPrefix`, `Headers`)
- Streaming via SSE (`stream=true`), yielding incremental text deltas
- Tool loop for user tools + internal synthetic tool for `GenerateObject`
- Strong tests: mapping, SSE decoding, tool loop, retries; env-gated integration tests
- Leave room for v0.x additions: embeddings, images, audio, and tool-definition helpers (without breaking v0 API)

## Milestones

### M1 — Project skeleton

- Initialize Go module and package layout (`ai`, `openai`, `internal/...`)
- Define exported core types (`Message`, `ContentPart`, `Tool`, `Schema`, `Usage`, `FinishReason`, `Error`)
- Define internal provider registry + interfaces (`internal/provider`)

### M2 — OpenAI client + model refs

- Implement `openai.Config`, `openai.Configure`, `openai.NewClient`
- Implement `openai.Chat(model)` and `client.Chat(model)` (client-bound model refs)
- Implement request building for `POST /chat/completions` (including tools)
- Implement HTTP retry policy (timeouts, 408/409/429/5xx; full jitter; `Retry-After`)

### M3 — SSE streaming

- Implement `internal/sse` decoder for `data:` frames and `[DONE]`
- Implement OpenAI streaming adapter → `ai.TextStream` interface
- Ensure tool calls during streaming trigger internal loop and continued streaming

### M4 — Tool loop

- Implement tool-call detection, handler execution, and message appending
- Enforce `ToolLoopOptions.MaxIterations` (default 5, per-request override)
- Ensure deterministic behavior + fake provider support for unit tests

### M5 — GenerateObject[T]

- Inject synthetic tool `__ai_return_json` when `Schema` provided (collision check)
- Parse tool-call args as raw JSON, validate (JSON Schema when provided), decode into `T`
- Implement bounded retry loop based on `Strict` + `MaxRetries` with sanitized errors
- Fallback behavior for providers without tools (JSON-only prompting)

### M6 — Tests + examples

- Unit tests: mapping, SSE, tool loop, `GenerateObject` retry/validation
- Env-gated integration tests against OpenAI (GenerateText/StreamText/GenerateObject)
- Examples: `GenerateText`, `StreamText`, `GenerateObject`

---

## Next milestones (v0.x)

### M7 — Integration + DX polish

- Env-gated integration tests for OpenAI and at least one OpenAI-compatible server
- Streaming ergonomics: optional `TextStream.Reader()` and/or iterator helpers
- Provider errors surfaced as `*ai.Error` (status/code/retryable) with helpers working reliably
- `GenerateObject` polish: merge `Usage` across loop/retries; improve correction prompts
- Optional: `StreamObject` (v0.1)

### M7.5 — MCP (Model Context Protocol)

- Add `mcp` package that adapts MCP tools → `[]ai.Tool` (no changes to `ai` public API)
- Add MCP transports:

  - Streamable HTTP (POST + optional SSE response, session headers)
  - Server event stream (GET SSE)
  - Stdio transport (local dev)

- Implement MCP lifecycle handshake (`initialize` + `notifications/initialized`) and basic capability/version checks
- Support server → client notifications (e.g. tools list changes) where transport permits
- Support auth:

  - Static headers
  - Optional OAuth flow/provider hooks (transport-dependent)

- Expand MCP surface:

  - Resources: list/read + templates
  - Prompts: list/get with arguments
  - Elicitation requests (server asks client for structured user input)

### M8 — New endpoints (v0.x)

- Embeddings:

  - `Embed` (single) and `EmbedMany` (batch) helpers
  - OpenAI `/embeddings` mapping + tests + examples

- Images:

  - `GenerateImage` helper (OpenAI images endpoint)
  - Response includes URLs and/or base64; tests + example

- Audio:

  - `Transcribe` helper (speech-to-text)
  - `GenerateSpeech` helper (text-to-speech)
  - Tests + examples (env-gated)

### M9 — Multimodal messages (v0.x)

- Support `ImagePart` in request mapping (OpenAI multimodal chat input)
- Support `AudioPart` in request mapping where supported
- Decide and document how multi-part inputs map to OpenAI / OpenAI-compatible providers

### M9 — Tool authoring helpers (v0.x)

- `tool(...)` helper for static tools (ergonomic builder around `Tool`)
- `dynamicTool(...)` helper for runtime-defined schema/tools (provider-agnostic surface)

### M10 — Agentic loop ergonomics (v0.x)

- Expose per-step transcripts (messages/tool calls/tool results) for multi-step tool loops
- Add callbacks/hooks:

  - `onStepFinish` / step summary info (`text`, tool calls/results, `finishReason`, `usage`)
  - `prepareStep` (allow changing model/tool choice/active tools per step)

- Standardize “response messages” accumulation to make conversation continuation easy

### M11 — Client controls + consistency (v0.x)

- Standardize per-request `headers`, `maxRetries`, `abortSignal`/timeouts across all endpoints
- Improve error taxonomy + typed errors for feature-specific failures (e.g. MCP call errors)
