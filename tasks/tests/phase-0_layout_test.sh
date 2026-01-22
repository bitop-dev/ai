#!/usr/bin/env bash
set -euo pipefail

python3 - <<'PY'
from pathlib import Path

phase_doc = Path("tasks/phase-0.md")
decision_doc = Path("tasks/phase-decision-0.md")

if not phase_doc.exists():
    raise SystemExit("Missing tasks/phase-0.md")
if not decision_doc.exists():
    raise SystemExit("Missing tasks/phase-decision-0.md")

phase_text = phase_doc.read_text()
for heading in [
    "## Proposed Directory Layout",
    "## Naming Conventions",
    "## Mapping from TypeScript Monorepo",
    "## Decision Rationale",
]:
    if heading not in phase_text:
        raise SystemExit(f"Missing heading in phase-0.md: {heading}")

packages_dir = Path("ai-sdk-6/packages")
if not packages_dir.exists():
    raise SystemExit("Missing ai-sdk-6/packages directory for mapping validation")

package_names = sorted(
    path.name for path in packages_dir.iterdir() if path.is_dir()
)
missing_packages = [
    name for name in package_names if f"packages/{name}" not in phase_text
]
if missing_packages:
    missing_list = ", ".join(missing_packages)
    raise SystemExit(
        "Missing package mappings in phase-0.md: "
        f"{missing_list}"
    )

decision_text = decision_doc.read_text()
for heading in [
    "# Phase 0 Decisions",
    "## Repository Layout Rationale",
    "## Naming Conventions Rationale",
    "## Documentation Placement",
]:
    if heading not in decision_text:
        raise SystemExit(f"Missing heading in phase-decision-0.md: {heading}")
PY
