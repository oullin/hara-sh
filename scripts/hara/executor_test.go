package main

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func executorTestHost(t *testing.T) (testHost, map[string]string, string) {
	t.Helper()

	env := map[string]string{"EXECUTOR_URL": "https://executor.example/mcp"}
	h := newT3Host(t, env, "node", "npm")
	profile := filepath.Join(h.home, ".t3", "userdata", "providers", "antigravity", "profile", "antigravity-acp")

	if err := os.MkdirAll(profile, 0o700); err != nil {
		t.Fatal(err)
	}

	header := filepath.Join(h.home, "headers.txt")
	env["EXECUTOR_HEADERS_FILE"] = header

	if err := os.WriteFile(header, []byte("X-Service-Id: fixture-client\nauthorization: Bearer fixture-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := h.dispatch([]string{"t3", "setup"}); err != nil {
		t.Fatal(err)
	}

	h.fake.calls = nil
	h.fake.respond = func(line string, cmd *exec.Cmd) error {
		if strings.HasPrefix(line, "node ") {
			fmt.Fprintln(cmd.Stdout, filepath.Join(h.home, "node"))
		}

		if strings.HasPrefix(line, "npm install ") {
			prefix := cmd.Args[3]
			proxy := filepath.Join(prefix, "node_modules", "mcp-remote", "dist", "proxy.js")

			if err := os.MkdirAll(filepath.Dir(proxy), 0o700); err != nil {
				return err
			}

			return os.WriteFile(proxy, nil, 0o600)
		}

		return nil
	}

	return h, env, filepath.Dir(profile)
}

func TestExecutorSetupPreservesProvidersAndSecretsStayPrivate(t *testing.T) {
	h, _, profile := executorTestHost(t)
	claude := filepath.Join(h.home, ".claude-hara", ".claude.json")
	cursor := filepath.Join(h.home, ".cursor", "mcp.json")

	for _, path := range []string{claude, cursor} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(`{"counter":9007199254740993,"mcpServers":{"other":{"command":"keep"},"executor":{"url":"http://127.0.0.1:4789/mcp"}}}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	codex := filepath.Join(h.home, ".codex-t3-hara", "config.toml")
	initial, _ := os.ReadFile(codex)
	initial = append(initial, []byte("\n[mcp_servers.other]\ncommand = 'keep'\n")...)

	if err := os.WriteFile(codex, initial, 0o600); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := h.dispatch([]string{"t3", "executor"}); err != nil {
			t.Fatal(err)
		}
	}

	for _, path := range []string{claude, cursor, filepath.Join(profile, "config", "mcp_config.json"), codex} {
		info, err := os.Stat(path)

		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("private config %s: %v", path, err)
		}
	}

	var c map[string]any
	content, _ := os.ReadFile(cursor)

	if err := json.Unmarshal(content, &c); err != nil {
		t.Fatal(err)
	}

	servers := c["mcpServers"].(map[string]any)

	if servers["other"].(map[string]any)["command"] != "keep" || servers["executor"].(map[string]any)["url"] != "https://executor.example/mcp" || !strings.Contains(string(content), "9007199254740993") {
		t.Fatal("Cursor repair changed unrelated configuration or lost integer precision")
	}

	content, _ = os.ReadFile(codex)

	if err := toml.Unmarshal(content, &c); err != nil {
		t.Fatal(err)
	}

	if c["model_provider"] != "hara-proxy" || c["mcp_servers"].(map[string]any)["other"].(map[string]any)["command"] != "keep" {
		t.Fatal("Codex inference or another MCP server changed")
	}

	content, _ = os.ReadFile(filepath.Join(profile, "config", "mcp_config.json"))

	if strings.Contains(string(content), "fixture-secret") || !strings.Contains(string(content), "--header-file") || !strings.Contains(string(content), "http-only") {
		t.Fatal("Antigravity must use the authenticated stdio bridge without secrets in its args")
	}

	state := filepath.Join(h.home, ".codex", "tools", "t3-executor")
	entries, _ := os.ReadDir(filepath.Join(state, "backups"))

	if len(entries) != 8 {
		t.Fatalf("want 3 originals and 5 distinct backups on rerun; got %d", len(entries))
	}

	for _, entry := range entries {
		info, _ := entry.Info()

		if info.Mode().Perm() != 0o600 {
			t.Fatal("backup is not private")
		}
	}

	for _, output := range []string{h.out.String(), h.errOut.String(), strings.Join(h.fake.calls, "\n")} {
		if strings.Contains(output, "fixture-secret") || strings.Contains(output, "fixture-client") {
			t.Fatal("credentials leaked into output or process arguments")
		}
	}
}

func TestExecutorPreflightRejectsInvalidConfigurationWithoutClientWrites(t *testing.T) {
	for _, scenario := range []string{"url", "name", "headers", "json", "toml", "private-state"} {
		t.Run(scenario, func(t *testing.T) {
			h, env, profile := executorTestHost(t)
			codex := filepath.Join(h.home, ".codex-t3-hara", "config.toml")

			switch scenario {
			case "url":
				env["EXECUTOR_URL"] = "http://executor.example/mcp"
			case "name":
				env["EXECUTOR_NAME"] = "../outside"
			case "headers":
				if err := os.WriteFile(env["EXECUTOR_HEADERS_FILE"], []byte("Authorization: first\nauthorization: second\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "json":
				path := filepath.Join(h.home, ".cursor", "mcp.json")

				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}

				if err := os.WriteFile(path, []byte(`{"mcpServers":[]}`), 0o600); err != nil {
					t.Fatal(err)
				}
			case "toml":
				if err := os.WriteFile(codex, []byte("not valid toml"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "private-state":
				env["T3_EXECUTOR_DIR"] = filepath.Join(h.root, "private")
			}

			before, _ := os.ReadFile(codex)

			if err := h.dispatch([]string{"t3", "executor"}); err == nil {
				t.Fatal("invalid configuration accepted")
			}

			after, _ := os.ReadFile(codex)

			if string(before) != string(after) || exists(filepath.Join(h.home, ".claude-hara", ".claude.json")) || exists(filepath.Join(profile, "config", "mcp_config.json")) {
				t.Fatal("failed setup changed a client configuration")
			}
		})
	}
}

func TestExecutorReportsAntigravityFailuresAndConfiguresOtherProviders(t *testing.T) {
	for _, scenario := range []string{"missing-profile", "ambiguous-profile", "invalid-override", "missing-node", "invalid-config", "install"} {
		t.Run(scenario, func(t *testing.T) {
			h, env, profile := executorTestHost(t)
			path := filepath.Join(profile, "config", "mcp_config.json")
			headerPath := filepath.Join(h.home, ".codex", "tools", "t3-executor", "executor.headers")

			if err := os.MkdirAll(filepath.Dir(headerPath), 0o700); err != nil {
				t.Fatal(err)
			}

			if err := os.WriteFile(headerPath, []byte("existing profile credentials"), 0o600); err != nil {
				t.Fatal(err)
			}

			switch scenario {
			case "missing-profile":
				if err := os.RemoveAll(profile); err != nil {
					t.Fatal(err)
				}
			case "ambiguous-profile":
				if err := os.MkdirAll(filepath.Join(filepath.Dir(profile), "second", "antigravity-acp"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "invalid-override":
				env["T3_ANTIGRAVITY_HOME"] = profile + "-missing"
			case "missing-node":
				h.look = func(string) bool { return false }
			case "invalid-config":
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}

				if err := os.WriteFile(path, []byte("invalid"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "install":
				h.fake.respond = func(line string, cmd *exec.Cmd) error {
					if strings.HasPrefix(line, "node ") {
						fmt.Fprintln(cmd.Stdout, filepath.Join(h.home, "node"))

						return nil
					}

					return errors.New("installation failed")
				}
			}

			before, _ := os.ReadFile(path)

			if err := h.dispatch([]string{"t3", "executor"}); err != nil {
				t.Fatal(err)
			}

			after, _ := os.ReadFile(path)
			header, _ := os.ReadFile(headerPath)

			if string(header) != "existing profile credentials" {
				t.Fatal("skipped Antigravity setup changed its existing credentials")
			}

			if string(before) != string(after) || !strings.Contains(h.out.String(), "Skipping Antigravity") || strings.Contains(h.out.String(), "plus Antigravity") {
				t.Fatalf("optional failure was not reported correctly: %s", h.out)
			}

			for _, client := range []string{filepath.Join(h.home, ".claude-hara", ".claude.json"), filepath.Join(h.home, ".cursor", "mcp.json"), filepath.Join(h.home, ".codex-t3-hara", "config.toml")} {
				content, err := os.ReadFile(client)

				if err != nil || !strings.Contains(string(content), "https://executor.example/mcp") {
					t.Fatalf("direct provider was not configured: %s, %v", client, err)
				}
			}
		})
	}
}

func TestExecutorNamesAndHeadersAreDynamicAndConnectionsStayIsolated(t *testing.T) {
	h, env, profile := executorTestHost(t)
	env["EXECUTOR_NAME"] = "first_tools"

	if err := h.dispatch([]string{"t3", "executor"}); err != nil {
		t.Fatal(err)
	}

	env["EXECUTOR_NAME"] = "second_tools"
	env["EXECUTOR_URL"] = "https://second.example/mcp"

	if err := os.WriteFile(env["EXECUTOR_HEADERS_FILE"], []byte("X-Api-Key: second-fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := h.dispatch([]string{"t3", "executor"}); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{filepath.Join(h.home, ".claude-hara", ".claude.json"), filepath.Join(h.home, ".cursor", "mcp.json"), filepath.Join(profile, "config", "mcp_config.json"), filepath.Join(h.home, ".codex-t3-hara", "config.toml")} {
		content, _ := os.ReadFile(path)

		if !strings.Contains(string(content), "first_tools") || !strings.Contains(string(content), "second_tools") || strings.Contains(string(content), "collections_executor") {
			t.Fatalf("connector names were not applied consistently: %s", path)
		}
	}

	state := filepath.Join(h.home, ".codex", "tools", "t3-executor")
	first, _ := os.ReadFile(filepath.Join(state, "first_tools.headers"))
	second, _ := os.ReadFile(filepath.Join(state, "second_tools.headers"))

	if !strings.Contains(string(first), "Authorization: Bearer fixture-secret") || !strings.Contains(string(second), "X-Api-Key: second-fixture") || strings.Contains(string(first), "second-fixture") {
		t.Fatal("named connections share or lost authentication headers")
	}

	var codex map[string]any
	content, _ := os.ReadFile(filepath.Join(h.home, ".codex-t3-hara", "config.toml"))

	if err := toml.Unmarshal(content, &codex); err != nil {
		t.Fatal(err)
	}

	server := codex["mcp_servers"].(map[string]any)["second_tools"].(map[string]any)

	if server["http_headers"].(map[string]any)["X-Api-Key"] != "second-fixture" {
		t.Fatal("custom Codex HTTP header was not preserved")
	}
}

func TestExecutorHeadersRejectMalformedNamesAndValues(t *testing.T) {
	for _, content := range []string{"", "# empty\n", "bad name: value\n", "Authorization: first\nauthorization: second\n", "Authorization: \n", "Authorization: value\x00injected\n", "X-Test: value\x7finjected\n", "missing-colon\n"} {
		path := filepath.Join(t.TempDir(), "headers")

		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}

		if _, err := executorHeaders(path); err == nil {
			t.Fatal("malformed header accepted")
		}
	}
}

func TestExecutorAntigravityDiscoveryRequiresOneProfile(t *testing.T) {
	h, env, profile := executorTestHost(t)

	if err := os.MkdirAll(filepath.Join(filepath.Dir(profile), "second", "antigravity-acp"), 0o700); err != nil {
		t.Fatal(err)
	}

	if _, err := h.executorAntigravityHome(); err == nil {
		t.Fatal("ambiguous profiles should not be chosen silently")
	}

	env["T3_ANTIGRAVITY_HOME"] = profile

	if got, err := h.executorAntigravityHome(); err != nil || got != profile {
		t.Fatalf("explicit profile: %s, %v", got, err)
	}
}
