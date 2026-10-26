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

## Claude repeatedly returns 429

Read the upstream error in **Logs → Error Request Logs**. `Usage credits are required for fast mode.` means the request selected paid fast mode, even if the subscription still has ordinary quota. Hara preserves the client's speed selection; the proxy cannot supply missing usage credits.

To use fast mode, enable and fund **Usage credits** in Claude's **Settings → Usage**, and check the spending limit. Work accounts may need a billing administrator to replenish credits or an organization Owner to enable fast mode. Every account receiving fast requests needs the required credits and access; an account without them can still serve standard requests. See [Anthropic's fast-mode requirements](https://code.claude.com/docs/en/fast-mode#requirements).

If you need service while credits are unavailable, turn off `/fast` in the client. To enforce standard speed across clients temporarily, add this under the existing `requests.payload` section of your configuration template, then apply it with `make up`:

```yaml
override:
  - models:
      - name: "claude-*"
        protocol: "claude"
    params:
      speed: "standard"
```

Remove this override once the accounts have fast-mode credits; leaving it in place forces standard speed even when the client requests fast mode.

Other 429 responses can indicate genuine rate or usage limits. Respect their retry/reset times; changing speed does not restore exhausted quota. The pinned proxy includes fixes that preserve cooldowns during concurrent requests and credential-file updates, and distinguish short per-minute limits from billing resets.

## Connection failures and cancellation logs

`EOF` or `connect: connection refused` from Anthropic indicates an upstream connection failed. Hara bounds the upstream TLS handshake to 15 seconds so a silent handshake can reach the proxy's retry/failover handling. This timeout does not limit generation or established streams, and cannot prevent a remote outage.

`499` and `response body close error: context canceled` commonly follow a client stopping a request. Check whether a turn actually failed before treating these as provider faults. A model-catalog refresh warning says the last valid catalog was retained; investigate network access if refreshes continue failing or models are missing.

Claude Code sends `HEAD /api/hello` to its configured Anthropic base URL to warm up the connection. Hara does not expose that route, so it returns a harmless 404; Claude Code ignores the result. Use `/healthz` for your own health checks. This is separate from inference failures.

## Container cannot reach localhost

Use `http://proxy:8317` inside tools containers. Host clients use `http://localhost:8317`. Use `host.docker.internal` for other host-local services.

## Tailscale does not connect

Start with `--profile tailscale`, then run:

```bash
docker compose -f local/compose.yaml exec tailscale tailscale status --peers=false
```

Authorise the device and check MagicDNS, HTTPS certificates, ACLs, and client tailnet membership. See [Network and TLS](./networking).

## hara.local does not respond

Run `make portless`, then `portless doctor`. The portless proxy must run in LAN mode; after a reboot, it only returns if you installed it with `sudo portless service install --lan`. Node clients also need `NODE_EXTRA_CA_CERTS="$HOME/.portless/ca.pem"`.

## T3 Code's Tailscale HTTPS switch turns itself off

Portless in LAN mode holds port 443 on the host's tailnet address. Run `make t3`, quit T3 Code with Cmd-Q, reopen it, and switch Tailscale HTTPS on again. T3 Code then serves `https://<host>.<your-tailnet>.ts.net:8443`; `T3_TAILSCALE_PORT=` picks another free port. Check it with `tailscale serve status`.

## T3 Code cannot pair a device

- **`/.well-known/t3/environment` returned undeclared status 404**: the host is not T3 Code. `https://hara.local` is Hara. Use the Tailscale HTTPS address from `tailscale serve status`, such as `https://<host>.<your-tailnet>.ts.net:8443`.
- **The environment credential is invalid**: the host is right, but the code is not. Codes come from **Create pairing code** in T3 Code's settings, work once and expire after five minutes. Create a new one and paste the full pairing URL.

See [Pair a phone](./clients#pair-a-phone).

## Codex or T3 Code reports `unsupported websocket request type: response.interrupt`

Codex's instant interruption sends `response.interrupt` when new input arrives during a response. CLIProxyAPI v8.0.15 rejects that event. Hara's Codex profile disables `features.instant_interrupt`, so steering input waits for the current response to finish and WebSockets remain enabled.

Run `make codex profile` for the proxy profile and `make t3` for T3 Code's Codex home, then restart the affected app. If your Codex app uses a Hara provider defined directly in `~/.codex/config.toml`, add `instant_interrupt = false` to its existing `[features]` table as well. Stopping a turn still works; only instant steering is disabled.

## Codex falls back to HTTP

```bash
docker compose -f local/compose.yaml run --rm --no-deps -T tools wssmoke http://proxy:8317
```

Then run `make codex smoke`. Reinstall the profile if the checkout moved. Server logs show `GET /v1/responses` for WebSockets and `POST /v1/responses` for HTTP. Check provider availability if the socket opens but cannot serve a turn.

## Host helper errors

- **`expected NAME=value` with a line number**: fix that line in `credentials.env`. See the supported [setting forms](./configuration#host-helper-settings).
- **`private state moved to ~/.hara-sh`**: move the old directory as shown, or set `LOCAL_DIR` to it.
- **`must be outside the repository`**: point `LOCAL_DIR` or `BACKUP_DIR` at a directory outside the checkout, including through symlinks.
- **`run hara from the hara-sh checkout`**: run Make from the repository root, or rebuild `bin/hara` with any Make action.
- **Codex cannot fetch its key after an update**: Codex logs `provider auth command .../scripts/hara-key failed to start` and the proxy answers `401` with `Missing API key`. Run `make codex profile`, and `make t3` if you use T3 Code, so the profiles call `bin/hara`. Fix any `config.toml` line that `make codex profile` names, then restart Codex or T3 Code; a running session keeps the old command.
- **The helper seems out of date**: delete `bin/hara`; the next Make action rebuilds it.

## Panel settings disappear

Startup renders the template again. Keep public settings in `config.yaml` and private provider settings in an [outside-Git template](./configuration#private-provider-settings). Quota priorities may change hourly.

## Report a problem

Include the failing command, image version, OS, and a sanitised error. Remove keys, callbacks, account names, tailnet names, paths, and prompts. Never attach secret files, rendered config, provider logins, or full logs.
