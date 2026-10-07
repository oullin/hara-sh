package main

import (
	"cmp"
	"errors"
	"os"
	"path/filepath"
)

// panel builds the management panel the proxy serves at /management.html: the upstream Management
// Center at PANEL_TAG plus panel/ledger.patch (the quota Ledger view). It writes panel/management.html;
// commit it, then `make up` (local/compose.yaml mounts it). Needs git and bun. To move to a newer
// upstream panel, bump PANEL_TAG; if the patch no longer applies, rebase it on the new tag and
// regenerate panel/ledger.patch.
func (h host) panel() error {
	repo := cmp.Or(h.getenv("PANEL_REPO"), "https://github.com/router-for-me/Cli-Proxy-API-Management-Center")
	tag := cmp.Or(h.getenv("PANEL_TAG"), "v1.25.3")

	if !h.look("bun") {
		return errors.New("bun is required (brew install oven-sh/bun/bun)")
	}

	dir, err := os.MkdirTemp("", "cli-proxy-panel")

	if err != nil {
		return err
	}

	defer os.RemoveAll(dir)

	h.log("cloning %s@%s", repo, tag)

	if err := h.call(h.command("git", "-c", "advice.detachedHead=false", "clone", "--quiet", "--depth", "1", "--branch", tag, repo, dir)); err != nil {
		return err
	}

	h.log("applying panel/ledger.patch")

	if err := h.call(h.command("git", "-C", dir, "apply", "--whitespace=nowarn", filepath.Join(h.root, "panel", "ledger.patch"))); err != nil {
		return err
	}

	h.log("building")

	install := h.command("bun", "install", "--frozen-lockfile")
	install.Dir, install.Stdout = dir, nil

	if err := h.call(install); err != nil {
		return err
	}

	build := h.command("bun", "run", "build")
	build.Dir, build.Stdout, build.Env = dir, nil, append(os.Environ(), "VERSION="+tag+"+ledger")

	if err := h.call(build); err != nil {
		return err
	}

	html, err := os.ReadFile(filepath.Join(dir, "dist", "index.html"))

	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(h.root, "panel", "management.html"), html, 0o644); err != nil {
		return err
	}

	h.log("wrote panel/management.html (%d bytes)", len(html))

	return nil
}
