#!/usr/bin/env bash
# T3 Code (t3.codes) through the proxy: its Claude and Codex provider instances run the CLIs, so
# each instance gets the proxy URL and key the same way `make claude` and `make codex` do.
#   scripts/t3.sh setup   write the Codex home, move T3 Code's Tailscale HTTPS port, print the settings
#   scripts/t3.sh smoke   one Claude and one Codex request with the instance settings (uses provider quota)
# Env: T3_URL (default https://hara.local when portless is installed, else http://localhost:8317),
#      T3_CODEX_HOME (default ~/.codex-t3-hara), T3_CLAUDE_HOME (default ~/.claude-hara),
#      T3_HOME (default ~/.t3), T3_TAILSCALE_PORT (default 8443), MODEL (Claude model for smoke).
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"

PORTLESS_CA="$HOME/.portless/ca.pem"
CODEX_DIR="${T3_CODEX_HOME:-$HOME/.codex-t3-hara}"
CLAUDE_DIR="${T3_CLAUDE_HOME:-$HOME/.claude-hara}"
DESKTOP_SETTINGS="${T3_HOME:-$HOME/.t3}/userdata/desktop-settings.json"
TAILSCALE_PORT="${T3_TAILSCALE_PORT:-8443}"
LOCAL_URL="http://localhost:${HARA_PORT:-8317}"

if [[ -n "${T3_URL:-}" ]]; then
  URL="${T3_URL%/}"
elif command -v portless >/dev/null && [[ -f "$PORTLESS_CA" ]]; then
  URL="https://hara.local"
else
  URL="$LOCAL_URL"
fi

lines() { printf '%s\n' "$@"; }

# Only hara.local needs a certificate authority named: localhost is plain HTTP, and Tailscale
# certificates are publicly trusted. Claude Code and Codex ignore the macOS keychain's copy.
CA=""
if [[ "$URL" == https://*.local || "$URL" == https://*.local:* ]]; then CA="$PORTLESS_CA"; fi

# Portless in LAN mode listens on 443 on every interface, the tailnet address included, so it answers
# T3 Code's Tailscale Serve URL with its own certificate and a 404, and T3 Code turns the switch off.
tailscale_port() {
  [[ -f "$DESKTOP_SETTINGS" ]] || { log "- T3 Code desktop settings not found at $DESKTOP_SETTINGS; open T3 Code once and rerun"; return 0; }
  if command -v jq >/dev/null; then
    local updated=""
    temp_file updated
    jq -c --argjson port "$TAILSCALE_PORT" '.tailscaleServePort = $port' "$DESKTOP_SETTINGS" >"$updated"
    cat "$updated" >"$DESKTOP_SETTINGS"
  elif command -v plutil >/dev/null; then
    plutil -replace tailscaleServePort -integer "$TAILSCALE_PORT" "$DESKTOP_SETTINGS"
  else
    log "- install jq, or add \"tailscaleServePort\": $TAILSCALE_PORT to $DESKTOP_SETTINGS while T3 Code is closed"
    return 0
  fi
  log "- T3 Code serves Tailscale HTTPS on port $TAILSCALE_PORT ($DESKTOP_SETTINGS)"
}

setup() {
  install_codex_config "$CODEX_DIR/config.toml" "$URL/v1"
  log "- Codex home for T3 Code: $CODEX_DIR/config.toml ($URL/v1)"
  tailscale_port
  [[ -z "$CA" || -f "$CA" ]] || log "! $CA is missing; run make portless, or set T3_URL=$LOCAL_URL"

  lines "" \
    "Quit T3 Code (Cmd-Q) and reopen it before changing any setting; it rewrites its settings from memory." \
    "Then add two provider instances in Settings -> Providers:" \
    "" \
    "Claude" \
    "  CLAUDE_CONFIG_DIR path   $CLAUDE_DIR" \
    "  Environment" \
    "    ANTHROPIC_BASE_URL     $URL" \
    "    ANTHROPIC_AUTH_TOKEN   output of: $SCRIPTS_DIR/hara-key claude-api-key   (Sensitive)" \
    "    ANTHROPIC_API_KEY      empty value"
  if [[ -n "$CA" ]]; then lines "    NODE_EXTRA_CA_CERTS    $CA"; fi
  lines "" \
    "Codex" \
    "  CODEX_HOME path          $CODEX_DIR" \
    "  Shadow home path         empty"
  if [[ -n "$CA" ]]; then lines "  Environment" "    CODEX_CA_CERTIFICATE   $CA"; fi
  lines "" "Tailscale HTTPS can now be switched on in T3 Code. Check both instances with: make t3 smoke"
}

smoke() {
  [[ -f "$CODEX_DIR/config.toml" ]] || die "no Codex home at $CODEX_DIR; run: make t3"
  # An empty certificate path is not "unset" to every client; pass the variables only when needed.
  local ca_env=()
  [[ -z "$CA" ]] || ca_env=(NODE_EXTRA_CA_CERTS="$CA" CODEX_CA_CERTIFICATE="$CA")

  if command -v claude >/dev/null; then
    local out="" key
    key="$(secret claude-api-key)"
    temp_file out
    env ${ca_env[@]+"${ca_env[@]}"} CLAUDE_CONFIG_DIR="$CLAUDE_DIR" ANTHROPIC_BASE_URL="$URL" \
      ANTHROPIC_AUTH_TOKEN="$key" ANTHROPIC_API_KEY="" \
      claude -p --model "${MODEL:-claude-haiku-4-5-20251001}" "Reply with exactly: pong" \
      </dev/null >"$out" 2>&1 || { cat "$out" >&2; die "the Claude request failed ($URL)"; }
    grep -qx "pong" "$out" || { cat "$out" >&2; die "Claude did not answer pong"; }
    log "ok: Claude answered pong ($URL)"
  else
    log "- claude is not installed; skipping the Claude check"
  fi

  env ${ca_env[@]+"${ca_env[@]}"} CODEX_HOME="$CODEX_DIR" CODEX_PROFILE="" PROXY_URL="$LOCAL_URL" \
    SETUP_HINT="make t3" "$SCRIPTS_DIR/codex-smoke.sh"
}

case "${1:-setup}" in
  setup) setup ;;
  smoke) smoke ;;
  *) die "usage: scripts/t3.sh setup|smoke" ;;
esac
