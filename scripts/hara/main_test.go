package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// fake records each command as one line, with the Compose prefix shortened to "compose".
type fake struct {
	root    string
	calls   []string
	envs    map[string][]string
	stdin   map[string]string
	respond func(line string, cmd *exec.Cmd) error
}

type testHost struct {
	host
	fake   *fake
	out    *bytes.Buffer
	errOut *bytes.Buffer
}

func (f *fake) run(cmd *exec.Cmd) error {
	line := strings.Replace(strings.Join(cmd.Args, " "), "docker compose -f "+filepath.Join(f.root, "local", "compose.yaml"), "compose", 1)
	f.calls = append(f.calls, line)
	f.envs[line] = cmd.Env

	if reader, ok := cmd.Stdin.(*strings.Reader); ok {
		content, _ := io.ReadAll(reader)
		f.stdin[line] = string(content)
	}

	if f.respond == nil {
		return nil
	}

	return f.respond(line, cmd)
}

func newTestHost(t *testing.T, env map[string]string, installed ...string) testHost {
	t.Helper()

	root := t.TempDir()

	for _, dir := range []string{"local", "codex", "panel", "bin"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	f := &fake{root: root, envs: map[string][]string{}, stdin: map[string]string{}}
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	home := t.TempDir()

	return testHost{
		host: host{
			root:   root,
			home:   home,
			state:  filepath.Join(home, ".hara-sh"),
			getenv: func(name string) string { return env[name] },
			stdin:  strings.NewReader(""),
			stdout: out,
			stderr: errOut,
			exec:   f.run,
			look:   func(name string) bool { return slices.Contains(installed, name) },
			sleep:  func(time.Duration) {},
			client: &http.Client{Timeout: time.Second},
		},
		fake:   f,
		out:    out,
		errOut: errOut,
	}
}

func failing(lines ...string) func(string, *exec.Cmd) error {
	return func(line string, _ *exec.Cmd) error {
		if slices.Contains(lines, line) {
			return exitStatus(1)
		}

		return nil
	}
}

func assertCalls(t *testing.T, got []string, want ...string) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Fatalf("commands:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestConfigureAppliesPrivateSettingsAndDefaults(t *testing.T) {
	home := t.TempDir()
	state := filepath.Join(t.TempDir(), "state")
	file := filepath.Join(t.TempDir(), "credentials.env")
	content := "# names only\n: \"${OP_ACCOUNT:=ignored.1password.com}\"\n: \"${OP_VAULT:=Work}\"\nOP_ITEM_NAME='hara item'\n\nexport LOCAL_DIR=" + state + "\n"

	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{"HARA_ENV_FILE": file, "OP_ACCOUNT": "team.1password.com"}
	got, err := configure(home, func(name string) string { return env[name] }, func(name, value string) error { env[name] = value; return nil })

	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{"OP_ACCOUNT": "team.1password.com", "OP_VAULT": "Work", "OP_ITEM_NAME": "hara item", "LOCAL_DIR": state}

	for name, value := range want {
		if env[name] != value {
			t.Errorf("%s = %q, want %q", name, env[name], value)
		}
	}

	if got != state {
		t.Errorf("state = %q, want %q", got, state)
	}
}

func TestConfigureDefaultsAndRejections(t *testing.T) {
	home := t.TempDir()
	env := map[string]string{}
	getenv := func(name string) string { return env[name] }
	setenv := func(name, value string) error { env[name] = value; return nil }

	if state, err := configure(home, getenv, setenv); err != nil || state != filepath.Join(home, ".hara-sh") || env["OP_ITEM_NAME"] != "cli-proxy-api" {
		t.Fatalf("defaults: state %q, err %v, env %v", state, err, env)
	}

	if err := os.Mkdir(filepath.Join(home, ".cli-proxy-api"), 0o700); err != nil {
		t.Fatal(err)
	}

	clear(env)

	if _, err := configure(home, getenv, setenv); err == nil || !strings.Contains(err.Error(), "mv ~/.cli-proxy-api ~/.hara-sh") {
		t.Fatalf("legacy state was not refused: %v", err)
	}

	file := filepath.Join(t.TempDir(), "credentials.env")

	if err := os.WriteFile(file, []byte("OP_VAULT=Work\n: \"${OP_VAULT:-Work}\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	env = map[string]string{"HARA_ENV_FILE": file, "LOCAL_DIR": t.TempDir()}

	if _, err := configure(home, getenv, setenv); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("invalid settings line was accepted: %v", err)
	}
}

func TestPrivateDirRefusesTheCheckout(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "state")

	if err := privateDir(outside, root); err != nil {
		t.Fatal(err)
	}

	if info, _ := os.Stat(outside); info.Mode().Perm() != 0o700 {
		t.Errorf("mode = %v, want 0700", info.Mode().Perm())
	}

	link := filepath.Join(t.TempDir(), "link")

	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}

	for _, dir := range []string{filepath.Join(root, "backups"), filepath.Join(link, "backups"), root} {
		if err := privateDir(dir, root); err == nil {
			t.Errorf("accepted %s inside the checkout", dir)
		}
	}
}

