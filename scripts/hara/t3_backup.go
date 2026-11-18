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

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 1 {
		return errors.New("usage: t3/backup.sh [--recipient age1...] [--dry-run] [destination-folder]")
	}
	if flags.NArg() == 1 {
		*output = flags.Arg(0)
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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return h.writeT3Backup(ctx, destination, *recipient, sources)
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
	settingsPath := filepath.Join(t3Home, "userdata", "settings.json")
	var settings struct {
		Providers map[string]struct {
			Config struct {
				Home   string `json:"homePath"`
				Shadow string `json:"shadowHomePath"`
			} `json:"config"`
		} `json:"providerInstances"`
	}
	content, err := os.ReadFile(settingsPath)
	if err != nil || json.Unmarshal(content, &settings) != nil {
		return nil, errors.New("cannot read T3 provider settings; refusing an incomplete backup")
	}
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
	home, err := filepath.EvalSymlinks(h.home)
	if err != nil {
		return nil, errors.New("cannot resolve the user home")
	}
	for i, path := range paths {
		if insidePath(path, h.home) {
			relative, _ := filepath.Rel(h.home, path)
			paths[i] = filepath.Join(home, relative)
		}
	}
	for _, path := range slices.Clone(paths) {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, errors.New("cannot inspect a backup source")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil, errors.New("a backup source has an unresolved symlink")
			}
			paths = append(paths, resolved)
		}
	}
	slices.Sort(paths)
	paths = slices.Compact(paths)
	var sources []backupSource
	for _, path := range paths {
		if !exists(path) {
			continue
		}
		contained := slices.ContainsFunc(sources, func(source backupSource) bool { return insidePath(path, source.Path) })
		if contained {
			continue
		}
		relative, err := filepath.Rel(home, path)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, errors.New("a backup source is outside the user home; review that source separately")
		}
		sources = append(sources, backupSource{Path: path, Name: filepath.ToSlash(filepath.Join("home", relative))})
	}
	return sources, nil
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
		PID int `json:"pid"`
	}
	if err == nil {
		if json.Unmarshal(content, &runtime) != nil {
			return errors.New("cannot verify T3 server state; quit T3 and check its runtime state")
		}
		if runtime.PID > 0 {
			process, findErr := os.FindProcess(runtime.PID)
			if findErr == nil {
				signalErr := process.Signal(syscall.Signal(0))
				process.Release()
				if signalErr == nil || errors.Is(signalErr, os.ErrPermission) {
					return errors.New("T3 server is running; finish agents, quit T3 and stop its background service before backing up")
				}
			}
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
	archiveErr := streamT3Archive(ctx, input, sources)
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
