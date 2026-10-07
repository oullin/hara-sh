#!/usr/bin/env bash
# Private client backups must stay outside the publishable checkout.
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"

SRC="$HOME/.claude"
DEST="${BACKUP_DIR:-${LOCAL_DIR:-$HOME/.cli-proxy-api}/backups}/claude"
umask 077
private_backup_dir "$DEST"
for file in settings.local.json CLAUDE.md .claude.json settings.json; do
  [[ -f "$SRC/$file" ]] || continue
  cp "$SRC/$file" "$DEST/$file"
  chmod 600 "$DEST/$file"
done
log "saved private Claude settings outside the repository"
