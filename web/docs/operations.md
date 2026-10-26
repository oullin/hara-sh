---
description: Operate your Hara Docker stack. Check accounts and quota, test API and WebSocket requests, rotate keys and back up private state.
---

# Operations

Run commands from the repository root. Use `--profile tailscale` on startup if you enabled it.

## Start, stop, and inspect

```bash
# Start or apply changes.
docker compose -f local/compose.yaml up -d --build --force-recreate

# Inspect containers and authenticated endpoints.
docker compose -f local/compose.yaml ps -a
docker compose -f local/compose.yaml run --rm --no-deps -T tools status

# Follow logs; stop without deleting private state.
docker compose -f local/compose.yaml logs -f --tail=100 proxy
docker compose -f local/compose.yaml --profile tailscale down
```

`init` exiting zero is expected. A fresh proxy can pass status with no provider accounts. [Connect one](./providers) before sending model requests.

## Accounts, models, and quota

```bash
docker compose -f local/compose.yaml run --rm --no-deps -T tools ops accounts
docker compose -f local/compose.yaml run --rm --no-deps -T tools ops models
docker compose -f local/compose.yaml run --rm --no-deps -T tools quota
docker compose -f local/compose.yaml run --rm --no-deps -T tools ops logs -n 500
```

Quota adjusts account priorities hourly. Pause it with `docker compose -f local/compose.yaml stop quota`; startup resumes it. Pausing leaves the last priorities in place.

The management panel's **Ledger** view shows provider totals and usage per credential; **Cards** keeps the account grid. Follow routing decisions with `docker compose -f local/compose.yaml logs -f quota`.

## Check a request

```bash
# Real Claude request; consumes provider usage.
docker compose -f local/compose.yaml run --rm --no-deps -T tools ops smoke

# WebSocket handshake, ping/pong, close and unauthenticated rejection.
docker compose -f local/compose.yaml run --rm --no-deps -T tools wssmoke -idle 2m http://proxy:8317
```

WebSocket smoke needs no provider account. Use `ops smoke -model MODEL` to select an available Claude model.

## Benchmark

```bash
docker compose -f local/compose.yaml run --rm --no-deps -T tools bench -n 5 http://proxy:8317
```

This sends real requests of roughly 5,000 prompt tokens. The first is cold; later requests test cache reuse. `-claude-model` and `-codex-model` select models; an empty value skips that API.

Compare time to first token, total time and cached input tokens before and after a routing change. Claude reports cache hits through `cache_read_input_tokens`; Codex uses `usage.input_tokens_details.cached_tokens`.

Inside tools containers, use `http://proxy:8317`. For another target, set `TOOLS_URL` for `ops` and `quota`, or pass the URL to `wssmoke` and `bench`.

## Rotate keys

```bash
docker compose -f local/compose.yaml run --rm --no-deps -T init rotate
docker compose -f local/compose.yaml up -d --build --force-recreate
```

