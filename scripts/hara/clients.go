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

	"github.com/pelletier/go-toml/v2"
)

// codexCheck is one Codex smoke run: the Codex home, its profile (empty uses config.toml), the
// proxy to probe, the command that installs the config, whether that command wrote it from
// codex/proxy.config.toml, and extra child environment.
type codexCheck struct {
	home      string
	profile   string
	proxyURL  string
	hint      string
	installed bool
	env       []string
}

// tomlTable is one [header] line of a TOML file with the lines that follow it, up to the next header,
// and the comment lines right above the header.
type tomlTable struct {
	name     string
	comments []string
	lines    []string
}

const (
	apiField        = "claude-api-key"
	managementField = "management-password"
	toolsImage      = "hara-tools:local"
	keyPlaceholder  = "__HARA_KEY_COMMAND__"
	fallbackWarning = "Falling back from WebSockets"
	defaultCodexURL = "http://localhost:8317/v1"
	proxyProfile    = "proxy"
	// Hara's own table in a Codex config, replaced whole by `make codex app`.
	haraProviderTable = "model_providers.hara-proxy"
)

var (
	baseURLLine     = regexp.MustCompile(`(?m)^base_url = .*$`)
	authCommandLine = regexp.MustCompile(`^\s*auth\.command\s*=\s*"((?:[^"\\]|\\.)*)"`)
	providerLine    = regexp.MustCompile(`^\s*model_provider\s*=\s*"((?:[^"\\]|\\.)*)"`)
	tomlHeaderLine  = regexp.MustCompile(`^\s*\[\[?\s*([^\]]*?)\s*\]\]?\s*(?:#.*)?$`)
	tomlKeyLine     = regexp.MustCompile(`^\s*("?[\w.-]+"?)\s*=\s*(.*?)\s*$`)
	tomlNameQuotes  = strings.NewReplacer(`"`, "", `'`, "", " ", "")
)

// key prints a credential from private local state; HARA_SECRET_PROVIDER=op reads 1Password instead.
// Codex runs it as its auth command, so standard output carries the credential and nothing else.
func (h host) key(args []string) error {
	if len(args) > 1 {
		return errors.New("usage: hara key [claude-api-key|management-password]")
	}

	field := apiField

	if len(args) == 1 {
		field = args[0]
	}

	value, err := h.secret(field)

	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(h.stdout, value)

	return err
}

