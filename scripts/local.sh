#!/usr/bin/env bash
# Run the proxy on this computer in Docker (local/compose.yaml), reachable on your tailnet and,
# through portless in LAN mode, as https://hara.local on your network.
#   scripts/local.sh up       render the config from 1Password, start, print the addresses
#   scripts/local.sh status   print the addresses and the Tailscale login state
#   scripts/local.sh logs     follow the proxy and Tailscale logs
#   scripts/local.sh down     stop (OAuth logins and the Tailscale identity are kept)
# Env: LOCAL_DIR (default ~/.cli-proxy-api), LOCAL_NAME (default hara, for <name>.local),
#      TS_HOSTNAME (default cliproxy),
#      TS_AUTHKEY (optional; without it, the first start prints a Tailscale login link).
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"
cd "$REPO_ROOT" || exit 1

LOCAL_DIR="${LOCAL_DIR:-$HOME/.cli-proxy-api}"
TS_HOSTNAME="${TS_HOSTNAME:-cliproxy}"
LOCAL_NAME="${LOCAL_NAME:-hara}"
LOCAL_PORT=8317
LOCAL_URL="http://localhost:$LOCAL_PORT"
export LOCAL_DIR TS_HOSTNAME

compose() { docker compose -f "$REPO_ROOT/local/compose.yaml" "$@"; }

render_local_config() {
  local config="$LOCAL_DIR/proxy/config.yaml"
  render_config "$config"
  chmod 600 "$config"
  log "rendered $config from 1Password"
}

wait_for_proxy() {
  for _ in {1..30}; do
    curl -fsS --max-time 2 "$LOCAL_URL/healthz" >/dev/null 2>&1 && return 0
    sleep 1
  done
  die "the proxy did not answer on $LOCAL_URL; run: make local-logs"
}

# portless serves https://<LOCAL_NAME>.local on this computer and announces it on the network
# (mDNS). LAN mode is a setting of the whole portless proxy: every portless app on this computer
# becomes <name>.local and reachable from the network while it is on.
start_portless() {
  command -v portless >/dev/null || { log "portless is not installed; skipping https://$LOCAL_NAME.local"; return 0; }

  # Exits 1 when a proxy already runs in .localhost mode; restart that one in LAN mode.
  # Binding port 443 asks for your password (sudo) when the proxy starts.
  if ! portless proxy start --lan 2>/dev/null; then
    log "restarting the portless proxy in LAN mode"
    portless proxy stop
    portless proxy start --lan
  fi
  # Aliases take the running proxy's TLD, so this registers <LOCAL_NAME>.local.
  portless alias "$LOCAL_NAME" "$LOCAL_PORT" --force >/dev/null
}

# Prints "<BackendState> <DNSName or -> <AuthURL or ->" once tailscaled has settled.
tailscale_state() {
  local state
  for _ in {1..20}; do
    state="$(compose exec -T tailscale tailscale status --json 2>/dev/null | python3 -c '
import json, sys
d = json.load(sys.stdin)
print(d.get("BackendState", "NoState"), (d.get("Self") or {}).get("DNSName", "").rstrip(".") or "-", d.get("AuthURL") or "-")
' 2>/dev/null || echo "NoState - -")"
    case "$state" in
      NoState* | Starting*" - -" | NeedsLogin*" - -") sleep 1 ;;
      *) break ;;
    esac
  done
  printf '%s\n' "$state"
}

print_addresses() {
  local state dns auth_url lan_url
  read -r state dns auth_url < <(tailscale_state)

  log "this computer:  $LOCAL_URL"
  lan_url="$(portless list 2>/dev/null | grep -Eo "https?://$LOCAL_NAME\.local(:[0-9]+)?( |$)" | head -1 | tr -d ' ' || true)"
  [[ -n "$lan_url" ]] && log "your network:   $lan_url  (other devices need the portless certificate, ~/.portless/ca.pem)"
  case "$state" in
    Running) log "your tailnet:   https://$dns  (base URL for other tools: https://$dns/v1)" ;;
    NeedsLogin) log "your tailnet:   not signed in yet. Open this link to add '$TS_HOSTNAME' to your tailnet,"
                log "                then run: make local-status"
                log "                $auth_url" ;;
    *) log "your tailnet:   Tailscale is $state; run: make local-logs" ;;
  esac
}

up() {
  docker info >/dev/null 2>&1 || die "Docker is not running"
  (umask 077 && mkdir -p "$LOCAL_DIR/proxy" "$LOCAL_DIR/tailscale")

  local was_running
  was_running="$(compose ps -q --status running proxy 2>/dev/null || true)"
  render_local_config

  compose up -d --remove-orphans
  # A running server keeps its old config until restarted.
  [[ -n "$was_running" ]] && compose restart proxy
  wait_for_proxy
  start_portless
  print_addresses
}

down() {
  compose down
  # Leaves the portless proxy itself running, in LAN mode, for your other apps.
  command -v portless >/dev/null && portless alias --remove "$LOCAL_NAME" >/dev/null 2>&1
  return 0
}

case "${1:-up}" in
  up) up ;;
  status) print_addresses ;;
  logs) compose logs -f --tail=100 ;;
  down) down ;;
  *) die "usage: scripts/local.sh [up|status|logs|down]" ;;
esac