func TestDispatchRejectsUnknownCommands(t *testing.T) {
	h := newTestHost(t, nil)

	for _, args := range [][]string{nil, {"nope"}, {"down", "extra"}, {"backup"}, {"backup", "vim"}, {"bench", "1", "a", "b", "c"}} {
		if err := h.dispatch(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
}

func TestToolsMapsLoopbackToTheProxyService(t *testing.T) {
	for _, tc := range []struct{ url, port, want string }{
		{"", "", "http://proxy:8317"},
		{"http://localhost:8317", "", "http://proxy:8317"},
		{"http://127.0.0.1:9000", "9000", "http://proxy:8317"},
		{"http://127.0.0.1:9000", "", "http://127.0.0.1:9000"},
		{"https://hara.local", "", "https://hara.local"},
	} {
		h := newTestHost(t, map[string]string{"URL": tc.url, "HARA_PORT": tc.port})

		if err := h.dispatch([]string{"tools", "quota"}); err != nil {
			t.Fatal(err)
		}

		line := "compose run --rm --no-deps -T tools quota"
		assertCalls(t, h.fake.calls, line)

		if !slices.Contains(h.fake.envs[line], "TOOLS_URL="+tc.want) {
			t.Errorf("URL %q: TOOLS_URL missing %q", tc.url, tc.want)
		}
	}
}

func TestWsSmokeAndBenchArguments(t *testing.T) {
	h := newTestHost(t, map[string]string{"URL": "https://hara.local"})

	for _, args := range [][]string{
		{"ws-smoke"},
		{"ws-smoke", "2m", "https://cliproxy.example.ts.net"},
		{"bench", "3", "claude-haiku", ""},
		{"bench"},
	} {
		if err := h.dispatch(args); err != nil {
			t.Fatal(err)
		}
	}

	assertCalls(t, h.fake.calls,
		"compose run --rm --no-deps -T tools wssmoke -idle 0s https://hara.local",
		"compose run --rm --no-deps -T tools wssmoke -idle 2m https://hara.local https://cliproxy.example.ts.net",
		"compose run --rm --no-deps -T tools bench -n 3 -claude-model claude-haiku -codex-model  https://hara.local",
		"compose run --rm --no-deps -T tools bench -n 5 -claude-model  -codex-model  https://hara.local",
	)
}

func TestStackShortcuts(t *testing.T) {
	h := newTestHost(t, nil)

	for _, command := range []string{"init", "rotate", "down"} {
		if err := h.dispatch([]string{command}); err != nil {
			t.Fatal(err)
		}
	}

	if err := h.dispatch([]string{"logs", "proxy"}); err != nil {
		t.Fatal(err)
	}

	assertCalls(t, h.fake.calls,
		"compose build init",
		"compose run --rm --no-deps -T init init",
		"compose run --rm --no-deps -T init rotate",
		"compose --profile tailscale down",
		"compose logs -f --tail=100 proxy",
	)

	if info, err := os.Stat(h.state); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("private state directory was not prepared: %v", err)
	}
}

func TestUpStopsWithoutDocker(t *testing.T) {
	h := newTestHost(t, nil)
	h.fake.respond = failing("docker info")

	if err := h.dispatch([]string{"up"}); err == nil || err.Error() != "Docker is not running" {
		t.Fatalf("err = %v", err)
	}

	assertCalls(t, h.fake.calls, "docker info")
}

func TestUpStartsTheStackAndPortless(t *testing.T) {
	h := newTestHost(t, map[string]string{"HARA_PORT": "9000"}, "portless")

	if err := h.dispatch([]string{"up"}); err != nil {
		t.Fatal(err)
	}

	assertCalls(t, h.fake.calls,
		"docker info",
		"compose up -d --build --force-recreate",
		"compose run --rm --no-deps -T tools health",
		"portless proxy start --lan",
		"portless alias hara 9000 --force",
		"compose ps",
		"compose run --rm --no-deps -T tools status",
		"compose --profile tailscale ps -q --status running tailscale",
	)

	if !strings.Contains(h.out.String(), "Host endpoint: http://localhost:9000") {
		t.Errorf("output: %s", h.out)
	}
}

func TestPortlessIsOptional(t *testing.T) {
	h := newTestHost(t, nil)

	if err := h.dispatch([]string{"portless"}); err != nil || len(h.fake.calls) != 0 || !strings.Contains(h.out.String(), "portless not installed") {
		t.Fatalf("err %v, calls %v, output %s", err, h.fake.calls, h.out)
	}

	h = newTestHost(t, nil, "portless")
	h.fake.respond = failing("portless proxy start --lan")

	if err := h.dispatch([]string{"portless"}); err != nil || !strings.Contains(h.out.String(), "https://hara.local is unavailable") {
		t.Fatalf("err %v, output %s", err, h.out)
	}

	assertCalls(t, h.fake.calls, "portless proxy start --lan")
}

func TestStatusWaitsForTailscale(t *testing.T) {
	const tailscale = "compose exec -T tailscale tailscale status --peers=false"

	h := newTestHost(t, nil)
	checks := 0
	h.fake.respond = func(line string, cmd *exec.Cmd) error {
		switch line {
		case "compose --profile tailscale ps -q --status running tailscale":
			fmt.Fprintln(cmd.Stdout, "abc123")
		case tailscale:
			checks++

			if checks < 3 {
				fmt.Fprintln(cmd.Stdout, "Tailscale is stopped: NoState")

				return nil
			}

			if checks == 3 {
				fmt.Fprintln(cmd.Stdout, "100.64.0.1 cliproxy")

				return nil
			}

			return exitStatus(1)
		}

		return nil
	}

	if err := h.dispatch([]string{"status"}); err != nil {
		t.Fatal(err)
	}

	if checks != 4 || strings.Count(h.out.String(), "Waiting for Tailscale") != 1 || !strings.Contains(h.out.String(), "Tailscale needs attention") {
		t.Fatalf("checks %d, output %s", checks, h.out)
	}
}

func TestPurgeKeepsPrivateState(t *testing.T) {
	h := newTestHost(t, nil, "portless")
	h.fake.respond = failing("portless alias --remove hara")

	if err := h.dispatch([]string{"purge"}); err != nil {
		t.Fatal(err)
	}

	assertCalls(t, h.fake.calls,
		"compose --profile tailscale --profile tools down --rmi all --volumes --remove-orphans",
		"docker builder prune -f",
		"portless alias --remove hara",
	)

	if !strings.Contains(h.out.String(), "Private state kept in "+h.state) {
		t.Errorf("output: %s", h.out)
	}
}

func TestImportOpPipesBothCredentials(t *testing.T) {
	env := map[string]string{"OP_ACCOUNT": "my.1password.com", "OP_VAULT": "Private", "OP_ITEM_NAME": "cli-proxy-api"}

	if err := newTestHost(t, env).dispatch([]string{"import-op"}); err == nil {
		t.Fatal("import-op ran without the 1Password CLI")
	}

	h := newTestHost(t, env, "op")
	h.fake.respond = func(line string, cmd *exec.Cmd) error {
		if strings.HasPrefix(line, "op read") {
			fmt.Fprintf(cmd.Stdout, "value-for-%s\n\n", filepath.Base(cmd.Args[len(cmd.Args)-1]))
		}

		return nil
	}

	if err := h.dispatch([]string{"import-op"}); err != nil {
		t.Fatal(err)
	}

	importLine := "compose run --rm --no-deps -T init import"
	assertCalls(t, h.fake.calls,
		"op read --account my.1password.com op://Private/cli-proxy-api/claude-api-key",
		"op read --account my.1password.com op://Private/cli-proxy-api/management-password",
		"compose build init",
		importLine,
	)

	if got := h.fake.stdin[importLine]; got != "value-for-claude-api-key\nvalue-for-management-password\n" {
		t.Errorf("import input = %q", got)
	}
}

func TestKey(t *testing.T) {
	h := newTestHost(t, nil)

	for _, args := range [][]string{{"--refresh"}, {"other"}, {apiField, managementField}} {
		if err := h.dispatch(append([]string{"key"}, args...)); err == nil {
			t.Errorf("accepted key %q", args)
		}
	}

	h.fake.respond = func(line string, cmd *exec.Cmd) error {
		if line == "docker image inspect "+toolsImage {
			return exitStatus(1)
		}

		fmt.Fprintln(cmd.Stdout, "output of", line)

		return nil
	}

	if err := h.dispatch([]string{"key"}); err != nil {
		t.Fatal(err)
	}

	assertCalls(t, h.fake.calls,
		"docker image inspect "+toolsImage,
		"compose build tools",
		"compose run --rm --no-deps -T tools key claude-api-key",
	)

	if h.out.String() != "output of compose run --rm --no-deps -T tools key claude-api-key\n" || !strings.Contains(h.errOut.String(), "compose build tools") {
		t.Errorf("the credential shared standard output with the build: %q", h.out)
	}

	h = newTestHost(t, map[string]string{"HARA_SECRET_PROVIDER": "op", "OP_ACCOUNT": "a", "OP_VAULT": "v", "OP_ITEM_NAME": "i"})

	if err := h.dispatch([]string{"key", managementField}); err != nil {
		t.Fatal(err)
	}

	assertCalls(t, h.fake.calls, "op read --account a op://v/i/management-password")
}

func TestBackupCopiesPrivateSettings(t *testing.T) {
	h := newTestHost(t, nil)
	source := filepath.Join(h.home, ".codex")

	if err := os.MkdirAll(filepath.Join(source, "openai.config.toml"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(source, "config.toml"), []byte("model = \"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := h.dispatch([]string{"backup", "codex"}); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(h.state, "backups", "codex")
	entries, _ := os.ReadDir(dest)
	info, err := os.Stat(filepath.Join(dest, "config.toml"))

	if err != nil || info.Mode().Perm() != 0o600 || len(entries) != 1 {
		t.Fatalf("backup: %v, entries %v", err, entries)
	}

	if !strings.Contains(h.out.String(), "saved private Codex settings") {
		t.Errorf("output: %s", h.out)
	}

	inside := newTestHost(t, nil)
	inside.getenv = func(name string) string {
		return map[string]string{"BACKUP_DIR": filepath.Join(inside.root, "backups")}[name]
	}

	if err := inside.dispatch([]string{"backup", "claude"}); err == nil {
		t.Fatal("backup was written inside the checkout")
	}
}

func TestCodexProfileInstallsTheHelperPath(t *testing.T) {
	codexHome := filepath.Join(t.TempDir(), "codex")
	h := newTestHost(t, map[string]string{"CODEX_HOME": codexHome})
	template := "auth.command = \"" + keyPlaceholder + "\"\n"

	if err := os.WriteFile(filepath.Join(h.root, "codex", "proxy.config.toml"), []byte(template), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := h.dispatch([]string{"codex-profile"}); err == nil || !strings.Contains(err.Error(), "bin/hara is missing") {
		t.Fatalf("installed a profile without the helper: %v", err)
	}

	h.root = filepath.Join(h.root, `quote"and\slash`)

	for _, dir := range []string{"codex", "bin"} {
		if err := os.MkdirAll(filepath.Join(h.root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	for path, content := range map[string]string{"codex/proxy.config.toml": template, "bin/hara": ""} {
		if err := os.WriteFile(filepath.Join(h.root, path), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := h.dispatch([]string{"codex-profile"}); err != nil {
		t.Fatal(err)
	}

	profile := filepath.Join(codexHome, "proxy.config.toml")
	content, _ := os.ReadFile(profile)
	escaped := strings.ReplaceAll(strings.ReplaceAll(filepath.Join(h.root, "bin", "hara"), `\`, `\\`), `"`, `\"`)

	if string(content) != "auth.command = \""+escaped+"\"\n" {
		t.Errorf("profile = %s", content)
	}

	if info, _ := os.Stat(profile); info.Mode().Perm() != 0o600 {
		t.Errorf("profile mode = %v", info.Mode().Perm())
	}
}

func TestCodexSmoke(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(healthy.Close)

	codexHome := t.TempDir()
	env := map[string]string{"CODEX_HOME": codexHome, "PROXY_URL": healthy.URL}

	if err := newTestHost(t, env).dispatch([]string{"codex-smoke"}); err == nil || !strings.Contains(err.Error(), "no Codex profile 'proxy'") {
		t.Fatalf("err = %v", err)
	}

	if err := os.WriteFile(filepath.Join(codexHome, "proxy.config.toml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	down := newTestHost(t, map[string]string{"CODEX_HOME": codexHome, "PROXY_URL": "http://127.0.0.1:1"})

	if err := down.dispatch([]string{"codex-smoke"}); err == nil || !strings.Contains(err.Error(), "does not answer") {
		t.Fatalf("err = %v", err)
	}

	for _, tc := range []struct{ output, want string }{
		{"warning: Falling back from WebSockets to HTTPS transport\npong\n", "answered over HTTP"},
		{"Pong.\n", "did not answer pong"},
		{"thinking\npong\n", ""},
	} {
		h := newTestHost(t, env)
		h.fake.respond = func(_ string, cmd *exec.Cmd) error {
			fmt.Fprint(cmd.Stdout, tc.output)

			return nil
		}

		err := h.dispatch([]string{"codex-smoke"})

		if tc.want == "" && (err != nil || !strings.Contains(h.out.String(), "ok: Codex answered pong")) {
			t.Errorf("good turn failed: %v", err)
		}

		if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
			t.Errorf("output %q: err = %v, want %q", tc.output, err, tc.want)
		}
	}
}

func TestPanelBuildsThePatchedUpstream(t *testing.T) {
	if err := newTestHost(t, nil).dispatch([]string{"panel"}); err == nil || !strings.Contains(err.Error(), "bun is required") {
		t.Fatalf("err = %v", err)
	}

	h := newTestHost(t, map[string]string{"PANEL_TAG": "v9"}, "bun")

	var buildDir string

	h.fake.respond = func(line string, cmd *exec.Cmd) error {
		if line != "bun run build" {
			return nil
		}

		buildDir = cmd.Dir

		if !slices.Contains(cmd.Env, "VERSION=v9+ledger") || cmd.Stdout != nil {
			return errors.New("build environment")
		}

		if err := os.MkdirAll(filepath.Join(cmd.Dir, "dist"), 0o755); err != nil {
			return err
		}

		return os.WriteFile(filepath.Join(cmd.Dir, "dist", "index.html"), []byte("<html>"), 0o644)
	}

	if err := h.dispatch([]string{"panel"}); err != nil {
		t.Fatal(err)
	}

	html, _ := os.ReadFile(filepath.Join(h.root, "panel", "management.html"))

	if string(html) != "<html>" || !strings.Contains(h.out.String(), "wrote panel/management.html (6 bytes)") {
		t.Errorf("panel %q, output %s", html, h.out)
	}

	if _, err := os.Stat(buildDir); !os.IsNotExist(err) {
		t.Errorf("build directory was kept: %v", err)
	}

	assertCalls(t, h.fake.calls,
		"git -c advice.detachedHead=false clone --quiet --depth 1 --branch v9 https://github.com/router-for-me/Cli-Proxy-API-Management-Center "+buildDir,
		"git -C "+buildDir+" apply --whitespace=nowarn "+filepath.Join(h.root, "panel", "ledger.patch"),
		"bun install --frozen-lockfile",
		"bun run build",
	)
}

func TestCallKeepsTheChildExitStatus(t *testing.T) {
	h := newTestHost(t, nil)
	h.exec = (*exec.Cmd).Run

	if err := h.call(h.command("sh", "-c", "exit 3")); !errors.Is(err, exitStatus(3)) {
		t.Fatalf("err = %v", err)
	}

	if err := h.call(h.command("hara-missing-command")); err == nil || errors.As(err, new(exitStatus)) {
		t.Fatalf("a missing command was reported as a child failure: %v", err)
	}
}
