# cli-proxy-api — common tasks. Run `make` for the list.
# Secrets are read from 1Password at run time and never echoed.

SHELL      := /bin/bash
# portless serves the local proxy here (scripts/local.sh); URL=http://localhost:8317 skips it.
URL        ?= https://hara.local
OP_ACCOUNT ?= my.1password.com
MODEL      ?= claude-haiku-4-5-20251001

export OP_ACCOUNT

# Cached in the macOS Keychain for 30 days (scripts/hara-key); 1Password is asked once a month.
HARA_KEY = $(CURDIR)/scripts/hara-key
API_KEY  = $$($(HARA_KEY) claude-api-key)
MGMT_KEY = $$($(HARA_KEY) management-password)
# hara.local uses the portless certificate authority; Node-based clients need it named.
PORTLESS_CA = $(HOME)/.portless/ca.pem

.DEFAULT_GOAL := help
.PHONY: help local local-status local-logs local-down panel claude alias keys-refresh codex-backup claude-backup \
        health models accounts quota quota-test smoke ws-smoke ws-smoke-test logs web-install web-dev web-test web-coverage web-deploy format-all lint complexity

help: ## Show this help
	@grep -hE '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

## --- Local proxy (Docker + Tailscale + portless) ---------------------------
# Runs the proxy on this computer (local/compose.yaml): https://hara.local on your network
# (portless in LAN mode) and https://cliproxy.<tailnet>.ts.net on your tailnet.

local: ## Start the proxy: https://hara.local + your tailnet (config from 1Password)
	./scripts/local.sh up

local-status: ## Print the local, network and tailnet addresses and the Tailscale login state
	@./scripts/local.sh status

local-logs: ## Follow the proxy and Tailscale logs (Ctrl-C to stop)
	./scripts/local.sh logs

local-down: ## Stop the proxy and remove hara.local (logins and Tailscale identity kept)
	./scripts/local.sh down

panel: ## Rebuild panel/management.html (upstream panel + panel/ledger.patch; needs bun)
	./scripts/build-panel.sh

## --- Clients ---------------------------------------------------------------
# Codex is not routed through the proxy: it uses its own ChatGPT login (~/.codex/config.toml).

claude: ## Run Claude Code through the proxy (pass flags with ARGS="...")
	@NODE_EXTRA_CA_CERTS=$(PORTLESS_CA) ANTHROPIC_BASE_URL=$(URL) ANTHROPIC_AUTH_TOKEN="$(API_KEY)" claude $(ARGS)

alias: ## Print the claude-hara shell alias for ~/.zshrc
	@echo "alias claude-hara='NODE_EXTRA_CA_CERTS=\$$HOME/.portless/ca.pem ANTHROPIC_BASE_URL=$(URL) ANTHROPIC_AUTH_TOKEN=\"\$$($(HARA_KEY))\" claude'"

keys-refresh: ## Re-fetch the cached keys from 1Password now (e.g. after rotating them)
	@for f in claude-api-key management-password; do $(HARA_KEY) --refresh $$f >/dev/null; done && echo "keys refreshed"

codex-backup: ## Copy ~/.codex/config.toml and openai.config.toml into codex/ (then commit)
	./scripts/backup-codex.sh

claude-backup: ## Copy ~/.claude settings into claude/ (Omniyat auto-mode section redacted), then commit
	./scripts/backup-claude.sh

## --- Operations ------------------------------------------------------------

health: ## Check the proxy health endpoint
	@curl -fsS --max-time 10 $(URL)/healthz && echo

models: ## List models available through the proxy
	@curl -fsS $(URL)/v1/models -H "Authorization: Bearer $(API_KEY)" \
	  | python3 -c 'import json,sys; [print(m["id"]) for m in json.load(sys.stdin)["data"]]'

accounts: ## List connected provider accounts
	@curl -fsS $(URL)/v0/management/auth-files -H "Authorization: Bearer $(MGMT_KEY)" \
	  | python3 -c 'import json,sys; [print("%-8s %-8s %s" % (f.get("provider"), f.get("status"), f.get("name"))) for f in json.load(sys.stdin).get("files",[])]'

quota: ## Usage per account: % used, reset times, weekly capacity expiring unused within 24h
	@cd scripts/quota && MGMT_KEY="$(MGMT_KEY)" URL=$(URL) go run .

quota-test: ## Run the scripts/quota Go tests (vet first)
	cd scripts/quota && go vet ./... && go test ./...

smoke: ## Send a test message through the proxy (MODEL=... to override)
	@curl -fsS $(URL)/v1/messages -H "x-api-key: $(API_KEY)" -H 'anthropic-version: 2023-06-01' \
	  -H 'content-type: application/json' \
	  -d '{"model":"$(MODEL)","max_tokens":16,"messages":[{"role":"user","content":"Reply with exactly: pong"}]}' \
	  | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["content"][0]["text"] if d.get("content") else d)'

# The proxy answers WebSocket pings itself, so ws-smoke needs no provider account. TS_URL adds the
# tailnet address (from `make local-status`); IDLE=2m holds each socket idle, then pings again.
ws-smoke: ## WebSocket checks per address: handshake, ping/pong, close, 401 without a key (IDLE, TS_URL)
	@cd scripts/wssmoke && API_KEY="$(API_KEY)" go run . -idle $${IDLE:-0s} http://localhost:8317 $(URL) $(TS_URL)

ws-smoke-test: ## Run the scripts/wssmoke Go tests (vet first)
	cd scripts/wssmoke && go vet ./... && go test ./...

logs: ## Show the last LINES lines of the server log (default 200)
	@curl -fsS "$(URL)/v0/management/logs?limit=$${LINES:-200}" -H "Authorization: Bearer $(MGMT_KEY)" \
	  | python3 -c 'import json,sys; print("\n".join(json.load(sys.stdin).get("lines",[])))'

## --- Landing page (web/: Vue + shadcn-vue + Tailwind CSS v4, served on www.hara.sh) ---

web-install: ## Install the landing page dependencies
	cd web && npm install

web-dev: ## Run the landing page locally (Vite)
	cd web && npm run dev

web-test: ## Run the landing page tests
	cd web && npm test

web-coverage: ## Landing page tests with coverage (fails under 100%)
	cd web && npm run coverage

web-deploy: ## Type-check, test at 100% and deploy the landing page to www.hara.sh
	cd web && npm run deploy

# Go is formatted from its module directory, so fmtkit also runs go vet there.
format-all: ## Format every TS/JS file and the Go in scripts/quota and scripts/wssmoke with fmtkit
	fmtkit format-all --ts
	cd scripts/quota && fmtkit format-all --go
	cd scripts/wssmoke && fmtkit format-all --go

lint: ## Lint the landing page TS with fmtkit (oxlint), writing nothing
	fmtkit lint web/src web/worker web/cloudflare.config.ts

complexity: ## Report TS and Go functions over fmtkit's complexity limits
	fmtkit complexity --ts web/src web/worker
	cd scripts/quota && fmtkit complexity --go .
	cd scripts/wssmoke && fmtkit complexity --go .
