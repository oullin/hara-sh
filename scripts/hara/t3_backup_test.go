package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// changeDuringCopy runs change once the archive has produced enough output to be part-way through a large file.
type changeDuringCopy struct {
	bytes.Buffer
	change func()
}

func backupFixture(t *testing.T) testHost {
	t.Helper()

	h := newTestHost(t, nil, "age", "sqlite3", "pgrep")

	if err := os.MkdirAll(filepath.Join(h.home, ".t3", "userdata"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(h.home, ".t3", "userdata", "settings.json"), []byte(`{"providerInstances":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	return h
}

func TestT3BackupRefusesLiveServerWithoutCreatingOutput(t *testing.T) {
	h := backupFixture(t)
	content, _ := json.Marshal(map[string]int{"pid": os.Getpid()})
	os.WriteFile(filepath.Join(h.home, ".t3", "userdata", "server-runtime.json"), content, 0o600)
	output := filepath.Join(t.TempDir(), "new-backup-folder")
	err := h.t3([]string{"backup", "--output", output, "--recipient", "unused"})

	if err == nil || !strings.Contains(err.Error(), "server is running") {
		t.Fatalf("expected a live-server refusal, got %v", err)
	}

	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("refusal created output")
	}
}

func TestT3BackupRejectsDestinationThroughSourceSymlink(t *testing.T) {
	h := backupFixture(t)
	link := filepath.Join(t.TempDir(), "alias")

	if err := os.Symlink(filepath.Join(h.home, ".t3"), link); err != nil {
		t.Fatal(err)
	}

	err := h.t3Backup([]string{"--dry-run", filepath.Join(link, "new-folder")})

	if err == nil || !strings.Contains(err.Error(), "outside every backed-up source") {
		t.Fatalf("expected recursive-output refusal, got %v", err)
	}
}

func TestT3BackupIncludesASymlinkedProviderHome(t *testing.T) {
	h := backupFixture(t)
	actual := filepath.Join(h.home, "provider-state")

	if err := os.Mkdir(actual, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(actual, filepath.Join(h.home, ".codex")); err != nil {
		t.Fatal(err)
	}

	actual, err := filepath.EvalSymlinks(actual)

	if err != nil {
		t.Fatal(err)
	}

	sources, err := h.t3BackupSources()

	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, source := range sources {
		found = found || source.Path == actual
	}

	if !found {
		t.Fatal("provider symlink was saved without its state directory")
	}
}

func TestT3BackupEncryptionRoundTripAndFailureCleanup(t *testing.T) {
	for _, name := range []string{"age", "age-keygen", "sqlite3"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skip(name + " not installed")
		}
	}

	h := backupFixture(t)
	key := filepath.Join(t.TempDir(), "identity")

	if err := exec.Command("age-keygen", "-o", key).Run(); err != nil {
		t.Fatal(err)
	}

	public, err := exec.Command("age-keygen", "-y", key).Output()

	if err != nil {
		t.Fatal(err)
	}

	private := filepath.Join(h.home, ".t3", "credentials.txt")

	if err := os.WriteFile(private, []byte("private-test-marker"), 0o600); err != nil {
		t.Fatal(err)
	}

	database := filepath.Join(h.home, ".t3", "state.sqlite")
	db := exec.Command("sqlite3", database)
	dbInput, err := db.StdinPipe()

	if err != nil {
		t.Fatal(err)
	}

	dbOutput, err := db.StdoutPipe()

	if err != nil {
		t.Fatal(err)
	}

	if err := db.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() { dbInput.Close(); db.Wait() }()

	io.WriteString(dbInput, "PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; CREATE TABLE proof(value); INSERT INTO proof VALUES ('restored'); SELECT 'ready';\n")
	lines := bufio.NewReader(dbOutput)

	for {
		line, err := lines.ReadString('\n')

		if err != nil {
			t.Fatal(err)
		}

		if line == "ready\n" {
			break
		}
	}

	output := t.TempDir()

	if err := h.t3Backup([]string{"--recipient", strings.TrimSpace(string(public)), output}); err != nil {
		t.Fatal(err)
	}

	files, _ := filepath.Glob(filepath.Join(output, "*.age"))

	if len(files) != 1 {
		t.Fatalf("expected one complete encrypted archive, got %d", len(files))
	}

	info, _ := os.Stat(files[0])

	if info.Mode().Perm() != 0o600 {
		t.Fatal("encrypted output is not owner-only")
	}

	ciphertext, _ := os.ReadFile(files[0])

	if bytes.Contains(ciphertext, []byte("private-test-marker")) {
		t.Fatal("plaintext appeared in the output")
	}

	plain, err := exec.Command("age", "--decrypt", "--identity", key, files[0]).Output()

	if err != nil {
		t.Fatal(err)
	}

	archive := readBackupTar(t, plain)

	if string(archive["home/.t3/credentials.txt"]) != "private-test-marker" {
		t.Fatal("private state was not recovered")
	}

	restored := filepath.Join(t.TempDir(), "restored.sqlite")
	os.WriteFile(restored, archive["home/.t3/state.sqlite"], 0o600)
	result, err := exec.Command("sqlite3", restored, "SELECT value FROM proof;").Output()

	if err != nil || string(result) != "restored\n" {
		t.Fatal("database snapshot did not restore")
	}

	failedOutput := t.TempDir()

	if err := h.t3Backup([]string{"--recipient", "invalid-recipient", failedOutput}); err == nil {
		t.Fatal("invalid encryption recipient was accepted")
	}

	remaining, _ := os.ReadDir(failedOutput)

	if len(remaining) != 0 {
		t.Fatal("failed encryption left an archive behind")
	}

	if err := os.WriteFile(filepath.Join(h.home, ".t3", "broken.sqlite"), []byte("SQLite format 3\x00broken"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := h.t3Backup([]string{"--recipient", strings.TrimSpace(string(public)), failedOutput}); err == nil {
		t.Fatal("corrupt database was silently omitted")
	}

	remaining, _ = os.ReadDir(failedOutput)

	if len(remaining) != 0 {
		t.Fatal("a failed SQLite snapshot left an archive behind")
	}
}

func readBackupTar(t *testing.T, content []byte) map[string][]byte {
	t.Helper()

	zipped, err := gzip.NewReader(bytes.NewReader(content))

	if err != nil {
		t.Fatal(err)
	}

	defer zipped.Close()

	reader := tar.NewReader(zipped)
	files := map[string][]byte{}

	for {
		header, err := reader.Next()

		if err == io.EOF {
			return files
		}

		if err != nil {
			t.Fatal(err)
		}

		files[header.Name], err = io.ReadAll(reader)

		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestT3ArchivePreservesSymlinksWithoutReadingTheirTargets(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "private-outside.txt")
	os.WriteFile(outside, []byte("not-in-archive"), 0o600)
	root := t.TempDir()
	os.Symlink(outside, filepath.Join(root, "link"))

	var output bytes.Buffer

	if err := streamT3Archive(context.Background(), &output, []backupSource{{Path: root, Name: "home/test"}}, io.Discard); err != nil {
		t.Fatal(err)
	}

	files := readBackupTar(t, output.Bytes())

	if len(files["home/test/link"]) != 0 {
		t.Fatal("a symlink's external contents were copied")
	}
}

func TestT3BackupAcceptsFlagsAfterTheDestination(t *testing.T) {
	h := backupFixture(t)
	output := t.TempDir()

	if err := h.t3Backup([]string{output, "--dry-run"}); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(h.out.String(), "No files copied") {
		t.Fatalf("dry run after the destination was not honoured: %q", h.out.String())
	}

	if err := h.t3Backup([]string{output, "--dry-run", "another"}); err == nil {
		t.Fatal("two destinations were accepted")
	}
}

func TestParseInterspersedStopsAtDoubleDash(t *testing.T) {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.Bool("dry-run", false, "")
	flags.String("recipient", "", "")
	got, err := parseInterspersed(flags, []string{"a", "--dry-run", "--", "--recipient"})

	if err != nil || !slices.Equal(got, []string{"a", "--recipient"}) {
		t.Fatalf("positional = %q, %v", got, err)
	}
}

func TestT3BackupSkipsAProviderHomeOutsideTheUserHome(t *testing.T) {
	h := backupFixture(t)
	outside := t.TempDir()
	settings := fmt.Sprintf(`{"providerInstances":{"codex":{"config":{"homePath":%q}}}}`, outside)

	if err := os.WriteFile(filepath.Join(h.home, ".t3", "userdata", "settings.json"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}

	sources, err := h.t3BackupSources()

	if err != nil {
		t.Fatal(err)
	}

	for _, source := range sources {
		if !strings.HasPrefix(source.Name, "home/") || insidePath(source.Path, outside) {
			t.Fatalf("outside source was kept: %+v", source)
		}
	}

	if len(sources) == 0 || !strings.Contains(h.errOut.String(), "outside the user home") {
		t.Fatalf("expected the T3 home and a skip warning, got %d sources and %q", len(sources), h.errOut.String())
	}
}

func TestT3BackupRejectsAT3HomeOutsideTheUserHome(t *testing.T) {
	custom := t.TempDir()

	if err := os.MkdirAll(filepath.Join(custom, "userdata"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(custom, "userdata", "settings.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	h := newTestHost(t, map[string]string{"T3_HOME": custom})

	if _, err := h.t3BackupSources(); err == nil || !strings.Contains(err.Error(), "T3 home must be inside") {
		t.Fatalf("expected a T3 home refusal, got %v", err)
	}
}

func TestT3BackupDistinguishesAReusedServerPID(t *testing.T) {
	for _, test := range []struct {
		name, startedAt string
		running         bool
	}{
		{"pid reused after T3 exited", "2000-01-01T00:00:00Z", false},
		{"T3 still running", time.Now().UTC().Format(time.RFC3339Nano), true},
		{"start time unknown", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := backupFixture(t)
			h.fake.respond = func(line string, cmd *exec.Cmd) error {
				if strings.HasPrefix(line, "ps -p ") {
					fmt.Fprintln(cmd.Stdout, "   05:00")
				}

				return nil
			}

			content, _ := json.Marshal(map[string]any{"pid": os.Getpid(), "startedAt": test.startedAt})

			if err := os.WriteFile(filepath.Join(h.home, ".t3", "userdata", "server-runtime.json"), content, 0o600); err != nil {
				t.Fatal(err)
			}

			err := h.t3BackupStopped()

			if test.running != (err != nil) {
				t.Fatalf("running = %v, err = %v", test.running, err)
			}
		})
	}
}

func TestParseElapsed(t *testing.T) {
	for value, want := range map[string]time.Duration{
		"05:07":       5*time.Minute + 7*time.Second,
		"02:32:37":    2*time.Hour + 32*time.Minute + 37*time.Second,
		"3-01:00:00":  73 * time.Hour,
		"10-00:00:01": 240*time.Hour + time.Second,
	} {
		if got, ok := parseElapsed(value); !ok || got != want {
			t.Errorf("parseElapsed(%q) = %v, %v; want %v", value, got, ok, want)
		}
	}

	for _, value := range []string{"", "5", "a:b", "1:2:3:4", "x-01:00"} {
		if _, ok := parseElapsed(value); ok {
			t.Errorf("parseElapsed(%q) was accepted", value)
		}
	}
}

func TestT3BackupWarnsAboutRunningProviderCLIs(t *testing.T) {
	h := backupFixture(t)
	h.fake.respond = func(line string, cmd *exec.Cmd) error {
		if line == "pgrep -x codex" {
			fmt.Fprintln(cmd.Stdout, "123")

			return nil
		}

		return exitStatus(1)
	}

	h.t3BackupWarnBusy()

	if got := h.errOut.String(); !strings.Contains(got, "codex still running") || strings.Contains(got, "claude") {
		t.Fatalf("warning = %q", got)
	}
}

func (w *changeDuringCopy) Write(content []byte) (int, error) {
	if w.change != nil && w.Len() > 64<<10 {
		w.change()
		w.change = nil
	}

	return w.Buffer.Write(content)
}

func largeRandomFile(t *testing.T, path string) {
	t.Helper()

	content := make([]byte, 1<<20)

	if _, err := rand.Read(content); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestT3ArchiveSkipsFilesRemovedDuringTheWalk(t *testing.T) {
	root := t.TempDir()
	largeRandomFile(t, filepath.Join(root, "a"))
	removed := filepath.Join(root, "b")

	if err := os.WriteFile(removed, []byte("temporary"), 0o600); err != nil {
		t.Fatal(err)
	}

	output := &changeDuringCopy{change: func() { os.Remove(removed) }}

	var warnings bytes.Buffer

	if err := streamT3Archive(context.Background(), output, []backupSource{{Path: root, Name: "home/test"}}, &warnings); err != nil {
		t.Fatal(err)
	}

	files := readBackupTar(t, output.Bytes())

	if _, ok := files["home/test/b"]; ok || len(files["home/test/a"]) != 1<<20 {
		t.Fatal("archive did not keep the stable file and skip the removed one")
	}

	if !strings.Contains(warnings.String(), "skipped home/test/b") {
		t.Fatalf("warnings = %q", warnings.String())
	}
}

func TestT3ArchiveFailsWhenAFileShrinksWhileCopied(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a")
	largeRandomFile(t, path)
	output := &changeDuringCopy{change: func() { os.Truncate(path, 0) }}
	err := streamT3Archive(context.Background(), output, []backupSource{{Path: root, Name: "home/test"}}, io.Discard)

	if err == nil || !strings.Contains(err.Error(), "shrank") {
		t.Fatalf("expected a shrink failure, got %v", err)
	}
}
