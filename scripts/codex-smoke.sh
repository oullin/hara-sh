#!/usr/bin/env bash
# Run one Codex turn through the local proxy and fail unless it completed over the WebSocket.
# Codex falls back to HTTP by itself when the socket fails, so a turn that answers is not enough:
# the fallback warning in its output is what tells the two transports apart.
#   scripts/codex-smoke.sh
# Env: CODEX_PROFILE (default proxy; empty uses the Codex home's config.toml),
#      PROXY_URL (default http://localhost:8317), SETUP_HINT (the command that installs the config).
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"

PROFILE="${CODEX_PROFILE-proxy}"
PROXY_URL="${PROXY_URL:-http://localhost:8317}"
CONFIG="${CODEX_HOME:-$HOME/.codex}/${PROFILE:+$PROFILE.}config.toml"

[[ -f "$CONFIG" ]] || die "no Codex config at $CONFIG; run: ${SETUP_HINT:-make codex profile}"
curl -fsS --max-time 5 "$PROXY_URL/healthz" >/dev/null 2>&1 || die "the proxy does not answer on $PROXY_URL; run: make up"

args=(exec --skip-git-repo-check)
[[ -n "$PROFILE" ]] && args+=(--profile "$PROFILE")
out=""
temp_file out
codex "${args[@]}" "Reply with exactly: pong" </dev/null >"$out" 2>&1 || {
  cat "$out" >&2
  die "the Codex turn failed"
}

if grep -q "Falling back from WebSockets" "$out"; then
  grep "Falling back from WebSockets" "$out" >&2
  die "Codex answered over HTTP: the WebSocket failed (see make ops logs)"
fi

grep -qx "pong" "$out" || { cat "$out" >&2; die "Codex did not answer pong"; }
log "ok: Codex answered pong over the WebSocket ($CONFIG, $PROXY_URL)"
