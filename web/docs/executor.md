---
description: Connect Executor MCP to T3 Code's Claude, Codex, Cursor and Antigravity providers, verify tool calls, and add the connector to Linear Agent.
---

# Connect Executor

Executor supplies MCP tools independently of Hara's model API. A working inference endpoint does not configure tool access. T3 Code reads each provider's own client configuration.

## Prepare T3 Code

Set up the Hara providers with [make t3](./clients#t3-code), and install and sign in to Cursor through T3's provider settings. Set up Antigravity too when you want that provider connected. Executor must already be deployed, with credentials that its MCP endpoint accepts.

Create a private header file outside the checkout, with one `Name: value` line per HTTP header from your secret manager. Header names are configurable; for example, a bearer-authenticated endpoint uses:

```text
Authorization: Bearer YOUR_TOKEN
```

Restrict that file to its owner (`chmod 600`). Keep shell tracing off. Pass only its path and the HTTPS MCP endpoint to the helper:

```bash
EXECUTOR_URL='https://executor.example.com/mcp' \
EXECUTOR_NAME='executor' \
EXECUTOR_HEADERS_FILE="$HOME/.private/executor.headers" \
make t3 executor
```

Replace the example endpoint with your Executor URL, including `/mcp`. `EXECUTOR_NAME` selects the same MCP entry name across providers and defaults to `executor`; names may contain letters, digits, underscores and hyphens. Set it to an existing entry's name when updating that connection. Different names keep separate credential files and preserve other connections.

Any valid HTTP header name is accepted, including `Authorization`, `X-Api-Key`, and Cloudflare Access headers. The file must contain at least one non-empty header; comments and blank lines are allowed. Duplicate names are rejected case-insensitively, along with invalid names and control characters in values. For a Cloudflare-protected deployment, supply its `CF-Access-Client-Id` and `CF-Access-Client-Secret` headers in this file.

The command validates the direct provider configurations, backs up previous bytes outside Git, and writes owner-only configuration files. Unrelated JSON values, MCP servers and Codex inference settings are preserved. Codex TOML is reserialized, so comments and formatting are retained in the backup rather than the updated file.

| Provider    | Configuration                                                       | Connection                                  |
| ----------- | ------------------------------------------------------------------- | ------------------------------------------- |
| Claude      | `~/.claude-hara/.claude.json`                                       | Named HTTP connection with supplied headers |
| Codex       | `~/.codex-t3-hara/config.toml`                                      | Named HTTP connection with supplied headers |
| Cursor      | `~/.cursor/mcp.json`                                                | Named HTTP connection with supplied headers |
| Antigravity | T3's selected profile under `~/.t3/userdata/providers/antigravity/` | Named local stdio bridge to HTTP            |

Cursor's selected entry replaces a stale localhost proxy if one exists. Antigravity's direct remote configuration did not expose the tools in the tested T3 runtime; the stdio bridge did. The bridge reads credentials from a private header file, keeping them out of process arguments.

Antigravity setup is optional. Missing or ambiguous profiles, unavailable Node.js/npm, invalid Antigravity configuration, and bridge installation failures produce a `Skipping Antigravity` report while the direct providers continue. Node.js/npm and the pinned `mcp-remote@0.14.3` installation are needed only for Antigravity.

Overrides for another installation:

| Variable              | Default or purpose                                                                           |
| --------------------- | -------------------------------------------------------------------------------------------- |
| `EXECUTOR_NAME`       | `executor`; selects the MCP entry name for every provider                                    |
| `T3_CODEX_HOME`       | `~/.codex-t3-hara`                                                                           |
| `T3_CLAUDE_HOME`      | `~/.claude-hara`                                                                             |
| `T3_CURSOR_HOME`      | `~/.cursor`                                                                                  |
| `T3_HOME`             | `~/.t3`                                                                                      |
| `T3_ANTIGRAVITY_HOME` | Exact existing T3 profile; otherwise discover the only initialized profile                   |
| `T3_EXECUTOR_DIR`     | `~/.codex/tools/t3-executor`, holding the bridge, `<name>.headers` files, caches and backups |

Use absolute paths for overrides. When multiple Antigravity profiles exist, choose one explicitly. Start fresh T3 chats after setup so each provider loads its configuration. Leave T3's live settings and account tokens unchanged.

`make t3 setup` rewrites the base Codex configuration. If you run it again later, rerun `make t3 executor` afterwards to restore that provider's MCP entry.

## Verify inside T3

CLI connection checks are useful but do not prove a model can call tools inside T3:

```bash
CLAUDE_CONFIG_DIR="$HOME/.claude-hara" claude mcp list
cursor-agent mcp list
cursor-agent mcp list-tools executor
```

If Cursor requests approval, enable the server with `cursor-agent mcp enable executor`. Substitute your chosen name in these commands when `EXECUTOR_NAME` differs from the default. Choose **Grok** in T3's **Cursor** provider for the E2E test. Cursor's Opus and Gemini usage limits can stop a turn before any tool call; that is a model quota failure even when the connector reports ready. Do not raise spending limits to run this check.

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

In Linear's **Settings → Agent personalization → MCP connectors**, choose **Custom URL**, enter your Executor HTTPS endpoint ending in `/mcp`, and open **Advanced**. Add the authentication header names and values required by your endpoint, then add the connector. For Cloudflare Access, use `CF-Access-Client-Id` and `CF-Access-Client-Secret` from your secret manager.

Linear stores the credentials and can use Executor's tools, so authorise that access explicitly. Successful discovery lists seven tools: `execute`, `skills`, `resume`, `create-artifact`, `edit-artifact`, `list-artifacts`, and `show-artifact`. Run the same read-only test in Linear Agent and inspect its activity trace. The tested Linear setup completed both calls successfully.

## Computer and browser access

Executor connectivity does not install local desktop control. T3's shared Playwright-backed browser and simulator tools are separate integrations. Desktop automation remains a proposed follow-up: install a local desktop MCP such as Peekaboo, configure it per provider, grant the required macOS permissions, and verify harmless app interactions. Browser tests should reuse one context and window, with explicit artifact paths under `~/.codex/cache/codex/browser-artifacts`.
