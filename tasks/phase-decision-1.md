# Phase 1 Decisions

## Module Identity Rationale
- Keep the module path aligned with the repo name (`github.com/vercel/ai-sdk-go`) to prevent import churn.
- Target Go 1.22 to rely on recent stdlib additions without pulling extra dependencies.
- Stay in `v0` until the core surface stabilizes, enabling faster iteration without breaking semver promises.

## Module Naming Conventions Rationale
- Use `pkg/` for public packages so users can infer import paths from folder names.
- Keep providers under `pkg/providers/` to make discovery and documentation consistent.
- Reserve `internal/` for shared helpers that should not be imported directly by SDK consumers.

## Streaming API Shape Rationale
- Default to iterator-style streams to mirror Go conventions (`Next`/`Value`/`Err`) and avoid goroutine leaks from unbuffered channels.
- Keep optional channel adapters in `providerutils` so teams can adopt channels without making them the primary API.
- Use a unified stream part union to normalize provider delta payloads and finish reasons across models.

## Request Options and Cancellation Rationale
- Standardize `RequestOptions` with headers, timeout, idempotency key, and metadata to align with provider HTTP requirements and retries.
- Treat context cancellation as the primary way to stop streams, with `Close()` delegating to cancellation for deterministic cleanup.
- Require stream loops to select on `ctx.Done()` so HTTP requests and background goroutines terminate promptly.

## Schema Validation and Dependency Policy Rationale
- Use `github.com/santhosh-tekuri/jsonschema/v5` for JSON Schema compatibility without adding a heavy dependency graph.
- Keep schema validation behind a small interface so teams can swap validators (or disable validation) without API churn.
- Prefer stdlib-first, minimal dependencies to reduce downstream maintenance and keep provider packages lightweight.

## Open Decisions
- None for module identity and versioning.
