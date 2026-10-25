package main

import (
	"cmp"
	"crypto/sha256"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const executorBridgeVersion = "0.14.3"

var (
	executorNamePattern       = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	executorHeaderNamePattern = regexp.MustCompile("^[!#$%&'*+\\-.^_`|~0-9A-Za-z]+$")
)

// t3Executor configures tools independently of the Hara inference endpoint. Credentials come
// from a private header file, never command arguments or repository templates.
func (h host) t3Executor(settings t3Settings) error {
	endpoint := h.getenv("EXECUTOR_URL")

	if err := validateExecutorURL(endpoint); err != nil {
		return err
	}

	name := cmp.Or(h.getenv("EXECUTOR_NAME"), "executor")

	if !executorNamePattern.MatchString(name) {
		return errors.New("EXECUTOR_NAME must contain only letters, digits, underscores and hyphens")
	}

	headers, err := executorHeaders(h.getenv("EXECUTOR_HEADERS_FILE"))

	if err != nil {
		return err
	}

	state := cmp.Or(h.getenv("T3_EXECUTOR_DIR"), filepath.Join(h.home, ".codex", "tools", "t3-executor"))
	headerPath := filepath.Join(state, name+".headers")
	claudePath := filepath.Join(settings.claudeHome, ".claude.json")
	cursorPath := filepath.Join(settings.cursorHome, "mcp.json")
	codexPath := filepath.Join(settings.codexHome, "config.toml")

	// Prepare and validate every config before installing packages or replacing any client file.
	direct := map[string]any{"url": endpoint, "headers": headers}
	configs := map[string][]byte{}

	for _, client := range []struct {
		path   string
		server map[string]any
	}{
		{claudePath, map[string]any{"type": "http", "url": endpoint, "headers": headers}},
		{cursorPath, direct},
	} {
		content, err := executorJSON(client.path, name, client.server)

		if err != nil {
			return err
		}

		configs[client.path] = content
	}

	configs[codexPath], err = executorTOML(codexPath, name, endpoint, headers)

	if err != nil {
		return err
	}

	for _, dir := range []string{state, filepath.Join(state, "backups"), filepath.Dir(claudePath), filepath.Dir(cursorPath), filepath.Dir(codexPath)} {
		if err := privateDir(dir, h.root); err != nil {
			return err
		}
	}

	providers := "Claude, Codex and Cursor"

	if path, content, err := h.executorAntigravityConfig(state, name, endpoint, headerPath); err != nil {
		h.log("! Skipping Antigravity: %v", err)
	} else {
		configs[path] = content
		configs[headerPath] = executorHeaderText(headers)
		providers += ", plus Antigravity"
	}

	for path, content := range configs {
		if err := executorWrite(path, content, filepath.Join(state, "backups")); err != nil {
			return err
		}
	}

	h.log("Installed %s MCP access for T3 %s. Private backups: %s", name, providers, filepath.Join(state, "backups"))
	h.log("Start fresh T3 chats. Wait for Claude MCP startup; approve %s in Cursor if prompted. Verify skills and execute in each provider; use Grok in Cursor when other models have exhausted their quota.", name)

	return nil
}

func executorHeaderText(headers map[string]string) []byte {
	var content strings.Builder

	for _, name := range slices.Sorted(maps.Keys(headers)) {
		fmt.Fprintf(&content, "%s: %s\n", name, headers[name])
	}

	return []byte(content.String())
}

// Antigravity is optional: profile, runtime or bridge failures are reported by the caller while
// the direct HTTP providers continue. Each named connection has its own credential file.
func (h host) executorAntigravityConfig(state, name, endpoint, headerPath string) (string, []byte, error) {
	profile, err := h.executorAntigravityHome()

	if err != nil {
		return "", nil, err
	}

	node, err := h.executorNode()

	if err != nil {
		return "", nil, err
	}

	bridge := filepath.Join(state, "remote-bridge")
	proxyPath := filepath.Join(bridge, "node_modules", "mcp-remote", "dist", "proxy.js")
	path := filepath.Join(profile, "config", "mcp_config.json")
	stdio := map[string]any{
		"command": node,
		"args":    []string{proxyPath, endpoint, "--transport", "http-only", "--header-file", headerPath},
		"env":     map[string]string{"MCP_REMOTE_CONFIG_DIR": filepath.Join(state, "auth")},
	}
	content, err := executorJSON(path, name, stdio)

	if err != nil {
		return "", nil, err
	}

	for _, dir := range []string{filepath.Join(state, "npm-cache"), filepath.Join(state, "auth"), filepath.Dir(path)} {
		if err := privateDir(dir, h.root); err != nil {
			return "", nil, err
		}
	}

	if err := h.executorInstall(state, bridge, proxyPath); err != nil {
		return "", nil, err
	}

	return path, content, nil
}

func validateExecutorURL(endpoint string) error {
	parsed, err := url.Parse(endpoint)

	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("EXECUTOR_URL must be an HTTPS MCP endpoint without credentials, query or fragment")
	}

	return nil
}

