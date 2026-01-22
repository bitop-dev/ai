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
    "### Telemetry Hooks",
]:
    if heading not in phase_text:
        raise SystemExit(f"Missing telemetry heading in phase-1.md: {heading}")

for snippet in [
    "`Telemetry` interface",
    "TelemetrySpan",
    "NoopTelemetry",
    "OpenTelemetry",
]:
    if snippet not in phase_text:
        raise SystemExit(f"Missing telemetry decision in phase-1.md: {snippet}")

decision_text = decision_doc.read_text()
for heading in [
    "## Telemetry Hooks Rationale",
]:
    if heading not in decision_text:
        raise SystemExit(f"Missing telemetry heading in phase-decision-1.md: {heading}")

for snippet in [
    "Telemetry",
    "NoopTelemetry",
    "OpenTelemetry",
]:
    if snippet not in decision_text:
        raise SystemExit(f"Missing telemetry rationale in phase-decision-1.md: {snippet}")
PY
