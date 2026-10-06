| `.github/workflows/` | CI: `make code check` and the quota image build, on every pull request |        |      |                                                                                  |
| `scripts/ops/`       | `make status` and `make ops accounts                                   | models | logs | smoke` (Go, standard library): every link checked, with the fix for each failure |
# cli-proxy-api

[CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) running in Docker on this computer at **https://hara.local**. It pools several Claude subscriptions behind one OpenAI/Anthropic-compatible API, for use from Claude Code and other clients. Upstream requests leave from this computer's own internet connection.

| Address                             | Reachable from                                                |
| ----------------------------------- | ------------------------------------------------------------- |
| `http://localhost:8317`             | this computer                                                 |
| `https://hara.local`                | this computer and devices on the same network (mDNS)          |
| `https://cliproxy.<tailnet>.ts.net` | your devices on the tailnet, anywhere (`/v1` for other tools) |

## Use it

| Client        | Command                                                                                     |
| ------------- | ------------------------------------------------------------------------------------------- |
| Claude Code   | `claude-hara` (alias in `~/.zshrc`; `make claude alias` prints it), or `make claude`        |
| Anything else | base URL `https://hara.local/v1`, key `claude-api-key`                                      |
| Codex         | `make codex` or `codex --profile proxy` (WebSockets); plain `codex` keeps its ChatGPT login |

Keys come from `scripts/hara-key`, which caches them in the macOS Keychain and asks 1Password at most once every 30 days.

## Start it

