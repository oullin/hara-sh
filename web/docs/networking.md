---
description: Use Hara on localhost or connect securely from another device with Tailscale HTTPS. Configure ports, tailnet access and optional LAN routing.
---

# Network and TLS

## Localhost

Host clients use `http://localhost:8317`. Docker binds only to `127.0.0.1`; other devices cannot reach this address. Set `HARA_PORT` to change the published port, then update your client URLs.

Tools containers use `http://proxy:8317`. Their `localhost` is the tools container itself. Use `host.docker.internal` for another service on the Docker host.

## Tailscale

For private HTTPS from your phone or another computer:

1. Enable MagicDNS and HTTPS certificates in your Tailscale account.
2. Start the optional container:

```bash
docker compose -f local/compose.yaml --profile tailscale up -d --build --force-recreate
docker compose -f local/compose.yaml exec tailscale tailscale status --peers=false
```

3. Open the login link and authorise the device.
4. Join your client device to the same tailnet. Use the reported `https://cliproxy.<your-tailnet>.ts.net` address.

Keep `--profile tailscale` on later startup commands, or export `COMPOSE_PROFILES=tailscale`. `TS_HOSTNAME` changes the device name; `TS_AUTHKEY` optionally supplies a private enrolment key. Identity persists in your state directory.

Use tailnet ACLs to restrict access. Management is available at the same address and requires its separate password. Avoid public port forwarding or Funnel.

## Optional LAN access

If portless is installed, `make up` serves Hara at `https://hara.local`; `make portless` repeats that step on its own. Without Make, run:

```bash
portless proxy start --lan
portless alias hara 8317 --force
```

Starting the proxy on port 443 may ask for sudo. Run `sudo portless service install --lan` once to start it at boot instead. LAN mode exposes every app served by that instance. Client devices need mDNS and trust in its public CA; never transfer the private CA key.

Host Node clients can set `NODE_EXTRA_CA_CERTS="$HOME/.portless/ca.pem"`. Codex reads `CODEX_CA_CERTIFICATE` instead. Neither uses the copy in the macOS keychain. Container tools do not inherit that CA or `.local` discovery; use the Docker service or Tailscale address.

LAN mode listens on port 443 on every interface, including the host's own Tailscale address. Tailscale Serve on that host must therefore use another port, such as 8443. Otherwise portless answers the tailnet URL with its own certificate and a 404. `make t3` sets T3 Code's Tailscale HTTPS to 8443 for this reason. The Hara Tailscale container is a separate tailnet device, so it is unaffected.

## Availability

The private proxy requires the host and Docker to stay running. Sleep, expired provider logins, and network changes can interrupt it. The public docs at `docs.hara.sh` are independent of your proxy.
