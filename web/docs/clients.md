---
description: Connect Claude Code, Codex, T3 Code and OpenAI-compatible clients to Hara. Configure your base URL, client key and WebSocket transport.
---

# Configure clients

On the Docker host, use `http://localhost:8317`. On another device, use your [Tailscale URL](./networking#tailscale). Install the client separately.

## Get the client key

Run this in a private terminal:

```bash
docker compose -f local/compose.yaml run --rm --no-deps -T tools key claude-api-key
```

Use this key for client requests. The management password and provider tokens are different credentials.

## Claude Code

With Bash and Make:

```bash
make claude
make claude ARGS='--model claude-haiku-4-5-20251001'
```

Or configure the environment directly:

```bash
export ANTHROPIC_BASE_URL='http://localhost:8317'
export ANTHROPIC_AUTH_TOKEN="$(./scripts/hara-key claude-api-key)"
claude
```

On Windows, set `ANTHROPIC_BASE_URL` and `ANTHROPIC_AUTH_TOKEN` in your shell or client settings. Use a normal terminal; a nested desktop session may use its own authentication.

Claude Code uses HTTP with SSE streaming; the Anthropic API does not use the Responses WebSocket.

## Codex

Install the private profile using the Bash helper:

```bash
make codex profile
make codex
```

It writes `~/.codex/proxy.config.toml`, uses `http://localhost:8317/v1`, and reads the key through `hara-key`. Your Codex release must support profile files, command-based auth, and Responses WebSockets.

After connecting an account, verify that Codex stays on WebSockets:

```bash
make codex smoke
```

For another host or port, edit `base_url` in the installed profile. `URL=` does not rewrite it. Reinstall the profile if you move this checkout. Without Bash, configure the URL and key directly in your client.

Plain `codex` continues to use your usual login. Select the `proxy` profile only when you want Hara. A working answer does not prove WebSocket transport; the smoke check rejects HTTP fallback.

## T3 Code

[T3 Code](https://t3.codes) runs Claude Code and Codex for you, so each of its provider instances needs the same URL and key as those clients. Prepare it on the Docker host:

```bash
make t3
```

It writes a Codex home that only uses Hara (`~/.codex-t3-hara`), sets T3 Code's Tailscale HTTPS port to 8443, and prints the values for this computer. Quit T3 Code with Cmd-Q and reopen it before changing any setting, then add two instances under **Settings → Providers**:

| Instance | Field                  | Value                                 |
| -------- | ---------------------- | ------------------------------------- |
| Claude   | CLAUDE_CONFIG_DIR path | `~/.claude-hara`                      |
|          | `ANTHROPIC_BASE_URL`   | `https://hara.local`, with no `/v1`   |
|          | `ANTHROPIC_AUTH_TOKEN` | Your client key, marked **Sensitive** |
|          | `ANTHROPIC_API_KEY`    | Empty value                           |
|          | `NODE_EXTRA_CA_CERTS`  | Absolute path to `~/.portless/ca.pem` |
| Codex    | CODEX_HOME path        | `~/.codex-t3-hara`                    |
|          | Shadow home path       | Empty                                 |
|          | `CODEX_CA_CERTIFICATE` | Absolute path to `~/.portless/ca.pem` |

The separate Claude config directory keeps a cached Anthropic login from replacing the client key. The empty `ANTHROPIC_API_KEY` stops Claude Code from asking for one. The Codex home reads the key through `hara-key`, so it needs no ChatGPT login and no shadow home.

Without portless, `make t3` uses `http://localhost:8317` and drops both certificate variables. `T3_URL=` picks another address. Pick models in the thread's model picker; `make ops models` lists them.

Check both instances with real requests:

```bash
make t3 smoke
```

This consumes provider usage. T3 Code's Tailscale HTTPS uses port 8443 because portless in LAN mode holds port 443; see [Network and TLS](./networking#optional-lan-access).

## OpenAI-compatible clients

| Setting  | Value on the host               |
| -------- | ------------------------------- |
| Base URL | `http://localhost:8317/v1`      |
| API key  | Your client key                 |
| Model    | A model from `tools ops models` |

Test a real request with curl:

```bash
client_key="$(./scripts/hara-key claude-api-key)"
curl --fail-with-body http://localhost:8317/v1/chat/completions \
  -H "Authorization: Bearer $client_key" \
  -H 'Content-Type: application/json' \
  -d '{"model":"YOUR_MODEL","messages":[{"role":"user","content":"Reply with pong"}]}'
unset client_key
```

Replace `YOUR_MODEL` with an available model. This consumes provider usage. Keep shell tracing off when handling keys.
