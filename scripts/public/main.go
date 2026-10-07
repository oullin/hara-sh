// Check publication candidates without printing private matching values.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type checker struct {
	root   string
	output io.Writer
}

var patterns = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"personal absolute path", regexp.MustCompile(`/(?:Users|home)/[\pL\pN_.-]+/`)},
	{"literal infrastructure account ID", regexp.MustCompile(`(?:accountId|account_id)["']?\s*[:=]\s*["'][a-f0-9]{32}["']`)},
	{"private key", regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----`)},
	{"provider credential", regexp.MustCompile(`\b(?:sk-[A-Za-z0-9_-]{20,}|gh[pousr]_[A-Za-z0-9]{20,}|AKIA[A-Z0-9]{16})\b`)},
}

func main() {
	history := flag.Bool("history", false, "also inspect all locally available Git refs")
	flag.Parse()

	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: public [--history]")
		os.Exit(2)
	}

	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()

	if err != nil {
		fmt.Fprintln(os.Stderr, "error: run the publication guard inside the repository")
		os.Exit(1)
	}

	check := checker{root: strings.TrimSpace(string(root)), output: os.Stdout}
	failed, err := check.currentFiles()

	if err == nil && *history {
		var historyFailed bool

		historyFailed, err = check.historyFiles()
		failed = failed || historyFailed
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}

	if failed || err != nil {
		os.Exit(1)
	}
}

func (check checker) git(args ...string) ([]byte, error) {
	command := exec.Command("git", args...)
	command.Dir = check.root
	content, err := command.Output()

	if err != nil {
		return nil, fmt.Errorf("git %s failed", args[0])
	}

	return content, nil
}

func privateFile(path string) bool {
	name := filepath.Base(path)

	return strings.HasPrefix(path, "claude/") || strings.HasPrefix(path, ".claude/") ||
		(strings.HasPrefix(path, "codex/") && path != "codex/proxy.config.toml") ||
		slices.Contains([]string{".env", ".dev.vars", "auth.json", ".claude.json"}, name)
}

func findings(path string, content []byte) []string {
	var issues []string

	if privateFile(path) {
		issues = append(issues, "personal client state or credential file")
	}

	for _, pattern := range patterns {
		var lines []string

		for i, line := range bytes.Split(content, []byte("\n")) {
			if pattern.pattern.Match(line) {
				lines = append(lines, strconv.Itoa(i+1))
			}
		}

		if len(lines) != 0 {
			issues = append(issues, pattern.name+" at line(s) "+strings.Join(lines, ", "))
		}
	}

	return issues
}

func (check checker) currentFiles() (bool, error) {
	content, err := check.git("ls-files", "--cached", "--others", "--exclude-standard", "-z")

	if err != nil {
		return false, err
	}

	paths := strings.Split(string(content), "\x00")

	slices.Sort(paths)

	paths = slices.Compact(paths)
	failed, reviewed := false, 0

	for _, path := range paths {
		if path == "" {
			continue
		}

		file := filepath.Join(check.root, path)
		info, err := os.Stat(file)

		if os.IsNotExist(err) {
			continue
		}

		if err != nil {
			return false, fmt.Errorf("inspect publication candidate %s failed", path)
		}

		if !info.Mode().IsRegular() {
			continue
		}

		content, err := os.ReadFile(file)

		if err != nil {
			return false, fmt.Errorf("read publication candidate %s failed", path)
		}

		reviewed++
		issues := findings(path, content)

		switch strings.ToLower(filepath.Ext(path)) {
		case ".py":
			issues = append(issues, "Python source is not permitted; use Go")
		case ".js", ".mjs", ".cjs", ".jsx", ".es", ".es6", ".mts", ".cts":
			issues = append(issues, "JavaScript and alternate script extensions are not permitted; use .ts")
		}

		for _, issue := range issues {
			fmt.Fprintf(check.output, "%s: %s\n", path, issue)
			failed = true
		}
	}

	result := "no guard findings"

	if failed {
		result = "blocked"
	}

	fmt.Fprintf(check.output, "Reviewed %d current publication candidates; %s.\n", reviewed, result)

	return failed, nil
}

func (check checker) historyFiles() (bool, error) {
	content, err := check.git("rev-list", "--objects", "--all")

	if err != nil {
		return false, err
	}

	failed, reviewed := false, 0

	for entry := range strings.SplitSeq(string(content), "\n") {
		oid, path, ok := strings.Cut(entry, " ")

		if !ok {
			continue
		}

		kind, err := check.git("cat-file", "-t", oid)

		if err != nil {
			return false, err
		}

		if strings.TrimSpace(string(kind)) != "blob" {
			continue
		}

		blob, err := check.git("cat-file", "blob", oid)

		if err != nil {
			return false, err
		}

		reviewed++

		for _, issue := range findings(path, blob) {
			fmt.Fprintf(check.output, "history %.12s %s: %s\n", oid, path, issue)
			failed = true
		}
	}

	result := "no guard findings"

	if failed {
		result = "publication blocked"
	}

	fmt.Fprintf(check.output, "Reviewed %d historical blobs; %s.\n", reviewed, result)

	return failed, nil
}
