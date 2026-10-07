# shellcheck shell=bash
# Shared settings and helpers for scripts/*. Source it; do not execute it.
#   source "$(dirname "${BASH_SOURCE[0]}")/lib/common.sh"

set -euo pipefail

# Read by the scripts that source this file.
# shellcheck disable=SC2034
SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck disable=SC2034
REPO_ROOT="$(dirname "$SCRIPTS_DIR")"

# shellcheck source=credentials.sh
source "$SCRIPTS_DIR/lib/credentials.sh"

log() { printf '%s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

# Refuse even an explicit backup destination that resolves inside this checkout.
private_backup_dir() {
  mkdir -p "$1"
  local resolved
  resolved="$(cd "$1" && pwd -P)"
  case "$resolved/" in
    "$REPO_ROOT/"*) die "client backups must be outside the repository" ;;
  esac
  chmod 700 "$resolved"
}

# temp_file VAR: create a private temp file, store its path in VAR, delete it on exit.
_TEMP_FILES=()
_cleanup_temp_files() { (( ${#_TEMP_FILES[@]} )) && rm -f "${_TEMP_FILES[@]}"; return 0; }
trap _cleanup_temp_files EXIT
temp_file() {
  local path
  path="$(umask 077 && mktemp "${TMPDIR:-/tmp}/cli-proxy-api.XXXXXX")"
  _TEMP_FILES+=("$path")
  printf -v "$1" '%s' "$path"
}

# secret FIELD: read local private state or the explicitly selected 1Password provider.
secret() { "$SCRIPTS_DIR/hara-key" "$1"; }

# install_codex_config DEST [BASE_URL]: codex/proxy.config.toml with this checkout's absolute
# key-helper path, and BASE_URL (ending in /v1) in place of the template's localhost URL.
install_codex_config() {
  local command_path replacement url profile=""
  # Escape for TOML first, then for the sed replacement.
  command_path="${SCRIPTS_DIR//\\/\\\\}"
  command_path="${command_path//\"/\\\"}/hara-key"
  replacement="$(printf '%s' "$command_path" | sed 's/[&|\\]/\\&/g')"
  url="$(printf '%s' "${2:-http://localhost:8317/v1}" | sed 's/[&|\\]/\\&/g')"
  temp_file profile
  sed -e "s|__HARA_KEY_COMMAND__|$replacement|" -e "s|^base_url = .*|base_url = \"$url\"|" \
    "$REPO_ROOT/codex/proxy.config.toml" >"$profile"
  mkdir -p "$(dirname "$1")"
  install -m 600 "$profile" "$1"
}
