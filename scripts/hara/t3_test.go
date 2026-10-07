package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const t3Template = "model_provider = \"hara-proxy\"\nbase_url = \"http://localhost:8317/v1\"\nauth.command = \"" + keyPlaceholder + "\"\n"

func newT3Host(t *testing.T, env map[string]string, installed ...string) testHost {
	t.Helper()

	h := newTestHost(t, env, installed...)

	for path, content := range map[string]string{"codex/proxy.config.toml": t3Template, "bin/hara": ""} {
		if err := os.WriteFile(filepath.Join(h.root, path), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	return h
}

func TestT3PicksTheProxyAddress(t *testing.T) {
	h := newT3Host(t, map[string]string{"T3_URL": "https://cliproxy.example.ts.net/"}, "portless")

	if settings, err := h.t3Settings(); err != nil || settings.url != "https://cliproxy.example.ts.net" || settings.ca != "" {
		t.Fatalf("T3_URL: %+v, %v", settings, err)
	}

	h = newT3Host(t, map[string]string{"HARA_PORT": "9000"}, "portless")

	if settings, _ := h.t3Settings(); settings.url != "http://localhost:9000" || settings.ca != "" {
		t.Fatalf("portless without its certificate authority: %+v", settings)
	}

	ca := filepath.Join(h.home, ".portless", "ca.pem")

	if err := os.MkdirAll(filepath.Dir(ca), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(ca, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if settings, _ := h.t3Settings(); settings.url != "https://hara.local" || settings.ca != ca {
		t.Fatalf("portless: %+v", settings)
	}

	if _, err := newT3Host(t, map[string]string{"T3_TAILSCALE_PORT": "443x"}).t3Settings(); err == nil {
		t.Fatal("accepted an invalid Tailscale port")
	}

	if err := h.dispatch([]string{"t3", "nope"}); err == nil {
		t.Fatal("accepted an unknown t3 action")
	}
}

func TestT3SetupWritesTheCodexHomeAndTailscalePort(t *testing.T) {
	h := newT3Host(t, map[string]string{"T3_URL": "https://hara.local"})
	desktop := filepath.Join(h.home, ".t3", "userdata", "desktop-settings.json")

	if err := h.dispatch([]string{"t3"}); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(h.out.String(), "T3 Code desktop settings not found") || !strings.Contains(h.out.String(), "ca.pem is missing") {
		t.Fatalf("output: %s", h.out)
	}

	if err := os.MkdirAll(filepath.Dir(desktop), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(desktop, []byte(`{"theme":"dark","tailscaleServePort":443,"window":{"width":1200}}`), 0o640); err != nil {
		t.Fatal(err)
	}

	h.out.Reset()

	if err := h.dispatch([]string{"t3", "setup"}); err != nil {
		t.Fatal(err)
	}

	config, _ := os.ReadFile(filepath.Join(h.home, ".codex-t3-hara", "config.toml"))

	if !strings.Contains(string(config), "base_url = \"https://hara.local/v1\"\n") || !strings.Contains(string(config), filepath.Join(h.root, "bin", "hara")) {
		t.Errorf("Codex config: %s", config)
	}

	var settings map[string]any

	content, _ := os.ReadFile(desktop)

	if err := json.Unmarshal(content, &settings); err != nil || settings["tailscaleServePort"] != float64(8443) || settings["theme"] != "dark" || settings["window"] == nil {
		t.Errorf("desktop settings: %s", content)
	}

	if info, _ := os.Stat(desktop); info.Mode().Perm() != 0o640 {
		t.Errorf("desktop settings mode = %v", info.Mode().Perm())
	}

	for _, want := range []string{
		"T3 Code serves Tailscale HTTPS on port 8443",
		"ANTHROPIC_BASE_URL     https://hara.local\n",
		filepath.Join(h.root, "bin", "hara") + " key claude-api-key",
		"NODE_EXTRA_CA_CERTS    " + filepath.Join(h.home, ".portless", "ca.pem"),
		"CODEX_CA_CERTIFICATE   ",
	} {
		if !strings.Contains(h.out.String(), want) {
			t.Errorf("output is missing %q:\n%s", want, h.out)
		}
	}

	if err := os.WriteFile(desktop, []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := h.dispatch([]string{"t3"}); err == nil {
		t.Error("rewrote desktop settings that are not a JSON object")
	}
}

func TestT3Smoke(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(healthy.Close)

	port := strings.TrimPrefix(healthy.URL, "http://127.0.0.1:")
	env := map[string]string{"HARA_PORT": port, "MODEL": "claude-test"}

	if err := newT3Host(t, env).dispatch([]string{"t3", "smoke"}); err == nil || !strings.Contains(err.Error(), "run: make t3") {
		t.Fatalf("smoke ran without the Codex home: %v", err)
	}

	h := newT3Host(t, env, "claude")
	codexHome := filepath.Join(h.home, ".codex-t3-hara")

	if err := h.dispatch([]string{"t3", "setup"}); err != nil {
		t.Fatal(err)
	}

	h.fake.calls = nil
	h.fake.respond = func(line string, cmd *exec.Cmd) error {
		switch {
		case strings.HasSuffix(line, "tools key claude-api-key"):
			fmt.Fprintln(cmd.Stdout, "client-key-value")
		case strings.HasPrefix(line, "claude "), strings.HasPrefix(line, "codex "):
			fmt.Fprintln(cmd.Stdout, "pong")
		}

		return nil
	}

	if err := h.dispatch([]string{"t3", "smoke"}); err != nil {
		t.Fatal(err)
	}

	claude := "claude -p --model claude-test Reply with exactly: pong"
	codex := "codex exec --skip-git-repo-check Reply with exactly: pong"
	assertCalls(t, h.fake.calls,
		"docker image inspect "+toolsImage,
		"compose run --rm --no-deps -T tools key claude-api-key",
		claude,
		codex,
	)

	for _, want := range []string{"ANTHROPIC_AUTH_TOKEN=client-key-value", "ANTHROPIC_API_KEY=", "ANTHROPIC_BASE_URL=http://localhost:" + port, "CLAUDE_CONFIG_DIR=" + filepath.Join(h.home, ".claude-hara")} {
		if !slices.Contains(h.fake.envs[claude], want) {
			t.Errorf("Claude environment is missing %s", want)
		}
	}

	if !slices.Contains(h.fake.envs[codex], "CODEX_HOME="+codexHome) || slices.ContainsFunc(h.fake.envs[codex], func(value string) bool { return strings.HasPrefix(value, "CODEX_CA_CERTIFICATE=") }) {
		t.Errorf("Codex environment: %v", h.fake.envs[codex])
	}

	if !strings.Contains(h.out.String(), "ok: Claude answered pong") || !strings.Contains(h.out.String(), "ok: Codex answered pong over the WebSocket ("+filepath.Join(codexHome, "config.toml")) {
		t.Errorf("output: %s", h.out)
	}

	skip := newT3Host(t, env)

	if err := skip.dispatch([]string{"t3", "setup"}); err != nil {
		t.Fatal(err)
	}

	skip.fake.respond = func(_ string, cmd *exec.Cmd) error { fmt.Fprintln(cmd.Stdout, "pong"); return nil }

	if err := skip.dispatch([]string{"t3", "smoke"}); err != nil || !strings.Contains(skip.out.String(), "claude is not installed") {
		t.Fatalf("err %v, output %s", err, skip.out)
	}
}
