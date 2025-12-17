# AI Agent Notes (github.com/bitop-dev/ai)

This repository is a Go library that intentionally keeps a **simple public DX**:

- Users import `github.com/bitop-dev/ai` for the core API/types.
- Users import `github.com/bitop-dev/ai/openai` (optional) for OpenAI config helpers and/or explicit provider init.

## Repo Layout (intentional)

### Public package: `package ai` (root)

The root is the only public package (besides provider helper packages like `openai/`).
It contains:

- `core_*.go`: public types and shared primitives (`Message`, `Tool`, streams, `Usage`, `Error`, etc.).
- `api_*.go`: public entrypoints (`GenerateText`, `StreamText`, `GenerateObject`, `StreamObject`, `Embed`, `EmbedMany`, `GenerateImage`, `Transcribe`, `GenerateSpeech`, ...).
- `tools_*.go`: public tool DX helpers (`NewTool`, `NewDynamicTool`) and public tool-related error types.
- `*_mapping.go`: mapping glue between public `ai` types and `internal/provider` types.
- `providers_init.go`: side-effect imports that register default providers (currently OpenAI) so `import "github.com/bitop-dev/ai"` works out of the box.

### Internal implementation: `internal/...`

All heavy implementation lives under `internal/` to keep the root package readable and stable:

- `internal/text`: tool-loop aware generate + stream text engine.
- `internal/object`: tool-enforced structured output + streaming partial objects.
- `internal/embeddings`: batch + parallel embedding helpers, cosine similarity implementation.
- `internal/images`: image batching + provider metadata merge helpers.
- `internal/audio`: audio input resolution (bytes/base64/url).
- `internal/schema`: JSON schema validation (used by tools + structured output).
- `internal/tools`: tool-call extraction + usage aggregation helpers (provider-type level).
- `internal/provider`: provider-agnostic interfaces and types, plus a registry.
- `internal/openai`: OpenAI (and OpenAI-compatible) provider implementation and registration.
- `internal/httpx`, `internal/sse`: shared HTTP retry + SSE parsing utilities.

## Key Design Constraint

Avoid import cycles:

- `internal/*` packages must **not** import `package ai`.
- `package ai` is the boundary layer that maps `ai.*` public types to/from `internal/provider` types and bridges tool execution.

## Provider Registration

Providers register themselves into `internal/provider` registry at init time.

- Default providers are enabled via `providers_init.go` using blank imports.
- Additional providers can be:
  - enabled by default (add blank import to `providers_init.go`), or
  - opt-in by requiring the user to `import _ "github.com/bitop-dev/ai/<provider>"`.

## Testing Conventions

- Unit tests live with the public API and use a fake provider registered in-process.
- Helpers that reference `testing` must live in `_test.go` files (e.g. `test_fake_provider_test.go`) to avoid leaking `testing` into the library build.
- Integration tests are env-gated (see `integration_test.go` and `.env.example`).

## Naming Conventions

- Prefer `api_*.go` for public entrypoints.
- Prefer `core_*.go` for shared public primitives.
- Keep implementation logic out of root where possible (put it under `internal/<area>` and keep root as thin wrappers).

