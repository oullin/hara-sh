#!/usr/bin/env bash
# Build the image and deploy the Worker with cf. Worker secrets are rendered from
# secrets.env.tpl (1Password references) into a temp file that is deleted afterwards.
#   scripts/deploy.sh [cf deploy flags...]
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"
cd "$REPO_ROOT" || exit 1

secrets_file=""
temp_file secrets_file
op inject -f -i secrets.env.tpl -o "$secrets_file" >/dev/null
npx cf deploy --secrets-file "$secrets_file" "$@"
