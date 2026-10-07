#!/usr/bin/env bash
# Private client backups must stay outside the publishable checkout.
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"

DEST="${BACKUP_DIR:-${LOCAL_DIR:-$HOME/.hara-sh}/backups}/codex"
umask 077
private_backup_dir "$DEST"
for file in config.toml openai.config.toml proxy.config.toml; do
  [[ -f "$HOME/.codex/$file" ]] || continue
  cp "$HOME/.codex/$file" "$DEST/$file"
  chmod 600 "$DEST/$file"
done
log "saved private Codex settings outside the repository"
