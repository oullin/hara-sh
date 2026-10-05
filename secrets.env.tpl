# Worker secrets, resolved from 1Password by scripts/deploy.sh at deploy time.
OBJECTSTORE_ENDPOINT={{ op://cloudflare/cli-proxy-api/r2-endpoint }}
OBJECTSTORE_BUCKET={{ op://cloudflare/cli-proxy-api/r2-bucket }}
OBJECTSTORE_ACCESS_KEY={{ op://cloudflare/cli-proxy-api/r2-access-key-id }}
OBJECTSTORE_SECRET_KEY={{ op://cloudflare/cli-proxy-api/r2-secret-access-key }}
MANAGEMENT_PASSWORD={{ op://cloudflare/cli-proxy-api/management-password }}
