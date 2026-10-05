#!/usr/bin/env bash
# Copy Worker secrets from 1Password to Cloudflare without printing them.
set -euo pipefail
cd "$(dirname "$0")/.."
ITEM="op://YOUR_VAULT/YOUR_ITEM"
put() { op read "$ITEM/$2" | npx wrangler secret put "$1" >/dev/null && echo "set $1"; }
put OBJECTSTORE_ENDPOINT r2-endpoint
put OBJECTSTORE_BUCKET r2-bucket
put OBJECTSTORE_ACCESS_KEY r2-access-key-id
put OBJECTSTORE_SECRET_KEY r2-secret-access-key
put MANAGEMENT_PASSWORD management-password
