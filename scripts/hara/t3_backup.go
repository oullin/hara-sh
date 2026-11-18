package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type backupSource struct {
	Path string `json:"-"`
	Name string `json:"name"`
}

func (h host) t3Backup(args []string) error {
	flags := flag.NewFlagSet("t3 backup", flag.ContinueOnError)
	flags.SetOutput(h.stderr)
	output := flags.String("output", ".", "destination folder (defaults to the caller's working directory)")
	recipient := flags.String("recipient", "", "age public recipient; omit for an interactive passphrase")
	dryRun := flags.Bool("dry-run", false, "check source roots and destination without copying data or creating an archive")

	positional, err := parseInterspersed(flags, args)

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}

		return err
	}

	if len(positional) > 1 {
		return errors.New("usage: t3/backup.sh [--recipient age1...] [--dry-run] [destination-folder]")
	}

	if len(positional) == 1 {
		*output = positional[0]
	}

	sources, err := h.t3BackupSources()

	if err != nil {
		return err
	}

	destination, err := backupDestination(*output, sources)

	if err != nil {
		return err
	}

	if *dryRun {
		fmt.Fprintf(h.stdout, "Would encrypt %d source roots into the selected destination. No files copied.\n", len(sources))

		return nil
	}

	for _, tool := range []string{"age", "sqlite3", "pgrep"} {
		if !h.look(tool) {
			return fmt.Errorf("%s is required for an encrypted, consistent backup", tool)
		}
	}

	if err := h.t3BackupStopped(); err != nil {
		return err
	}

	h.t3BackupWarnBusy()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	defer cancel()

	return h.writeT3Backup(ctx, destination, *recipient, sources)
}

