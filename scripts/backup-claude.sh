#!/usr/bin/env bash
# Copy the Claude Code config from ~/.claude into claude/. settings.json is copied with
# autoMode.environment redacted, because it describes Omniyat-internal systems.
#   scripts/backup-claude.sh
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"

SRC="$HOME/.claude"
DEST="$REPO_ROOT/claude"
# jq keeps the key order, so a backup only differs where the settings changed.
REDACT='if (.autoMode.environment // [] | length) > 0
  then .autoMode.environment = ["<redacted: \(.autoMode.environment | length) Omniyat-specific auto mode environment lines; restore them from ~/.claude/settings.json>"]
  else . end'

mkdir -p "$DEST"
cp -p "$SRC/settings.local.json" "$SRC/CLAUDE.md" "$SRC/.claude.json" "$DEST/"
jq --indent 2 "$REDACT" "$SRC/settings.json" >"$DEST/settings.json"
log "backed up ~/.claude config to claude/ (autoMode.environment redacted)"
