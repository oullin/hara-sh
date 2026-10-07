---
description: Connect Claude Code, Codex and OpenAI-compatible clients to Hara. Configure your base URL, client key and WebSocket transport.
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

With Make:

```bash
make claude
make claude ARGS='--model claude-haiku-4-5-20251001'
```

Or configure the environment directly:

```bash
export ANTHROPIC_BASE_URL='http://localhost:8317'
export ANTHROPIC_AUTH_TOKEN="$(./bin/hara key claude-api-key)"
claude
```

`bin/hara` is the [host helper](./operations#make-and-the-host-helper); any `make` action that needs it, such as `make status`, builds it first. On Windows, set `ANTHROPIC_BASE_URL` and `ANTHROPIC_AUTH_TOKEN` in your shell or client settings. Use a normal terminal; a nested desktop session may use its own authentication.

Claude Code uses HTTP with SSE streaming; the Anthropic API does not use the Responses WebSocket.

## Codex

Install the private profile with Make:

```bash
make codex profile
make codex
```

It writes `~/.codex/proxy.config.toml` (or `$CODEX_HOME/proxy.config.toml`), uses `http://localhost:8317/v1`, and has Codex run `bin/hara key claude-api-key` by absolute path whenever it needs the key. Your Codex release must support profile files, command-based auth, and Responses WebSockets.

After connecting an account, verify that Codex stays on WebSockets:

```bash
make codex smoke
```

For another host or port, edit `base_url` in the installed profile. `URL=` does not rewrite it. Run `make codex profile` again if you move this checkout or if an update changes the helper; profiles installed before `bin/hara` call the removed `scripts/hara-key`. Without Make, configure the URL and key directly in your client.

Plain `codex` continues to use your usual login. Select the `proxy` profile only when you want Hara. A working answer does not prove WebSocket transport; the smoke check rejects HTTP fallback.

## OpenAI-compatible clients

| Setting  | Value on the host               |
| -------- | ------------------------------- |
| Base URL | `http://localhost:8317/v1`      |
| API key  | Your client key                 |
| Model    | A model from `tools ops models` |

Test a real request with curl:

```bash
client_key="$(./bin/hara key claude-api-key)"
curl --fail-with-body http://localhost:8317/v1/chat/completions \
  -H "Authorization: Bearer $client_key" \
  -H 'Content-Type: application/json' \
  -d '{"model":"YOUR_MODEL","messages":[{"role":"user","content":"Reply with pong"}]}'
unset client_key
```

Replace `YOUR_MODEL` with an available model. This consumes provider usage. Keep shell tracing off when handling keys.
