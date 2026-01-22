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

## Open Decisions
- Module layout (single vs multi-module) remains open until Phase 0 tasks are complete.
- Module path naming will be finalized in the module decision task.