1. In the Tailscale admin console, enable **MagicDNS** and **HTTPS certificates** (DNS page).
2. Run `make up`. It renders the config from 1Password, starts the proxy and a Tailscale container, and registers `hara.local` with [portless](https://github.com/vercel-labs/portless). Starting portless on port 443 asks for your password (sudo). It ends with `make status`. The first time, that prints a Tailscale link that adds the device `cliproxy` to your tailnet; open it, then run `make status` again for the tailnet address.
3. Sign in each provider account, as in [Add an account](#add-an-account).
4. Check it: `make status` should show no `✗`; then `make ops smoke` and `make ops ws-smoke`. For Codex, run `make codex profile` once, then `make codex smoke`.

## Everyday commands

```bash
make up                # start, or apply config.yaml changes
make status            # check every link, containers to accounts, with the fix for each failure
make logs [service]    # follow the container logs: all, or proxy, tailscale, quota
make down              # stop (logins and Tailscale identity kept)
make ops               # usage, reset times and capacity about to expire, per account (ops quota)
make ops accounts      # connected provider accounts
make ops smoke         # one Claude request
make codex smoke       # one Codex turn; fails if it fell back from WebSockets to HTTP
make ops ws-smoke      # WebSockets through each address (IDLE=2m, TS_URL=https://cliproxy.<tailnet>.ts.net)
make ops bench         # time to first token and prompt-cache hits, Claude and Codex (N=5)
make ops logs          # last server log lines (LINES=500)
make code check        # what CI runs: shellcheck, gofmt, Go tests, web type-check and coverage
```

Run `make` for every command: `up`, `down`, `status` and `logs` run the proxy, and the rest are grouped as `make <area> [action]` (`claude`, `codex`, `ops`, `web`, `code`). An unknown action fails with the list of choices. The `ops` actions use `https://hara.local`; pass `URL=http://localhost:8317` to skip portless.

`make status` explains a failure instead of a 502:

```text
✗ proxy container     exited; run: make up, then make logs proxy
✓ tailscale container running
✓ quota container     running
✗ this computer       http://localhost:8317  the proxy does not answer; run: make logs proxy
✗ your network        https://hara.local  502; portless does not reach the proxy; run: make up
! your tailnet        not signed in: open https://login.tailscale.com/a/... to add this device, then run: make status
- panel               not checked: your network fails
- client key          not checked: this computer fails
- accounts            not checked: this computer fails
```

## Change things

| Task                         | Do this                                                           |
| ---------------------------- | ----------------------------------------------------------------- |
| Change proxy config          | edit `config.yaml`, then `make up`                                |
| Upgrade CLIProxyAPI          | bump the proxy image tag in `local/compose.yaml`, then `make up`  |
| Rebuild the management panel | `make code panel`, commit `panel/management.html`, then `make up` |
| Add a provider account       | see [Add an account](#add-an-account)                             |
| Rotate a key                 | change it in 1Password, then `make ops keys` and `make up`        |
| Back up local client configs | `make codex backup` / `make claude backup`, then commit           |

### Add an account

1. Open a **private window** signed in to only that account. Otherwise the browser's current login is connected, usually the wrong one.
2. In https://hara.local/management.html (password: `management-password`), start the provider's OAuth flow and **Copy Link** into that window.
3. The browser ends on a failed `localhost:…/callback?code=…` page. Paste that URL into the panel's **Callback URL** field and submit it once.
4. Check that `make ops accounts` shows the new file.

## How it works

- **Containers:** `local/compose.yaml` runs `eceasy/cli-proxy-api` on its own Docker network, published on `127.0.0.1:8317`, where portless picks it up, with a health check on `/healthz` (`make status` reports it). A Tailscale container serves `https://cliproxy.<tailnet>.ts.net` to `http://proxy:8317` with userspace networking, so no extra privileges are needed; `localhost:8317` and `hara.local` never depend on it. Until it is signed in it waits for the login (`TS_BOOT_TIMEOUT`), keeping one login link, instead of restarting every minute with a new one. The `quota` container (built from `scripts/quota`) starts once the proxy is healthy and reaches it at `http://proxy:8317`.
- **State:** config, OAuth logins and logs (10 MB rotation, 512 MB cap) live in `~/.cli-proxy-api/proxy`, the Tailscale identity in `~/.cli-proxy-api/tailscale`, and the `quota` container's copy of the management password in `~/.cli-proxy-api/quota` (owner-only). `LOCAL_DIR=…` moves all three.
- **Config:** `config.yaml` holds 1Password references only. `make up` renders it into `~/.cli-proxy-api/proxy/config.yaml` and restarts the proxy, which overwrites changes made in the management panel. Copy any panel change into `config.yaml` first.
- **Secrets:** stored in 1Password, item `op://cloudflare/cli-proxy-api`:
    - `claude-api-key` is the only client key.
    - `management-password` is written to the rendered config only as a bcrypt hash. The `quota` container gets the plain value as a compose secret file.
    - The `codex-api-key` and `r2-*` fields are no longer used.
- **Routing:** session affinity keeps each conversation on one account for up to an hour, because prompt caches are per account, and fails over when that account hits its limit (the first turn after a failover rebuilds the cache). Subagents stay on their parent's account.
    - **Claude:** sessions are identified by Claude Code's session ID. The proxy leaves Claude Code's own `cache_control` markers alone and adds markers only for other clients. Check `cache_read_input_tokens` in responses.
    - **Codex:** the cache follows `prompt_cache_key`, which the proxy passes through or derives from the session. Check `usage.input_tokens_details.cached_tokens`.
    - **With quota routing:** a bound session outranks priority, so a boost only steers new sessions and failovers, and never moves a conversation with a warm cache.
- **Limits and resets:** Claude and Codex accounts each have a 5-hour window, which starts at the first request after the previous one ends, and a weekly window. When an account is rate-limited, the proxy reads the provider's reset time (Claude response headers, Codex `usage_limit_reached` body), skips that account until then, and returns it to rotation by itself. Without a reset time it backs off from 1 second up to 30 minutes. Cooldowns are saved next to each auth file (`save-cooldown-status`), so a restart keeps them instead of relearning each from a 429. A usage limit cools down only the model that hit it (`model-level-cooling`), so an exhausted Opus weekly window leaves Sonnet on that account in rotation. When every account is cooling down, the proxy waits at most 5 seconds between retry rounds (`max-retry-interval`) and then answers with the limit, instead of holding the request for 30.
- **Unused capacity:** whatever a weekly window has left when it resets is lost. `make ops quota` reads each account's live usage through the management API, with the token substituted on the server, and lists the weekly windows that reset within 24 hours with capacity left.
- **Use-it-or-lose-it routing:** the `quota` container starts with the proxy (`make up`) and runs `quota -route` at start and then every hour. It raises the priority of each Claude or Codex account whose weekly window resets within 24 hours with capacity left, the sooner the reset the higher, and returns the others to priority 0. The proxy always serves from the highest-priority accounts that are not cooling down, so expiring capacity is used first and the other accounts take over when a boosted one hits its limit. Its decisions are in `make logs quota`.
- **Management panel:** the proxy serves this repo's build of the Management Center, `panel/management.html`: upstream `v1.25.3` plus `panel/ledger.patch`, which adds the quota **Ledger** view (provider totals, then one row per credential; the card grid stays under **Cards**). `local/compose.yaml` mounts it read-only and `config.yaml` turns off the panel auto-update. To move to a newer upstream panel, bump `PANEL_TAG` in `scripts/build-panel.sh` and run `make code panel`; if the patch no longer applies, rebase it on the new tag.
- **Speed:**
    - Codex requests through the proxy run in Fast mode (`service_tier: priority`, more usage per request): a `payload.default` rule in `config.yaml` adds it, because the Codex app sends no tier to a custom provider. The proxy drops any client tier other than `priority` before the rule runs, so a client cannot opt out per request.
    - First-token buffering stays off (`stream-bootstrap-buffering`): the first token is not held back to retry an overloaded account silently.
    - `make ops bench` measures time to first token, total time and prompt-cache hits per address and API. Run it before and after a change.
- **WebSockets:** the proxy serves the Codex Responses socket on `/v1/responses` (and `/backend-api/codex/responses`) and the AI Studio relay on `/v1/ws`. Both require the client key and answer 401 without it; v8 enables the relay's `ws-auth` by default, so `config.yaml` does not set it. `make ops ws-smoke` checks the handshake, a ping/pong round trip, an optional idle hold, the closing handshake and the 401, through `localhost:8317`, `hara.local` and, with `TS_URL`, the tailnet. The proxy answers pings itself, so it needs no provider account. A `response.create` that no account can serve gets no error frame: the proxy closes the socket.
- **Clients:** Claude Code talks to the proxy over HTTP, with SSE streaming; the Anthropic API has no WebSocket transport. Codex uses the Responses WebSocket through the `proxy` profile (`codex/proxy.config.toml`, `supports_websockets = true`, base URL `http://localhost:8317/v1`, key from `scripts/hara-key`). When the socket fails, Codex prints `Falling back from WebSockets to HTTPS transport` and continues over HTTP, so a working answer alone does not prove the transport; `make codex smoke` fails on that warning. In `make ops logs`, a socket turn shows `responses websocket: client connected` and `GET /v1/responses`, an HTTP turn `POST /v1/responses`.
- **Codex without the proxy:** plain `codex` keeps its own ChatGPT login. `~/.codex/config.toml` keeps a provider named `hara` as an alias of that login, because threads started while Codex went through the old hosted proxy remember that name.
- **Options:** `LOCAL_NAME=…` changes `hara.local`; `TS_HOSTNAME=…` changes the tailnet name; `TS_AUTHKEY=…` joins the tailnet without the browser link.

| Path                     | Concern                                                                                                                                    |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `config.yaml`            | CLIProxyAPI config template (1Password references)                                                                                         |
| `local/compose.yaml`     | The proxy and Tailscale containers                                                                                                         |
| `local/serve.json`       | Tailscale HTTPS serve config                                                                                                               |
| `panel/`                 | The management panel build and `ledger.patch`; `make code panel` (`scripts/build-panel.sh`)                                                |
| `scripts/local.sh`       | `make up`, `make down`, `make status`, `make logs`: render the config, start and stop, portless                                            |
| `scripts/hara-key`       | Keychain-cached keys from 1Password                                                                                                        |
| `codex/`                 | Backups of `~/.codex` config; `proxy.config.toml` is the Codex profile for the proxy (`make codex profile`)                                |
| `scripts/codex-smoke.sh` | `make codex smoke`: one Codex turn, fails on the HTTP fallback                                                                             |
| `scripts/wssmoke/`       | `make ops ws-smoke` (Go, standard library): WebSocket checks per address; `make code test`                                                 |
| `scripts/bench/`         | `make ops bench` (Go, standard library): first-token latency and prompt-cache hits; `make code test`                                       |
| `scripts/quota/`         | `make ops quota` and the `quota` container (Go 1.27): usage, resets, use-it-or-lose-it routing; `make code test`                           |
| `scripts/ops/`           | `make status` and `make ops accounts`, `models`, `logs`, `smoke` (Go, standard library): every link checked, with the fix for each failure |
| `scripts/backup-*`       | Copy the Codex and Claude Code client configs into `codex/` and `claude/`                                                                  |
| `scripts/lib/`           | Shared shell helpers, including `render_config`                                                                                            |
| `web/`                   | Landing page on hara.sh                                                                                                                    |
| `.github/workflows/`     | CI on every pull request: `make code check` and the quota image build                                                                      |

## Landing page (`web/`)

[hara.sh](https://hara.sh) is served by the Worker `hara-web`. `web/worker/index.ts` 301-redirects `www.hara.sh` to the apex and serves the static assets Vite built. The earlier Alloy registry Workers (`hara`, `hara-staging`) are deleted.

- **Stack:**
    - Vite, Vue 3 and TypeScript.
    - [shadcn-vue](https://www.shadcn-vue.com) for every UI primitive: Button, Card, Badge, Separator, Tabs. They are vendored in `web/src/components/ui/` and excluded from lint and coverage.
    - Tailwind CSS v4.
    - Design tokens ported from the Geist design system in `web/src/styles/tokens.css`: gray, alpha-gray and purple scales, border shadows, radius and the centered 1448px page column. The primary color is purple-900.
- **Commands:** `make web dev`, `make web test`, `make web coverage` (100% enforced), and `make web deploy` (type-check, coverage, then `cf deploy`).
- **SEO:** canonical `https://hara.sh/`, Open Graph and Twitter cards (`public/og.png`, rendered from `og/og.svg`), JSON-LD, `robots.txt`, `sitemap.xml`, and real 404s. Lighthouse SEO scores 100.
- **Content:** lives in `web/src/lib/content.ts`. The dot-matrix motion (`web/src/lib/dots.ts` and `style.css`) respects reduced-motion settings.
- **Cloudflare account:** personal account "Ollin". The `cf` profile `personal` is bound to this directory.

## Gotchas

- **Account priorities:** the `quota` container owns the priority of Claude and Codex accounts. A priority set in the management panel is overwritten within the hour; to stop it, run `docker compose -f local/compose.yaml stop quota`.
- **Availability:** the proxy is up only while this computer is awake, and requests go out through whatever network it is on.
- **portless LAN mode:** `.local` names exist only in portless LAN mode, which applies to the whole portless proxy. While it is on, every portless app on this computer is `<name>.local` and reachable from the network you are connected to, and portless keeps LAN mode for later starts. To switch back: `portless proxy stop && PORTLESS_LAN=0 portless proxy start`. `make down` removes `hara.local` but leaves the portless proxy running for your other apps.
- **Certificates:** `hara.local` uses portless's own certificate authority, which this computer trusts. Node-based clients such as Claude Code need `NODE_EXTRA_CA_CERTS=~/.portless/ca.pem` (the alias and `make claude` set it). Other devices must trust that file first. The tailnet address has a public certificate and needs nothing.
- **`.local` stays on the network:** mDNS names do not cross Tailscale. Away from this network, use the tailnet address.
- **Nested Claude Code:** `claude-hara` run inside a Claude desktop session uses the app's own login, gets a 401 from the proxy, and appears to hang. Run it from a normal terminal.
- **Redacted backup:** `claude/settings.json` has `autoMode.environment` redacted (Omniyat-internal). Keep your local block when you restore it.
- **No Cursor provider:** Cursor is not an upstream provider, because the only plugin is an untrusted native library. Cursor works as a client through the base URL above.
