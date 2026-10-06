# cli-proxy-api — common tasks: make up | down | status | logs, or make <area> [action]. Run `make` for the list.
# Secrets are read from 1Password at run time and never echoed.

SHELL      := /bin/bash
# portless serves the local proxy here (scripts/local.sh); URL=http://localhost:8317 skips it.
URL        ?= https://hara.local
OP_ACCOUNT ?= my.1password.com
MODEL      ?= claude-haiku-4-5-20251001
# Codex model for `make ops bench`.
CODEX_MODEL ?= gpt-5.5

export OP_ACCOUNT

# Cached in the macOS Keychain for 30 days (scripts/hara-key); 1Password is asked once a month.
HARA_KEY = $(CURDIR)/scripts/hara-key
API_KEY  = $$($(HARA_KEY) claude-api-key)
MGMT_KEY = $$($(HARA_KEY) management-password)
# hara.local uses the portless certificate authority; Node-based clients need it named.
PORTLESS_CA = $(HOME)/.portless/ca.pem

# `make ops quota`, `make logs proxy`: the words after an area (or logs) are its action or argument,
# not targets, so they get an empty rule here. Each area runs <area>/<action>, which also works alone.
AREAS := claude codex ops web code
ifneq ($(filter $(firstword $(MAKECMDGOALS)),$(AREAS) logs),)
ACTION := $(word 2,$(MAKECMDGOALS))
WORDS  := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
.PHONY: $(WORDS)
$(WORDS):
	@:
endif

.DEFAULT_GOAL := help
.PHONY: help up down status logs $(AREAS) \
        claude/run claude/alias claude/backup codex/run codex/profile codex/smoke codex/backup \
        ops/health ops/models ops/accounts ops/quota ops/logs ops/smoke ops/ws-smoke ops/bench ops/keys \
        web/dev web/install web/test web/coverage web/deploy code/format code/lint code/complexity code/test code/panel

help:
	@echo "make up | down | status | logs [service]    the proxy on this computer"
	@echo "make <area> [action]                        the first action is the default"
	@echo
	@grep -hE '^[a-z]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{n = split($$2, p, " :: "); if (n == 1) printf "  \033[36m%-7s\033[0m %s\n", $$1, p[1]; else printf "\n  \033[36m%-7s\033[0m %s\n          %s\n", $$1, p[1], p[2]}'

## --- The proxy on this computer (Docker + Tailscale + portless) ------------------------------
# https://hara.local on your network (portless in LAN mode) and https://cliproxy.<tailnet>.ts.net
# on your tailnet (local/compose.yaml).

up: ## Start the proxy, or apply config.yaml changes (config from 1Password); prints the addresses
	./scripts/local.sh up

down: ## Stop the proxy and remove hara.local (logins and Tailscale identity kept)
	./scripts/local.sh down

status: ## Print the local, network and tailnet addresses and the Tailscale login state
	@./scripts/local.sh status

logs: ## Follow the container logs: all, or one of proxy, tailscale, quota (Ctrl-C to stop)
	./scripts/local.sh logs $(ACTION)

## --- claude, codex: the clients -----------------------------------------------------------
# Claude Code speaks HTTP (with SSE streaming) to the proxy. Codex goes through it with the `proxy`
# profile (codex/proxy.config.toml) over the Responses WebSocket; plain `codex` keeps its ChatGPT login.

claude: claude/$(or $(ACTION),run) ## run | alias | backup :: Run Claude Code through the proxy (flags in ARGS="..."), print the claude-hara alias for ~/.zshrc, back up ~/.claude settings into claude/ (Omniyat auto-mode section redacted)

claude/run:
	@NODE_EXTRA_CA_CERTS=$(PORTLESS_CA) ANTHROPIC_BASE_URL=$(URL) ANTHROPIC_AUTH_TOKEN="$(API_KEY)" claude $(ARGS)

claude/alias:
	@echo "alias claude-hara='NODE_EXTRA_CA_CERTS=\$$HOME/.portless/ca.pem ANTHROPIC_BASE_URL=$(URL) ANTHROPIC_AUTH_TOKEN=\"\$$($(HARA_KEY))\" claude'"

claude/backup:
	./scripts/backup-claude.sh

codex: codex/$(or $(ACTION),run) ## run | profile | smoke | backup :: Run Codex through the proxy over WebSockets (flags in ARGS="..."), install the proxy profile, one turn that fails on the HTTP fallback, back up ~/.codex configs into codex/

codex/run:
	@codex --profile proxy $(ARGS)

codex/profile:
	install -m 644 codex/proxy.config.toml $(HOME)/.codex/proxy.config.toml

codex/smoke:
	./scripts/codex-smoke.sh

codex/backup:
	./scripts/backup-codex.sh

## --- ops: requests to the running proxy (URL) ----------------------------------------------

