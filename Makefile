# cli-proxy-api — common tasks. Run `make` for the list.
# Secrets are read from 1Password at run time and never echoed.

SHELL      := /bin/bash
URL        := https://proxy.hara.sh
OP_ACCOUNT ?= my.1password.com
OP_ITEM    := op://cloudflare/cli-proxy-api
BUCKET     := cli-proxy-api
MODEL      ?= claude-haiku-4-5-20251001

export OP_ACCOUNT

API_KEY  = $$(op read $(OP_ITEM)/api-key)
MGMT_KEY = $$(op read $(OP_ITEM)/management-password)

.DEFAULT_GOAL := help
.PHONY: help install types check deploy deploy-dry config-push \
        claude alias health models accounts smoke logs logs-cf

help: ## Show this help
	@grep -hE '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

## --- Development -----------------------------------------------------------

install: ## Install npm dependencies (cf, wrangler, containers)
	npm install

types: ## Regenerate Worker types from cloudflare.config.ts
	npx cf workers types >/dev/null

check: types ## Type-check the Worker
	npx tsc -p .

## --- Deployment ------------------------------------------------------------

deploy: check ## Build the image and deploy (secrets injected from 1Password)
	./scripts/deploy.sh

deploy-dry: check ## Build and validate without uploading
	npx cf deploy --dry-run

config-push: ## Render config.yaml from 1Password and upload it to R2
	./scripts/config-push.sh

## --- Claude Code -----------------------------------------------------------

claude: ## Run Claude Code through the proxy (pass flags with ARGS="...")
	@ANTHROPIC_BASE_URL=$(URL) ANTHROPIC_AUTH_TOKEN="$(API_KEY)" claude $(ARGS)

alias: ## Print the claude-hara shell alias for ~/.zshrc
	@echo "alias claude-hara='ANTHROPIC_BASE_URL=$(URL) ANTHROPIC_AUTH_TOKEN=\"\$$(op read --account $(OP_ACCOUNT) $(OP_ITEM)/api-key)\" claude'"

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

logs-cf: ## Show Worker/container stdout from Cloudflare observability (MINUTES=... to override)
	@NOW=$$(($$(date +%s)*1000)); FROM=$$((NOW-$${MINUTES:-15}*60*1000)); \
	npx cf observability telemetry query --body "{\"queryId\":\"make-logs\",\"view\":\"events\",\"limit\":500,\"timeframe\":{\"from\":$$FROM,\"to\":$$NOW},\"parameters\":{}}" \
	  | python3 -c 'import json,sys; ev=sorted(json.load(sys.stdin)["events"]["events"],key=lambda e:e.get("timestamp",0)); [print(e.get("$$metadata",{}).get("message","")) for e in ev]'
