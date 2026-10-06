# cli-proxy-api

[CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) running in Docker on this computer at **https://hara.local**. It pools several Claude subscriptions behind one OpenAI/Anthropic-compatible API, for use from Claude Code and other clients. Upstream requests leave from this computer's own internet connection.

| Address                             | Reachable from                                                |
| ----------------------------------- | ------------------------------------------------------------- |
| `http://localhost:8317`             | this computer                                                 |
| `https://hara.local`                | this computer and devices on the same network (mDNS)          |
| `https://cliproxy.<tailnet>.ts.net` | your devices on the tailnet, anywhere (`/v1` for other tools) |

## Use it

| Client        | Command                                                                       |
| ------------- | ----------------------------------------------------------------------------- |
| Claude Code   | `claude-hara` (alias in `~/.zshrc`; `make alias` prints it), or `make claude` |
| Anything else | base URL `https://hara.local/v1`, key `claude-api-key`                        |
| Codex         | not proxied: `codex` uses its own ChatGPT login                               |

Keys come from `scripts/hara-key`, which caches them in the macOS Keychain and asks 1Password at most once every 30 days.

## Start it

1. In the Tailscale admin console, enable **MagicDNS** and **HTTPS certificates** (DNS page).
2. Run `make local`. It renders the config from 1Password, starts the proxy and a Tailscale container, and registers `hara.local` with [portless](https://github.com/vercel-labs/portless). Starting portless on port 443 asks for your password (sudo). The first time, it prints a Tailscale link that adds the device `cliproxy` to your tailnet; open it, then run `make local-status` for the tailnet address.
3. Sign in each provider account, as in [Add an account](#add-an-account).
4. Check it: `make health`, `make accounts`, `make smoke`.

## Everyday commands

```bash
make local         # start, or apply config.yaml changes
make local-status  # addresses and Tailscale login state
make health        # is it up?
make accounts      # connected provider accounts
make quota         # usage, reset times and capacity about to expire, per account
make smoke         # one Claude request
make logs          # last server log lines (LINES=500)
make local-logs    # follow the container logs
make local-down    # stop (logins and Tailscale identity kept)
```

Run `make` for every target. The operations targets use `https://hara.local`; pass `URL=http://localhost:8317` to skip portless.

## Change things

| Task                         | Do this                                                                 |
| ---------------------------- | ----------------------------------------------------------------------- |
| Change proxy config          | edit `config.yaml`, then `make local`                                   |
| Upgrade CLIProxyAPI          | bump the proxy image tag in `local/compose.yaml`, then `make local`     |
| Rebuild the management panel | `make panel`, commit `panel/management.html`, then `make local`         |
| Add a provider account       | see [Add an account](#add-an-account)                                   |
| Rotate a key                 | change it in 1Password, then `make keys-refresh` and `make local`       |
| Back up local client configs | `make codex-backup` / `make claude-backup`, then commit                 |

### Add an account

1. Open a **private window** signed in to only that account. Otherwise the browser's current login is connected, usually the wrong one.
2. In https://hara.local/management.html (password: `management-password`), start the provider's OAuth flow and **Copy Link** into that window.
3. The browser ends on a failed `localhost:…/callback?code=…` page. Paste that URL into the panel's **Callback URL** field and submit it once.
4. Check that `make accounts` shows the new file.

## How it works

- **Containers:** `local/compose.yaml` runs `eceasy/cli-proxy-api` inside the network of a Tailscale container. The `quota` container (built from `scripts/quota`) shares that network too. Tailscale serves `https://cliproxy.<tailnet>.ts.net` to it with userspace networking, so no extra privileges are needed. The proxy is also published on `127.0.0.1:8317`, where portless picks it up.
- **State:** config, OAuth logins and logs (10 MB rotation, 512 MB cap) live in `~/.cli-proxy-api/proxy`, the Tailscale identity in `~/.cli-proxy-api/tailscale`, and the `quota` container's copy of the management password in `~/.cli-proxy-api/quota` (owner-only). `LOCAL_DIR=…` moves all three.
- **Config:** `config.yaml` holds 1Password references only. `make local` renders it into `~/.cli-proxy-api/proxy/config.yaml` and restarts the proxy, which overwrites changes made in the management panel. Copy any panel change into `config.yaml` first.
- **Secrets:** stored in 1Password, item `op://YOUR_VAULT/YOUR_ITEM`:
    - `claude-api-key` is the only client key.
    - `management-password` is written to the rendered config only as a bcrypt hash. The `quota` container gets the plain value as a compose secret file.
    - The `codex-api-key` and `r2-*` fields are no longer used.
- **Routing:** session affinity keeps each conversation on one account to reuse prompt caches, and fails over when an account hits its limit.
- **Limits and resets:** Claude and Codex accounts each have a 5-hour window, which starts at the first request after the previous one ends, and a weekly window. When an account is rate-limited, the proxy reads the provider's reset time (Claude response headers, Codex `usage_limit_reached` body), skips that account until then, and returns it to rotation by itself. Without a reset time it backs off from 1 second up to 30 minutes. Cooldowns are kept in memory, so after a restart the proxy relearns them from one 429 per exhausted account.
- **Unused capacity:** whatever a weekly window has left when it resets is lost. `make quota` reads each account's live usage through the management API, with the token substituted on the server, and lists the weekly windows that reset within 24 hours with capacity left.
- **Use-it-or-lose-it routing:** the `quota` container starts with the proxy (`make local`) and runs `quota -route` at start and then every hour. It raises the priority of each Claude or Codex account whose weekly window resets within 24 hours with capacity left, the sooner the reset the higher, and returns the others to priority 0. The proxy always serves from the highest-priority accounts that are not cooling down, so expiring capacity is used first and the other accounts take over when a boosted one hits its limit. Its decisions are in `make local-logs`.
- **Management panel:** the proxy serves this repo's build of the Management Center, `panel/management.html`: upstream `v1.25.3` plus `panel/ledger.patch`, which adds the quota **Ledger** view (provider totals, then one row per credential; the card grid stays under **Cards**). `local/compose.yaml` mounts it read-only and `config.yaml` turns off the panel auto-update. To move to a newer upstream panel, bump `PANEL_TAG` in `scripts/build-panel.sh` and run `make panel`; if the patch no longer applies, rebase it on the new tag.
- **Codex:** Codex uses its own ChatGPT login. `~/.codex/config.toml` keeps a provider named `hara` as an alias of that login, because threads started while Codex went through the old hosted proxy remember that name.
- **Options:** `LOCAL_NAME=…` changes `hara.local`; `TS_HOSTNAME=…` changes the tailnet name; `TS_AUTHKEY=…` joins the tailnet without the browser link.

| Path                 | Concern                                                                             |
| -------------------- | ----------------------------------------------------------------------------------- |
| `config.yaml`        | CLIProxyAPI config template (1Password references)                                  |
| `local/compose.yaml` | The proxy and Tailscale containers                                                  |
| `local/serve.json`   | Tailscale HTTPS serve config                                                        |
| `panel/`             | The management panel build and `ledger.patch`; `make panel` (`scripts/build-panel.sh`) |
| `scripts/local.sh`   | `make local*`: render the config, start and stop, portless, print the addresses     |
| `scripts/hara-key`   | Keychain-cached keys from 1Password                                                 |
| `scripts/quota/`     | `make quota` and the `quota` container (Go 1.27): usage, resets, use-it-or-lose-it routing; `make quota-test` |
| `scripts/backup-*`   | Copy the Codex and Claude Code client configs into `codex/` and `claude/`           |
| `scripts/lib/`       | Shared shell helpers, including `render_config`                                     |
| `web/`               | Landing page on hara.sh                                                             |

## Landing page (`web/`)

[hara.sh](https://hara.sh) is served by the Worker `hara-web`. `web/worker/index.ts` 301-redirects `www.hara.sh` to the apex and serves the static assets Vite built. The earlier Alloy registry Workers (`hara`, `hara-staging`) are deleted.

- **Stack:**
    - Vite, Vue 3 and TypeScript.
    - [shadcn-vue](https://www.shadcn-vue.com) for every UI primitive: Button, Card, Badge, Separator, Tabs. They are vendored in `web/src/components/ui/` and excluded from lint and coverage.
    - Tailwind CSS v4.
    - Design tokens ported from the Geist design system in `web/src/styles/tokens.css`: gray, alpha-gray and purple scales, border shadows, radius and the centered 1448px page column. The primary color is purple-900.
- **Commands:** `make web-dev`, `make web-test`, `make web-coverage` (100% enforced), and `make web-deploy` (type-check, coverage, then `cf deploy`).
- **SEO:** canonical `https://hara.sh/`, Open Graph and Twitter cards (`public/og.png`, rendered from `og/og.svg`), JSON-LD, `robots.txt`, `sitemap.xml`, and real 404s. Lighthouse SEO scores 100.
- **Content:** lives in `web/src/lib/content.ts`. The dot-matrix motion (`web/src/lib/dots.ts` and `style.css`) respects reduced-motion settings.
- **Cloudflare account:** personal account "Ollin". The `cf` profile `personal` is bound to this directory.

## Gotchas

- **Account priorities:** the `quota` container owns the priority of Claude and Codex accounts. A priority set in the management panel is overwritten within the hour; to stop it, run `docker compose -f local/compose.yaml stop quota`.
- **Availability:** the proxy is up only while this computer is awake, and requests go out through whatever network it is on.
- **portless LAN mode:** `.local` names exist only in portless LAN mode, which applies to the whole portless proxy. While it is on, every portless app on this computer is `<name>.local` and reachable from the network you are connected to, and portless keeps LAN mode for later starts. To switch back: `portless proxy stop && PORTLESS_LAN=0 portless proxy start`. `make local-down` removes `hara.local` but leaves the portless proxy running for your other apps.
- **Certificates:** `hara.local` uses portless's own certificate authority, which this computer trusts. Node-based clients such as Claude Code need `NODE_EXTRA_CA_CERTS=~/.portless/ca.pem` (the alias and `make claude` set it). Other devices must trust that file first. The tailnet address has a public certificate and needs nothing.
- **`.local` stays on the network:** mDNS names do not cross Tailscale. Away from this network, use the tailnet address.
- **Nested Claude Code:** `claude-hara` run inside a Claude desktop session uses the app's own login, gets a 401 from the proxy, and appears to hang. Run it from a normal terminal.
- **Redacted backup:** `claude/settings.json` has `autoMode.environment` redacted (Omniyat-internal). Keep your local block when you restore it.
- **No Cursor provider:** Cursor is not an upstream provider, because the only plugin is an untrusted native library. Cursor works as a client through the base URL above.
