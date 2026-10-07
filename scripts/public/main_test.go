package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingsRedactMatchedContent(t *testing.T) {
	for _, tc := range []struct{ name, content, want string }{
		{"personal path", "/" + "Users/" + "private-user/project/", "personal absolute path"},
		{"Unicode personal path", "/" + "home/" + "utilisateuré/project/", "personal absolute path"},
		{"account ID", `account_id: "` + strings.Repeat("a", 32) + `"`, "literal infrastructure account ID"},
		{"private key", "-----BEGIN " + "PRIVATE KEY-----", "private key"},
		{"provider token", "sk-" + strings.Repeat("a", 30), "provider credential"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issues := findings("config.txt", []byte("safe\n"+tc.content+"\n"+tc.content))

			if len(issues) != 1 || !strings.Contains(issues[0], tc.want) || !strings.Contains(issues[0], "2, 3") || strings.Contains(issues[0], tc.content) {
				t.Fatal("finding omitted its location or disclosed private content")
			}
		})
	}

	for _, path := range []string{"claude/settings.json", ".claude/settings.json", "codex/config.toml", "nested/.env", "auth.json"} {
		if !privateFile(path) {
			t.Errorf("private file was allowed: %s", path)
		}
	}

	for _, path := range []string{"codex/proxy.config.toml", "web/docs/clients.md", ".env.example"} {
		if privateFile(path) {
			t.Errorf("public example was blocked: %s", path)
		}
	}
}

func newRepository(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	gitForTest(t, root, "init", "-q")
	gitForTest(t, root, "config", "user.name", "Fixture")
	gitForTest(t, root, "config", "user.email", "fixture@users.noreply.github.com")

	return root
}

func gitForTest(t *testing.T, root string, args ...string) {
	t.Helper()

	command := exec.Command("git", args...)
	command.Dir = root

	if err := command.Run(); err != nil {
		t.Fatal("Git fixture command failed:", err)
	}
}

func TestCurrentFilesRejectPythonAndIgnoreDeletedCandidates(t *testing.T) {
	root := newRepository(t)

	for _, path := range []string{"README.md", "helper.py"} {
		if err := os.WriteFile(filepath.Join(root, path), []byte("public fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	gitForTest(t, root, "add", ".")

	var output bytes.Buffer
	check := checker{root: root, output: &output}
	failed, err := check.currentFiles()

	if err != nil || !failed || !strings.Contains(output.String(), "Python source is not permitted") || !strings.Contains(output.String(), "Reviewed 2") {
		t.Fatal("publication check allowed Python or failed to deduplicate tracked files")
	}

	if err := os.Remove(filepath.Join(root, "helper.py")); err != nil {
		t.Fatal(err)
	}

	output.Reset()
	failed, err = check.currentFiles()

	if err != nil || failed || !strings.Contains(output.String(), "Reviewed 1") {
		t.Fatal("deleted candidate blocked publication")
	}
}

func TestHistoryFindsPrivateFilesDeletedFromCurrentTree(t *testing.T) {
	root := newRepository(t)
	private := filepath.Join(root, "claude", "settings.json")

	if err := os.MkdirAll(filepath.Dir(private), 0o700); err != nil {
		t.Fatal(err)
	}

	content := "sk-" + strings.Repeat("x", 30)

	if err := os.WriteFile(private, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	gitForTest(t, root, "add", ".")
	gitForTest(t, root, "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
	gitForTest(t, root, "rm", "-q", "claude/settings.json")
	gitForTest(t, root, "-c", "commit.gpgsign=false", "commit", "-qm", "remove fixture")

	var output bytes.Buffer
	check := checker{root: root, output: &output}

	if failed, err := check.currentFiles(); err != nil || failed {
		t.Fatal("clean current tree was blocked")
	}

	output.Reset()
	failed, err := check.historyFiles()

	if err != nil || !failed || !strings.Contains(output.String(), "claude/settings.json") || !strings.Contains(output.String(), "provider credential") || strings.Contains(output.String(), content) {
		t.Fatal("historical private content was missed or disclosed")
	}
}
