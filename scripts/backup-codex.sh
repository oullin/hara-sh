#!/usr/bin/env bash
# Copy the Codex CLI config (~/.codex/config.toml and the openai and proxy profiles) into codex/.
# None of them holds a secret: the proxy profile reads its key through scripts/hara-key.
#   scripts/backup-codex.sh
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"

DEST="$REPO_ROOT/codex"

mkdir -p "$DEST"
cp -p "$HOME/.codex/config.toml" "$HOME/.codex/openai.config.toml" "$DEST/"
[[ -f "$HOME/.codex/proxy.config.toml" ]] && cp -p "$HOME/.codex/proxy.config.toml" "$DEST/"
log "backed up ~/.codex config to codex/"
