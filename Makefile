# cli-proxy-api — make up | down | status | logs [service], or make <area> [action]. Run `make` for the list.
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

# Each area lists its actions, the first being the default, and one line of help.
AREAS    := claude codex ops web code
SERVICES := proxy tailscale quota

claude_ACTIONS := run alias backup
claude_HELP    := Claude Code through the proxy (flags in ARGS="..."); alias prints claude-hara for ~/.zshrc; backup copies ~/.claude into claude/
codex_ACTIONS  := run profile smoke backup
codex_HELP     := Codex through the proxy over WebSockets (flags in ARGS="..."); profile installs it; smoke fails on the HTTP fallback; backup copies ~/.codex into codex/
ops_ACTIONS    := quota accounts models logs smoke ws-smoke bench keys
ops_HELP       := Usage per account, accounts, models, server log (LINES), one Claude request (MODEL), WebSockets (IDLE, TS_URL), first-token timing (N, CODEX_MODEL), re-fetch the cached keys
web_ACTIONS    := dev install test coverage deploy
web_HELP       := The landing page on hara.sh: Vite dev server, dependencies, tests, coverage (fails under 100%), type-check, test and deploy
code_ACTIONS   := check format lint test complexity panel
code_HELP      := check runs what CI runs (lint, Go tests, web coverage); format and lint with fmtkit and shellcheck; Go tests; complexity limits; rebuild the panel (needs bun)

# `make ops quota`, `make logs proxy`: the words after an area (or logs) name its action or service,
# not targets, so they get an empty rule here; anything else fails with the choices.
FIRST := $(firstword $(MAKECMDGOALS))
WORDS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))

ifneq ($(filter $(FIRST),$(AREAS)),)
ACTION := $(or $(firstword $(WORDS)),$(firstword $($(FIRST)_ACTIONS)))
ifeq ($(filter $(ACTION),$($(FIRST)_ACTIONS)),)
$(error make $(FIRST): no action '$(ACTION)'; choose one of: $($(FIRST)_ACTIONS))
endif
endif

ifeq ($(FIRST),logs)
SERVICE := $(firstword $(WORDS))
ifneq ($(filter-out $(SERVICES),$(SERVICE)),)
$(error make logs: no service '$(SERVICE)'; choose one of: $(SERVICES))
endif
endif

ifneq ($(WORDS),)
.PHONY: $(WORDS)
$(WORDS):
	@:
endif

# Each area runs <area>/<action>; `make ops/quota` works too.
$(foreach a,$(AREAS),$(eval $(a): $(a)/$(if $(filter $(a),$(FIRST)),$(ACTION),$(firstword $($(a)_ACTIONS)))))

.DEFAULT_GOAL := help
.PHONY: help up down status logs $(AREAS) $(foreach a,$(AREAS),$(addprefix $(a)/,$($(a)_ACTIONS)))

space := $(subst ,, )
help:
	@printf '%s\n' \
	  'make up | down | status | logs [service]   the proxy on this computer' \
	  'make <area> [action]                       the first action is the default' \
	  '' \
	  '  up      Start the proxy, or apply config.yaml changes (config from 1Password), then run status' \
	  '  down    Stop the proxy and remove hara.local (logins and Tailscale identity kept)' \
	  '  status  Check every link, from the containers to the accounts, and print the addresses' \
	  '  logs    Follow the container logs: all, or one of $(SERVICES) (Ctrl-C to stop)'
	@$(foreach a,$(AREAS),printf '\n  \033[36m%-7s\033[0m %s\n          %s\n' '$(a)' '$(subst $(space), | ,$($(a)_ACTIONS))' '$($(a)_HELP)';)

## --- The proxy on this computer (Docker + Tailscale + portless) ------------------------------
# https://hara.local on your network (portless in LAN mode) and https://cliproxy.<tailnet>.ts.net
# on your tailnet (local/compose.yaml).

up:
	./scripts/local.sh up

