# cli-proxy-api

Deployment of [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) on Cloudflare Containers at **https://proxy.hara.sh**, managed with the [`cf` CLI](https://blog.cloudflare.com/cloudflare-cf-cli-launch/).

- **Runtime:** a Worker (`src/index.ts`) forwards every request to a single container running `eceasy/cli-proxy-api` (`Dockerfile`). The container is managed by the `CliProxy` Durable Object, as configured in `cloudflare.config.ts`.
- **State:** `config.yaml` and the OAuth token files live in the R2 bucket `cli-proxy-api`, under `config/config.yaml` and `auths/*.json`. The server uses its built-in object store (`OBJECTSTORE_*`) to read and write them. The container disk is only a cache.
- **Secrets:** 1Password, account `my.1password.com`, vault `cloudflare`, item `cli-proxy-api`.
  - `claude-api-key`: the main client API key (Claude Code, scripts), which can reach every provider.
  - `codex-api-key`: the Codex CLI key. The Worker limits it to Codex/GPT models, so it only uses the Codex accounts.
  - `management-password`: the password for the management API and panel.
  - `r2-endpoint`, `r2-bucket`, `r2-access-key-id`, `r2-secret-access-key`: the R2 S3 credentials.
- **Account:** the personal Cloudflare account "Ollin" (`accountId` in `cloudflare.config.ts`). The `cf` auth profile `personal` is bound to this directory with `cf auth activate personal .`.

## Source layout

| File | Concern |
|---|---|
| `src/index.ts` | Worker entry: sends Codex-key requests to the guard and everything else straight to the container |
| `src/upstream.ts` | The single `CliProxy` Durable Object instance, pinned to Western Europe |
| `src/container/cli-proxy.ts` | The Durable Object that owns the container: port, sleep timeout, environment |
| `src/container/explicit-image.ts` | Workaround that makes start() pass the image explicitly (`@cloudflare/containers` 0.3.7) |
| `src/auth/client-key.ts` | Reads the client key from `Authorization` or `x-api-key`, with a constant-time comparison |
| `src/codex/policy.ts` | Which models and paths the Codex key may use |
| `src/codex/guard.ts` | Enforces that policy and filters `/v1/models` |
| `src/http/errors.ts` | JSON error responses |

| Script | Purpose |
|---|---|
| `scripts/deploy.sh` | Renders `secrets.env.tpl` from 1Password and runs `cf deploy` (`make deploy`) |
| `scripts/push-config.sh` | Renders `config.yaml`, hashes the management password, uploads to R2 and reloads the live server (`make config-push`) |
| `scripts/hara-key` | Keychain-cached secrets (standalone; used by Codex `auth.command`) |
| `scripts/backup-codex.sh` / `backup-claude.sh` | Copy `~/.codex` and `~/.claude` config into `codex/` and `claude/` |
| `scripts/lib/common.sh` | Shared settings (1Password account, proxy URL, bucket) and helpers (`temp_file`, `secret`, `log`, `die`) |
| `scripts/lib/redact_claude_settings.py` | Redacts `autoMode.environment` from the Claude settings backup |

## Requirements

Node, Docker Desktop (running), the 1Password CLI (`op`), and `cf` (installed as a dev dependency). `wrangler` stays installed because `cf` uses it as the bundler (`wrangler.config.ts`).

## Keys on this Mac

`scripts/hara-key [claude-api-key|codex-api-key|management-password]` prints a secret from the 1Password item.

- It caches the value in the macOS login Keychain (service `cli-proxy-api`) for 30 days, so 1Password prompts at most once a month.
- Codex (`auth.command`), the `claude-hara` alias and every `make` target use it.
- After rotating a key in 1Password, run `make keys-refresh`.
- To remove the cached values, run `scripts/hara-key --clear claude-api-key` and `scripts/hara-key --clear management-password`.
- Override the cache lifetime with `HARA_KEY_MAX_AGE_DAYS`.

## Common tasks

Run `make` to list every target. Secrets are read from 1Password when a target runs.

