# cli-proxy-api — common tasks. Run `make` for the list.
# Secrets are read from 1Password at run time and never echoed.

SHELL      := /bin/bash
URL        := https://proxy.hara.sh
OP_ACCOUNT ?= my.1password.com
OP_ITEM    := op://YOUR_VAULT/YOUR_ITEM
BUCKET     := cli-proxy-api
MODEL      ?= claude-haiku-4-5-20251001

export OP_ACCOUNT

# Cached in the macOS Keychain for 30 days (scripts/hara-key); 1Password is asked once a month.
HARA_KEY = $(CURDIR)/scripts/hara-key
API_KEY  = $$($(HARA_KEY) claude-api-key)
MGMT_KEY = $$($(HARA_KEY) management-password)

.DEFAULT_GOAL := help
.PHONY: help install types check dev test coverage format-all lint complexity deploy deploy-dry config-push \
        claude codex codex-direct codex-backup claude-backup codex-smoke alias keys-refresh health models accounts smoke logs logs-cf tail tail-codex

help: ## Show this help
	@grep -hE '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

## --- Development -----------------------------------------------------------

install: ## Install npm dependencies (cf, wrangler, containers)
	npm install

types: ## Regenerate Worker types from cloudflare.config.ts
	npx cf workers types >/dev/null

check: types ## Type-check the Worker, tests and Node tooling config
	npx tsc -p .
	npx tsc -p tsconfig.node.json

dev: ## Run the Worker + container locally against the dev R2 bucket (secrets from 1Password)
	./scripts/dev.sh

test: ## Run the Vitest suite
	npx vitest run

coverage: ## Run the Vitest suite with coverage (fails under 100%)
	npx vitest run --coverage

format-all: ## Format every TS/JS file with fmtkit (oxlint --fix, oxfmt, structural passes)
	fmtkit format-all --ts

lint: ## Lint TS/JS with fmtkit (oxlint), writing nothing
	fmtkit lint src cloudflare.config.ts

complexity: ## Report TS functions over fmtkit's complexity limits
	fmtkit complexity --ts src

## --- Deployment ------------------------------------------------------------

deploy: check coverage ## Build the image and deploy (secrets injected from 1Password)
	./scripts/deploy.sh

deploy-dry: check ## Build and validate without uploading
	npx cf deploy --dry-run

config-push: ## Render config.yaml from 1Password and upload it to R2
	./scripts/push-config.sh

## --- Claude Code -----------------------------------------------------------

claude: ## Run Claude Code through the proxy (pass flags with ARGS="...")
	@ANTHROPIC_BASE_URL=$(URL) ANTHROPIC_AUTH_TOKEN="$(API_KEY)" claude $(ARGS)

## --- Codex CLI -------------------------------------------------------------
# ~/.codex/config.toml routes Codex through the proxy by default (provider `hara`,
# key fetched from 1Password via auth.command). Backups live in codex/.

codex: ## Run Codex CLI (proxy is the default provider; pass flags with ARGS="...")
	@codex $(ARGS)

codex-direct: ## Run Codex CLI with the direct ChatGPT login (profile `openai`)
	@codex --profile openai $(ARGS)

codex-backup: ## Copy ~/.codex/config.toml and openai.config.toml into codex/ (then commit)
	./scripts/backup-codex.sh

claude-backup: ## Copy ~/.claude settings into claude/ (Omniyat auto-mode section redacted), then commit
	./scripts/backup-claude.sh

codex-smoke: ## Run one non-interactive Codex turn through the proxy
	@codex exec --skip-git-repo-check "Reply with exactly: pong" </dev/null

alias: ## Print the claude-hara shell alias for ~/.zshrc
	@echo "alias claude-hara='ANTHROPIC_BASE_URL=$(URL) ANTHROPIC_AUTH_TOKEN=\"\$$($(HARA_KEY))\" claude'"

keys-refresh: ## Re-fetch the cached keys from 1Password now (e.g. after rotating them)
	@for f in claude-api-key codex-api-key management-password; do $(HARA_KEY) --refresh $$f >/dev/null; done && echo "keys refreshed"

## --- Operations ------------------------------------------------------------

health: ## Check the proxy health endpoint
	@curl -fsS --max-time 60 $(URL)/healthz && echo

models: ## List models available through the proxy
	@curl -fsS $(URL)/v1/models -H "Authorization: Bearer $(API_KEY)" \
	  | python3 -c 'import json,sys; [print(m["id"]) for m in json.load(sys.stdin)["data"]]'

accounts: ## List connected provider accounts
	@curl -fsS $(URL)/v0/management/auth-files -H "Authorization: Bearer $(MGMT_KEY)" \
	  | python3 -c 'import json,sys; [print("%-8s %-8s %s" % (f.get("provider"), f.get("status"), f.get("name"))) for f in json.load(sys.stdin).get("files",[])]'

smoke: ## Send a test message through the proxy (MODEL=... to override)
	@curl -fsS $(URL)/v1/messages -H "x-api-key: $(API_KEY)" -H 'anthropic-version: 2023-06-01' \
	  -H 'content-type: application/json' \
	  -d '{"model":"$(MODEL)","max_tokens":16,"messages":[{"role":"user","content":"Reply with exactly: pong"}]}' \
	  | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["content"][0]["text"] if d.get("content") else d)'

logs: ## Show the last LINES lines of the server log (default 200)
	@curl -fsS "$(URL)/v0/management/logs?limit=$${LINES:-200}" -H "Authorization: Bearer $(MGMT_KEY)" \
	  | python3 -c 'import json,sys; print("\n".join(json.load(sys.stdin).get("lines",[])))'

# Live logs use wrangler (cf has no tail command yet) with the personal Cloudflare login,
# kept apart from the default (work) wrangler login.
WRANGLER = XDG_CONFIG_HOME=$(HOME)/.claude/work/wrangler-personal CLOUDFLARE_ACCOUNT_ID=YOUR_ACCOUNT_ID npx wrangler

tail: ## Stream live Worker logs (Ctrl-C to stop)
	@$(WRANGLER) tail cli-proxy-api --format pretty

tail-codex: ## Stream only Codex request lines: model, tier, effort (Ctrl-C to stop)
	@$(WRANGLER) tail cli-proxy-api --format pretty | grep --line-buffered 'codex request'

logs-cf: ## Show Worker/container stdout from Cloudflare observability (MINUTES=... to override)
	@NOW=$$(($$(date +%s)*1000)); FROM=$$((NOW-$${MINUTES:-15}*60*1000)); \
	npx cf observability telemetry query --body "{\"queryId\":\"make-logs\",\"view\":\"events\",\"limit\":500,\"timeframe\":{\"from\":$$FROM,\"to\":$$NOW},\"parameters\":{}}" \
	  | python3 -c 'import json,sys; ev=sorted(json.load(sys.stdin)["events"]["events"],key=lambda e:e.get("timestamp",0)); [print(e.get("$$metadata",{}).get("message","")) for e in ev]'
