---
description: Connect Executor MCP to T3 Code's Claude, Codex, Cursor and Antigravity providers, verify tool calls, and add the connector to Linear Agent.
---

# Connect Executor

Executor supplies MCP tools independently of Hara's model API. A working inference endpoint does not configure tool access. T3 Code reads each provider's own client configuration.

## Prepare T3 Code

Set up the Hara providers with [make t3](./clients#t3-code), and install and sign in to Cursor and Antigravity through T3's provider settings. Executor must already be deployed with a Cloudflare Access service token authorized for its MCP endpoint.

Create a private header file outside the checkout, with these two lines filled from your secret manager:

```text
CF-Access-Client-Id: YOUR_CLIENT_ID
CF-Access-Client-Secret: YOUR_CLIENT_SECRET
```

Restrict that file to its owner (`chmod 600`). Keep shell tracing off. Pass only its path and the HTTPS MCP endpoint to the helper:

```bash
EXECUTOR_URL='https://executor.example.com/mcp' \
EXECUTOR_HEADERS_FILE="$HOME/.private/executor.headers" \
make t3 executor
```

Replace the example endpoint with your Executor URL, including `/mcp`. This command needs Node.js and npm and installs the pinned `mcp-remote@0.14.3` bridge. It validates all client files first, backs up previous bytes outside Git, and writes owner-only configuration files. Unrelated JSON values, MCP servers and Codex inference settings are preserved. Codex TOML is reserialized, so comments and formatting are retained in the backup rather than the updated file.

| Provider    | Configuration                                                       | Connection                                       |
| ----------- | ------------------------------------------------------------------- | ------------------------------------------------ |
| Claude      | `~/.claude-hara/.claude.json`                                       | `collections_executor`, HTTP with Access headers |
| Codex       | `~/.codex-t3-hara/config.toml`                                      | `collections_executor`, HTTP with Access headers |
| Cursor      | `~/.cursor/mcp.json`                                                | `executor`, HTTP with Access headers             |
| Antigravity | T3's selected profile under `~/.t3/userdata/providers/antigravity/` | `executor`, local stdio bridge to HTTP           |

Cursor's `executor` entry replaces a stale localhost proxy if one exists. Antigravity's direct remote configuration did not expose the tools in the tested T3 runtime; the stdio bridge did. The bridge reads credentials from a private header file, keeping them out of process arguments.

Overrides for another installation:

| Variable              | Default or purpose                                                                |
| --------------------- | --------------------------------------------------------------------------------- |
| `T3_CODEX_HOME`       | `~/.codex-t3-hara`                                                                |
| `T3_CLAUDE_HOME`      | `~/.claude-hara`                                                                  |
| `T3_CURSOR_HOME`      | `~/.cursor`                                                                       |
| `T3_HOME`             | `~/.t3`                                                                           |
| `T3_ANTIGRAVITY_HOME` | Exact existing T3 profile; otherwise discover the only initialized profile        |
| `T3_EXECUTOR_DIR`     | `~/.codex/tools/t3-executor`, holding the bridge, header file, caches and backups |

Use absolute paths for overrides. When multiple Antigravity profiles exist, choose one explicitly. Start fresh T3 chats after setup so each provider loads its configuration. Leave T3's live settings and account tokens unchanged.

`make t3 setup` rewrites the base Codex configuration. If you run it again later, rerun `make t3 executor` afterwards to restore that provider's MCP entry.

## Verify inside T3

CLI connection checks are useful but do not prove a model can call tools inside T3:

```bash
CLAUDE_CONFIG_DIR="$HOME/.claude-hara" claude mcp list
cursor-agent mcp list
cursor-agent mcp list-tools executor
```

If Cursor requests approval, enable the server with `cursor-agent mcp enable executor`. Choose **Grok** in T3's **Cursor** provider for the E2E test. Cursor's Opus and Gemini usage limits can stop a turn before any tool call; that is a model quota failure even when the connector reports ready. Do not raise spending limits to run this check.

In a new T3 chat, select each provider and send:

```text
Run a read-only Executor MCP test. Call skills with name "execute", then
execute this code:

const r = await tools.executor.coreTools.connections.list({});
if (!r.ok) return r;
return {check: "executor-e2e", connectionCount: r.data.connections.length};

Report the actual result. Wait for MCP startup if needed. If the tools are
absent or disabled, stop and report that blocker. Do not substitute shell
or HTTP commands or modify files.
```

Expand the activity trace and confirm both `skills` and `execute` ran. Passing means the expected marker came back through Executor. A zero connection count is valid: upstream services still need their own saved connections before integration-specific calls can work. Claude may connect asynchronously after the first turn starts; wait for MCP startup before treating missing initial tools as a failure.

The live setup was verified on T3 Code nightly `0.0.46-nightly.20261011.2994` with Antigravity ACP `1.3.0`: Claude, Codex, Cursor with Grok, and Antigravity each completed the two calls and returned their test marker. This records that setup's result; rerun the check after installation or upgrades.

## Connect Linear Agent

In Linear's **Settings → Agent personalization → MCP connectors**, choose **Custom URL**, enter your Executor HTTPS endpoint ending in `/mcp`, and open **Advanced**. Add the `CF-Access-Client-Id` and `CF-Access-Client-Secret` authentication headers from your secret manager, then add the connector.

Linear stores the credentials and can use Executor's tools, so authorise that access explicitly. Successful discovery lists seven tools: `execute`, `skills`, `resume`, `create-artifact`, `edit-artifact`, `list-artifacts`, and `show-artifact`. Run the same read-only test in Linear Agent and inspect its activity trace. The tested Linear setup completed both calls successfully.

## Computer and browser access

Executor connectivity does not install local desktop control. T3's shared Playwright-backed browser and simulator tools are separate integrations. Desktop automation remains a proposed follow-up: install a local desktop MCP such as Peekaboo, configure it per provider, grant the required macOS permissions, and verify harmless app interactions. Browser tests should reuse one context and window, with explicit artifact paths under `~/.codex/cache/codex/browser-artifacts`.
