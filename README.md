# cli-proxy-api

Deployment of [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) on Cloudflare Containers at **https://proxy.hara.sh**, managed with the [`cf` CLI](https://blog.cloudflare.com/cloudflare-cf-cli-launch/).

- **Runtime:** a Worker (`src/index.ts`) forwards every request to a single container running `eceasy/cli-proxy-api` (`Dockerfile`). The container is managed by the `CliProxy` Durable Object, as configured in `cloudflare.config.ts`.
- **State:** `config.yaml` and the OAuth token files live in the R2 bucket `cli-proxy-api`, under `config/config.yaml` and `auths/*.json`. The server uses its built-in object store (`OBJECTSTORE_*`) to read and write them. The container disk is only a cache.
- **Secrets:** 1Password, account `my.1password.com`, vault `cloudflare`, item `cli-proxy-api`.
  - `api-key`: the client API key.
  - `management-password`: the password for the management API and panel.
  - `r2-endpoint`, `r2-bucket`, `r2-access-key-id`, `r2-secret-access-key`: the R2 S3 credentials.
- **Account:** the personal Cloudflare account "Ollin" (`accountId` in `cloudflare.config.ts`). The `cf` auth profile `personal` is bound to this directory with `cf auth activate personal .`.

## Requirements

Node, Docker Desktop (running), the 1Password CLI (`op`), and `cf` (installed as a dev dependency). `wrangler` stays installed because `cf` uses it as the bundler (`wrangler.config.ts`).

## Common tasks

Run `make` to list every target. Secrets are read from 1Password when a target runs.

```bash
make install       # npm dependencies
make deploy        # type-check, build the image, deploy (secrets from 1Password)
make deploy-dry    # build and validate without uploading
make config-push   # render config.yaml from 1Password and upload it to R2
make claude        # run Claude Code through the proxy (ARGS="..." for flags)
make codex         # run Codex CLI through the proxy (`hara` profile, ARGS="..." for flags)
make codex-smoke   # one non-interactive Codex turn through the proxy
make codex-backup  # copy ~/.codex/config.toml + hara.config.toml into codex/
make alias         # print the claude-hara alias for ~/.zshrc
make health        # /healthz
make accounts      # connected provider accounts
make models        # models available through the proxy
make smoke         # send a test message (MODEL=... to override)
make logs          # last server log lines (LINES=... to override)
make logs-cf       # Worker request logs from Cloudflare (MINUTES=... to override)
```

- **Upgrade upstream:** bump the image tag in `Dockerfile`, then run `make deploy`.
- **Change config:** edit `config.yaml`, then run `make config-push`. Changes made in the management panel are written straight to R2, and `config-push` overwrites them.
- **Logs:** `make logs` reads the server log files (rotated at 10 MB, capped at 512 MB in total, lost on container restart). `make logs-cf` shows Worker request logs from Cloudflare observability.

## Codex CLI

`~/.codex/config.toml` defines the `hara` model provider (`https://proxy.hara.sh/v1`, Responses API, key from `HARA_PROXY_API_KEY`), and `~/.codex/hara.config.toml` is the `hara` profile (`model_provider = "hara"`, `model = "gpt-6.1-sol"`). Codex 0.134+ no longer supports `[profiles.*]` tables inside `config.toml`. Backups of both files live in `codex/`; refresh them with `make codex-backup`.

## Logging in to providers

1. Open https://proxy.hara.sh/management.html and sign in with `management-password`.
2. Start the OAuth flow for a provider (Claude, Codex, Antigravity, ...).
3. After you sign in, the browser lands on a `localhost` URL that fails to load. Paste that URL back into the panel. Device-code providers do not need this step.
4. The tokens are saved to R2 under `auths/`.

### Accounts in the pool

| Provider | Accounts | Login flow |
|---|---|---|
| Claude | 2 | Claude OAuth, done once per account. Sign out of claude.ai (or use a private window) before adding the second, otherwise the same account is authorised again. |
| Codex | 1 | Codex OAuth, or the device-code login |

`routing.session-affinity` keeps each conversation on one account, so prompt caches are reused. The proxy fails over to the other account when one is rate-limited.

Cursor is **not** an upstream provider. CLIProxyAPI has no built-in support, and the only plugin is a third-party native library that would run with access to every stored token. Cursor can still use this proxy as a client: set the OpenAI base URL to `https://proxy.hara.sh/v1` with the `api-key`.

## Using the proxy

```bash
curl https://proxy.hara.sh/v1/models -H "Authorization: Bearer $(op read --account my.1password.com op://cloudflare/cli-proxy-api/api-key)"
```

The container sleeps after 30 minutes idle. The first request after that takes a few seconds while it starts.
