# Phase 1 Decisions

## Module Identity Rationale
- Keep the module path aligned with the repo name (`github.com/vercel/ai-sdk-go`) to prevent import churn.
- Target Go 1.22 to rely on recent stdlib additions without pulling extra dependencies.
- Stay in `v0` until the core surface stabilizes, enabling faster iteration without breaking semver promises.

## Module Naming Conventions Rationale
- Use `pkg/` for public packages so users can infer import paths from folder names.
- Keep providers under `pkg/providers/` to make discovery and documentation consistent.
- Reserve `internal/` for shared helpers that should not be imported directly by SDK consumers.

## Open Decisions
- None for module identity and versioning.