ops: ops/$(or $(ACTION),health) ## health | models | accounts | quota | logs | smoke | ws-smoke | bench | keys :: Health, models, accounts, usage per account, the last LINES server log lines (default 200), one Claude request (MODEL), WebSocket checks (IDLE, TS_URL), first-token and cache timing (N, MODEL, CODEX_MODEL), re-fetch the cached keys from 1Password

ops/health:
	@curl -fsS --max-time 10 $(URL)/healthz && echo

ops/models:
	@curl -fsS $(URL)/v1/models -H "Authorization: Bearer $(API_KEY)" \
	  | python3 -c 'import json,sys; [print(m["id"]) for m in json.load(sys.stdin)["data"]]'

ops/accounts:
	@curl -fsS $(URL)/v0/management/auth-files -H "Authorization: Bearer $(MGMT_KEY)" \
	  | python3 -c 'import json,sys; [print("%-8s %-8s %s" % (f.get("provider"), f.get("status"), f.get("name"))) for f in json.load(sys.stdin).get("files",[])]'

# % used, reset times, weekly capacity expiring unused within 24h.
ops/quota:
	@cd scripts/quota && MGMT_KEY="$(MGMT_KEY)" URL=$(URL) go run .

ops/logs:
	@curl -fsS "$(URL)/v0/management/logs?limit=$${LINES:-200}" -H "Authorization: Bearer $(MGMT_KEY)" \
	  | python3 -c 'import json,sys; print("\n".join(json.load(sys.stdin).get("lines",[])))'

ops/smoke:
	@curl -fsS $(URL)/v1/messages -H "x-api-key: $(API_KEY)" -H 'anthropic-version: 2023-06-01' \
	  -H 'content-type: application/json' \
	  -d '{"model":"$(MODEL)","max_tokens":16,"messages":[{"role":"user","content":"Reply with exactly: pong"}]}' \
	  | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["content"][0]["text"] if d.get("content") else d)'

# Handshake, ping/pong, close and 401 without a key, per address. The proxy answers pings itself,
# so it needs no provider account. TS_URL adds the tailnet address (from `make status`);
# IDLE=2m holds each socket idle, then pings again.
ops/ws-smoke:
	@cd scripts/wssmoke && API_KEY="$(API_KEY)" go run . -idle $${IDLE:-0s} http://localhost:8317 $(URL) $(TS_URL)

# Real requests: N per API per address (about 5k prompt tokens each, mostly cached). The first of
# each series is cold, the rest should hit the prompt cache. CODEX_MODEL= skips Codex, MODEL= Claude.
ops/bench:
	@cd scripts/bench && API_KEY="$(API_KEY)" go run . -n $${N:-5} -claude-model "$(MODEL)" -codex-model "$(CODEX_MODEL)" http://localhost:8317 $(URL)

# After rotating a key in 1Password; `make up` then applies it to the proxy.
ops/keys:
	@for f in claude-api-key management-password; do $(HARA_KEY) --refresh $$f >/dev/null; done && echo "keys refreshed"

## --- web: the landing page (web/: Vue + shadcn-vue + Tailwind CSS v4, served on hara.sh) ---

web: web/$(or $(ACTION),dev) ## dev | install | test | coverage | deploy :: Run the landing page locally (Vite), install its dependencies, test it, test with coverage (fails under 100%), type-check, test and deploy it to hara.sh

web/dev:
	cd web && npm run dev

web/install:
	cd web && npm install

web/test:
	cd web && npm test

web/coverage:
	cd web && npm run coverage

web/deploy:
	cd web && npm run deploy

## --- code: formatting, linting, the Go tests and the panel build ---------------------------

GO_MODULES := scripts/quota scripts/wssmoke scripts/bench

code: code/$(or $(ACTION),format) ## format | lint | complexity | test | panel :: Format the TS/JS and the Go with fmtkit, lint the landing page TS (writes nothing), report functions over fmtkit's complexity limits, vet and test the Go in scripts/, rebuild panel/management.html (needs bun)

# Go is formatted from its module directory, so fmtkit also runs go vet there.
code/format:
	fmtkit format-all --ts
	@for m in $(GO_MODULES); do (cd $$m && fmtkit format-all --go) || exit 1; done

code/lint:
	fmtkit lint web/src web/worker web/cloudflare.config.ts

code/complexity:
	fmtkit complexity --ts web/src web/worker
	@for m in $(GO_MODULES); do (cd $$m && fmtkit complexity --go .) || exit 1; done

code/test:
	@for m in $(GO_MODULES); do echo "$$m"; (cd $$m && go vet ./... && go test ./...) || exit 1; done

# Upstream panel + panel/ledger.patch; commit it, then `make up`.
code/panel:
	./scripts/build-panel.sh
