#!/usr/bin/env bash
# Run the Worker and its container locally (Vite + workerd + Docker) against the dev bucket.
# Secrets are rendered from 1Password into .dev.vars for the session and deleted on exit.
#   scripts/dev.sh [cf dev flags...]
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"
cd "$REPO_ROOT" || exit 1

DEV_BUCKET="cli-proxy-api-dev"

grep -q "^OBJECTSTORE_BUCKET=$DEV_BUCKET$" dev.vars.tpl || die "dev.vars.tpl must target the $DEV_BUCKET bucket"

# Seed the dev bucket with the current config; never reload the production server.
R2_BUCKET="$DEV_BUCKET" SKIP_RELOAD=1 "$SCRIPTS_DIR/push-config.sh"

# On exit: delete the rendered secrets and stop the local container workerd leaves behind.
cleanup_dev() {
	rm -f .dev.vars
	_cleanup_temp_files

	local containers
	containers="$(docker ps -q --filter name=workerd-cli-proxy-api 2>/dev/null || true)"

	if [[ -n "$containers" ]]; then
		# shellcheck disable=SC2086 # one container id per word
		docker stop $containers >/dev/null && log "stopped local dev container(s)"
	fi
}
trap cleanup_dev EXIT
(umask 077 && op inject -f -i dev.vars.tpl -o .dev.vars >/dev/null)
log "rendered .dev.vars from 1Password (deleted on exit)"

npx cf dev "$@"
