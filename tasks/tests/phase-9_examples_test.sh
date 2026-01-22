#!/usr/bin/env bash
set -euo pipefail

examples_dir="examples"
if [[ ! -d "${examples_dir}" ]]; then
  echo "Missing examples/ directory" >&2
  exit 1
fi

expected=(
  text_generation
  multi_turn_chat
  streaming_sse
  tool_calling_auto
  tool_calling_manual
  structured_output
  embeddings_rerank
  image_generation
  speech_generation
  transcription
)

for example in "${expected[@]}"; do
  path="${examples_dir}/${example}/main.go"
  if [[ ! -f "${path}" ]]; then
    echo "Missing example: ${path}" >&2
    exit 1
  fi
done

go test ./examples/...
