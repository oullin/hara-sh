# cli-proxy-api

Deployment of [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) on Cloudflare Containers at **https://proxy.hara.sh**.

- **Runtime:** a Worker (`src/index.ts`) forwards every request to a single container running `eceasy/cli-proxy-api` (`Dockerfile`).
- **State:** `config.yaml` and OAuth token files live in the R2 bucket `cli-proxy-api` (`config/config.yaml`, `auths/*.json`) through the upstream object store (`OBJECTSTORE_*`). The container disk is only a cache.
- **Secrets:** stored in 1Password, vault `cloudflare`, item `cli-proxy-api`. Fields:
  - `api-key`: the client API key.
  - `management-password`: the management API password.
  - `r2-endpoint`, `r2-bucket`, `r2-access-key-id`, `r2-secret-access-key`: the R2 connection details.

## Requirements

Node, Docker Desktop (running), the 1Password CLI (`op`), and a wrangler login on the personal Cloudflare account "Ollin" (pinned as `account_id` in `wrangler.jsonc`).

The default wrangler login on this machine belongs to a work account. The personal login is kept in a separate config directory, so export these before running the commands below:

```bash
export XDG_CONFIG_HOME=$HOME/.claude/work/wrangler-personal OP_ACCOUNT=my.1password.com
```

## Common tasks

```bash
npm install
npm run secrets:push   # copy Worker secrets from 1Password to Cloudflare
npm run config:push    # render config.yaml from 1Password and upload it to R2
npm run deploy         # build the image, deploy the Worker and container
npm run tail           # stream logs
```

- **Upgrade upstream:** bump the tag in `Dockerfile`, then `npm run deploy`.
- **Config changes:** edit `config.yaml` and run `npm run config:push`, or use the management panel. If you push again, the management password is re-hashed on the next start. The management panel writes its changes straight to R2, and `config:push` overwrites them.

## Logging in to providers

1. Open https://proxy.hara.sh/management.html and sign in with `management-password`.
2. Start the OAuth flow for a provider (Claude, Codex, Antigravity, and others).
3. After signing in, the browser is redirected to a `localhost` URL that fails to load. Paste that URL back into the panel to finish the login. Device-code providers need no paste.
4. The tokens are saved to R2 under `auths/`.

## Using the proxy

```bash
curl https://proxy.hara.sh/v1/models -H "Authorization: Bearer $(op read op://cloudflare/cli-proxy-api/api-key)"
```

The container sleeps after 30 minutes idle. The first request after that takes a few seconds while the container starts.
