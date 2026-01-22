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
    "### Schema Validation",
]:
    if heading not in phase_text:
        raise SystemExit(f"Missing schema heading in phase-1.md: {heading}")

for snippet in [
    "github.com/santhosh-tekuri/jsonschema/v5",
    "Expose a minimal validation interface",
    "Dependency policy: stdlib-first, minimal dependencies",
]:
    if snippet not in phase_text:
        raise SystemExit(f"Missing schema decision in phase-1.md: {snippet}")

decision_text = decision_doc.read_text()
for heading in [
    "## Schema Validation and Dependency Policy Rationale",
]:
    if heading not in decision_text:
        raise SystemExit(f"Missing schema heading in phase-decision-1.md: {heading}")

for snippet in [
    "jsonschema",
    "swap validators",
    "stdlib-first",
]:
    if snippet not in decision_text:
        raise SystemExit(f"Missing rationale detail in phase-decision-1.md: {snippet}")
PY
