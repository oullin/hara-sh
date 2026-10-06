# Local `make dev` secrets, rendered from 1Password into .dev.vars by scripts/dev.sh.
# Same credentials as production, but a separate R2 bucket: a local server must never share
# the production bucket (both would rotate the same OAuth refresh tokens).
OBJECTSTORE_ENDPOINT={{ op://cloudflare/cli-proxy-api/r2-endpoint }}
OBJECTSTORE_BUCKET=cli-proxy-api-dev
OBJECTSTORE_ACCESS_KEY={{ op://cloudflare/cli-proxy-api/r2-access-key-id }}
OBJECTSTORE_SECRET_KEY={{ op://cloudflare/cli-proxy-api/r2-secret-access-key }}
MANAGEMENT_PASSWORD={{ op://cloudflare/cli-proxy-api/management-password }}
CODEX_API_KEY={{ op://cloudflare/cli-proxy-api/codex-api-key }}
