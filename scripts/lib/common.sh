# shellcheck shell=bash
# Shared settings and helpers for scripts/*. Source it; do not execute it.
#   source "$(dirname "${BASH_SOURCE[0]}")/lib/common.sh"

set -euo pipefail

# Read by the scripts that source this file.
# shellcheck disable=SC2034
SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck disable=SC2034
REPO_ROOT="$(dirname "$SCRIPTS_DIR")"

export OP_ACCOUNT="${OP_ACCOUNT:-my.1password.com}"
# shellcheck disable=SC2034
PROXY_URL="${PROXY_URL:-https://proxy.hara.sh}"
# shellcheck disable=SC2034
R2_BUCKET="${R2_BUCKET:-cli-proxy-api}"

log() { printf '%s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

# temp_file VAR: create a private temp file, store its path in VAR, delete it on exit.
_TEMP_FILES=()
_cleanup_temp_files() { (( ${#_TEMP_FILES[@]} )) && rm -f "${_TEMP_FILES[@]}"; return 0; }
trap _cleanup_temp_files EXIT
temp_file() {
  local path
  path="$(umask 077 && mktemp -t cli-proxy-api)"
  _TEMP_FILES+=("$path")
  printf -v "$1" '%s' "$path"
}

# secret FIELD: print a secret from the Keychain cache (1Password at most every 30 days).
secret() { "$SCRIPTS_DIR/hara-key" "$1"; }
