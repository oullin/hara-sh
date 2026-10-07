#!/usr/bin/env bash
# Install the public profile template with this checkout's absolute key-helper path.
# shellcheck source=lib/common.sh
source "$(dirname "$0")/lib/common.sh"

install_codex_config "${CODEX_HOME:-$HOME/.codex}/proxy.config.toml"
log "installed the proxy profile; run: codex --profile proxy"