// keyCommand returns the command that prints field on standard output, building the tools image first if needed.
func (h host) keyCommand(field string) (*exec.Cmd, error) {
	if err := validateCredentialField(field); err != nil {
		return nil, err
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
	if h.getenv("HARA_SECRET_PROVIDER") != "op" {
		value, err := localCredential(h.state, field)

		if !errors.Is(err, os.ErrNotExist) {
			return value, err
		}
	}

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
		"codex":  {"Codex", h.codexHome(), []string{"config.toml", "openai.config.toml", "proxy.config.toml"}},
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
	dest := filepath.Join(h.codexHome(), proxyProfile+".config.toml")

	if err := h.installCodexConfig(dest, defaultCodexURL); err != nil {
		return err
	}

	h.log("installed the proxy profile; run: codex --profile proxy")
	h.warnMissingAuthCommands(h.codexHome())

	return nil
}

// codexApp makes Hara the provider of the Codex app, which reads config.toml of the usual Codex home
// and takes no --profile from the Dock. The template's model_provider, [features] keys and
// [model_providers.hara-proxy] table are merged into the file in place; every other line stays, and the
// previous file goes to the private backups first.
func (h host) codexApp() error {
	if err := h.backup([]string{"codex"}); err != nil {
		return err
	}

	rendered, err := h.renderCodexConfig(defaultCodexURL)

	if err != nil {
		return err
	}

	dest := filepath.Join(h.codexHome(), "config.toml")
	current, err := os.ReadFile(dest)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	merged, err := mergeCodexApp(string(current), rendered)

	if err != nil {
		return fmt.Errorf("%s: %w", dest, err)
	}

	if err := checkCodexApp(merged, filepath.Join(h.root, "bin", "hara"), defaultCodexURL); err != nil {
		return fmt.Errorf("%s: %w", dest, err)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}

	if err := writePrivate(dest, []byte(merged)); err != nil {
		return err
	}

	h.log("installed the Hara provider in %s; quit and reopen the Codex app (plain codex uses it too)", dest)
	h.warnMissingAuthCommands(h.codexHome())

	return nil
}

// mergeCodexApp applies the rendered template to current: top-level keys and the keys of tables the
// app also writes, such as [features], are set in place, Hara's own provider table is replaced whole,
// and what the template lacks is left alone. A file that is not TOML is reported, not rewritten.
func mergeCodexApp(current, rendered string) (string, error) {
	if strings.TrimSpace(current) != "" {
		if err := toml.Unmarshal([]byte(current), &map[string]any{}); err != nil {
			return "", fmt.Errorf("not valid TOML: %w", err)
		}
	}

	preamble, tables := splitTOML(current)
	keys, additions := splitTOML(rendered)
	preamble = mergeTOMLKeys(preamble, keys)

	for _, addition := range additions {
		i := slices.IndexFunc(tables, func(table tomlTable) bool { return table.name == addition.name })

		switch {
		case i < 0:
			tables = append(tables, addition)
		case addition.name == haraProviderTable:
			tables[i].lines = addition.lines
		default:
			tables[i].lines = mergeTOMLKeys(tables[i].lines, addition.lines)
		}
	}

	return joinTOML(preamble, tables), nil
}

// checkCodexApp parses content and confirms that Codex would reach Hara through it; the merge keeps
// the file's own layout, so a key set in a way it does not see, such as an inline table, surfaces here.
func checkCodexApp(content, command, baseURL string) error {
	config := map[string]any{}

	if err := toml.Unmarshal([]byte(content), &config); err != nil {
		return fmt.Errorf("the merged file is not valid TOML (%w); add the provider by hand", err)
	}

	for _, want := range []struct {
		path  string
		value any
	}{
		{"model_provider", "hara-proxy"},
		{"features.instant_interrupt", false},
		{haraProviderTable + ".base_url", baseURL},
		{haraProviderTable + ".auth.command", command},
	} {
		if got := tomlLookup(config, want.path); got != want.value {
			return fmt.Errorf("the merged file sets %s = %v, not %v; add the provider by hand", want.path, got, want.value)
		}
	}

	return nil
}

func tomlLookup(config map[string]any, path string) any {
	var value any = config

	for _, key := range strings.Split(path, ".") {
		table, ok := value.(map[string]any)

		if !ok {
			return nil
		}

		value = table[key]
	}

	return value
}

// splitTOML returns the lines before the first table header and each table with its header line.
// Comments and blank lines stay with the lines they follow, except comments right above a header,
// which describe that table. Names drop quotes: [a."b"] is a.b.
func splitTOML(content string) ([]string, []tomlTable) {
	var (
		preamble []string
		tables   []tomlTable
	)

	previous := &preamble

	for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		if match := tomlHeaderLine.FindStringSubmatch(line); match != nil {
			var comments []string

			*previous, comments = detachComments(*previous)
			tables = append(tables, tomlTable{name: tomlNameQuotes.Replace(match[1]), comments: comments, lines: []string{line}})
			previous = &tables[len(tables)-1].lines
		} else {
			*previous = append(*previous, line)
		}
	}

	return preamble, tables
}

// detachComments splits the run of comment lines that ends lines off it.
func detachComments(lines []string) ([]string, []string) {
	end := len(lines)

	for end > 0 && strings.HasPrefix(strings.TrimSpace(lines[end-1]), "#") {
		end--
	}

	return lines[:end], lines[end:]
}

// joinTOML is the inverse of splitTOML, with one blank line between the parts.
func joinTOML(preamble []string, tables []tomlTable) string {
	chunks := []string{strings.Join(trimBlankLines(preamble), "\n")}

	for _, table := range tables {
		chunks = append(chunks, strings.Join(append(slices.Clone(table.comments), trimBlankLines(table.lines)...), "\n"))
	}

	return strings.Join(slices.DeleteFunc(chunks, func(chunk string) bool { return chunk == "" }), "\n\n") + "\n"
}

// mergeTOMLKeys sets each key assigned in from on into, replacing its line or appending one.
func mergeTOMLKeys(into, from []string) []string {
	for _, line := range from {
		if match := tomlKeyLine.FindStringSubmatch(line); match != nil {
			into = setTOMLKey(into, strings.Trim(match[1], `"`), match[2])
		}
	}

	return into
}

func setTOMLKey(lines []string, key, value string) []string {
	assigned := regexp.MustCompile(`^\s*"?` + regexp.QuoteMeta(key) + `"?\s*=`)

	if i := slices.IndexFunc(lines, assigned.MatchString); i >= 0 {
		lines[i] = key + " = " + value

		return lines
	}

	return append(trimBlankLines(lines), key+" = "+value)
}

func trimBlankLines(lines []string) []string {
	blank := func(line string) bool { return strings.TrimSpace(line) == "" }
	start, end := 0, len(lines)

	for start < end && blank(lines[start]) {
		start++
	}

	for end > start && blank(lines[end-1]) {
		end--
	}

	return lines[start:end]
}

