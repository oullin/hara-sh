package main

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// setting is one line of the private settings file; a fallback only applies to an empty variable.
type setting struct {
	name     string
	value    string
	fallback bool
}

var (
	settingName    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	settingDefault = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*):=(.*)\}$`)
)

// repository finds the checkout from the binary in bin/, or from the working directory upwards.
func repository() (string, error) {
	var candidates []string

	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(filepath.Dir(executable)))
	}

	if dir, err := os.Getwd(); err == nil {
		for ; ; dir = filepath.Dir(dir) {
			candidates = append(candidates, dir)

			if dir == filepath.Dir(dir) {
				break
			}
		}
	}

	for _, dir := range candidates {
		if info, err := os.Stat(filepath.Join(dir, "local", "compose.yaml")); err == nil && info.Mode().IsRegular() {
			return dir, nil
		}
	}

	return "", errors.New("run hara from the hara-sh checkout, or build it into bin/ with make")
}

// configure applies the optional private settings file and returns the private state directory.
// Starting on an empty default directory would generate new keys and orphan existing logins.
func configure(home string, getenv func(string) string, setenv func(string, string) error) (string, error) {
	if getenv("LOCAL_DIR") == "" && !exists(filepath.Join(home, ".hara-sh")) && isDir(filepath.Join(home, ".cli-proxy-api")) {
		return "", errors.New("private state moved to ~/.hara-sh; run: mv ~/.cli-proxy-api ~/.hara-sh")
	}

	defaultState := filepath.Join(home, ".hara-sh")
	path := cmp.Or(getenv("HARA_ENV_FILE"), filepath.Join(cmp.Or(getenv("LOCAL_DIR"), defaultState), "credentials.env"))
	settings, err := readSettings(path)

	if err != nil {
		return "", err
	}

	for _, setting := range settings {
		if setting.fallback && getenv(setting.name) != "" {
			continue
		}

		if err := setenv(setting.name, setting.value); err != nil {
			return "", err
		}
	}

	for _, setting := range [][2]string{
		{"OP_ACCOUNT", "my.1password.com"},
		{"OP_VAULT", "Private"},
		{"OP_ITEM_NAME", "cli-proxy-api"},
		{"LOCAL_DIR", defaultState},
	} {
		if err := setenv(setting[0], cmp.Or(getenv(setting[0]), setting[1])); err != nil {
			return "", err
		}
	}

	return getenv("LOCAL_DIR"), nil
}

// readSettings parses NAME=value lines (optionally exported or quoted) and `: "${NAME:=value}"` defaults;
// values are literal, never expanded. The file belongs to the local operator and holds names only, never secret values.
func readSettings(path string) ([]setting, error) {
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return nil, nil
	}

	content, err := os.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var settings []setting

	for i, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parsed, ok := parseSetting(line)

		if !ok {
			return nil, fmt.Errorf("%s line %d: expected NAME=value or : \"${NAME:=value}\"", path, i+1)
		}

		settings = append(settings, parsed)
	}

	return settings, nil
}

func parseSetting(line string) (setting, bool) {
	if rest, ok := strings.CutPrefix(line, ":"); ok {
		match := settingDefault.FindStringSubmatch(unquote(strings.TrimSpace(rest)))

		if match == nil {
			return setting{}, false
		}

		return setting{name: match[1], value: unquote(match[2]), fallback: true}, true
	}

	name, value, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(line, "export ")), "=")

	if !ok || !settingName.MatchString(name) {
		return setting{}, false
	}

	return setting{name: name, value: unquote(strings.TrimSpace(value))}, true
}

func unquote(value string) string {
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		return value[1 : len(value)-1]
	}

	return value
}

// privateDir creates dir with owner-only access and refuses one that resolves inside the checkout.
func privateDir(dir, root string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	resolved, err := filepath.EvalSymlinks(dir)

	if err != nil {
		return err
	}

	resolved, err = filepath.Abs(resolved)

	if err != nil {
		return err
	}

	if realRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = realRoot
	}

	if strings.HasPrefix(resolved+string(filepath.Separator), root+string(filepath.Separator)) {
		return errors.New("private state and client backups must be outside the repository")
	}

	return os.Chmod(resolved, 0o700)
}

func exists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}
