# AI Handoff: `github.com/bitop-dev/ai`

This document is written for an AI coding agent that will work on this repository later. It describes the library’s goals, architecture, and conventions, and points to the right files for deeper context.

## What this repo is

`github.com/bitop-dev/ai` is a Go AI SDK with:

- A **provider-agnostic public API** (`package ai` at repo root)
- A current focus on **OpenAI + OpenAI-compatible providers**
- Feature areas: text + streaming, tools + agentic loops, structured output (objects), embeddings, images, audio, multimodal (images), and MCP.

## Design goals

- Keep the **public DX simple**:
  - Users import `github.com/bitop-dev/ai` for core APIs + types.
  - Users import `github.com/bitop-dev/ai/openai` for configuration + model references.
  - Users import `github.com/bitop-dev/ai/mcp` for MCP clients/tool bridging.
- Keep `package ai` stable; move logic into `internal/...` as it grows.
- Prefer correctness and predictable behavior:
  - Tool loop bounds, schema validation, usage aggregation, retry policy, and good error typing.

## Repo layout (high level)

### Public API: root `package ai`

The root is the main public package. Files are grouped by prefix:

- `core_*.go` — public types and primitives (`Message`, parts, tools, streams, errors, usage).
- `api_*.go` — public entrypoints (text/object/embeddings/images/audio/agents wrappers).
- `*_mapping.go` — mapping glue between `ai` public types and `internal/provider` types.
- `tools_*.go` — tool helpers + tool error types.
- `providers_init.go` — default provider registration via blank imports.

### Internal implementation: `internal/...`

All “real” implementation lives in internal packages to keep the root readable:

- `internal/provider` — provider interfaces and shared provider types.
- `internal/openai` — OpenAI/OpenAI-compatible implementation (chat completions, embeddings, images, audio).
- `internal/text` — tool-loop aware text generate/stream, step tracking.
- `internal/object` — structured output enforcement via internal tool call + streaming partial JSON.
- `internal/embeddings` — batching/parallelization helpers + cosine similarity.
- `internal/images` — image batching helpers + metadata merge.
- `internal/audio` — audio input resolution helper.
- `internal/agents` — internal entrypoint wrapper for agentic loops (thin wrapper over `internal/text`).
- `internal/httpx`, `internal/sse` — HTTP retry and SSE parsing utilities.

### MCP: `mcp/`

Standalone MCP client + transports + adapters:

- HTTP transport (including SSE response variants)
- stdio transport for local servers
- tools adapter to `[]ai.Tool`
- resources/prompts helpers
- notification/listen loop
- caching + auto-refresh
- auth hooks + OAuth client credentials helper

## Key architectural constraint (avoid cycles)

`internal/*` packages must **not** import `package ai` (root), or you will create import cycles.

The direction is:

- `package ai` imports `internal/...`
- `internal/...` never imports `ai`

All mapping between public types and provider types happens in `package ai`.

## Provider model

Providers are resolved by `ModelRef.Provider()`:

- Public interface: `type ModelRef interface { Provider() string; Name() string }`
- `providerForModel` uses the internal provider registry (`internal/provider`).
- OpenAI model refs are created via `openai.Chat(...)`, `openai.Image(...)`, etc.

Provider registration:

- Providers register themselves (init-time) in `internal/provider` registry.
- Default providers are enabled by blank import(s) in `providers_init.go`.

## Public API surface (what users call)

### Text

- `GenerateText(ctx, GenerateTextRequest)`
- `StreamText(ctx, StreamTextRequest)` returning `*TextStream` with `Next/Delta/Err/Close` plus `Iter()` and `Reader()`

### Tools + loops

- Tools are provided as `[]Tool` on `BaseRequest`.
- Helpers:
  - `NewTool(name, ToolSpec[Input,Output])`
  - `NewDynamicTool(name, DynamicToolSpec)`
- Loop control:
  - `ToolLoopOptions.MaxIterations` (default 5)
  - `ToolLoopOptions.StopWhen` (optional)
- Step ergonomics:
  - `BaseRequest.OnStepFinish`
  - `BaseRequest.PrepareStep`
  - `GenerateTextResponse.Steps` / `TextStream.Steps()`
- Conversation continuation:
  - `GenerateTextResponse.Response.Messages`
  - `TextStream.Response().Messages`
- Tool lifecycle hooks (streaming tool args):
  - `Tool.OnInputStart`, `Tool.OnInputDelta`, `Tool.OnInputAvailable`
- Tool progress:
  - `BaseRequest.OnToolProgress` + `ToolExecutionMeta.Report(...)`

