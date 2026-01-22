# Phase 0 - Repository Structure

## Goals
- Establish a consistent Go repository layout that scales to many providers.
- Make it obvious where core APIs, provider adapters, tests, and examples live.
- Document the mapping from the TypeScript monorepo to Go packages.

## Scope
- Define top-level directories and naming conventions.
- Specify public vs internal packages and how they are referenced.
- Outline where phase docs, design docs, and migration notes live.
- Decide on module layout (single module vs multi-module).

## Proposed Directory Layout
```
./
  tasks/
    phase-0.md
    phase-1.md
    ...
  cmd/
    ai-sdk-tool/        # optional CLI or dev utility
  internal/
    streaming/          # shared stream parsing helpers
    httpx/              # HTTP helpers shared by providers
    schema/             # JSON schema helpers/validation
  pkg/
    ai/                 # high-level SDK APIs (GenerateText, StreamText, etc)
    provider/           # interfaces, shared types, error types
    providerutils/      # tools, schema types, stream parsing, helpers
    registry/           # provider registry and middleware
    gateway/            # Vercel gateway provider
    providers/
      openai/
      anthropic/
      google/
      ...
  examples/
    text-generation/
    streaming/
    tools/
    embeddings/
    images/
    speech/
    transcription/
    reranking/
  docs/
    api/
    migration/
    providers/
```

## Naming Conventions
- Public packages live under `pkg/` and are imported as `<module>/pkg/<name>`.
- Provider packages are always `pkg/providers/<provider>` and expose a `Create<Provider>()` factory.
- Provider import paths follow `<module>/pkg/providers/<provider>`.
- Internal helpers that should not be imported by consumers live under `internal/` and are never referenced by external modules.
- Package names use lowercase, no dashes; file names mirror package content (e.g. `stream_parts.go`).
- Tests live next to sources and mirror file names with `_test.go`.

## Mapping from TypeScript Monorepo
- `packages/ai` -> `pkg/ai`
- `packages/provider` -> `pkg/provider`
- `packages/provider-utils` -> `pkg/providerutils`
- `packages/gateway` -> `pkg/gateway`
- `packages/openai-compatible` -> `pkg/providers/openai-compatible`
- `packages/<provider>` -> `pkg/providers/<provider>`
- `packages/mcp` -> `pkg/mcp`
- UI/framework packages (`react`, `vue`, `svelte`, `rsc`, `angular`) -> Go examples or HTTP helpers as needed.
- Codemod, devtools, and JS build packages are not ported.
- `packages/test-server` -> `internal/testserver` (Go-native test fixtures)
- `packages/valibot` -> not ported; replace with schema adapter in `pkg/providerutils`

## Documentation Layout
- Phase docs remain in `tasks/` and act as the project roadmap.
- Public docs live in `docs/` with API reference and migration notes.
- Provider-specific notes live in `docs/providers/<provider>.md`.

## Testing and Examples Layout
- Unit tests: alongside code in `pkg/**` and `internal/**`.
- Integration tests: grouped by provider and guarded by env vars.
- Examples: runnable Go programs in `examples/` with minimal dependencies.

## Deliverables
- Directory map and naming conventions.
- Written mapping between TS packages and Go packages.
- Initial decisions for module layout and internal/public boundaries.

## Dependencies
- None.

## Open Decisions
- Single module vs multi-module layout for providers.
- Module path naming (recommended default: `ai-sdk-go`).
- Whether to include a CLI in `cmd/` or keep the repo library-only.
