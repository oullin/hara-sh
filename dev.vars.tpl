# Local `make dev` secrets, rendered from 1Password into .dev.vars by scripts/dev.sh.
# Same credentials as production, but a separate R2 bucket: a local server must never share
# the production bucket (both would rotate the same OAuth refresh tokens).
OBJECTSTORE_ENDPOINT={{ op://YOUR_VAULT/YOUR_ITEM/r2-endpoint }}
OBJECTSTORE_BUCKET=cli-proxy-api-dev
OBJECTSTORE_ACCESS_KEY={{ op://YOUR_VAULT/YOUR_ITEM/r2-access-key-id }}
OBJECTSTORE_SECRET_KEY={{ op://YOUR_VAULT/YOUR_ITEM/r2-secret-access-key }}
MANAGEMENT_PASSWORD={{ op://YOUR_VAULT/YOUR_ITEM/management-password }}
CODEX_API_KEY={{ op://YOUR_VAULT/YOUR_ITEM/codex-api-key }}
