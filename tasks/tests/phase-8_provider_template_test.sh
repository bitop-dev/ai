#!/usr/bin/env bash
set -euo pipefail

python3 - <<'PY'
from pathlib import Path

phase_doc = Path("tasks/phase-8.md")
template_doc = Path("docs/providers/template.md")

if not phase_doc.exists():
    raise SystemExit("Missing tasks/phase-8.md")
if not template_doc.exists():
    raise SystemExit("Missing docs/providers/template.md")

phase_text = phase_doc.read_text()
for heading in [
    "### Provider Port Checklist",
    "### Docs Template",
]:
    if heading not in phase_text:
        raise SystemExit(f"Missing heading in phase-8.md: {heading}")

for snippet in [
    "docs/providers/<provider>.md",
    "docs/providers/template.md",
]:
    if snippet not in phase_text:
        raise SystemExit(f"Missing provider doc reference in phase-8.md: {snippet}")

template_text = template_doc.read_text()
for heading in [
    "# Provider Template",
    "## Configuration",
    "## Language Models",
    "## Tools and Structured Output",
    "## Streaming",
    "## Checklist",
]:
    if heading not in template_text:
        raise SystemExit(f"Missing heading in docs/providers/template.md: {heading}")

if "ProviderOptions" not in template_text:
    raise SystemExit("Missing ProviderOptions example in template.md")
PY
