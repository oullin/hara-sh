---
description: Set up Hara with Docker on Linux, macOS or Windows. Generate private keys, connect a provider account and make your first API request.
---

# Set up the project

You need [Docker with Compose v2](https://docs.docker.com/compose/install/). On Windows, use Linux containers. Go and 1Password are optional.

Already running Hara with 1Password? [Import your existing keys](./configuration#migrate-from-1password) before starting.

## 1. Get the project

```bash
git clone https://github.com/oullin/hara-sh.git
cd hara-sh
```

Or download the repository as a ZIP and open a terminal in the extracted directory.

## 2. Create a private state directory

::: code-group

```bash [macOS / Linux]
mkdir -p "$HOME/.hara-sh"
chmod 700 "$HOME/.hara-sh"
```

```powershell [Windows]
$env:LOCAL_DIR = Join-Path $env:USERPROFILE '.hara-sh'
New-Item -ItemType Directory -Force $env:LOCAL_DIR | Out-Null
```

:::

Keep this directory outside the checkout. On Windows, restrict its file permissions and set `LOCAL_DIR` in each new shell. It stores your keys, provider logins, and logs.

## 3. Start Hara

```bash
docker compose -f local/compose.yaml up -d --build --force-recreate
docker compose -f local/compose.yaml run --rm --no-deps -T tools status
```

The first build takes a few minutes. Startup generates two keys and keeps them on subsequent runs. The `init` container exits after setup; the proxy and quota service stay running.

Expected result: health, panel, and both keys pass. **No provider accounts** is normal until you connect one.

## 4. Connect an account and client

Retrieve the management password, then open `http://localhost:8317/management.html` on the Docker host and sign in:

```bash
docker compose -f local/compose.yaml run --rm --no-deps -T tools key management-password
```

Follow [Connect providers](./providers), then retrieve the client API key:

```bash
docker compose -f local/compose.yaml run --rm --no-deps -T tools key claude-api-key
```

These commands display secrets. Use a private terminal and keep the values out of Git and screenshots.

Configure your client with:

| Setting                    | Value                               |
| -------------------------- | ----------------------------------- |
| OpenAI-compatible base URL | `http://localhost:8317/v1`          |
| Claude Code base URL       | `http://localhost:8317`             |
| API key                    | The client key above                |
| Model                      | A model listed by the command below |

```bash
docker compose -f local/compose.yaml run --rm --no-deps -T tools ops models
```

Continue with [client examples](./clients). For access from your phone or another computer, [enable Tailscale](./networking#tailscale).

## Everyday commands

```bash
# Stop; keep keys and provider logins.
docker compose -f local/compose.yaml --profile tailscale down

# Restart or apply config changes.
docker compose -f local/compose.yaml up -d --build --force-recreate

# Inspect startup failures.
docker compose -f local/compose.yaml logs --tail=100 init proxy quota
```

Using Make? `make up`, `make status`, and `make down` are shortcuts that run these commands through the [host helper](./operations#make-and-the-host-helper). With [portless](./networking#optional-lan-access) installed, `make up` also serves Hara at `https://hara.local`. See [Operations](./operations) for rotation and request checks, or [Troubleshooting](./troubleshooting) if a check fails.
