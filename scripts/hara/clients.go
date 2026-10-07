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
	"regexp"
	"slices"
	"strings"
)

// codexCheck is one Codex smoke run: the Codex home, its profile (empty uses config.toml), the
// proxy to probe, the command that installs the config and extra child environment.
type codexCheck struct {
	home     string
	profile  string
	proxyURL string
	hint     string
	env      []string
}

const (
	apiField        = "claude-api-key"
	managementField = "management-password"
	toolsImage      = "hara-tools:local"
	keyPlaceholder  = "__HARA_KEY_COMMAND__"
	fallbackWarning = "Falling back from WebSockets"
	defaultCodexURL = "http://localhost:8317/v1"
)

var baseURLLine = regexp.MustCompile(`(?m)^base_url = .*$`)

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

	cmd, err := h.keyCommand(field)

	if err != nil {
		return err
	}

	return h.call(cmd)
}

// keyCommand returns the command that prints field on standard output, building the tools image first if needed.
func (h host) keyCommand(field string) (*exec.Cmd, error) {
	switch field {
	case apiField, managementField:
	case "--refresh", "--clear":
		return nil, errors.New("use make ops import-op to sync 1Password, or make ops keys to rotate local keys")
	default:
		return nil, errors.New("unknown credential field")
	}

	if h.getenv("HARA_SECRET_PROVIDER") == "op" {
		return h.opRead(field), nil
	}

	// The tools image is never published, and an implicit build inside `run` fails while stdout is captured.
	if !h.quiet(h.command("docker", "image", "inspect", toolsImage)) {
		build := h.compose("build", "tools")
		build.Stdout = h.stderr

		if err := h.call(build); err != nil {
			return nil, err
		}
	}

	return h.compose("run", "--rm", "--no-deps", "-T", "tools", "key", field), nil
}

// secret returns a client credential for a child process's environment.
func (h host) secret(field string) (string, error) {
	cmd, err := h.keyCommand(field)

	if err != nil {
		return "", err
	}

	value, err := h.output(cmd)

	return strings.TrimRight(value, "\n"), err
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

// codexProfile installs the public profile template as the `proxy` profile of the usual Codex home.
func (h host) codexProfile() error {
	dest := filepath.Join(h.codexHome(), "proxy.config.toml")

	if err := h.installCodexConfig(dest, defaultCodexURL); err != nil {
		return err
	}

	h.log("installed the proxy profile; run: codex --profile proxy")

	return nil
}

func (h host) codexHome() string {
	return cmp.Or(h.getenv("CODEX_HOME"), filepath.Join(h.home, ".codex"))
}

// installCodexConfig writes codex/proxy.config.toml to dest with this checkout's absolute helper
// path, and baseURL (ending in /v1) in place of the template's base_url.
func (h host) installCodexConfig(dest, baseURL string) error {
	command := filepath.Join(h.root, "bin", "hara")

	if info, err := os.Stat(command); err != nil || !info.Mode().IsRegular() {
		return errors.New("bin/hara is missing; build it with any make action, such as make codex profile")
	}

	template, err := os.ReadFile(filepath.Join(h.root, "codex", "proxy.config.toml"))

	if err != nil {
		return err
	}

	if !bytes.Contains(template, []byte(keyPlaceholder)) {
		return errors.New("codex/proxy.config.toml has no " + keyPlaceholder + " placeholder")
	}

	profile := strings.ReplaceAll(string(template), keyPlaceholder, tomlEscape(command))
	profile = baseURLLine.ReplaceAllLiteralString(profile, `base_url = "`+tomlEscape(baseURL)+`"`)

	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}

	return writePrivate(dest, []byte(profile))
}

func tomlEscape(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)
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

// codexSmoke checks the proxy profile of the usual Codex home.
// Env: CODEX_PROFILE (default proxy), PROXY_URL (default http://localhost:8317).
func (h host) codexSmoke() error {
	return h.checkCodex(codexCheck{
		home:     h.codexHome(),
		profile:  cmp.Or(h.getenv("CODEX_PROFILE"), "proxy"),
		proxyURL: cmp.Or(h.getenv("PROXY_URL"), "http://localhost:8317"),
		hint:     "make codex profile",
	})
}

// checkCodex runs one Codex turn through the proxy and fails unless it completed over the WebSocket.
// Codex falls back to HTTP by itself when the socket fails, so a turn that answers is not enough:
// the fallback warning in its output is what tells the two transports apart.
func (h host) checkCodex(check codexCheck) error {
	config := filepath.Join(check.home, "config.toml")
	args := []string{"exec", "--skip-git-repo-check"}

	if check.profile != "" {
		config = filepath.Join(check.home, check.profile+".config.toml")
		args = append(args, "--profile", check.profile)
	}

	if info, err := os.Stat(config); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("no Codex config at %s; run: %s", config, check.hint)
	}

	if !h.healthy(check.proxyURL) {
		return fmt.Errorf("the proxy does not answer on %s; run: make up", check.proxyURL)
	}

	var out bytes.Buffer

	cmd := h.command("codex", append(args, "Reply with exactly: pong")...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, &out, &out

	if len(check.env) != 0 {
		cmd.Env = append(os.Environ(), check.env...)
	}

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

	h.log("ok: Codex answered pong over the WebSocket (%s, %s)", config, check.proxyURL)

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