func (h host) executorNode() (string, error) {
	if !h.look("node") || !h.look("npm") {
		return "", errors.New("Node.js and npm are required for the Antigravity MCP bridge")
	}

	node, err := h.output(h.command("node", "-p", "process.execPath"))

	if err != nil {
		return "", err
	}

	node = strings.TrimSpace(node)

	if !filepath.IsAbs(node) {
		return "", errors.New("Node.js did not report an absolute executable path")
	}

	return node, nil
}

func (h host) executorInstall(state, bridge, proxyPath string) error {
	install := h.command("npm", "install", "--prefix", bridge, "--no-audit", "--no-fund", "--no-package-lock", "mcp-remote@"+executorBridgeVersion)
	install.Env = append(os.Environ(), "npm_config_cache="+filepath.Join(state, "npm-cache"))

	if err := h.call(install); err != nil {
		return err
	}

	if !exists(proxyPath) {
		return errors.New("the MCP bridge installation did not produce proxy.js")
	}

	return nil
}

func executorHeaders(path string) (map[string]string, error) {
	if path == "" {
		return nil, errors.New("EXECUTOR_HEADERS_FILE must name a private HTTP header file")
	}

	content, err := os.ReadFile(path)

	if err != nil {
		return nil, err
	}

	headers := map[string]string{}

	for line := range strings.SplitSeq(string(content), "\n") {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, value, ok := strings.Cut(line, ":")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)

		if !ok || !executorHeaderNamePattern.MatchString(name) || !validExecutorHeaderValue(value) {
			return nil, errors.New("Executor header file contains an invalid HTTP header")
		}

		name = http.CanonicalHeaderKey(name)

		if _, present := headers[name]; present {
			return nil, errors.New("Executor header file contains a duplicate HTTP header")
		}

		headers[name] = value
	}

	if len(headers) == 0 {
		return nil, errors.New("Executor header file must contain at least one HTTP header")
	}

	return headers, nil
}

func validExecutorHeaderValue(value string) bool {
	if value == "" {
		return false
	}

	for _, char := range value {
		if char == 127 || (char < 32 && char != '\t') {
			return false
		}
	}

	return true
}

func (h host) executorAntigravityHome() (string, error) {
	if profile := h.getenv("T3_ANTIGRAVITY_HOME"); profile != "" {
		if !isDir(profile) {
			return "", errors.New("T3_ANTIGRAVITY_HOME must name an existing T3 Antigravity profile")
		}

		return profile, nil
	}

	base := filepath.Join(cmp.Or(h.getenv("T3_HOME"), filepath.Join(h.home, ".t3")), "userdata", "providers", "antigravity")
	entries, err := os.ReadDir(base)

	if err != nil {
		return "", errors.New("set up Antigravity in T3 first, or set T3_ANTIGRAVITY_HOME")
	}

	var profiles []string

	for _, entry := range entries {
		if entry.IsDir() && isDir(filepath.Join(base, entry.Name(), "antigravity-acp")) {
			profiles = append(profiles, filepath.Join(base, entry.Name()))
		}
	}

	if len(profiles) != 1 {
		return "", errors.New("set T3_ANTIGRAVITY_HOME to the intended profile; automatic discovery requires exactly one")
	}

	return profiles[0], nil
}

func executorJSON(path, name string, server map[string]any) ([]byte, error) {
	content, err := os.ReadFile(path)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	config := map[string]jsontext.Value{}

	if err == nil && (json.Unmarshal(content, &config) != nil || config == nil) {
		return nil, fmt.Errorf("%s is not a valid JSON object", path)
	}

	servers := map[string]jsontext.Value{}

	if raw, ok := config["mcpServers"]; ok {
		if json.Unmarshal(raw, &servers) != nil || servers == nil {
			return nil, fmt.Errorf("mcpServers in %s is not a JSON object", path)
		}
	}

	servers[name], err = json.Marshal(server)

	if err != nil {
		return nil, err
	}

	config["mcpServers"], err = json.Marshal(servers)

	if err != nil {
		return nil, err
	}

	return json.Marshal(config, jsontext.WithIndent("  "))
}

func executorTOML(path, name, endpoint string, headers map[string]string) ([]byte, error) {
	content, err := os.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("read T3 Codex config: %w; run make t3 first", err)
	}

	config := map[string]any{}

	if err := toml.Unmarshal(content, &config); err != nil {
		return nil, fmt.Errorf("%s is not valid TOML", path)
	}

	servers, ok := config["mcp_servers"].(map[string]any)

	if _, present := config["mcp_servers"]; present && !ok {
		return nil, errors.New("T3 Codex mcp_servers must be a table")
	}

	if servers == nil {
		servers = map[string]any{}
	}

	servers[name] = map[string]any{"url": endpoint, "http_headers": headers}
	config["mcp_servers"] = servers

	return toml.Marshal(config)
}

// executorWrite preserves the previous bytes in a unique private backup before an atomic update.
func executorWrite(path string, content []byte, backups string) error {
	previous, err := os.ReadFile(path)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err == nil {
		id := sha256.Sum256([]byte(path))
		backup := filepath.Join(backups, fmt.Sprintf("%x-%s", id[:8], time.Now().UTC().Format("20060102T150405.000000000")))

		if err := writePrivate(backup, previous); err != nil {
			return err
		}
	}

	return writePrivate(path, append(content, '\n'))
}