### Agent wrapper

- `ai.Agent` lives in `api_agent.go` and is a convenience wrapper around `GenerateText/StreamText`.
- Internally, agentic looping is implemented by `internal/text` and surfaced via `internal/agents`.

### Structured output (objects)

- `GenerateObject[T]`
- `StreamObject[T]`
- Enforced via internal tool `__ai_return_json` + JSON Schema validation.

### Embeddings

- `Embed`
- `EmbedMany` (supports parallel calls)
- `CosineSimilarity`

### Images

- `GenerateImage` (supports batching for `N`, `MaxImagesPerCall`, and `MaxParallelCalls`)

### Audio

- `Transcribe`
- `GenerateSpeech`

### Multimodal chat inputs

- Chat message inputs support `ai.ImagePart` and helpers (`ImageURL/ImageBytes/ImageBase64`).
- Chat `ai.AudioPart` is **not supported** for OpenAI chat-completions in this library; use audio endpoints.

### MCP

- `mcp.NewClient(...)` + transports + tools/resources/prompts.
- MCP tools can be converted into `[]ai.Tool` and used with `GenerateText/StreamText`.

## Error model

Provider errors are wrapped as `*ai.Error` with:

- `Provider`, `Code`, `Status`, `Retryable`, `Cause`

Helpers:

- `IsRateLimited`, `IsAuth`, `IsTimeout`, `IsCanceled`

Tool errors:

- `NoSuchToolError`
- `InvalidToolInputError`
- `ToolExecutionError`

Feature-specific “no output” errors:

- `NoImageGeneratedError` (`IsNoImageGenerated`)
- `NoTranscriptGeneratedError` (`IsNoTranscriptGenerated`)
- `NoSpeechGeneratedError` (`IsNoSpeechGenerated`)

## Retries vs timeouts

Two different layers exist:

1. **HTTP retry** (provider transport):
   - Global default: `openai.Config.MaxRetries`
   - Per-request override:
     - text/object: `BaseRequest.MaxRetries`
     - embeddings/images/audio: `MaxRetries` on the request struct
2. **Schema/parse retry** (GenerateObject only):
   - `GenerateObjectRequest.MaxRetries`

Timeouts:

- text/object: `BaseRequest.Timeout`
- embeddings/images/audio: `Timeout` field on the request

All timeouts use `context.WithTimeout`.

## Where to look for existing implementations

- Text loop engine: `internal/text/generate.go`, `internal/text/stream.go`, `internal/text/steps.go`
- Agent internal entrypoint: `internal/agents/agents.go`
- Public wrappers + wiring:
  - `api_text.go` (core wrapper logic)
  - `api_agent.go` (Agent wrapper)
  - `api_object.go`, `api_embeddings.go`, `api_images.go`, `api_audio.go`
- Mapping glue: `ai_request_mapping.go`, `ai_provider_mapping.go`
- OpenAI provider:
  - chat: `internal/openai/provider.go`
  - embeddings/images/audio: `internal/openai/*.go`
- JSON Schema: `schema_validation.go`, `internal/schema/validate.go`
- MCP: `mcp/client.go` and `mcp/transport_*.go`

## How to add a new feature (pattern)

1. Add/extend public request/response types in `core_types.go` or a new `api_*.go` request type (depending on feature).
2. Implement public entrypoint in `api_<feature>.go`:
   - validate inputs
   - map public types → provider types
   - call internal implementation
   - map provider results/errors → public types
3. Put heavy logic in `internal/<feature>/...`.
4. Extend provider interfaces in `internal/provider` if needed.
5. Implement in `internal/openai` (for now) and add unit tests.
6. Add an example under `examples/<feature>/`.
7. Add/extend docs under `docs/`.

## How to add a new provider (pattern)

1. Add provider package (e.g. `internal/<provider>` and optionally public helper package like `openai/`).
2. Implement the required interfaces from `internal/provider`:
   - text: `provider.Provider` (Generate + Stream)
   - plus optional interfaces for embeddings/images/audio as supported
3. Register the provider in init.
4. Provide `ModelRef` constructors for that provider.
5. Decide whether it’s enabled by default via `providers_init.go` or opt-in via blank import.

## Testing & commands

- Unit tests: `go test ./...`
- Integration tests: env-gated in `integration_test.go` (see `.env.example`)
- Examples: `go run ./examples/<name>`

## Docs for humans (and agents)

- Full developer docs index: `docs/README.md`
- Repo instructions: `AGENTS.md`
- Release notes: `CHANGELOG.md`

