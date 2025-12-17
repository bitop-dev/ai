# Tasks (v0)

## Decision checks (quick)

- [x] Confirm default `openai.Config.MaxRetries` (`2`)
- [x] Confirm JSON Schema validator choice (`santhosh-tekuri/jsonschema/v5`)
- [x] Confirm `ai` resolves a client-bound OpenAI `ModelRef` internally via unexported interface/type assertion

## Build tasks

### Repo + types

- [x] `go mod init` + minimal README
- [x] Create packages: `ai`, `openai`, `internal/provider`, `internal/openai`, `internal/sse`, `internal/httpx` (optional)
- [x] Implement exported types in `ai` per PRD (`Message`, parts, tools, schema, usage, errors)

### Provider plumbing

- [x] Internal provider registry keyed by `ModelRef.Provider()`
- [x] Map exported requests → internal request format

### OpenAI client/provider

- [x] Implement `openai.Config`, `Configure`, `NewClient`
- [x] Implement `openai.Chat(model)` and `client.Chat(model)` model refs
- [x] Implement chat completion request/response structs internally (no leakage to `ai`)
- [x] Implement HTTP retry policy (timeouts, 408/409/429/5xx, jitter, `Retry-After`)

### SSE + streaming

- [x] Implement SSE decoder (`data:` frames, `[DONE]`, error propagation)
- [x] Implement OpenAI streaming → `ai.TextStream` (`Next/Delta/Message/Err/Close`)
- [x] Support “stream continues across tool calls” behavior

### Tools + loops

- [x] Detect tool calls in model outputs (non-stream + stream)
- [x] Execute tool handlers; append tool result messages
- [x] Enforce `ToolLoopOptions.MaxIterations` (default 5; per-request override)

### GenerateObject

- [x] Inject synthetic tool `__ai_return_json` when `Schema` present (collision check)
- [x] Validate JSON against schema (when provided) and decode into `T`
- [x] Implement bounded retry loop (`Strict`, `MaxRetries`)
- [x] JSON-only prompting fallback when tools unsupported

### Tests

- [x] Unit tests: request mapping
- [x] Unit tests: SSE parsing
- [x] Unit tests: tool loop behavior
- [x] Unit tests: `GenerateObject` validation + retries (fake provider)
- [ ] Integration tests (env-gated): OpenAI GenerateText/StreamText/GenerateObject

### Examples

- [x] Example: `GenerateText`
- [x] Example: `StreamText`
- [x] Example: `GenerateObject[T]`
- [x] Example: `StreamObject[T]`

---

# Tasks (v0.x / next)

### Integration + DX

- [x] Integration tests (env-gated): OpenAI-compatible server via `OPENAI_BASE_URL`
- [x] Streaming DX: `TextStream.Reader()` helper
- [x] Streaming DX: `TextStream.Iter()` (or channel helper)

### Errors

- [x] Return `*ai.Error` from OpenAI provider (status/code/retryable + cause)
- [x] Make `IsRateLimited/IsAuth/IsTimeout/IsCanceled` reliable for provider errors

### GenerateObject polish

- [x] Merge `Usage` across tool loop + retries in `GenerateObject`
- [x] Merge `Usage` across internal iterations in `StreamText` (when tools used)
- [x] Improve correction prompts (sanitized + bounded)
- [x] `StreamObject` (v0.1) + example

### Embeddings

- [x] Add `Embed` API (single input)
- [x] Add `EmbedMany` API (batch)
- [x] Implement OpenAI `/embeddings` provider support + unit tests
- [x] Add examples for embeddings
- [x] Add per-request `headers` + `maxRetries` for embeddings
- [x] Add `providerOptions.openai` support (e.g. `dimensions`, `encoding_format`)
- [x] Add `maxParallelCalls` batching for `EmbedMany`
- [x] Add `CosineSimilarity` helper

### Images

- [x] Add `GenerateImage` API
- [x] Implement OpenAI images provider support + unit tests
- [x] Add example for images
- [x] Add batching (`n`, `maxImagesPerCall`) + `maxParallelCalls`
- [x] Add per-request `headers` + `maxRetries` and `providerOptions.openai` (quality/style)

### Audio

- [x] Add `Transcribe` API (speech-to-text)
- [x] Add `GenerateSpeech` API (text-to-speech)
- [x] Implement OpenAI audio provider support + unit tests
- [x] Add examples for audio

### Tool authoring helpers

- [x] Add typed tool helper (`ai.NewTool`)
- [x] Add dynamic tool helper (`ai.NewDynamicTool`)