```bash
make install       # npm dependencies
make deploy        # type-check, build the image, deploy (secrets from 1Password)
make deploy-dry    # build and validate without uploading
make config-push   # render config.yaml from 1Password and upload it to R2
make claude        # run Claude Code through the proxy (ARGS="..." for flags)
make codex         # run Codex CLI (proxy is the default provider; ARGS="..." for flags)
make codex-direct  # run Codex CLI with the direct ChatGPT login
make codex-smoke   # one non-interactive Codex turn through the proxy
make codex-backup  # copy ~/.codex/config.toml + openai.config.toml into codex/
make claude-backup # copy ~/.claude settings into claude/ (Omniyat section redacted)
make alias         # print the claude-hara alias for ~/.zshrc
make keys-refresh  # re-fetch the Keychain-cached keys from 1Password now
make health        # /healthz
make accounts      # connected provider accounts
make models        # models available through the proxy
make smoke         # send a test message (MODEL=... to override)
make logs          # last server log lines (LINES=... to override)
make logs-cf       # Worker request logs from Cloudflare (MINUTES=... to override)
```

- **Upgrade upstream:** bump the image tag in `Dockerfile`, then run `make deploy`.
- **Change config:** edit `config.yaml`, then run `make config-push`. It uploads the file to R2 and reloads the running server through the management API. Changes made in the management panel are written straight to R2, and `config-push` overwrites them.
- **Logs:** `make logs` reads the server log files (rotated at 10 MB, capped at 512 MB in total, lost on container restart). `make logs-cf` shows Worker request logs from Cloudflare observability.

## Codex CLI

Plain `codex` goes through the proxy:

- `~/.codex/config.toml` sets `model_provider = "hara"` and defines the `hara` provider (`https://proxy.hara.sh/v1`, Responses API).
- Codex gets its own key, `codex-api-key`, from `scripts/hara-key` (`auth.command`), so no environment variable is needed.
- **Codex accounts only:** with that key, the Worker (`src/index.ts`):
  - allows only `/v1/responses`, `/v1/chat/completions` and `/v1/models`;
  - rejects any model that does not match `gpt-*`, `codex-*` or `o<digit>*` with a 403;
  - removes non-Codex models from `/v1/models`;
  - blocks WebSockets, whose model names it cannot inspect.

  The proxy serves those models only from the Codex OAuth accounts, so Codex never uses a Claude account. The guard filters both model-list formats: the OpenAI `data` list and the `models` catalog the Codex app reads.
- **Fast mode by default:** the Codex app hides its Fast toggle for custom providers and sends no `service_tier`. The Worker therefore adds `service_tier: "priority"` to Codex-key requests that have no tier (`DEFAULT_SERVICE_TIER` in `src/codex/policy.ts`). An explicit tier, including `"default"`, is left as sent.
- **Checking the tier:** OpenAI always reports `service_tier: "default"` in its responses, so check the requested tier with `make logs-cf`. Each Codex request logs a line such as `codex request model=gpt-6.1-sol tier=priority (default) effort=high`, with no prompt content.
- `codex --profile openai` (or `make codex-direct`) bypasses the proxy and uses the direct ChatGPT login. The profile lives in `~/.codex/openai.config.toml` and uses `gpt-5.5`, because `gpt-6.1-sol` is rejected for that login when used directly.
- Codex 0.134+ no longer supports `[profiles.*]` tables inside `config.toml`.
- Backups of both files live in `codex/`. Refresh them with `make codex-backup`, then commit.

## Claude Code config backup

`claude/` holds copies of `~/.claude/settings.json`, `settings.local.json`, `CLAUDE.md` and `.claude.json`, refreshed with `make claude-backup`.

- **Redaction:** `settings.json` is identical to the original, except that `autoMode.environment` is replaced by a placeholder. That section describes Omniyat-internal systems and stays only in `~/.claude/settings.json`.
- **Restoring:** keep the existing `autoMode.environment` block when you restore the file.
- **No secrets:** none of these files holds a key or token.

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

Cursor is **not** an upstream provider. CLIProxyAPI has no built-in support, and the only plugin is a third-party native library that would run with access to every stored token. Cursor can still use this proxy as a client: set the OpenAI base URL to `https://proxy.hara.sh/v1` with the `claude-api-key`.

## Using the proxy

```bash
curl https://proxy.hara.sh/v1/models -H "Authorization: Bearer $(op read --account my.1password.com op://YOUR_VAULT/YOUR_ITEM/claude-api-key)"
```

The container sleeps after 30 minutes idle. The first request after that takes a few seconds while it starts.
