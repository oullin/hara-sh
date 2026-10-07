---
description: Develop and deploy the Hara landing page and documentation. Build the Vue and VitePress site, run checks and configure Cloudflare hosting.
---

# Website and documentation

The Vue landing page and VitePress docs are static assets served by one Cloudflare Worker. The build renders the landing page into HTML and hydrates it in the browser. Website builds do not read proxy credentials or start Docker.

## Develop

Use Node.js 22.18+ or 24+ and npm:

```bash
make web install
make web dev    # Landing page.
make web docs   # Docs at http://localhost:5174/.
```

| Path                               | Edit                             |
| ---------------------------------- | -------------------------------- |
| `web/src/components/sections/`     | Landing layout                   |
| `web/src/lib/content.ts`           | Landing copy and examples        |
| `web/src/styles/design-tokens.css` | Shared palette, type and spacing |
| `web/docs/*.md`                    | Guides                           |
| `web/docs/.vitepress/config.ts`    | Navigation, search and metadata  |

## Verify

```bash
make format-all
make code check
```

Checks cover shell lint, containerised Go tests and credential tests, the web build, and 100% application coverage. Development checks need Docker, shellcheck, Go for gofmt, Node.js, and installed web dependencies. Server users need only Docker.

Keep browser artefacts outside the checkout. Test desktop/mobile navigation, search, direct links, and missing routes before publishing.

Each docs page needs a unique frontmatter `description`. Use British English in public copy; preserve API headers, commands and configuration keys as written. The VitePress config generates canonical URLs, social metadata and structured data. Keep missing pages out of search results and sitemaps.

## Repository layout

| Path                                                            | Purpose                                                       |
| --------------------------------------------------------------- | ------------------------------------------------------------- |
| `config.yaml`, `local/`                                         | Public proxy template, Compose services and Tailscale serving |
| `scripts/tools/`                                                | Docker tools image, credential initialisation and tests       |
| `scripts/public/`                                               | Publication guard and CI source-language checks               |
| `scripts/ops/`, `quota/`, `wssmoke/`, `bench/` under `scripts/` | Status, quota routing, transport checks and benchmarks        |
| `scripts/*.sh`, `scripts/hara-key`                              | Optional host helpers and private client backups              |
| `codex/proxy.config.toml`                                       | Portable client profile template                              |
| `panel/`                                                        | Management panel build and Ledger patch                       |
| `web/`, `.github/workflows/`                                    | Public website, documentation and CI checks                   |

## Deploy your fork

Set up your own Cloudflare profile with the `cf` CLI. Replace the domains and origins in `web/cloudflare.config.ts`, `web/worker/host.ts`, the HTML metadata, VitePress config, robots files, and sitemaps.

```bash
make web deploy
```

Run deployment from `web/` when invoking `cf` directly. Only the static site and routing Worker are published; your Docker proxy stays private.

## Rebuild the management panel

Install Bun and run `make code panel`. It applies `panel/ledger.patch` to the tagged upstream panel and updates `panel/management.html`. Review the result and recreate the proxy. A newer upstream tag may require rebasing the patch.

Set `PANEL_TAG` to test a newer upstream release. The panel is mounted read-only, and automatic upstream panel updates are disabled so that the Ledger patch remains in use.
