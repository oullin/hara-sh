#!/usr/bin/env bash
# Copy the Codex CLI config (~/.codex/config.toml and the openai profile) into codex/.
# Neither file holds a secret: Codex uses its own ChatGPT login.
#   scripts/backup-codex.sh
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"

DEST="$REPO_ROOT/codex"

mkdir -p "$DEST"
cp -p "$HOME/.codex/config.toml" "$HOME/.codex/openai.config.toml" "$DEST/"
log "backed up ~/.codex config to codex/"
