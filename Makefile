# hara-sh — make up | portless | down | purge | status | logs [service], or make <area> [action]. Run `make` for the list.
# Private credentials are read through Docker; 1Password is optional.

SHELL      := /bin/bash
# Host clients use loopback; containerized tools map it to the proxy service.
URL        ?= http://localhost:$(or $(HARA_PORT),8317)
MODEL      ?= claude-haiku-4-5-20251001
# Codex model for `make ops bench`.
CODEX_MODEL ?= gpt-5.5

export OP_ACCOUNT OP_VAULT OP_ITEM_NAME

# The host helper (scripts/hara, built into bin/) reads private Docker state; HARA_SECRET_PROVIDER=op opts into 1Password.
HARA         = $(CURDIR)/bin/hara
HARA_SOURCES = $(filter-out %_test.go,$(wildcard scripts/hara/*.go)) scripts/hara/go.mod
API_KEY      = $$($(HARA) key claude-api-key)
# hara.local uses the portless certificate authority; Node-based clients need it named.
PORTLESS_CA = $(wildcard $(HOME)/.portless/ca.pem)

# Each area lists its actions, the first being the default, and one line of help.
AREAS    := claude codex ops web code
SERVICES := proxy tailscale quota

claude_ACTIONS := run backup
claude_HELP    := Claude Code through the proxy (flags in ARGS="..."); backup copies client settings outside the repository
codex_ACTIONS  := run profile smoke backup
codex_HELP     := Codex through the proxy over WebSockets (flags in ARGS="..."); profile installs it; smoke fails on the HTTP fallback; backup copies client settings outside the repository
ops_ACTIONS    := quota accounts models logs smoke ws-smoke bench keys import-op
ops_HELP       := Usage per account, accounts, models, server log (LINES), one Claude request (MODEL), WebSockets (IDLE, TS_URL), first-token timing (N, CODEX_MODEL), rotate local keys or import optional 1Password credentials
web_ACTIONS    := dev install test coverage build docs deploy
web_HELP       := The landing page and docs on hara.sh: Vite dev server, dependencies, tests, coverage (fails under 100%), type-check, test and deploy
code_ACTIONS   := check test complexity panel public
code_HELP      := check runs what CI runs (Go tests, web build and coverage, privacy guard); Go tests; complexity limits; rebuild the panel (needs bun); public-file privacy guard

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
.PHONY: help init up portless down purge status logs format-all $(AREAS) $(foreach a,$(AREAS),$(addprefix $(a)/,$($(a)_ACTIONS)))

space := $(subst ,, )
help:
	@printf '%s\n' \
	  'make init | up | portless | down | purge | status | logs [service]   the proxy on this computer' \
	  'make <area> [action]                                                 the first action is the default' \
	  '' \
	  '  up        Start Docker services, initialize private keys, apply config.yaml, run portless, then status' \
	  '  portless  Serve the proxy at https://hara.local through portless in LAN mode (skipped if not installed)' \
	  '  down      Stop the proxy (credentials, logins and Tailscale identity kept)' \
	  '  purge     Remove containers, networks, images, build cache and the hara.local route (private state kept)' \
	  '  status    Check containers, proxy, panel, keys and accounts; print available addresses' \
	  '  logs      Follow the container logs: all, or one of $(SERVICES) (Ctrl-C to stop)' \
	  '' \
	  'make format-all                                                      the only formatter and lint: fmtkit for TS/Vue and every Go module'
	@$(foreach a,$(AREAS),printf '\n  \033[36m%-7s\033[0m %s\n          %s\n' '$(a)' '$(subst $(space), | ,$($(a)_ACTIONS))' '$($(a)_HELP)';)

## --- The host helper --------------------------------------------------------------------

# Built on first use and after source changes. Without host Go, Docker cross-compiles it for this computer.
$(HARA): $(HARA_SOURCES)
	@mkdir -p $(@D)
	@if command -v go >/dev/null; then \
	  cd scripts/hara && CGO_ENABLED=0 go build -trimpath -o $@ .; \
	else \
	  docker run --rm -u "$$(id -u):$$(id -g)" -e CGO_ENABLED=0 -e GOCACHE=/tmp/go-cache \
	    -e GOOS="$$(uname -s | tr '[:upper:]' '[:lower:]')" -e GOARCH="$$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" \
	    -v "$(CURDIR)/scripts/hara:/src:ro" -v "$(@D):/out" -w /src golang:1.27-alpine go build -trimpath -o /out/hara .; \
	fi

init up portless down purge status logs claude/run claude/backup codex/profile codex/smoke codex/backup code/panel: $(HARA)
$(addprefix ops/,$(ops_ACTIONS)): $(HARA)

## --- The Docker Compose stack (Tailscale is optional) -----------------------------------

init:
	$(HARA) init

up:
	$(HARA) up

portless:
	$(HARA) portless

down:
	$(HARA) down

purge:
	$(HARA) purge

status:
	@$(HARA) status

logs:
	$(HARA) logs $(SERVICE)

## --- claude, codex: the clients -----------------------------------------------------------
# Claude Code speaks HTTP (with SSE streaming) to the proxy. Codex goes through it with the `proxy`
# profile (codex/proxy.config.toml) over the Responses WebSocket; plain `codex` keeps its ChatGPT login.

claude/run:
	@NODE_EXTRA_CA_CERTS=$(PORTLESS_CA) ANTHROPIC_BASE_URL=$(URL) ANTHROPIC_AUTH_TOKEN="$(API_KEY)" claude $(ARGS)

# Client backups remain private, outside the repository.
claude/backup:
	$(HARA) backup claude

codex/run:
	@codex --profile proxy $(ARGS)

codex/profile:
	$(HARA) codex-profile

codex/smoke:
	$(HARA) codex-smoke

codex/backup:
	$(HARA) backup codex

## --- ops: requests to the running proxy (URL) ----------------------------------------------

# Go tools are built once in Docker, with no host Go installation.
OPS = $(HARA) tools ops

# % used, reset times, weekly capacity expiring unused within 24h.
ops/quota:
	@URL=$(URL) $(HARA) tools quota

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
	@URL=$(URL) $(HARA) ws-smoke $${IDLE:-0s} $(TS_URL)

# Real requests: N per API per address (about 5k prompt tokens each, mostly cached). The first of
# each series is cold, the rest should hit the prompt cache. CODEX_MODEL= skips Codex, MODEL= Claude.
ops/bench:
	@URL=$(URL) $(HARA) bench $${N:-5} "$(MODEL)" "$(CODEX_MODEL)"

# Rotation is explicit; apply it with make up and refresh running clients.
ops/keys:
	$(HARA) rotate

ops/import-op:
	$(HARA) import-op

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

GO_MODULES := scripts/hara scripts/ops scripts/quota scripts/wssmoke scripts/bench scripts/tools scripts/public

# What CI runs (.github/workflows/ci.yml). Formatting and lint stay in `make format-all`.
code/check: code/test code/public
	cd web && npm run build && npm run coverage

# Go is formatted from its module directory, so fmtkit also runs go vet there.
format-all:
	fmtkit format-all --ts
	@for m in $(GO_MODULES); do (cd $$m && fmtkit format-all --go) || exit 1; done

code/test:
	docker build --target verify -f scripts/tools/Dockerfile .

code/complexity:
	fmtkit complexity --ts web/src web/worker
	@for m in $(GO_MODULES); do (cd $$m && fmtkit complexity --go .) || exit 1; done

# Upstream panel + panel/ledger.patch; commit it, then `make up`.
code/panel:
	$(HARA) panel

code/public:
	cd scripts/public && go run .