// warnMissingAuthCommands names each Codex config in home whose auth command no longer exists. An
// install rewrites only its own file, so a provider added to config.toml by hand keeps calling a removed
// helper, such as the scripts/hara-key that bin/hara replaced, and Codex then sends no key.
func (h host) warnMissingAuthCommands(home string) {
	paths, _ := filepath.Glob(filepath.Join(home, "*.toml"))

	for _, path := range paths {
		content, err := os.ReadFile(path)

		if err != nil {
			continue
		}

		for number, line := range strings.Split(string(content), "\n") {
			match := authCommandLine.FindStringSubmatch(line)

			if match == nil {
				continue
			}

			if command := tomlUnescape(match[1]); filepath.IsAbs(command) && !exists(command) {
				h.log(`! %s:%d runs %s, which no longer exists; set auth.command = "%s" and auth.args = ["key", "%s"], then restart Codex`,
					path, number+1, command, tomlEscape(filepath.Join(h.root, "bin", "hara")), apiField)
			}
		}
	}
}

func (h host) codexHome() string {
	return cmp.Or(h.getenv("CODEX_HOME"), filepath.Join(h.home, ".codex"))
}

// installCodexConfig writes the rendered codex/proxy.config.toml to dest.
func (h host) installCodexConfig(dest, baseURL string) error {
	profile, err := h.renderCodexConfig(baseURL)

	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}

	return writePrivate(dest, []byte(profile))
}

// renderCodexConfig returns codex/proxy.config.toml with this checkout's absolute helper path, and
// baseURL (ending in /v1) in place of the template's base_url.
func (h host) renderCodexConfig(baseURL string) (string, error) {
	command := filepath.Join(h.root, "bin", "hara")

	if info, err := os.Stat(command); err != nil || !info.Mode().IsRegular() {
		return "", errors.New("bin/hara is missing; build it with any make action, such as make codex profile")
	}

	template, err := os.ReadFile(filepath.Join(h.root, "codex", "proxy.config.toml"))

	if err != nil {
		return "", err
	}

	if !bytes.Contains(template, []byte(keyPlaceholder)) {
		return "", errors.New("codex/proxy.config.toml has no " + keyPlaceholder + " placeholder")
	}

	profile := strings.ReplaceAll(string(template), keyPlaceholder, tomlEscape(command))

	return baseURLLine.ReplaceAllLiteralString(profile, `base_url = "`+tomlEscape(baseURL)+`"`), nil
}

func tomlEscape(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)
}

func tomlUnescape(value string) string {
	return strings.NewReplacer(`\\`, `\`, `\"`, `"`).Replace(value)
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
	profile := cmp.Or(h.getenv("CODEX_PROFILE"), proxyProfile)

	return h.checkCodex(codexCheck{
		home:      h.codexHome(),
		profile:   profile,
		proxyURL:  cmp.Or(h.getenv("PROXY_URL"), "http://localhost:8317"),
		hint:      "make codex profile",
		installed: profile == proxyProfile,
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

	if check.installed {
		if err := h.checkCodexProvider(config, check.hint); err != nil {
			return err
		}
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

// checkCodexProvider fails unless a config installed from codex/proxy.config.toml still selects the
// template's provider. Without it Codex uses its built-in OpenAI provider and the smoke turn never
// reaches Hara; this happens when Codex recreates a missing profile file with only its own state.
// Profiles chosen with CODEX_PROFILE are the user's own and may name their provider differently.
func (h host) checkCodexProvider(config, hint string) error {
	template := filepath.Join(h.root, "codex", "proxy.config.toml")
	want, err := codexModelProvider(template)

	if err != nil {
		return err
	}

	if want == "" {
		return errors.New(template + " sets no model_provider")
	}

	got, err := codexModelProvider(config)

	if err != nil {
		return err
	}

	if got != want {
		return fmt.Errorf("%s does not route Codex through Hara (model_provider is %q, want %q); run: %s", config, got, want, hint)
	}

	return nil
}

// codexModelProvider returns the top-level model_provider of a Codex config, or "" when it sets none.
func codexModelProvider(path string) (string, error) {
	content, err := os.ReadFile(path)

	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(content), "\n") {
		// Keys after the first table header belong to that table, not to the top level.
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			break
		}

		if match := providerLine.FindStringSubmatch(line); match != nil {
			return tomlUnescape(match[1]), nil
		}
	}

	return "", nil
}

func (h host) healthy(proxyURL string) bool {
	response, err := h.client.Get(strings.TrimRight(proxyURL, "/") + "/healthz")

	if err != nil {
		return false
	}

	defer response.Body.Close()

	return response.StatusCode < http.StatusBadRequest
}
