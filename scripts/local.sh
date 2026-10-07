#!/usr/bin/env bash
# Portable optional shortcuts around the Docker-only Compose workflow.
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"
cd "$REPO_ROOT" || exit 1

LOCAL_DIR="${LOCAL_DIR:-$HOME/.hara-sh}"
export LOCAL_DIR

# Prevent writing credentials into the source checkout, even through a symlink.
private_backup_dir "$LOCAL_DIR"
compose() { docker compose -f "$REPO_ROOT/local/compose.yaml" "$@"; }

# Host loopback URLs need the Docker service address when used inside tools.
tools() {
  local target="${URL:-http://localhost:${HARA_PORT:-8317}}"
  case "$target" in
    "http://localhost:${HARA_PORT:-8317}"|"http://127.0.0.1:${HARA_PORT:-8317}") target="http://proxy:8317" ;;
  esac
  case "${1:-}" in
    ws-smoke) shift; set -- wssmoke -idle "${1:-0s}" "$target" "${@:2}" ;;
    benchmark) shift; set -- bench -n "${1:-5}" -claude-model "$2" -codex-model "$3" "$target" ;;
  esac
  TOOLS_URL="$target" compose run --rm --no-deps -T tools "$@"
}

status() {
  compose ps
  tools status
  log "Host endpoint: http://localhost:${HARA_PORT:-8317}"
  # Tailscale is optional; show its login/address only when enabled and running.
  if [[ -n "$(compose --profile tailscale ps -q --status running tailscale)" ]]; then
    wait_tailscale
    compose exec -T tailscale tailscale status --peers=false || log "! Tailscale needs attention; authorize its device before using the tailnet URL"
  fi
}

# A just-started node reports NoState, then offline, for a few seconds; a node that needs login settles at once.
wait_tailscale() {
  local attempt state starting='NoState|Starting|offline|failed to connect'
  for attempt in {1..15}; do
    state="$(compose exec -T tailscale tailscale status --peers=false 2>&1 || true)"
    [[ -n "$state" && ! "$state" =~ $starting ]] && return 0
    (( attempt == 1 )) && log "Waiting for Tailscale to connect..."
    sleep 1
  done
}

# Optional https://hara.local through portless in LAN mode; skipped when portless is not installed.
# Starting the proxy on port 443 may ask for sudo unless `portless service install --lan` was run.
portless_route() {
  command -v portless >/dev/null || { log "portless not installed; skipping https://hara.local"; return 0; }
  portless proxy start --lan || { log "! portless proxy did not start; https://hara.local is unavailable"; return 0; }
  portless alias hara "${HARA_PORT:-8317}" --force
}

up() {
  docker info >/dev/null 2>&1 || die "Docker is not running"
  # Initialization never regenerates existing keys. Restart readers after rendering.
  compose up -d --build --force-recreate
  tools health
  portless_route
  status
}

# Docker rebuilds or pulls everything on the next `up`; private state in LOCAL_DIR is kept.
purge() {
  compose --profile tailscale --profile tools down --rmi all --volumes --remove-orphans
  docker builder prune -f
  if command -v portless >/dev/null; then portless alias --remove hara || true; fi
  log "Private state kept in $LOCAL_DIR; delete it yourself to remove keys and provider logins."
}

import_op() {
  command -v op >/dev/null || die "install and sign in to the optional 1Password CLI first"
  local api management
  api="$(op read --account "$OP_ACCOUNT" "op://$OP_VAULT/$OP_ITEM_NAME/claude-api-key")"
  management="$(op read --account "$OP_ACCOUNT" "op://$OP_VAULT/$OP_ITEM_NAME/management-password")"
  compose build init
  printf '%s\n%s\n' "$api" "$management" | compose run --rm --no-deps -T init import
}

case "${1:-up}" in
  up) up ;;
  portless) portless_route ;;
  init) compose build init && compose run --rm --no-deps -T init init ;;
  import-op) import_op ;;
  rotate) compose run --rm --no-deps -T init rotate ;;
  status) status ;;
  tools) tools "${@:2}" ;;
  ws-smoke) tools ws-smoke "${@:2}" ;;
  bench) tools benchmark "${@:2}" ;;
  logs) compose logs -f --tail=100 "${@:2}" ;;
  down) compose --profile tailscale down ;;
  purge) purge ;;
  *) die "usage: scripts/local.sh up|portless|init|import-op|rotate|status|tools COMMAND|logs [service]|down|purge" ;;
esac
