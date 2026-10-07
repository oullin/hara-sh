---
description: Fix Hara startup, authentication, provider, network and WebSocket problems. Diagnose your Docker stack and share errors safely.
---

# Troubleshooting

Start here:

```bash
docker compose -f local/compose.yaml ps -a
docker compose -f local/compose.yaml run --rm --no-deps -T tools status
docker compose -f local/compose.yaml logs --tail=100 init proxy quota
```

## Startup fails

- Check `docker info`, Compose v2, and Linux-container mode on Windows.
- If port 8317 is occupied, set `HARA_PORT` and update client URLs.
- On Windows, set an absolute `LOCAL_DIR` in the current shell.
- `init` should exit zero. Its log explains template or credential errors.

Apply changes with `up -d --build --force-recreate`. The public config contains placeholders; do not mount it directly as runtime config.

## Existing installation needs an import

[Import the original keys](./configuration#migrate-from-1password). If only one secret file exists, restore both from the same backup. Do not delete files to bypass initialisation.

## Permission denied

Create the state directory as your normal user before startup. If Docker created it as root on Linux, restore its ownership. Check Docker Desktop file sharing and host permissions. Keep credential files private.

## API returns 401

Use the **client key** for API calls and the **management password** for the panel. After rotation or import, recreate services and refresh client settings. Do not rotate repeatedly as a diagnostic step.

For Claude Code, try a normal terminal outside a nested desktop session.

## No models or provider requests fail

Run `tools ops accounts`, `tools ops models`, and `tools quota` with the Compose command above. Connect an account that supports your selected model. A healthy empty proxy cannot answer requests.

Provider cooldowns survive restarts. Use an eligible account/model or wait for the reset; restarting does not restore capacity.

## Container cannot reach localhost

Use `http://proxy:8317` inside tools containers. Host clients use `http://localhost:8317`. Use `host.docker.internal` for other host-local services.

## Tailscale does not connect

Start with `--profile tailscale`, then run:

```bash
docker compose -f local/compose.yaml exec tailscale tailscale status --peers=false
```

Authorise the device and check MagicDNS, HTTPS certificates, ACLs, and client tailnet membership. See [Network and TLS](./networking).

## Codex falls back to HTTP

```bash
docker compose -f local/compose.yaml run --rm --no-deps -T tools wssmoke http://proxy:8317
```

Then run `make codex smoke`. Reinstall the profile if the checkout moved. Server logs show `GET /v1/responses` for WebSockets and `POST /v1/responses` for HTTP. Check provider availability if the socket opens but cannot serve a turn.

## Panel settings disappear

Startup renders the template again. Keep public settings in `config.yaml` and private provider settings in an [outside-Git template](./configuration#private-provider-settings). Quota priorities may change hourly.

## Report a problem

Include the failing command, image version, OS, and a sanitised error. Remove keys, callbacks, account names, tailnet names, paths, and prompts. Never attach secret files, rendered config, provider logins, or full logs.
