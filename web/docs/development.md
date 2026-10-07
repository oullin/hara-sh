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

`make format-all` is the only formatter and lint: it runs fmtkit over the TS/Vue sources and every Go module (with go vet). `make code check` covers containerised Go tests and credential tests, the web build, 100% application coverage and the publication guard. Development checks need Docker, fmtkit, Go, Node.js, and installed web dependencies. Server users need only Docker: without host Go, `make` builds the `bin/hara` host helper in a container.

Keep browser artefacts outside the checkout. Test desktop/mobile navigation, search, direct links, and missing routes before publishing.

Each docs page needs a unique frontmatter `description`. Use British English in public copy; preserve API headers, commands and configuration keys as written. The VitePress config generates canonical URLs, social metadata and structured data. Keep missing pages out of search results and sitemaps.

## Repository layout

| Path                                                            | Purpose                                                       |
| --------------------------------------------------------------- | ------------------------------------------------------------- |
| `config.yaml`, `local/`                                         | Public proxy template, Compose services and Tailscale serving |
| `scripts/tools/`                                                | Docker tools image, credential initialisation and tests       |
| `scripts/public/`                                               | Publication guard and CI source-language checks               |
| `scripts/ops/`, `quota/`, `wssmoke/`, `bench/` under `scripts/` | Status, quota routing, transport checks and benchmarks        |
| `scripts/hara/`                                                 | Host helper behind `make`, built into `bin/hara`              |
| `codex/proxy.config.toml`                                       | Portable client profile template                              |
| `panel/`                                                        | Management panel build and Ledger patch                       |
| `web/`, `.github/workflows/`                                    | Public website, documentation and CI checks                   |

## Host helper

`scripts/hara` is the Go program behind the Make targets. It uses only the standard library and runs `docker`, `git`, `bun`, `op`, `portless`, `claude` and `codex` as child processes:

| File          | Commands                                                           |
| ------------- | ------------------------------------------------------------------ |
| `stack.go`    | Compose stack, containerised tools, Tailscale wait, portless       |
| `clients.go`  | `key`, client backups, Codex profile and WebSocket smoke           |
| `t3.go`       | T3 Code setup and smoke                                            |
| `panel.go`    | Management panel build                                             |
| `settings.go` | Checkout discovery, `credentials.env` parsing, private directories |

Tests replace the child processes with a recorder, so they need no Docker or network. Run them with `cd scripts/hara && go test ./...`; `make code check` also runs them in the tools image.

## Deploy your fork

Set up your own Cloudflare profile with the `cf` CLI. Replace the domains and origins in `web/cloudflare.config.ts`, `web/worker/host.ts`, the HTML metadata, VitePress config, robots files, and sitemaps.

```bash
make web deploy
```

Run deployment from `web/` when invoking `cf` directly. Only the static site and routing Worker are published; your Docker proxy stays private.

## Rebuild the management panel

Install Bun and run `make code panel`. It applies `panel/ledger.patch` to the tagged upstream panel and updates `panel/management.html`. Review the result and reload the panel; the proxy serves the new file without a restart. A newer upstream tag may require rebasing the patch.

Set `PANEL_TAG` to test a newer upstream release. The panel is mounted read-only, and automatic upstream panel updates are disabled so that the Ledger patch remains in use.
