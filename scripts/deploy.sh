#!/usr/bin/env bash
# Build and deploy with cf, uploading Worker secrets resolved from 1Password.
set -euo pipefail
cd "$(dirname "$0")/.."
export OP_ACCOUNT="${OP_ACCOUNT:-my.1password.com}"
SECRETS="$(mktemp -t cli-proxy-secrets)"
trap 'rm -f "$SECRETS"' EXIT
op inject -i secrets.env.tpl -o "$SECRETS" -f >/dev/null
npx cf deploy --secrets-file "$SECRETS" "$@"
