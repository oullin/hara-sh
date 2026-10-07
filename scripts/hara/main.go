// Command hara is the host helper behind make: it drives the Docker Compose stack, prints client
// credentials, installs the Codex profile, backs up client settings and rebuilds the panel.
//
//	hara up|portless|init|import-op|rotate|status|down|purge
//	hara logs [SERVICE]
//	hara tools COMMAND [ARGS...]
//	hara ws-smoke [IDLE] [TS_URL]
//	hara bench [N] CLAUDE_MODEL CODEX_MODEL
//	hara key [claude-api-key|management-password]
//	hara backup claude|codex
//	hara codex-profile | codex-smoke
//	hara panel
//
// Env: LOCAL_DIR (private state, default ~/.hara-sh), HARA_PORT, URL, HARA_SECRET_PROVIDER=op, OP_*.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// exitStatus carries a child's exit code; the child already reported why it failed.
type exitStatus int

// host runs the helpers against this checkout; tests replace exec, look, sleep and the streams.
type host struct {
	root   string
	home   string
	state  string
	getenv func(string) string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	exec   func(*exec.Cmd) error
	look   func(string) bool
	sleep  func(time.Duration)
	client *http.Client
}

const usage = "usage: hara up|portless|init|import-op|rotate|status|tools COMMAND|ws-smoke|bench|logs [service]|down|purge|key [field]|backup claude|codex|codex-profile|codex-smoke|panel"

func main() {
	err := run(os.Args[1:])

	if err == nil {
		return
	}

	var status exitStatus

	if !errors.As(err, &status) {
		fmt.Fprintln(os.Stderr, "error:", err)
		status = 1
	}

	os.Exit(int(status))
}

func run(args []string) error {
	home, err := os.UserHomeDir()

	if err != nil {
		return err
	}

	root, err := repository()

	if err != nil {
		return err
	}

	state, err := configure(home, os.Getenv, os.Setenv)

	if err != nil {
		return err
	}

	h := host{
		root:   root,
		home:   home,
		state:  state,
		getenv: os.Getenv,
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
		exec:   (*exec.Cmd).Run,
		look:   func(name string) bool { _, err := exec.LookPath(name); return err == nil },
		sleep:  time.Sleep,
		client: &http.Client{Timeout: 5 * time.Second},
	}

	return h.dispatch(args)
}

func (status exitStatus) Error() string {
	return fmt.Sprintf("exit status %d", int(status))
}

func (h host) dispatch(args []string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}

	command, rest := args[0], args[1:]

	if action, ok := h.stackCommands()[command]; ok {
		if err := privateDir(h.state, h.root); err != nil {
			return err
		}

		return action(rest)
	}

	switch command {
	case "key":
		return h.key(rest)
	case "backup":
		return h.backup(rest)
	case "codex-profile":
		return h.codexProfile()
	case "codex-smoke":
		return h.codexSmoke()
	case "panel":
		return h.panel()
	default:
		return errors.New(usage)
	}
}

func (h host) log(format string, args ...any) {
	fmt.Fprintf(h.stdout, format+"\n", args...)
}

func (h host) command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = h.stdin, h.stdout, h.stderr

	return cmd
}

// call runs cmd and turns its failure into the child's exit status.
func (h host) call(cmd *exec.Cmd) error {
	err := h.exec(cmd)

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitStatus(exitErr.ExitCode())
	}

	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(cmd.Args[0]), err)
	}

	return nil
}

// output runs cmd and returns its standard output; standard error still reaches the terminal.
func (h host) output(cmd *exec.Cmd) (string, error) {
	var out bytes.Buffer

	cmd.Stdout = &out
	err := h.call(cmd)

	return out.String(), err
}

// quiet runs cmd with no input or output and reports only whether it succeeded.
func (h host) quiet(cmd *exec.Cmd) bool {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil

	return h.call(cmd) == nil
}
