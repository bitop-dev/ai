#!/usr/bin/env bash
set -euo pipefail

python3 - <<'PY'
from pathlib import Path

phase_doc = Path("tasks/phase-1.md")
decision_doc = Path("tasks/phase-decision-1.md")

if not phase_doc.exists():
    raise SystemExit("Missing tasks/phase-1.md")
if not decision_doc.exists():
    raise SystemExit("Missing tasks/phase-decision-1.md")

phase_text = phase_doc.read_text()
for heading in [
    "## Detailed Decisions",
    "### Module Identity",
    "### Public Package Naming",
]:
    if heading not in phase_text:
        raise SystemExit(f"Missing heading in phase-1.md: {heading}")

for snippet in [
    "github.com/vercel/ai-sdk-go",
    "Minimum Go version: 1.22",
    "Version policy: semantic versioning",
]:
    if snippet not in phase_text:
        raise SystemExit(f"Missing module identity detail in phase-1.md: {snippet}")

decision_text = decision_doc.read_text()
for heading in [
    "# Phase 1 Decisions",
    "## Module Identity Rationale",
    "## Module Naming Conventions Rationale",
]:
    if heading not in decision_text:
        raise SystemExit(f"Missing heading in phase-decision-1.md: {heading}")

if "github.com/vercel/ai-sdk-go" not in decision_text:
    raise SystemExit("Missing module path in phase-decision-1.md")
PY
