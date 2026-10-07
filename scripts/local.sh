#!/usr/bin/env bash
# Portable optional shortcuts around the Docker-only Compose workflow.
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"
cd "$REPO_ROOT" || exit 1

LOCAL_DIR="${LOCAL_DIR:-$HOME/.cli-proxy-api}"
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
    compose exec -T tailscale tailscale status --peers=false || log "! Tailscale needs attention; authorize its device before using the tailnet URL"
  fi
}

up() {
  docker info >/dev/null 2>&1 || die "Docker is not running"
  # Initialization never regenerates existing keys. Restart readers after rendering.
  compose up -d --build --force-recreate
  tools health
  status
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
  init) compose build init && compose run --rm --no-deps -T init init ;;
  import-op) import_op ;;
  rotate) compose run --rm --no-deps -T init rotate ;;
  status) status ;;
  tools) tools "${@:2}" ;;
  ws-smoke) tools ws-smoke "${@:2}" ;;
  bench) tools benchmark "${@:2}" ;;
  logs) compose logs -f --tail=100 "${@:2}" ;;
  down) compose --profile tailscale down ;;
  *) die "usage: scripts/local.sh up|init|import-op|rotate|status|tools COMMAND|logs [service]|down" ;;
esac
