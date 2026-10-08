package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// t3Settings are T3 Code's provider settings; its Claude and Codex instances run the CLIs, so each
// one gets the proxy URL and key the same way `make claude` and `make codex` do.
type t3Settings struct {
	url        string
	local      string
	ca         string
	codexHome  string
	claudeHome string
	desktop    string
	port       int
}

// t3 prepares (setup) or checks (smoke) T3 Code's provider instances.
// Env: T3_URL (default https://hara.local when portless is installed, else http://localhost:8317),
// T3_CODEX_HOME (default ~/.codex-t3-hara), T3_CLAUDE_HOME (default ~/.claude-hara),
// T3_HOME (default ~/.t3), T3_TAILSCALE_PORT (default 8443), MODEL (Claude model for smoke).
func (h host) t3(args []string) error {
	if len(args) > 1 {
		return errors.New("usage: hara t3 setup|smoke")
	}

	settings, err := h.t3Settings()

	if err != nil {
		return err
	}

	action := "setup"

	if len(args) == 1 {
		action = args[0]
	}

	switch action {
	case "setup":
		return h.t3Setup(settings)
	case "smoke":
		return h.t3Smoke(settings)
	default:
		return errors.New("usage: hara t3 setup|smoke")
	}
}

func (h host) t3Settings() (t3Settings, error) {
	portless := filepath.Join(h.home, ".portless", "ca.pem")
	settings := t3Settings{
		local:      "http://localhost:" + h.port(),
		codexHome:  cmp.Or(h.getenv("T3_CODEX_HOME"), filepath.Join(h.home, ".codex-t3-hara")),
		claudeHome: cmp.Or(h.getenv("T3_CLAUDE_HOME"), filepath.Join(h.home, ".claude-hara")),
		desktop:    filepath.Join(cmp.Or(h.getenv("T3_HOME"), filepath.Join(h.home, ".t3")), "userdata", "desktop-settings.json"),
	}

	port, err := strconv.Atoi(cmp.Or(h.getenv("T3_TAILSCALE_PORT"), "8443"))

	if err != nil || port < 1 || port > 65535 {
		return settings, errors.New("T3_TAILSCALE_PORT must be a port number")
	}

	settings.port = port

	switch {
	case h.getenv("T3_URL") != "":
		settings.url = strings.TrimSuffix(h.getenv("T3_URL"), "/")
	case h.look("portless") && exists(portless):
		settings.url = "https://hara.local"
	default:
		settings.url = settings.local
	}

	// Only hara.local needs a certificate authority named: localhost is plain HTTP, and Tailscale
	// certificates are publicly trusted. Claude Code and Codex ignore the macOS keychain's copy.
	if parsed, err := url.Parse(settings.url); err == nil && parsed.Scheme == "https" && strings.HasSuffix(parsed.Hostname(), ".local") {
		settings.ca = portless
	}

	return settings, nil
}

// caEnv names the certificate authority only when needed; an empty path is not "unset" to every client.
func (settings t3Settings) caEnv() []string {
	if settings.ca == "" {
		return nil
	}

	return []string{"NODE_EXTRA_CA_CERTS=" + settings.ca, "CODEX_CA_CERTIFICATE=" + settings.ca}
}

func (h host) t3Setup(settings t3Settings) error {
	if err := h.installCodexConfig(filepath.Join(settings.codexHome, "config.toml"), settings.url+"/v1"); err != nil {
		return err
	}

	h.log("- Codex home for T3 Code: %s (%s/v1)", filepath.Join(settings.codexHome, "config.toml"), settings.url)

	if err := h.t3TailscalePort(settings); err != nil {
		return err
	}

	if settings.ca != "" && !exists(settings.ca) {
		h.log("! %s is missing; run make portless, or set T3_URL=%s", settings.ca, settings.local)
	}

	lines := []string{
		"",
		"Quit T3 Code (Cmd-Q) and reopen it before changing any setting; it rewrites its settings from memory.",
		"Then add two provider instances in Settings -> Providers:",
		"",
		"Claude",
		"  CLAUDE_CONFIG_DIR path   " + settings.claudeHome,
		"  Environment",
		"    ANTHROPIC_BASE_URL     " + settings.url,
		"    ANTHROPIC_AUTH_TOKEN   output of: " + filepath.Join(h.root, "bin", "hara") + " key claude-api-key   (Sensitive)",
		"    ANTHROPIC_API_KEY      empty value",
	}

	if settings.ca != "" {
		lines = append(lines, "    NODE_EXTRA_CA_CERTS    "+settings.ca)
	}

	lines = append(lines, "", "Codex", "  CODEX_HOME path          "+settings.codexHome, "  Shadow home path         empty")

	if settings.ca != "" {
		lines = append(lines, "  Environment", "    CODEX_CA_CERTIFICATE   "+settings.ca)
	}

	lines = append(lines, "", "Tailscale HTTPS can now be switched on in T3 Code. Check both instances with: make t3 smoke")
	h.log("%s", strings.Join(lines, "\n"))

	return nil
}

