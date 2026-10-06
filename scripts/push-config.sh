#!/usr/bin/env bash
# Render config.yaml from 1Password, store it in R2, and reload the running server.
#   scripts/push-config.sh
#   R2_BUCKET=cli-proxy-api-dev SKIP_RELOAD=1 scripts/push-config.sh   # dev bucket only
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"
cd "$REPO_ROOT" || exit 1

PLACEHOLDER="__MANAGEMENT_PASSWORD_BCRYPT__"

# Only a bcrypt hash of the management password is stored. htpasswd emits $2y$; Go's bcrypt
# and the server's "already hashed" check both accept the equivalent $2a$ prefix.
management_password_hash() {
  # shellcheck disable=SC2016 # literal $2y$/$2a$ prefixes
  secret management-password | htpasswd -niBC 10 "" | tr -d ':\n' | sed 's/^\$2y\$/$2a$/'
}

render_config() {
  local out="$1" hash
  op inject -f -i config.yaml -o "$out" >/dev/null
  hash="$(management_password_hash)"
  sed -i '' "s|$PLACEHOLDER|$hash|" "$out"
  grep -q "$PLACEHOLDER" "$out" && die "management password placeholder was not replaced"
  return 0
}

upload_to_r2() {
  npx cf r2 objects put config/config.yaml --bucket-name "$R2_BUCKET" \
    --file "$1" --content-type text/yaml -q >/dev/null
  log "uploaded config/config.yaml to R2 bucket $R2_BUCKET"
}

# The server only reads R2 at startup, so also PUT the file through the management API;
# the server applies it immediately and writes it back to R2 itself.
reload_live_server() {
  local status
  status=$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$PROXY_URL/v0/management/config.yaml" \
    -H "Authorization: Bearer $(secret management-password)" \
    -H 'content-type: application/yaml' --data-binary @"$1")
  if [[ "$status" == 200 ]]; then
    log "reloaded config on $PROXY_URL"
  else
    log "live reload skipped (HTTP $status); the config applies on the next container start"
  fi
}

config_file=""
temp_file config_file
render_config "$config_file"
upload_to_r2 "$config_file"

# SKIP_RELOAD=1 uploads only (used for the dev bucket, which must never reload production).
if [[ "${SKIP_RELOAD:-0}" == 1 ]]; then
	log "live reload skipped (SKIP_RELOAD=1)"
else
	reload_live_server "$config_file"
fi
