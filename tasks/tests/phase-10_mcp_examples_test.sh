#!/usr/bin/env bash
set -euo pipefail

required=(
  "examples/mcp-stdio/main.go"
  "examples/mcp-http/main.go"
  "docs/providers/mcp.md"
)

for path in "${required[@]}"; do
  if [[ ! -f "${path}" ]]; then
    echo "Missing file: ${path}" >&2
    exit 1
  fi
done

go test ./examples/mcp-stdio ./examples/mcp-http
