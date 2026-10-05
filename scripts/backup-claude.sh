#!/usr/bin/env bash
# Copy the Claude Code config from ~/.claude into claude/. settings.json is copied with
# autoMode.environment redacted, because it describes Omniyat-internal systems.
#   scripts/backup-claude.sh
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"

SRC="$HOME/.claude"
DEST="$REPO_ROOT/claude"

mkdir -p "$DEST"
cp -p "$SRC/settings.local.json" "$SRC/CLAUDE.md" "$SRC/.claude.json" "$DEST/"
python3 "$SCRIPTS_DIR/lib/redact_claude_settings.py" "$SRC/settings.json" "$DEST/settings.json"
log "backed up ~/.claude config to claude/ (autoMode.environment redacted)"
