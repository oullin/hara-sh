---
description: Connect OAuth accounts and API keys to Hara. Pool providers, preserve session affinity and route requests using available quota.
---

# Connect providers

Open `http://localhost:8317/management.html` on the Docker host and enter your management password. Provider logins stay in your private state directory.

## OAuth accounts

1. Select the provider and start its sign-in flow.
2. Open **Copy Link** in a private browser window signed in to the intended account.
3. Complete authorisation.
4. If the browser ends at an unreachable `localhost:…/callback` page, paste the full URL into the panel's **Callback URL** field and submit it once.

The callback URL contains an authorisation code. Keep it private.

Check that the account and its models appear:

```bash
docker compose -f local/compose.yaml run --rm --no-deps -T tools ops accounts
docker compose -f local/compose.yaml run --rm --no-deps -T tools ops models
```

Repeat for each account you want to pool. Available sign-in flows depend on the pinned proxy version and installed plugins.

## API keys

Use the panel's provider or OpenAI-compatible settings to add an endpoint and key. Match the provider's model names and API format.

Startup overwrites runtime configuration from its template. For persistent provider keys, use an [outside-Git private template](./configuration#private-provider-settings). Never add them to the public `config.yaml`.

## Routing

New sessions use eligible accounts at the highest priority. Later turns and subagents stay on the same account for up to an hour. If it becomes unavailable, Hara can fail over; the new account may need to rebuild its prompt cache.

Quota routing checks Claude and Codex usage hourly and prefers unused weekly capacity that expires within 24 hours.

## Remove an account

Delete it through the panel and recheck the account list. To revoke access, also remove the authorisation at the provider. Deleting the local login does not revoke the upstream token.