// t3TailscalePort moves T3 Code's Tailscale Serve port. Portless in LAN mode listens on 443 on every
// interface, the tailnet address included, so it answers T3 Code's Tailscale Serve URL with its own
// certificate and a 404, and T3 Code turns the switch off.
func (h host) t3TailscalePort(settings t3Settings) error {
	content, err := os.ReadFile(settings.desktop)

	if errors.Is(err, os.ErrNotExist) {
		h.log("- T3 Code desktop settings not found at %s; open T3 Code once and rerun", settings.desktop)

		return nil
	}

	if err != nil {
		return err
	}

	var desktop map[string]json.RawMessage

	if err := json.Unmarshal(content, &desktop); err != nil || desktop == nil {
		return fmt.Errorf("%s is not a JSON object", settings.desktop)
	}

	// A running T3 Code writes this file from memory, so leave it alone when nothing changes.
	if string(desktop["tailscaleServePort"]) == strconv.Itoa(settings.port) {
		h.log("- T3 Code already serves Tailscale HTTPS on port %d (%s)", settings.port, settings.desktop)

		return nil
	}

	desktop["tailscaleServePort"] = json.RawMessage(strconv.Itoa(settings.port))
	updated, err := json.Marshal(desktop)

	if err != nil {
		return err
	}

	// Rewrite in place so the file keeps its owner and mode.
	if err := os.WriteFile(settings.desktop, append(updated, '\n'), 0o600); err != nil {
		return err
	}

	h.log("- T3 Code serves Tailscale HTTPS on port %d (%s)", settings.port, settings.desktop)

	return nil
}

// t3Smoke sends one Claude and one Codex request with the instance settings; it uses provider quota.
func (h host) t3Smoke(settings t3Settings) error {
	if !exists(filepath.Join(settings.codexHome, "config.toml")) {
		return fmt.Errorf("no Codex home at %s; run: make t3", settings.codexHome)
	}

	if err := h.t3Claude(settings); err != nil {
		return err
	}

	return h.checkCodex(codexCheck{
		home:     settings.codexHome,
		proxyURL: settings.local,
		hint:     "make t3",
		env:      append(settings.caEnv(), "CODEX_HOME="+settings.codexHome),
	})
}

func (h host) t3Claude(settings t3Settings) error {
	if !h.look("claude") {
		h.log("- claude is not installed; skipping the Claude check")

		return nil
	}

	key, err := h.secret(apiField)

	if err != nil {
		return err
	}

	var out strings.Builder

	model := cmp.Or(h.getenv("MODEL"), "claude-haiku-4-5-20251001")
	cmd := h.command("claude", "-p", "--model", model, "Reply with exactly: pong")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, &out, &out
	cmd.Env = append(append(os.Environ(), settings.caEnv()...),
		"CLAUDE_CONFIG_DIR="+settings.claudeHome,
		"ANTHROPIC_BASE_URL="+settings.url,
		"ANTHROPIC_AUTH_TOKEN="+key,
		"ANTHROPIC_API_KEY=",
	)

	if err := h.call(cmd); err != nil {
		fmt.Fprint(h.stderr, out.String())

		return fmt.Errorf("the Claude request failed (%s)", settings.url)
	}

	if !slices.Contains(strings.Split(out.String(), "\n"), "pong") {
		fmt.Fprint(h.stderr, out.String())

		return errors.New("Claude did not answer pong")
	}

	h.log("ok: Claude answered pong (%s)", settings.url)

	return nil
}
