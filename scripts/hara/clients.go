package main

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const (
	apiField        = "claude-api-key"
	managementField = "management-password"
	toolsImage      = "hara-tools:local"
	keyPlaceholder  = "__HARA_KEY_COMMAND__"
	fallbackWarning = "Falling back from WebSockets"
)

// key prints a client credential from local Docker state; HARA_SECRET_PROVIDER=op reads 1Password instead.
// Codex runs it as its auth command, so standard output carries the credential and nothing else.
func (h host) key(args []string) error {
	if len(args) > 1 {
		return errors.New("usage: hara key [claude-api-key|management-password]")
	}

	field := apiField

	if len(args) == 1 {
		field = args[0]
	}

	switch field {
	case apiField, managementField:
	case "--refresh", "--clear":
		return errors.New("use make ops import-op to sync 1Password, or make ops keys to rotate local keys")
	default:
		return errors.New("unknown credential field")
	}

	if h.getenv("HARA_SECRET_PROVIDER") == "op" {
		return h.call(h.opRead(field))
	}

	// The tools image is never published, and an implicit build inside `run` fails while stdout is captured.
	if !h.quiet(h.command("docker", "image", "inspect", toolsImage)) {
		build := h.compose("build", "tools")
		build.Stdout = h.stderr

		if err := h.call(build); err != nil {
			return err
		}
	}

	return h.call(h.compose("run", "--rm", "--no-deps", "-T", "tools", "key", field))
}

func (h host) opRead(field string) *exec.Cmd {
	item := fmt.Sprintf("op://%s/%s/%s", h.getenv("OP_VAULT"), h.getenv("OP_ITEM_NAME"), field)

	return h.command("op", "read", "--account", h.getenv("OP_ACCOUNT"), item)
}

// backup copies client settings into private state; backups must stay outside the publishable checkout.
func (h host) backup(args []string) error {
	clients := map[string]struct {
		name   string
		source string
		files  []string
	}{
		"claude": {"Claude", filepath.Join(h.home, ".claude"), []string{"settings.local.json", "CLAUDE.md", ".claude.json", "settings.json"}},
		"codex":  {"Codex", filepath.Join(h.home, ".codex"), []string{"config.toml", "openai.config.toml", "proxy.config.toml"}},
	}

	if len(args) != 1 {
		return errors.New("usage: hara backup claude|codex")
	}

	client, ok := clients[args[0]]

	if !ok {
		return errors.New("usage: hara backup claude|codex")
	}

	dest := filepath.Join(cmp.Or(h.getenv("BACKUP_DIR"), filepath.Join(h.state, "backups")), args[0])

	if err := privateDir(dest, h.root); err != nil {
		return err
	}

	for _, file := range client.files {
		if err := copyPrivate(filepath.Join(client.source, file), filepath.Join(dest, file)); err != nil {
			return err
		}
	}

	h.log("saved private %s settings outside the repository", client.name)

	return nil
}

// copyPrivate copies a regular file with owner-only access; a missing source is skipped.
func copyPrivate(source, dest string) error {
	if info, err := os.Stat(source); err != nil || !info.Mode().IsRegular() {
		return nil
	}

	content, err := os.ReadFile(source)

	if err != nil {
		return err
	}

	if err := os.WriteFile(dest, content, 0o600); err != nil {
		return err
	}

	return os.Chmod(dest, 0o600)
}

// codexProfile installs the public profile template with this checkout's absolute helper path.
func (h host) codexProfile() error {
	command := filepath.Join(h.root, "bin", "hara")

	if info, err := os.Stat(command); err != nil || !info.Mode().IsRegular() {
		return errors.New("bin/hara is missing; run: make codex profile")
	}

	template, err := os.ReadFile(filepath.Join(h.root, "codex", "proxy.config.toml"))

	if err != nil {
		return err
	}

	if !bytes.Contains(template, []byte(keyPlaceholder)) {
		return errors.New("codex/proxy.config.toml has no " + keyPlaceholder + " placeholder")
	}

	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(command)
	profile := bytes.ReplaceAll(template, []byte(keyPlaceholder), []byte(escaped))
	dir := cmp.Or(h.getenv("CODEX_HOME"), filepath.Join(h.home, ".codex"))

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	if err := writePrivate(filepath.Join(dir, "proxy.config.toml"), profile); err != nil {
		return err
	}

	h.log("installed the proxy profile; run: codex --profile proxy")

	return nil
}

// writePrivate replaces path atomically with an owner-only file.
func writePrivate(path string, content []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".hara-*")

	if err != nil {
		return err
	}

	defer os.Remove(temporary.Name())

	defer temporary.Close()

	if err := temporary.Chmod(0o600); err != nil {
		return err
	}

	if _, err := temporary.Write(content); err != nil {
		return err
	}

	if err := temporary.Close(); err != nil {
		return err
	}

	return os.Rename(temporary.Name(), path)
}

// codexSmoke runs one Codex turn through the proxy and fails unless it completed over the WebSocket.
// Codex falls back to HTTP by itself when the socket fails, so a turn that answers is not enough:
// the fallback warning in its output is what tells the two transports apart.
// Env: CODEX_PROFILE (default proxy), PROXY_URL (default http://localhost:8317).
func (h host) codexSmoke() error {
	profile := cmp.Or(h.getenv("CODEX_PROFILE"), "proxy")
	proxyURL := cmp.Or(h.getenv("PROXY_URL"), "http://localhost:8317")
	file := filepath.Join(cmp.Or(h.getenv("CODEX_HOME"), filepath.Join(h.home, ".codex")), profile+".config.toml")

	if info, err := os.Stat(file); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("no Codex profile '%s'; run: make codex profile", profile)
	}

	if !h.healthy(proxyURL) {
		return fmt.Errorf("the proxy does not answer on %s; run: make up", proxyURL)
	}

	var out bytes.Buffer

	cmd := h.command("codex", "exec", "--profile", profile, "--skip-git-repo-check", "Reply with exactly: pong")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, &out, &out

	if err := h.call(cmd); err != nil {
		h.stderr.Write(out.Bytes())

		return errors.New("the Codex turn failed")
	}

	lines := strings.Split(out.String(), "\n")

	var fallback []string

	for _, line := range lines {
		if strings.Contains(line, fallbackWarning) {
			fallback = append(fallback, line)
		}
	}

	if len(fallback) != 0 {
		fmt.Fprintln(h.stderr, strings.Join(fallback, "\n"))

		return errors.New("Codex answered over HTTP: the WebSocket failed (see make ops logs)")
	}

	if !slices.Contains(lines, "pong") {
		h.stderr.Write(out.Bytes())

		return errors.New("Codex did not answer pong")
	}

	h.log("ok: Codex answered pong over the WebSocket (profile %s, %s)", profile, proxyURL)

	return nil
}

func (h host) healthy(proxyURL string) bool {
	response, err := h.client.Get(strings.TrimRight(proxyURL, "/") + "/healthz")

	if err != nil {
		return false
	}

	defer response.Body.Close()

	return response.StatusCode < http.StatusBadRequest
}
