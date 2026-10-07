# cli-proxy-api — make up | down | status | logs [service], or make <area> [action]. Run `make` for the list.
# Private credentials are read through Docker; 1Password is optional.

SHELL      := /bin/bash
# Host clients use loopback; containerized tools map it to the proxy service.
URL        ?= http://localhost:$(or $(HARA_PORT),8317)
MODEL      ?= claude-haiku-4-5-20251001
# Codex model for `make ops bench`.
CODEX_MODEL ?= gpt-5.5

export OP_ACCOUNT OP_VAULT OP_ITEM_NAME

# The helper reads private Docker state; HARA_SECRET_PROVIDER=op opts into 1Password.
HARA_KEY = $(CURDIR)/scripts/hara-key
API_KEY  = $$($(HARA_KEY) claude-api-key)
# hara.local uses the portless certificate authority; Node-based clients need it named.
PORTLESS_CA = $(wildcard $(HOME)/.portless/ca.pem)

# Each area lists its actions, the first being the default, and one line of help.
AREAS    := claude codex ops web code
SERVICES := proxy tailscale quota

claude_ACTIONS := run alias backup
claude_HELP    := Claude Code through the proxy (flags in ARGS="..."); alias prints claude-hara for ~/.zshrc; backup copies client settings outside the repository
codex_ACTIONS  := run profile smoke backup
codex_HELP     := Codex through the proxy over WebSockets (flags in ARGS="..."); profile installs it; smoke fails on the HTTP fallback; backup copies client settings outside the repository
ops_ACTIONS    := quota accounts models logs smoke ws-smoke bench keys import-op
ops_HELP       := Usage per account, accounts, models, server log (LINES), one Claude request (MODEL), WebSockets (IDLE, TS_URL), first-token timing (N, CODEX_MODEL), rotate local keys or import optional 1Password credentials
web_ACTIONS    := dev install test coverage build docs deploy
web_HELP       := The landing page and docs on hara.sh: Vite dev server, dependencies, tests, coverage (fails under 100%), type-check, test and deploy
code_ACTIONS   := check format lint test complexity panel public
code_HELP      := check runs what CI runs (lint, Go tests, web coverage); format and lint with fmtkit and shellcheck; Go tests; complexity limits; rebuild the panel (needs bun); public-file privacy guard

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
.PHONY: help init up down status logs $(AREAS) $(foreach a,$(AREAS),$(addprefix $(a)/,$($(a)_ACTIONS)))

space := $(subst ,, )
help:
	@printf '%s\n' \
	  'make init | up | down | status | logs [service]   the proxy on this computer' \
	  'make <area> [action]                       the first action is the default' \
	  '' \
	  '  up      Start Docker services, initialize private keys, apply config.yaml, then run status' \
	  '  down    Stop the proxy (credentials, logins and Tailscale identity kept)' \
	  '  status  Check containers, proxy, panel, keys and accounts; print available addresses' \
	  '  logs    Follow the container logs: all, or one of $(SERVICES) (Ctrl-C to stop)'
	@$(foreach a,$(AREAS),printf '\n  \033[36m%-7s\033[0m %s\n          %s\n' '$(a)' '$(subst $(space), | ,$($(a)_ACTIONS))' '$($(a)_HELP)';)

## --- The Docker Compose stack (Tailscale is optional) -----------------------------------

init:
	./scripts/local.sh init

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

# Client backups remain private, outside the repository.
claude/backup:
	./scripts/backup-claude.sh

codex/run:
	@codex --profile proxy $(ARGS)

codex/profile:
	./scripts/install-codex-profile.sh

codex/smoke:
	./scripts/codex-smoke.sh

codex/backup:
	./scripts/backup-codex.sh

## --- ops: requests to the running proxy (URL) ----------------------------------------------

# Go tools are built once in Docker, with no host Go installation.
OPS = ./scripts/local.sh tools ops

# % used, reset times, weekly capacity expiring unused within 24h.
ops/quota:
	@URL=$(URL) ./scripts/local.sh tools quota

ops/accounts:
	@URL=$(URL) $(OPS) accounts

ops/models:
	@URL=$(URL) $(OPS) models

ops/logs:
	@URL=$(URL) $(OPS) logs -n $${LINES:-200}

ops/smoke:
	@URL=$(URL) $(OPS) smoke -model "$(MODEL)"

# Handshake, ping/pong, close and 401 without a key, per address. The proxy answers pings itself,
# so it needs no provider account. TS_URL adds the tailnet address (from `make status`);
# IDLE=2m holds each socket idle, then pings again.
ops/ws-smoke:
	@URL=$(URL) ./scripts/local.sh ws-smoke $${IDLE:-0s} $(TS_URL)

# Real requests: N per API per address (about 5k prompt tokens each, mostly cached). The first of
# each series is cold, the rest should hit the prompt cache. CODEX_MODEL= skips Codex, MODEL= Claude.
ops/bench:
	@URL=$(URL) ./scripts/local.sh bench $${N:-5} "$(MODEL)" "$(CODEX_MODEL)"

# Rotation is explicit; apply it with make up and refresh running clients.
ops/keys:
	./scripts/local.sh rotate

ops/import-op:
	./scripts/local.sh import-op

## --- web: the landing page (web/: Vue + shadcn-vue + Tailwind CSS v4, served on hara.sh) ---

web/dev:
	cd web && npm run dev

web/install:
	cd web && npm install

web/test:
	cd web && npm test

web/coverage:
	cd web && npm run coverage

web/build:
	cd web && npm run build

web/docs:
	cd web && npm run docs:dev

web/deploy:
	cd web && npm run deploy

## --- code: checks, formatting and the panel build ------------------------------------------

GO_MODULES := scripts/ops scripts/quota scripts/wssmoke scripts/bench scripts/tools scripts/public
SHELL_SCRIPTS := scripts/*.sh scripts/hara-key scripts/lib/common.sh

# What CI runs (.github/workflows/ci.yml). Without fmtkit, lint checks the shell scripts and gofmt only.
code/check: code/lint code/test code/public
	cd web && npm run build && npm run coverage

# Go is formatted from its module directory, so fmtkit also runs go vet there.
code/format:
	fmtkit format-all --ts
	@for m in $(GO_MODULES); do (cd $$m && fmtkit format-all --go) || exit 1; done

code/lint:
	shellcheck $(SHELL_SCRIPTS)
	@unformatted="$$(gofmt -l $(GO_MODULES))"; [[ -z "$$unformatted" ]] || { echo "gofmt needed: $$unformatted"; exit 1; }
	@if command -v fmtkit >/dev/null; then fmtkit lint web/src web/worker web/scripts web/docs/.vitepress web/cloudflare.config.ts; else echo "fmtkit is not installed; skipping the TS lint"; fi

code/test:
	docker build --target verify -f scripts/tools/Dockerfile .

code/complexity:
	fmtkit complexity --ts web/src web/worker
	@for m in $(GO_MODULES); do (cd $$m && fmtkit complexity --go .) || exit 1; done

# Upstream panel + panel/ledger.patch; commit it, then `make up`.
code/panel:
	./scripts/build-panel.sh

code/public:
	cd scripts/public && go run .

# Compatibility entrypoint for the full formatter.
.PHONY: format-all
format-all: code/format
