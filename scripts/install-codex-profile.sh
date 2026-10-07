#!/usr/bin/env bash
# Install the public profile template with this checkout's absolute key-helper path.
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"

profile_dir="${CODEX_HOME:-$HOME/.codex}"
mkdir -p "$profile_dir"
# Escape for TOML first, then for the sed replacement.
command_path="${SCRIPTS_DIR//\\/\\\\}"
command_path="${command_path//\"/\\\"}/hara-key"
replacement="$(printf '%s' "$command_path" | sed 's/[&|\\]/\\&/g')"
profile=""
temp_file profile
sed "s|__HARA_KEY_COMMAND__|$replacement|" "$REPO_ROOT/codex/proxy.config.toml" >"$profile"
install -m 600 "$profile" "$profile_dir/proxy.config.toml"
log "installed the proxy profile; run: codex --profile proxy"
