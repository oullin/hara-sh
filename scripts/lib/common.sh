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

# render_config OUT [IN]: inject 1Password secrets into IN (default config.yaml) and write OUT,
# replacing the management password placeholder with its bcrypt hash.
render_config() {
  local out="$1" in="${2:-$REPO_ROOT/config.yaml}" placeholder="__MANAGEMENT_PASSWORD_BCRYPT__" hash
  op inject -f -i "$in" -o "$out" >/dev/null
  hash="$(_management_password_hash)"
  sed -i '' "s|$placeholder|$hash|" "$out"
  grep -q "$placeholder" "$out" && die "management password placeholder was not replaced"
  return 0
}

# Only a bcrypt hash of the management password is stored. htpasswd emits $2y$; Go's bcrypt
# and the server's "already hashed" check both accept the equivalent $2a$ prefix.
_management_password_hash() {
  # shellcheck disable=SC2016 # literal $2y$/$2a$ prefixes
  secret management-password | htpasswd -niBC 10 "" | tr -d ':\n' | sed 's/^\$2y\$/$2a$/'
}
