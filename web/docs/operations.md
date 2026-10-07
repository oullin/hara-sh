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

## Make shortcuts

Bash and Make are optional. These commands run the same Docker tools:

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

Use `make ops bench N=5 CODEX_MODEL=MODEL` for the benchmark and `make ops logs LINES=500` for recent server logs. `MODEL` selects the Claude smoke/benchmark model. `BACKUP_DIR` changes the private client backup destination. Run `make` to list every action and its options.
