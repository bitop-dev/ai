# Phase 0 Decisions

## Repository Layout Rationale
- Use a clear split between `pkg/` (public APIs) and `internal/` (shared helpers) to make exported surfaces explicit.
- Keep provider implementations under `pkg/providers/` so provider discovery and documentation stay predictable.
- Reserve `cmd/` for optional tooling so library consumers are not forced into a CLI dependency.
- Group examples by capability to mirror the SDK surface area and simplify onboarding.

## Naming Conventions Rationale
- Standardize import paths as `<module>/pkg/<name>` so users can infer paths from package names.
- Use lowercase package names and snake_case filenames to align with Go conventions and avoid ambiguity.
- Keep provider factories as `Create<Provider>` to make instantiation consistent across adapters.

## Documentation Placement
- Keep phase docs in `tasks/` to preserve the roadmap near implementation work.
- Use `docs/` for public-facing guidance and migration notes to avoid mixing planning and published docs.

## Module Layout and Module Path
- Use a single Go module to avoid cross-module replace directives while the SDK is still evolving.
- Adopt `github.com/vercel/ai-sdk-go` so package import paths remain stable and match the project name.
- Keep public packages in `pkg/` so users can import `github.com/vercel/ai-sdk-go/pkg/...` regardless of provider.

## Open Decisions
- None for Phase 0.