// parseInterspersed accepts flags before and after the destination, as in "backup /dest --recipient age1...".
func parseInterspersed(flags *flag.FlagSet, args []string) ([]string, error) {
	var positional []string

	for {
		if err := flags.Parse(args); err != nil {
			return nil, err
		}

		rest := flags.Args()

		if consumed := len(args) - len(rest); consumed > 0 && args[consumed-1] == "--" {
			return append(positional, rest...), nil
		}

		if len(rest) == 0 {
			return positional, nil
		}

		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

// t3BackupWarnBusy reports provider CLIs that may change their homes while they are copied; it never blocks the backup.
func (h host) t3BackupWarnBusy() {
	var running []string

	for name, pattern := range map[string][]string{
		"claude":       {"-x", "claude"},
		"codex":        {"-x", "codex"},
		"cursor-agent": {"-f", "(^|/)cursor-agent( |$)"},
	} {
		output, err := h.output(exec.Command("pgrep", pattern...))

		if err == nil && strings.TrimSpace(output) != "" {
			running = append(running, name)
		}
	}

	if len(running) > 0 {
		slices.Sort(running)

		fmt.Fprintf(h.stderr, "Warning: %s still running; files removed during the backup are skipped, and files rewritten in place stop it.\n", strings.Join(running, ", "))
	}
}

func (h host) t3BackupHome() string {
	if value := h.getenv("T3_HOME"); value != "" {
		return value
	}

	return filepath.Join(h.home, ".t3")
}

func (h host) t3BackupSources() ([]backupSource, error) {
	t3Home := h.t3BackupHome()

	if !isDir(t3Home) {
		return nil, errors.New("T3 home does not exist; set T3_HOME for a custom installation")
	}

	paths := []string{t3Home}

	for _, relative := range []string{
		".codex", ".codex-t3-hara", ".claude", ".claude-hara", ".claude.json", ".cursor", ".agents",
		"Library/Application Support/t3code-v2", "Library/Application Support/t3code",
	} {
		paths = append(paths, filepath.Join(h.home, filepath.FromSlash(relative)))
	}

	providers, err := h.t3ProviderHomes(t3Home)

	if err != nil {
		return nil, err
	}

	home, err := filepath.EvalSymlinks(h.home)

	if err != nil {
		return nil, errors.New("cannot resolve the user home")
	}

	if err := requireInsideHome(t3Home, home); err != nil {
		return nil, err
	}

	paths, err = h.resolveBackupRoots(append(paths, providers...), home)

	if err != nil {
		return nil, err
	}

	return h.backupSourcesWithin(paths, home), nil
}

// t3ProviderHomes lists the custom and shadow homes T3's provider settings declare, expanding ~/.
func (h host) t3ProviderHomes(t3Home string) ([]string, error) {
	var settings struct {
		Providers map[string]struct {
			Config struct {
				Home   string `json:"homePath"`
				Shadow string `json:"shadowHomePath"`
			} `json:"config"`
		} `json:"providerInstances"`
	}

	content, err := os.ReadFile(filepath.Join(t3Home, "userdata", "settings.json"))

	if err != nil || json.Unmarshal(content, &settings) != nil {
		return nil, errors.New("cannot read T3 provider settings; refusing an incomplete backup")
	}

	var paths []string

	for _, provider := range settings.Providers {
		for _, path := range []string{provider.Config.Home, provider.Config.Shadow} {
			if path == "" {
				continue
			}

			if strings.HasPrefix(path, "~/") {
				path = filepath.Join(h.home, path[2:])
			}

			if !filepath.IsAbs(path) {
				return nil, errors.New("a provider home is relative; use an absolute or ~/ path before backing up")
			}

			paths = append(paths, path)
		}
	}

	return paths, nil
}

// requireInsideHome refuses a T3 home outside the user home: it is the point of the backup, so it is never skipped.
func requireInsideHome(t3Home, home string) error {
	t3Real, err := filepath.EvalSymlinks(t3Home)

	if err != nil {
		return errors.New("cannot resolve the T3 home")
	}

	if t3Real == home || !insidePath(t3Real, home) {
		return errors.New("the T3 home must be inside the user home; back up a custom T3_HOME separately")
	}

	return nil
}

// resolveBackupRoots rewrites paths under the user home to its resolved form, adds the target of each symlinked
// root, and returns the sorted, unique result.
func (h host) resolveBackupRoots(paths []string, home string) ([]string, error) {
	for i, path := range paths {
		if insidePath(path, h.home) {
			relative, _ := filepath.Rel(h.home, path)
			paths[i] = filepath.Join(home, relative)
		}
	}

	for _, path := range slices.Clone(paths) {
		resolved, err := symlinkTarget(path)

		if err != nil {
			return nil, err
		}

		if resolved != "" {
			paths = append(paths, resolved)
		}
	}

	slices.Sort(paths)

	return slices.Compact(paths), nil
}

// symlinkTarget returns the resolved target of a symlinked root, or "" for a missing path or an ordinary one.
func symlinkTarget(path string) (string, error) {
	info, err := os.Lstat(path)

	if os.IsNotExist(err) {
		return "", nil
	}

	if err != nil {
		return "", errors.New("cannot inspect a backup source")
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return "", nil
	}

	resolved, err := filepath.EvalSymlinks(path)

	if err != nil {
		return "", errors.New("a backup source has an unresolved symlink")
	}

	return resolved, nil
}

// backupSourcesWithin keeps existing, sorted paths that no earlier source contains, warning about those outside home.
func (h host) backupSourcesWithin(paths []string, home string) []backupSource {
	var sources []backupSource

	for _, path := range paths {
		if !exists(path) || slices.ContainsFunc(sources, func(source backupSource) bool { return insidePath(path, source.Path) }) {
			continue
		}

		if path == home || !insidePath(path, home) {
			fmt.Fprintf(h.stderr, "Warning: skipping %s because it is outside the user home; back it up separately.\n", path)

			continue
		}

		relative, _ := filepath.Rel(home, path)
		sources = append(sources, backupSource{Path: path, Name: filepath.ToSlash(filepath.Join("home", relative))})
	}

	return sources
}

func insidePath(path, parent string) bool {
	relative, err := filepath.Rel(parent, path)

	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func backupDestination(path string, sources []backupSource) (string, error) {
	destination, err := filepath.Abs(path)

	if err != nil {
		return "", err
	}
	// Resolve existing ancestors too, so a symlink cannot put the archive inside its own input.
	ancestor, tail := destination, ""

	for !exists(ancestor) {
		tail = filepath.Join(filepath.Base(ancestor), tail)
		ancestor = filepath.Dir(ancestor)
	}

	real, err := filepath.EvalSymlinks(ancestor)

	if err != nil {
		return "", err
	}

	destination = filepath.Join(real, tail)

	for _, source := range sources {
		resolved, err := filepath.EvalSymlinks(source.Path)

		if err != nil || insidePath(destination, resolved) {
			return "", errors.New("the backup destination must be outside every backed-up source")
		}
	}

	return destination, nil
}

func (h host) t3BackupStopped() error {
	content, err := os.ReadFile(filepath.Join(h.t3BackupHome(), "userdata", "server-runtime.json"))

	var runtime struct {
		PID       int    `json:"pid"`
		StartedAt string `json:"startedAt"`
	}

	if err == nil {
		if json.Unmarshal(content, &runtime) != nil {
			return errors.New("cannot verify T3 server state; quit T3 and check its runtime state")
		}

		if runtime.PID > 0 && h.t3ServerAlive(runtime.PID, runtime.StartedAt) {
			return errors.New("T3 server is running; finish agents, quit T3 and stop its background service before backing up (if T3 is already stopped, delete userdata/server-runtime.json in the T3 home)")
		}
	} else if !os.IsNotExist(err) {
		return errors.New("cannot read T3 server state")
	}

	output, err := h.output(exec.Command("pgrep", "-f", "/T3 Code[^/]*[.]app/Contents/MacOS/"))

	if err == nil && strings.TrimSpace(output) != "" {
		return errors.New("T3 desktop is running; quit it before backing up its browser storage")
	}

	if err != nil {
		var status exitStatus

		if !errors.As(err, &status) || status != 1 {
			return errors.New("could not check whether T3 desktop is stopped")
		}
	}

	return nil
}

// t3ServerAlive reports whether the recorded server PID still belongs to T3. A PID is reused only after T3 exits,
// so a live process that started well after T3 recorded its start is a different program. Unknown states count as alive.
func (h host) t3ServerAlive(pid int, startedAt string) bool {
	process, err := os.FindProcess(pid)

	if err != nil {
		return false
	}

	signalErr := process.Signal(syscall.Signal(0))
	process.Release()

	if signalErr != nil && !errors.Is(signalErr, os.ErrPermission) {
		return false
	}

	started, err := time.Parse(time.RFC3339Nano, startedAt)

	if err != nil {
		return true
	}

	output, err := h.output(exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "etime="))

	if err != nil {
		return true
	}

	elapsed, ok := parseElapsed(strings.TrimSpace(output))

	if !ok {
		return true
	}

	return !time.Now().Add(-elapsed).After(started.Add(time.Minute))
}

// parseElapsed reads the portable ps etime format, [[dd-]hh:]mm:ss.
func parseElapsed(value string) (time.Duration, bool) {
	days := 0

	if before, after, found := strings.Cut(value, "-"); found {
		count, err := strconv.Atoi(before)

		if err != nil || count < 0 {
			return 0, false
		}

		days, value = count, after
	}

	parts := strings.Split(value, ":")

	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}

	seconds := 0

	for _, part := range parts {
		count, err := strconv.Atoi(part)

		if err != nil || count < 0 {
			return 0, false
		}

		seconds = seconds*60 + count
	}

	return time.Duration(days)*24*time.Hour + time.Duration(seconds)*time.Second, true
}

func (h host) writeT3Backup(ctx context.Context, destination, recipient string, sources []backupSource) error {
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}

	name := "t3-backup-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".tar.gz.age"
	partial := filepath.Join(destination, name+".partial")
	file, err := os.OpenFile(partial, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)

	if err != nil {
		return err
	}

	defer file.Close()

	defer os.Remove(partial)

	args := []string{"--encrypt"}

	if recipient == "" {
		args = append(args, "--passphrase")
	} else {
		args = append(args, "--recipient", recipient)
	}

	command := exec.CommandContext(ctx, "age", args...)
	command.Stdout, command.Stderr = file, h.stderr
	input, err := command.StdinPipe()

	if err != nil {
		return err
	}

	if err := command.Start(); err != nil {
		input.Close()

		return err
	}

	archiveErr := streamT3Archive(ctx, input, sources, h.stderr)
	input.Close()
	encryptErr := command.Wait()

	if archiveErr != nil {
		return archiveErr
	}

	if encryptErr != nil {
		return errors.New("age encryption failed; incomplete archive removed")
	}

	if err := h.t3BackupStopped(); err != nil {
		return errors.New("T3 restarted during backup; incomplete archive removed")
	}

	if err := file.Sync(); err != nil {
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	if err := os.Rename(partial, filepath.Join(destination, name)); err != nil {
		return err
	}

	fmt.Fprintf(h.stdout, "Created %s (%d source roots). Keep its passphrase or private recipient key separately.\n", name, len(sources))

	return nil
}
