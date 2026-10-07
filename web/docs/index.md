---
description: Run your AI accounts through Hara, a self-hosted proxy with account pooling, session affinity and OpenAI- and Anthropic-compatible APIs.
---

# One centre for every model

Hara runs [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) in Docker. Connect your AI accounts and use them through one OpenAI- and Anthropic-compatible endpoint.

## Get started

1. **[Set up the project](./setup)** — start Docker and generate private keys.
2. **[Connect providers](./providers)** — add an account in the management panel.
3. **[Configure clients](./clients)** — point Claude Code, Codex, or another client at Hara.

The server needs Docker Compose. Go, Python, and the operations tools run inside containers. 1Password and Tailscale are optional.

## What Hara handles

| Feature          | Behaviour                                                     |
| ---------------- | ------------------------------------------------------------- |
| Account pooling  | Use multiple accounts behind one endpoint                     |
| Session affinity | Keep a conversation and its subagents on the same account     |
| Failover         | Move to an eligible account when the current one hits a limit |
| Quota routing    | Prefer weekly capacity that would otherwise expire unused     |
| Private state    | Keep keys, provider logins, and logs outside Git              |

The default URL is `http://localhost:8317`. [Tailscale](./networking#tailscale) adds private HTTPS access from other devices.

## Common tasks

- [Check status, usage, or transport](./operations)
- [Change configuration](./configuration)
- [Fix a connection](./troubleshooting)
- [Develop the website](./development)
- [Prepare a public repository](./privacy)
