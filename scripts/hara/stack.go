package main

import (
	"cmp"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// A just-started node reports NoState, then offline, for a few seconds; a node that needs login settles at once.
var tailscaleStarting = regexp.MustCompile(`NoState|Starting|offline|failed to connect`)

// stackCommands are the Docker Compose shortcuts; each one keeps LOCAL_DIR private first.
func (h host) stackCommands() map[string]func([]string) error {
	return map[string]func([]string) error{
		"up":        h.noArgs(h.up),
		"portless":  h.noArgs(h.portless),
		"init":      h.noArgs(h.initialise),
		"import-op": h.noArgs(h.importOp),
		"rotate":    h.noArgs(func() error { return h.call(h.compose("run", "--rm", "--no-deps", "-T", "init", "rotate")) }),
		"status":    h.noArgs(h.status),
		"down":      h.noArgs(func() error { return h.call(h.compose("--profile", "tailscale", "down")) }),
		"purge":     h.noArgs(h.purge),
		"tools":     h.tools,
		"ws-smoke":  h.wsSmoke,
		"bench":     h.bench,
		"logs": func(services []string) error {
			return h.call(h.compose(append([]string{"logs", "-f", "--tail=100"}, services...)...))
		},
	}
}

func (h host) noArgs(action func() error) func([]string) error {
	return func(args []string) error {
		if len(args) != 0 {
			return errors.New(usage)
		}

		return action()
	}
}

func (h host) port() string {
	return cmp.Or(h.getenv("HARA_PORT"), "8317")
}

func (h host) compose(args ...string) *exec.Cmd {
	return h.command("docker", append([]string{"compose", "-f", filepath.Join(h.root, "local", "compose.yaml")}, args...)...)
}

// tools runs one containerised tool; host loopback URLs become the proxy's Docker service address.
func (h host) tools(args []string) error {
	cmd := h.compose(append([]string{"run", "--rm", "--no-deps", "-T", "tools"}, args...)...)
	cmd.Env = append(os.Environ(), "TOOLS_URL="+h.toolsURL())

	return h.call(cmd)
}

func (h host) toolsURL() string {
	local := "http://localhost:" + h.port()
	target := cmp.Or(h.getenv("URL"), local)

	if target == local || target == "http://127.0.0.1:"+h.port() {
		return "http://proxy:8317"
	}

	return target
}

// wsSmoke takes the idle time, then any extra addresses (the tailnet URL).
func (h host) wsSmoke(args []string) error {
	idle := "0s"

	if len(args) > 0 {
		idle, args = cmp.Or(args[0], idle), args[1:]
	}

	return h.tools(append([]string{"wssmoke", "-idle", idle, h.toolsURL()}, args...))
}

// bench takes the request count, the Claude model and the Codex model; an empty model skips its API.
func (h host) bench(args []string) error {
	if len(args) > 3 {
		return errors.New("usage: hara bench [N] CLAUDE_MODEL CODEX_MODEL")
	}

	values := make([]string, 3)
	copy(values, args)

	return h.tools([]string{"bench", "-n", cmp.Or(values[0], "5"), "-claude-model", values[1], "-codex-model", values[2], h.toolsURL()})
}

func (h host) status() error {
	if err := h.call(h.compose("ps")); err != nil {
		return err
	}

	if err := h.tools([]string{"status"}); err != nil {
		return err
	}

	h.log("Host endpoint: http://localhost:%s", h.port())

	// Tailscale is optional; show its login/address only when enabled and running.
	running, _ := h.output(h.compose("--profile", "tailscale", "ps", "-q", "--status", "running", "tailscale"))

	if strings.TrimSpace(running) == "" {
		return nil
	}

	h.waitTailscale()

	if h.call(h.compose("exec", "-T", "tailscale", "tailscale", "status", "--peers=false")) != nil {
		h.log("! Tailscale needs attention; authorize its device before using the tailnet URL")
	}

	return nil
}

func (h host) waitTailscale() {
	for attempt := 1; attempt <= 15; attempt++ {
		cmd := h.compose("exec", "-T", "tailscale", "tailscale", "status", "--peers=false")

		var state strings.Builder

		cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, &state, &state
		_ = h.call(cmd)

		if state.Len() > 0 && !tailscaleStarting.MatchString(state.String()) {
			return
		}

		if attempt == 1 {
			h.log("Waiting for Tailscale to connect...")
		}

		h.sleep(time.Second)
	}
}

// portless serves https://hara.local in LAN mode; skipped when portless is not installed.
// Starting the proxy on port 443 may ask for sudo unless `portless service install --lan` was run.
func (h host) portless() error {
	if !h.look("portless") {
		h.log("portless not installed; skipping https://hara.local")

		return nil
	}

	if h.call(h.command("portless", "proxy", "start", "--lan")) != nil {
		h.log("! portless proxy did not start; https://hara.local is unavailable")

		return nil
	}

	return h.call(h.command("portless", "alias", "hara", h.port(), "--force"))
}

// up never regenerates existing keys; readers restart after the configuration is rendered.
func (h host) up() error {
	if !h.quiet(h.command("docker", "info")) {
		return errors.New("Docker is not running")
	}

	if err := h.call(h.compose("up", "-d", "--build", "--force-recreate")); err != nil {
		return err
	}

	if err := h.tools([]string{"health"}); err != nil {
		return err
	}

	if err := h.portless(); err != nil {
		return err
	}

	return h.status()
}

func (h host) initialise() error {
	if err := h.call(h.compose("build", "init")); err != nil {
		return err
	}

	return h.call(h.compose("run", "--rm", "--no-deps", "-T", "init", "init"))
}

// purge leaves Docker to rebuild or pull everything on the next up; private state in LOCAL_DIR is kept.
func (h host) purge() error {
	if err := h.call(h.compose("--profile", "tailscale", "--profile", "tools", "down", "--rmi", "all", "--volumes", "--remove-orphans")); err != nil {
		return err
	}

	if err := h.call(h.command("docker", "builder", "prune", "-f")); err != nil {
		return err
	}

	if h.look("portless") {
		_ = h.call(h.command("portless", "alias", "--remove", "hara"))
	}

	h.log("Private state kept in %s; delete it yourself to remove keys and provider logins.", h.state)

	return nil
}

func (h host) importOp() error {
	if !h.look("op") {
		return errors.New("install and sign in to the optional 1Password CLI first")
	}

	values := make([]string, 2)

	for i, field := range []string{apiField, managementField} {
		value, err := h.output(h.opRead(field))

		if err != nil {
			return err
		}

		values[i] = strings.TrimRight(value, "\n")
	}

	if err := h.call(h.compose("build", "init")); err != nil {
		return err
	}

	cmd := h.compose("run", "--rm", "--no-deps", "-T", "init", "import")
	cmd.Stdin = strings.NewReader(values[0] + "\n" + values[1] + "\n")

	return h.call(cmd)
}
