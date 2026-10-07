---
description: Configure Hara ports, private state, provider settings and routing. Import existing keys from 1Password and upgrade your Docker deployment.
---

# Configuration

Edit `config.yaml`, then recreate the services:

```bash
docker compose -f local/compose.yaml up -d --build --force-recreate
```

The initialiser renders `$LOCAL_DIR/proxy/config.yaml`, replacing `__API_KEY__` and `__MANAGEMENT_PASSWORD_BCRYPT__` with your private key and password hash. Panel-only config edits are overwritten on startup.

## Settings

| Variable               | Default                  | Use                                      |
| ---------------------- | ------------------------ | ---------------------------------------- |
| `LOCAL_DIR`            | `$HOME/.cli-proxy-api`   | Private state; set explicitly on Windows |
| `HARA_PORT`            | `8317`                   | Published host port                      |
| `HARA_CONFIG_TEMPLATE` | Repository `config.yaml` | Absolute path to a private template      |
| `TZ`                   | `UTC`                    | Container timezone                       |
| `COMPOSE_PROFILES`     | unset                    | `tailscale` enables private HTTPS        |
| `TS_HOSTNAME`          | `cliproxy`               | Tailscale device name                    |
| `TS_AUTHKEY`           | unset                    | Optional private enrolment key           |
| `TOOLS_URL`            | `http://proxy:8317`      | Target for Compose tools                 |
| `URL`                  | `http://localhost:8317`  | Target for Make operations and Claude    |

Set variables in your shell or a private Compose `.env` file. Keep keys out of it. Use absolute paths outside Git for state and private templates. Changing the host port requires updating client URLs; it does not change container port 8317 or the installed Codex profile.

## Private provider settings

Copy `config.yaml` to a private path outside the checkout. Add provider configuration there and retain both quoted credential placeholders. Set `HARA_CONFIG_TEMPLATE` to that absolute path before startup.

Use the [upstream configuration reference](https://github.com/router-for-me/CLIProxyAPI/blob/main/config.example.yaml) for the version pinned in `local/compose.yaml`. Back up the private template with your state.

## Migrate from 1Password

Keep the existing state directory. Import the original keys before starting the new workflow:

```bash
export OP_ACCOUNT='my.1password.com'
export OP_VAULT='Private'
export OP_ITEM_NAME='cli-proxy-api'
./scripts/local.sh import-op
make up
```

Use your own account, vault, and item. The item needs `claude-api-key` and `management-password` fields. The helper requires Bash and a signed-in `op` CLI, reads without printing, and imports into private files. Subsequent startup needs no `op`.

For another secret manager, export exactly two lines—client key first, management password second—to a private file:

```bash
docker compose -f local/compose.yaml build init
docker compose -f local/compose.yaml run --rm --no-deps -T init import < /path/to/private/credential-export
```

Each value must be 20–72 UTF-8 bytes without embedded line breaks. Import replaces both credentials. Remove the export afterwards and recreate services. Initialisation refuses to replace keys from an older installation automatically.

To read directly from 1Password in a host client, set `HARA_SECRET_PROVIDER=op` with the same `OP_*` settings. Sync changes before restarting the server. Optional Bash settings live in `$LOCAL_DIR/credentials.env`; Compose does not source that file.

## Routing defaults

- Round-robin for new sessions; one-hour affinity for conversations and subagents.
- Persistent, model-specific cooldowns; at most five seconds between retries when all accounts are cooling down.
- `service_tier: priority` for Codex models. Remove that payload rule for standard-tier usage.
- File logs capped at 512 MB; debug and request logging disabled.

Quota priorities are updated hourly. Plugin binaries installed inside the container are lost on recreation unless you configure persistence.

An existing session's affinity takes precedence over quota priority. Priority changes steer new sessions and failovers; they do not move conversations with a warm cache. A failover can require rebuilding that account's prompt cache.

`model-level-cooling` keeps a limit on one model from excluding every model on the account. Cooldowns survive restarts and honour provider reset times when available. First-token buffering is disabled through `stream-bootstrap-buffering`; enabling it trades initial latency for transparent retries.

## Private state layout

| Directory under `LOCAL_DIR` | Contents                                                                 |
| --------------------------- | ------------------------------------------------------------------------ |
| `secrets/`                  | Client key and management password; tools mount them read-only           |
| `proxy/`                    | Rendered config, provider logins, persistent cooldowns and rotating logs |
| `tailscale/`                | Optional device identity                                                 |
| `backups/`                  | Host client settings saved by the optional Bash helpers                  |

Logs rotate in 10 MB files with a 512 MB total cap. Protect the whole state directory and include your outside-Git provider template in private backups.

## Upgrade

Back up private state, update the proxy image tag in `local/compose.yaml`, and recreate services. Run [status and smoke checks](./operations#check-a-request) afterwards.
