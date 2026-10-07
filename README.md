# Hara

A self-hosted [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) stack that pools your AI accounts behind one OpenAI- and Anthropic-compatible endpoint.

**[Get started →](https://docs.hara.sh/setup)** · [Documentation](https://docs.hara.sh/) · [Website](https://hara.sh/)

The server needs Docker with Compose v2. Go tools run inside containers; Make, Go, 1Password, Tailscale and portless are optional. With Make, `make up` builds the `bin/hara` host helper and starts the stack.

- [Connect providers](https://docs.hara.sh/providers)
- [Configure Claude Code, Codex, T3 Code and other clients](https://docs.hara.sh/clients)
- [Operate the proxy](https://docs.hara.sh/operations)
- [Fix a problem](https://docs.hara.sh/troubleshooting)

## Development

```bash
make web install
make web dev
make format-all
make code check
```

See [website development](https://docs.hara.sh/development) for requirements, docs editing and deployment. Keep credentials and runtime state outside Git; read [privacy and publication](https://docs.hara.sh/privacy) before publishing a fork.