down:
	./scripts/local.sh down

status:
	@./scripts/local.sh status

logs:
	./scripts/local.sh logs $(SERVICE)

## --- claude, codex: the clients -----------------------------------------------------------
# Claude Code speaks HTTP (with SSE streaming) to the proxy. Codex goes through it with the `proxy`
# profile (codex/proxy.config.toml) over the Responses WebSocket; plain `codex` keeps its ChatGPT login.

claude/run:
	@NODE_EXTRA_CA_CERTS=$(PORTLESS_CA) ANTHROPIC_BASE_URL=$(URL) ANTHROPIC_AUTH_TOKEN="$(API_KEY)" claude $(ARGS)

claude/alias:
	@echo "alias claude-hara='NODE_EXTRA_CA_CERTS=\$$HOME/.portless/ca.pem ANTHROPIC_BASE_URL=$(URL) ANTHROPIC_AUTH_TOKEN=\"\$$($(HARA_KEY))\" claude'"

# settings.json is copied with its Omniyat auto-mode section redacted.
claude/backup:
	./scripts/backup-claude.sh

codex/run:
	@codex --profile proxy $(ARGS)

codex/profile:
	install -m 644 codex/proxy.config.toml $(HOME)/.codex/proxy.config.toml

codex/smoke:
	./scripts/codex-smoke.sh

codex/backup:
	./scripts/backup-codex.sh

## --- ops: requests to the running proxy (URL) ----------------------------------------------

# Built rather than `go run`, which adds its own "exit status 1" to every failure.
OPS_BIN := $(or $(LOCAL_DIR),$(HOME)/.cli-proxy-api)/bin/ops
OPS = (cd scripts/ops && go build -o $(OPS_BIN) .) && API_KEY="$(API_KEY)" MGMT_KEY="$(MGMT_KEY)" $(OPS_BIN)

# % used, reset times, weekly capacity expiring unused within 24h.
ops/quota:
	@cd scripts/quota && MGMT_KEY="$(MGMT_KEY)" URL=$(URL) go run .

ops/accounts:
	@$(OPS) accounts -url $(URL)

ops/models:
	@$(OPS) models -url $(URL)

ops/logs:
	@$(OPS) logs -url $(URL) -n $${LINES:-200}

ops/smoke:
	@$(OPS) smoke -url $(URL) -model "$(MODEL)"

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

## --- code: checks, formatting and the panel build ------------------------------------------

GO_MODULES := scripts/ops scripts/quota scripts/wssmoke scripts/bench
SHELL_SCRIPTS := scripts/*.sh scripts/hara-key scripts/lib/common.sh

# What CI runs (.github/workflows/ci.yml). Without fmtkit, lint checks the shell scripts and gofmt only.
code/check: code/lint code/test
	cd web && npx vue-tsc -b && npm run coverage

# Go is formatted from its module directory, so fmtkit also runs go vet there.
code/format:
	fmtkit format-all --ts
	@for m in $(GO_MODULES); do (cd $$m && fmtkit format-all --go) || exit 1; done

code/lint:
	shellcheck $(SHELL_SCRIPTS)
	@unformatted="$$(gofmt -l $(GO_MODULES))"; [[ -z "$$unformatted" ]] || { echo "gofmt needed: $$unformatted"; exit 1; }
	@if command -v fmtkit >/dev/null; then fmtkit lint web/src web/worker web/cloudflare.config.ts; else echo "fmtkit is not installed; skipping the TS lint"; fi

code/test:
	@for m in $(GO_MODULES); do echo "$$m"; (cd $$m && go vet ./... && go test ./...) || exit 1; done

code/complexity:
	fmtkit complexity --ts web/src web/worker
	@for m in $(GO_MODULES); do (cd $$m && fmtkit complexity --go .) || exit 1; done

# Upstream panel + panel/ledger.patch; commit it, then `make up`.
code/panel:
	./scripts/build-panel.sh
