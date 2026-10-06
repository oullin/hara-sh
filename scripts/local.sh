#!/usr/bin/env bash
# Run the proxy on this computer in Docker (local/compose.yaml), reachable on your tailnet and,
# through portless in LAN mode, as https://hara.local on your network.
#   scripts/local.sh up              render the config from 1Password, start, print the addresses
#   scripts/local.sh status          check every link and print the addresses (scripts/ops)
#   scripts/local.sh logs [service]  follow the proxy, Tailscale and quota logs (or one service's)
#   scripts/local.sh down            stop (OAuth logins and the Tailscale identity are kept)
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

# The quota service reads the management password from this file (compose secret).
write_quota_key() {
  (umask 077 && mkdir -p "$LOCAL_DIR/quota" && secret management-password >"$LOCAL_DIR/quota/management-password")
}

wait_for_proxy() {
  for _ in {1..30}; do
    curl -fsS --max-time 2 "$LOCAL_URL/healthz" >/dev/null 2>&1 && return 0
    sleep 1
  done
  die "the proxy did not answer on $LOCAL_URL; run: make logs"
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

# Right after a start, Tailscale needs a few seconds before it is up or has a login link.
wait_for_tailscale() {
  for _ in {1..20}; do
    compose exec -T tailscale tailscale status --json --peers=false 2>/dev/null \
      | grep -qE '"BackendState": "Running"|"AuthURL": "https' && return 0
    sleep 1
  done
  return 0
}

# Checks every link, from the containers to the accounts, and says how to fix what is broken
# (scripts/ops). Exits 1 when a check fails.
status() {
  local bin="$LOCAL_DIR/bin/ops"
  (cd "$SCRIPTS_DIR/ops" && go build -o "$bin" .)
  API_KEY="$(secret claude-api-key)" MGMT_KEY="$(secret management-password)" \
    "$bin" status -local "$LOCAL_URL" -network "https://$LOCAL_NAME.local"
}

up() {
  docker info >/dev/null 2>&1 || die "Docker is not running"
  (umask 077 && mkdir -p "$LOCAL_DIR/proxy" "$LOCAL_DIR/tailscale")

  local was_running
  was_running="$(compose ps -q --status running proxy 2>/dev/null || true)"
  render_local_config
  write_quota_key

  # --build rebuilds the quota image when scripts/quota changed (cached otherwise).
  compose up -d --build --remove-orphans
  # A running server keeps its old config until restarted.
  [[ -n "$was_running" ]] && compose restart proxy
  wait_for_proxy
  start_portless
  wait_for_tailscale
  status
}

down() {
  compose down
  # Leaves the portless proxy itself running, in LAN mode, for your other apps.
  command -v portless >/dev/null && portless alias --remove "$LOCAL_NAME" >/dev/null 2>&1
  return 0
}

case "${1:-up}" in
  up) up ;;
  status) status ;;
  logs) compose logs -f --tail=100 "${@:2}" ;;
  down) down ;;
  *) die "usage: scripts/local.sh [up|status|logs [service]|down]" ;;
esac
