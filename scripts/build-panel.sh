#!/usr/bin/env bash
# Build the management panel the proxy serves at /management.html: the upstream
# Management Center at PANEL_TAG plus panel/ledger.patch (the quota Ledger view).
# Writes panel/management.html; commit it, then `make up` (local/compose.yaml mounts it).
#   scripts/build-panel.sh
# Needs git and bun. To move to a newer upstream panel, bump PANEL_TAG; if the patch
# no longer applies, rebase it on the new tag and regenerate panel/ledger.patch.
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"
cd "$REPO_ROOT" || exit 1

PANEL_REPO="${PANEL_REPO:-https://github.com/router-for-me/Cli-Proxy-API-Management-Center}"
PANEL_TAG="${PANEL_TAG:-v1.25.3}"

command -v bun >/dev/null || die "bun is required (brew install oven-sh/bun/bun)"

build_dir="$(mktemp -d -t cli-proxy-panel)"
trap 'rm -rf "$build_dir"; _cleanup_temp_files' EXIT

log "cloning $PANEL_REPO@$PANEL_TAG"
git -c advice.detachedHead=false clone --quiet --depth 1 --branch "$PANEL_TAG" "$PANEL_REPO" "$build_dir"

log "applying panel/ledger.patch"
git -C "$build_dir" apply --whitespace=nowarn "$REPO_ROOT/panel/ledger.patch"

log "building"
(cd "$build_dir" && bun install --frozen-lockfile >/dev/null && VERSION="$PANEL_TAG+ledger" bun run build >/dev/null)

cp "$build_dir/dist/index.html" panel/management.html
log "wrote panel/management.html ($(wc -c <panel/management.html | tr -d ' ') bytes)"
