# cli-proxy-api

[CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) on Cloudflare Containers at **https://proxy.hara.sh**. It pools several Claude and Codex subscriptions behind one OpenAI/Anthropic-compatible API, for use from Claude Code and Codex.

## Use it

| Client        | Command                                                      | Uses                |
| ------------- | ------------------------------------------------------------ | ------------------- |
| Claude Code   | `claude-hara` (alias in `~/.zshrc`), or `make claude`        | all Claude accounts |
| Codex         | `codex` (the proxy is the default provider), or `make codex` | Codex accounts only |
| Codex, direct | `codex --profile openai`, or `make codex-direct`             | your ChatGPT login  |
| Anything else | base URL `https://proxy.hara.sh/v1`, key `claude-api-key`    | all accounts        |

Keys come from `scripts/hara-key`, which caches them in the macOS Keychain and asks 1Password at most once every 30 days.

## Everyday commands

```bash
make health        # is it up?
make accounts      # connected provider accounts
make smoke         # one Claude request
make codex-smoke   # one Codex turn
make tail-codex    # live: model, tier and effort of each Codex request
make logs          # last server log lines (LINES=500)
```

Run `make` for every target.

## Change things

| Task                         | Do this                                                                                   |
| ---------------------------- | ----------------------------------------------------------------------------------------- |
| Deploy code                  | `make deploy` (runs type-check and 100% coverage first)                                   |
| Change proxy config          | edit `config.yaml`, then `make config-push` (uploads to R2 and reloads the live server)   |
| Upgrade CLIProxyAPI          | bump the image tag in `Dockerfile`, then `make deploy`                                    |
| Add a provider account       | see [Add an account](#add-an-account)                                                     |
| Rotate a key                 | change it in 1Password, then `make keys-refresh` (and `make config-push` for client keys) |
| Back up local client configs | `make codex-backup` / `make claude-backup`, then commit                                   |

### Add an account

1. Open a **private window** signed in to only that account. Otherwise the browser's current login is connected, usually the wrong one.
2. In https://proxy.hara.sh/management.html (password: `management-password`), start the provider's OAuth flow and **Copy Link** into that window.
3. The browser ends on a failed `localhost:…/callback?code=…` page. Paste that URL into the panel's **Callback URL** field and submit it once.
4. Check that `make accounts` shows the new file.

## Develop

```bash
make install       # dependencies
make dev           # run the Worker locally (Vite + workerd)
make check         # tsc for src/, test/ and the Node tooling config
make coverage      # vitest, fails under 100%
make format-all    # fmtkit (oxlint --fix, oxfmt, structural passes)
make lint          # fmtkit lint, read-only
make deploy-dry    # build with Vite and validate without uploading
```

- **Stack:** [Hono](https://hono.dev) handles routing and middleware, [Effect](https://effect.website) runs side effects as services with tagged errors, and [better-result](https://better-result.dev) handles pure policy decisions as `Result`s. [Vite](https://vite.dev) with `@cloudflare/vite-plugin` bundles the Worker for `cf dev` and `cf deploy`.
- **Imports:** `@/…` refers to `src/`, and `@test/…` refers to `test/`.

| Path                     | Concern                                                                                   |
| ------------------------ | ----------------------------------------------------------------------------------------- |
| `src/index.ts`           | Worker entry: exports the Hono app and the `CliProxy` Durable Object                      |
| `src/app.ts`             | Main router: Codex key → `codexRoutes`, everything else → container                       |
| `src/codex/routes.ts`    | Codex-key router: `/v1/models`, `/v1/responses`, `/v1/chat/completions`, else 403/405     |
| `src/codex/authorize.ts` | Pure policy (better-result): parse body, allow Codex models, default tier, filter lists   |
| `src/codex/errors.ts`    | Policy failures as tagged errors carrying their HTTP status                               |
| `src/codex/program.ts`   | Side effects (Effect): read body, audit log, forward, list models                         |
| `src/effect/run.ts`      | Hono → Effect bridge: provides `Upstream` and maps every typed error to a response        |
| `src/upstream.ts`        | `Upstream` Effect service: the single `CliProxy` Durable Object, pinned to Western Europe |
| `src/http/env.ts`        | Hono bindings/variables and the middleware that provides `Upstream` per request           |
| `src/http/errors.ts`     | JSON error responses                                                                      |
| `src/auth/client-key.ts` | Reads the client key; constant-time comparison                                            |
| `src/codex/policy.ts`    | Codex model pattern, allowed paths, default service tier                                  |
| `src/container/*`        | The Durable Object that owns the container, plus the explicit-image workaround            |
| `scripts/`               | deploy, config push, key cache, config backups (`lib/common.sh` shared)                   |
| `test/`                  | Vitest suite, one file per module                                                         |

## How it works

- **Runtime:** a Worker forwards requests to a single container running `eceasy/cli-proxy-api`. The container sleeps after 30 minutes idle, so the first request after that takes a few seconds.
- **State:** the config and OAuth tokens live in the R2 bucket `cli-proxy-api`, and the container disk is only a cache. Server log files (10 MB rotation, 512 MB cap) are lost on restart.
- **Secrets:** stored in 1Password, item `op://cloudflare/cli-proxy-api`:
    - `claude-api-key` is the main client key.
    - `codex-api-key` is restricted to Codex.
    - `management-password` is stored in R2 only as a bcrypt hash.
    - The `r2-*` fields hold the R2 S3 credentials.
- **Codex isolation:** the Worker restricts the Codex key:
    - It allows only `gpt-*`, `codex-*` and `o<n>` models.
    - It allows only `/v1/responses`, `/v1/chat/completions` and `/v1/models`, and no WebSockets.
    - It filters model lists, so Codex never draws from the Claude accounts.
- **Fast mode:** Codex-key requests without a `service_tier` are sent as `priority`; an explicit tier is left as sent. OpenAI always reports `default` in its responses, so `make tail-codex` is the way to verify the tier.
- **Routing:** session affinity keeps each conversation on one account to reuse prompt caches, and fails over when an account hits its limit.
- **Cloudflare account:** personal account "Ollin". The `cf` profile `personal` is bound to this directory. Live logs use wrangler with the personal login kept in `~/.claude/work/wrangler-personal`.

## Gotchas

- **Region:** Anthropic's OAuth rejects the default container region with a 403, which is why the container is pinned to Western Europe (`src/upstream.ts`).
- **No Fast toggle:** the Codex desktop app shows no Fast toggle for custom providers. Fast mode is still applied, by the Worker.
- **Panel changes:** `make config-push` overwrites changes made in the management panel. Copy any panel change into `config.yaml` first.
- **Nested Claude Code:** `claude-hara` run inside a Claude desktop session uses the app's own login, gets a 401 from the proxy, and appears to hang. Run it from a normal terminal.
- **Redacted backup:** `claude/settings.json` has `autoMode.environment` redacted (Omniyat-internal). Keep your local block when you restore it.
- **No Cursor provider:** Cursor is not an upstream provider, because the only plugin is an untrusted native library. Cursor works as a client through the base URL above.
