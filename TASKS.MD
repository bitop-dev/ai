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

---

# Tasks (v0.x / next up)

### MCP (Model Context Protocol)

- [x] Add `mcp` client package that adapts MCP tools → `[]ai.Tool`
- [x] Add Streamable HTTP transport basics (POST + optional SSE response)
- [x] Add stdio transport (local)
- [x] Add MCP tools DX options (prefix + allow/deny + schema overrides)
- [x] Add MCP resource/prompt helper converters
- [x] Add MCP tool examples (`mcp_tools`, `mcp_stream_text`)
- [x] Implement MCP lifecycle handshake (`initialize` + `notifications/initialized`)
- [x] Support SSE responses for POST + GET server event stream (Streamable HTTP)
- [x] Add notification hook (`Client.OnNotification`) + `Client.Listen`
- [x] Add cached list helpers + invalidation on `*_list_changed` notifications (`ToolsCached`, `ListResourcesCached`, `ListResourceTemplatesCached`, `ListPromptsCached`)
- [x] Add resource templates (`resources/templates/list`)
- [x] Add higher-level “auto refresh” helpers (callbacks that re-fetch tools/resources/prompts on list change)
- [x] Improve auth story (static headers + dynamic headers via `HTTPTransport.HeaderProvider`)
- [x] Add OAuth hook surface (via `HTTPTransport.AuthProvider` / `HeaderProvider`)
- [x] Add full OAuth flow helpers (client credentials) (`OAuthClientCredentialsProvider`)
- [x] Expand elicitation beyond stdio where transport supports it (`Client.Listen` handles `elicitation/create`)
- [x] Add MCP error taxonomy basics (`ClientError`, `RPCError`, `CallToolError`)
- [x] Add richer HTTP error context (`HTTPStatusError`) and basic init vs call separation via `ClientError.Op`
- [x] Add richer error context (response headers/session id) and helper predicates (`IsInitError`, `IsCallToolError`, `IsHTTPStatusError`, `IsRPCError`)

### Multimodal messages

- [x] Implement `ImagePart` request mapping (OpenAI multimodal messages)
- [x] Implement `AudioPart` request mapping (where supported)
- [x] Add tests + examples for multimodal message inputs

### Tool lifecycle + progress

- [x] Add tool input lifecycle hooks for streaming (`onInputStart`, `onInputDelta`, `onInputAvailable`)
- [x] Add a pattern for streaming “tool status/progress” during tool execution (optional, non-breaking)

### Steps / agentic loop ergonomics

- [x] Expose step transcripts from multi-step tool loops (text/tool calls/tool results per step)
- [x] Add `onStepFinish` callback and step summary types
- [x] Add `prepareStep` hook (override model/toolChoice/activeTools/messages per step)
- [x] Standardize conversation continuation helper (`response.messages` equivalent)

### Consistency / client controls

- [x] Standardize `headers` + `maxRetries` across all endpoints (text/object/images/audio/embeddings)
- [x] Add per-request timeout/abort plumbing across all endpoints (where possible)
- [x] Expand typed error helpers for feature-specific failures (esp. MCP)
