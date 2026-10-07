# shellcheck shell=bash
# Optional private installation settings. Only names belong here, never secret values.
credentials_file="${HARA_ENV_FILE:-${LOCAL_DIR:-$HOME/.cli-proxy-api}/credentials.env}"
if [[ -f "$credentials_file" ]]; then
  # The file belongs to the local operator and is deliberately outside version control.
  # shellcheck disable=SC1090
  source "$credentials_file"
fi
export OP_ACCOUNT="${OP_ACCOUNT:-my.1password.com}"
export OP_VAULT="${OP_VAULT:-Private}"
export OP_ITEM_NAME="${OP_ITEM_NAME:-cli-proxy-api}"
