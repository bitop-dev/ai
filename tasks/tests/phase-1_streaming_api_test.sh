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
    "### Streaming API Shape",
    "### Streaming Strategy",
]:
    if heading not in phase_text:
        raise SystemExit(f"Missing streaming heading in phase-1.md: {heading}")

for snippet in [
    "Standard `RequestOptions` includes headers, timeout, idempotency key",
    "Iterator-first streaming API",
    "StreamToChannel",
    "Context cancellation terminates",
]:
    if snippet not in phase_text:
        raise SystemExit(f"Missing streaming decision in phase-1.md: {snippet}")

decision_text = decision_doc.read_text()
for heading in [
    "## Streaming API Shape Rationale",
    "## Request Options and Cancellation Rationale",
]:
    if heading not in decision_text:
        raise SystemExit(f"Missing heading in phase-decision-1.md: {heading}")

for snippet in [
    "iterator-style streams",
    "RequestOptions",
    "context cancellation",
]:
    if snippet not in decision_text:
        raise SystemExit(f"Missing rationale detail in phase-decision-1.md: {snippet}")
PY
