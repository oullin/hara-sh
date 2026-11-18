package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
	if err := streamT3Archive(context.Background(), &output, []backupSource{{Path: root, Name: "home/test"}}); err != nil {
		t.Fatal(err)
	}
	files := readBackupTar(t, output.Bytes())
	if len(files["home/test/link"]) != 0 {
		t.Fatal("a symlink's external contents were copied")
	}
}
