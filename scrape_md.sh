#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <url>"
  exit 1
fi

URL="$1"
OUT_DIR="/Users/nickcecere/Projects/GOLANG/ai/_refrences"

: "${RAITO_TOKEN:?RAITO_TOKEN is not set}"

mkdir -p "$OUT_DIR"
tmp="$(mktemp)"

curl -sS -X POST 'https://raito.bitop.dev/v1/scrape' \
  -H "Authorization: Bearer ${RAITO_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "$(jq -n --arg url "$URL" '{url:$url, formats:["markdown"]}')" \
  > "$tmp"

title="$(jq -r '.data.metadata.title // "untitled"' "$tmp")"
filename="$(printf '%s' "$title" \
  | tr '[:upper:]' '[:lower:]' \
  | sed -E 's/[^a-z0-9]+/-/g; s/^-+|-+$//g')"

jq -r '.data.markdown' "$tmp" > "${OUT_DIR}/${filename}.md"

rm -f "$tmp"

echo "Wrote: ${OUT_DIR}/${filename}.md"