This replaces both client and management keys, retaining provider logins. Update client settings, restart clients that cached the old key, and log back into the panel. For 1Password, [import the changed values](./configuration#migrate-from-1password) instead.

## Backups

Stop the stack and copy `$LOCAL_DIR` into encrypted private storage. It contains keys, rendered config, provider logins, logs, and Tailscale identity. Restore the same state path before starting again.

## Make and the host helper

Make is optional. Its targets call `bin/hara`, a small Go program built from `scripts/hara` on first use and rebuilt when that source changes. Without Go on the host, Make builds it in a `golang` container for your system. `bin/hara` is ignored by Git; delete it to force a rebuild.

These commands run the same Docker tools as above:

| Task                         | Command                                                    |
| ---------------------------- | ---------------------------------------------------------- |
| Start / stop / status        | `make up` / `make down` / `make status`                    |
| Serve `https://hara.local`   | `make portless` (also run by `make up`)                    |
| Remove containers and images | `make purge` (private state kept)                          |
| Accounts / models / quota    | `make ops accounts` / `make ops models` / `make ops quota` |
| Request / WebSocket check    | `make ops smoke` / `make ops ws-smoke IDLE=2m`             |
| Real Codex transport check   | `make codex smoke`                                         |
| Prepare / check T3 Code      | `make t3` / `make t3 smoke`                                |
| Rotate / import keys         | `make ops keys` / `make ops import-op`                     |
| Back up host client settings | `make claude backup` / `make codex backup`                 |

Client backups are separate from server backups and remain outside Git. `URL` selects a Make target; `TS_URL` adds a tailnet URL to WebSocket smoke.

Use `make ops bench N=5 CODEX_MODEL=MODEL` for the benchmark and `make ops logs LINES=500` for recent server logs. `MODEL` selects the Claude smoke/benchmark model. Run `make` to list every action and its options.

## Latency and recovery

Hara keeps conversations on their original account to reuse prompt caches. A transient failure can use another account for one turn while retaining the original binding; quota and authentication failures still move the binding. Temporary upstream failures cool an account for five seconds instead of the upstream default of 60 seconds. Usage-limit reset times are unchanged.

Every eligible account can be tried, with at most one additional retry round. Streaming allows one recovery attempt before any output and sends heartbeats every 15 seconds after the stream starts. Partially delivered responses are never replayed, and pre-output heartbeats stay off so they do not prematurely commit the response. Upstream TLS handshakes have a 15-second limit; generation and established streams have no new time limit. Response buffering remains off to preserve first-token latency.

`make up` validates Compose configuration, builds the tools image, and pulls the pinned proxy image before recreating services. Preparation failures leave the running containers in place. Startup waits up to 60 seconds for readiness before reporting success. Recreation interrupts active requests, so finish important turns first. Runtime container logs rotate separately from the proxy's own log files.

Use `make code resilience` on a Docker host with Go to exercise the pinned image against synthetic upstream failures. It uses disposable containers, synthetic credentials, and a local stub; it does not call AI providers or read your account files. CI runs the same checks. Use `make ops bench` to measure real inference latency and cache reuse separately.

The host auth helper reads the existing private credential file directly, so starting a T3 provider session does not start an extra Docker container just to retrieve the key. It bounds and validates the credential before printing it, retains the container fallback when the file is missing, and keeps the explicit 1Password mode. Credentials stay outside the checkout.

For T3 latency, check the effort level selected in T3 itself: per-request options can override provider-home defaults. Auxiliary title and branch-name generation rarely need the same effort level as difficult coding work. Lower effort or a smaller model is a quality/cost tradeoff; Hara does not change these choices. Keep prompt-cache affinity and benchmark before reducing compaction windows or changing endpoints. A local HTTPS hop is often much smaller than model reasoning time.

You can also run the helper directly from any directory; it finds the checkout from its own location. Run `./bin/hara` without arguments to list its commands:

```bash
./bin/hara status
./bin/hara logs proxy
./bin/hara key claude-api-key   # Prints a secret.
./bin/hara tools ops accounts
```

Besides the [stack settings](./configuration#settings) such as `LOCAL_DIR` and `HARA_PORT`, and its [optional settings file](./configuration#host-helper-settings), the helper reads:

| Variable                                                                                      | Effect                                                                                     |
| --------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| `URL`                                                                                         | Target for `tools`, `ws-smoke` and `bench`; host loopback maps to `http://proxy:8317`      |
| `HARA_SECRET_PROVIDER=op`                                                                     | `key` reads 1Password through `OP_ACCOUNT`, `OP_VAULT` and `OP_ITEM_NAME`                  |
| `BACKUP_DIR`                                                                                  | Client backup destination (default `$LOCAL_DIR/backups`)                                   |
| `CODEX_HOME`                                                                                  | Codex settings directory for `codex-profile` and `codex-smoke`                             |
| `CODEX_PROFILE`, `PROXY_URL`                                                                  | Profile and proxy address checked by `make codex smoke`                                    |
| `T3_URL`, `T3_CODEX_HOME`, `T3_CLAUDE_HOME`, `T3_CURSOR_HOME`, `T3_HOME`, `T3_TAILSCALE_PORT` | Address, Codex, Claude and Cursor homes, T3 Code settings and Tailscale port for `make t3` |
| `PANEL_TAG`, `PANEL_REPO`                                                                     | Upstream panel release and repository for `make code panel`                                |
