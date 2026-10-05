#!/usr/bin/env bash
# Render config.yaml from 1Password and upload it to R2 as config/config.yaml.
set -euo pipefail
cd "$(dirname "$0")/.."
BUCKET="${BUCKET:-cli-proxy-api}"
OUT="config.rendered.yaml"
trap 'rm -f "$OUT"' EXIT
op inject -i config.yaml -o "$OUT" -f >/dev/null
npx wrangler r2 object put "$BUCKET/config/config.yaml" --file "$OUT" --content-type text/yaml --remote
