# Phase 1 - Foundation and Layout

## Goals
- Define the Go module identity (module path, version policy, Go version).
- Agree on public package names, naming conventions, and API ergonomics.
- Document what we will and will not port from the TS SDK.
- Establish the baseline error model, logging, and streaming style.

## Scope
- Module naming and versioning strategy.
- Public API design conventions (function names, options structs, context usage).
- Error strategy: typed errors and error wrapping conventions.
- Streaming API shape and cancellation approach.
- Backwards compatibility philosophy with AI SDK v6 (parity vs idiomatic Go).
- High-level package map (reinforces phase 0).

## Detailed Decisions
### Module Identity
- Module path: `github.com/vercel/ai-sdk-go`.
- Minimum Go version: 1.22 for generics and `slices`, `cmp` helpers.
- Version policy: semantic versioning with `v0` until core stability.
- Single module for all packages; public imports use `<module>/pkg/<name>` and providers use `<module>/pkg/providers/<provider>`.
- Module naming: all public packages live under `pkg/` and match their import path suffix.

### Public Package Naming
- Core: `pkg/ai` -> `ai` import path `ai-sdk-go/pkg/ai`.
- Interfaces/types: `pkg/provider`.
- Utilities: `pkg/providerutils`.
- Registry/middleware: `pkg/registry`.
- Providers: `pkg/providers/<provider>`.

### API Shape and Conventions
- All public entrypoints accept `context.Context`.
- Options are structs with optional pointer fields for clarity and defaults.
- Streaming results return an iterator-style wrapper with `Next()`/`Value()`/`Err()`/`Close()`; channels are optional convenience helpers.
- Errors are returned as Go `error` with typed errors implementing `Is` checks.
- Expose `Warnings` in results rather than logging by default.
- Standard `RequestOptions` includes headers, timeout, idempotency key, per-call metadata, and provider-specific overrides via `ProviderOptions`.
- Provider-specific overrides are carried via a `ProviderOptions` map on requests.

### Error Strategy
- Provide typed error types in `provider` (or `ai` for core) with `Unwrap` support.
- Use sentinel values for common categories (auth, rate-limit, invalid request, invalid response).
- Keep raw provider response fields on error structs for debugging.

### Streaming Strategy
### Streaming API Shape
- Iterator-first streaming API (`StreamText`/`StreamObject`) returning a `Stream` wrapper with `Next()`/`Value()`/`Err()`/`Close()`.
- Optional channel adapter helpers (`StreamToChannel`, `StreamFromChannel`) live in `providerutils` for users who prefer channel consumption.
- Stream parts use a unified type shared across providers for delta text, tool calls, finish reasons, and metadata.

### Streaming Strategy
- `StreamText` and `StreamObject` return stream parts using the shared stream part union.
- Context cancellation terminates in-flight HTTP requests and closes iterators; `Close()` must be idempotent and trigger cancellation.
- Stream goroutines must select on `ctx.Done()` and return `context.Canceled` when callers cancel.
- Convenience helpers pipe streams to `http.ResponseWriter` for SSE.

### Schema Validation
- Default schema validation library: `github.com/santhosh-tekuri/jsonschema/v5`.
- Expose a minimal validation interface so projects can replace the validator.
- Dependency policy: stdlib-first, minimal dependencies; third-party libs allowed only for clear gaps (schema validation) and wrapped behind interfaces to keep swaps low-friction.

### Telemetry Hooks
- Define a lightweight `Telemetry` interface for request/response timing, usage, and errors.
- `Telemetry` starts a request span and returns a `TelemetrySpan` for lifecycle callbacks.
- `TelemetrySpan.End` records duration, usage counters, warnings, and response metadata.
- `TelemetrySpan.Error` records duration plus the error payload for failed requests.
- `Telemetry` receives operation name, provider/model identifiers, and request metadata.
- Provide `NoopTelemetry` and `NoopSpan` helpers when telemetry is disabled.
- OpenTelemetry: no built-in dependency; users can adapt hooks to OTel if desired.

### Compatibility and Deviations
- Keep the same high-level API names: `GenerateText`, `StreamText`, `GenerateObject`, `StreamObject`, `Embed`, `GenerateImage`, `GenerateSpeech`, `Transcribe`, `Rerank`.
- Allow more idiomatic Go options and naming if necessary, but keep parity in behavior and defaults.
- JS-only runtime packages (React/Vue/Svelte/RSC, codemod, devtools) are excluded.

## Deliverables
- Documented module path and Go version in phase doc (and later in `go.mod`).
- API style guide (context, options, errors, streaming).
- Written compatibility matrix vs AI SDK v6 features.
- Streaming API decision and schema validation choice.
- Telemetry hook interface contract.

## Dependencies
- Phase 0.

## Notes and Non-Goals
- UI framework packages are not ported.
- JS-only build/test tooling is not ported.
- Valibot and Zod adapters are replaced by JSON Schema support or Go-native validation.

## Open Decisions
- Telemetry scope (OpenTelemetry integration vs optional hooks only).
