# shellcheck shell=bash
# Optional private installation settings. Only names belong here, never secret values.
# Starting on an empty default directory would generate new keys and orphan existing logins.
if [[ -z "${LOCAL_DIR:-}" && ! -e "$HOME/.hara-sh" && -d "$HOME/.cli-proxy-api" ]]; then
  printf 'error: private state moved to ~/.hara-sh; run: mv ~/.cli-proxy-api ~/.hara-sh\n' >&2
  exit 1
fi
credentials_file="${HARA_ENV_FILE:-${LOCAL_DIR:-$HOME/.hara-sh}/credentials.env}"
if [[ -f "$credentials_file" ]]; then
  # The file belongs to the local operator and is deliberately outside version control.
  # shellcheck disable=SC1090
  source "$credentials_file"
fi
export OP_ACCOUNT="${OP_ACCOUNT:-my.1password.com}"
export OP_VAULT="${OP_VAULT:-Private}"
export OP_ITEM_NAME="${OP_ITEM_NAME:-cli-proxy-api}"
