#!/usr/bin/env bash
# Render config.yaml from 1Password and upload it to R2 as config/config.yaml.
set -euo pipefail
cd "$(dirname "$0")/.."
export OP_ACCOUNT="${OP_ACCOUNT:-my.1password.com}"
BUCKET="${BUCKET:-cli-proxy-api}"
OUT="$(mktemp -t cli-proxy-config)"
trap 'rm -f "$OUT"' EXIT
op inject -i config.yaml -o "$OUT" -f >/dev/null
# Store only a bcrypt hash of the management password in R2.
HASH="$(op read op://cloudflare/cli-proxy-api/management-password | htpasswd -niBC 10 "" | tr -d ':\n' | sed 's/^\$2y\$/$2a$/')"
HASH="$HASH" python3 -c 'import os,sys; p=sys.argv[1]; s=open(p).read(); open(p,"w").write(s.replace("__MANAGEMENT_PASSWORD_BCRYPT__", os.environ["HASH"]))' "$OUT"
npx cf r2 objects put config/config.yaml --bucket-name "$BUCKET" --file "$OUT" --content-type text/yaml -q >/dev/null
echo "uploaded config/config.yaml to $BUCKET"
