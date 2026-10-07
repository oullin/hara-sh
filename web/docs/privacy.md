---
description: Prepare Hara for open-source publication. Keep credentials outside Git, scan the current files and history, and review release requirements.
---

# Privacy and publication

Keep private state outside the repository. The website publishes only static assets and public documentation.

## Keep out of Git

- Secret files, rendered config, and private provider templates.
- OAuth logins, Tailscale identity, client settings, and backups.
- Operational logs, callbacks, and screenshots containing account details.

Ignore rules do not remove files already committed. Local Compose mounts are not encrypted storage; protect the host disk and backups.

## Check the current snapshot

```bash
make code public
gitleaks dir . --redact
```

The publication guard rejects Python and JavaScript source and reports personal paths, client-state files, and recognisable credentials without printing matched values. Write server tools in Go and web scripts in TypeScript. Gitleaks adds broader secret detection. Review generated assets and examples too; neither tool detects every private fact.

## Check history

```bash
(cd scripts/public && go run . --history)
gitleaks git --log-opts='--all' --redact
```

A clean current tree can still have private data in old commits. Before changing visibility, either publish a fresh reviewed snapshot or rewrite every affected branch and tag with `git filter-repo` and coordinate replacement of old clones.

Remote refs, forks, and caches may retain old objects. Rotate any exposed credentials even after removing them from history.

## Before release

Choose a licence, preserve upstream notices, and review every ref you intend to publish. Keep scan reports redacted and private. Publish neither runtime state nor backups with the site or source archive.
