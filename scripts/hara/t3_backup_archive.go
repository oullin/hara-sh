package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func streamT3Archive(ctx context.Context, output io.Writer, sources []backupSource) error {
	scratch, err := os.MkdirTemp("", "t3-backup-snapshots-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	gzipWriter := gzip.NewWriter(output)
	tarWriter := tar.NewWriter(gzipWriter)
	manifest, err := json.MarshalIndent(map[string]any{
		"version": 1, "sources": sources,
		"restore": "Decrypt and extract into a private staging directory, reinstall T3/provider CLIs, then restore home/ entries with all apps stopped. Sign in again where required. Review absolute symlinks and machine paths; project repositories and macOS Keychain are not included.",
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := tarWriter.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o600, Size: int64(len(manifest))}); err != nil {
		return err
	}
	if _, err := tarWriter.Write(manifest); err != nil {
		return err
	}
	for _, source := range sources {
		if err := archiveT3Source(ctx, tarWriter, source, scratch); err != nil {
			return err
		}
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	return gzipWriter.Close()
}

func backupTransient(path string) bool {
	name := filepath.Base(path)
	return name == "SingletonLock" || name == "SingletonSocket" || name == "SingletonCookie" ||
		name == "server-runtime.json" || strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-shm") ||
		strings.HasSuffix(name, "-journal")
}

func archiveT3Source(ctx context.Context, writer *tar.Writer, source backupSource, scratch string) error {
	return filepath.WalkDir(source.Path, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return errors.New("could not read a backup source; no complete archive created")
		}
		if backupTransient(path) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			return nil
		}
		target := ""
		if info.Mode()&os.ModeSymlink != 0 {
			target, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		relative, err := filepath.Rel(source.Path, path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, target)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(filepath.Join(source.Name, relative))
		if info.Mode().IsRegular() {
			return archiveT3File(ctx, writer, header, path, scratch)
		}
		return writer.WriteHeader(header)
	})
}

func archiveT3File(ctx context.Context, writer *tar.Writer, header *tar.Header, path, scratch string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	magic := make([]byte, 16)
	n, readErr := file.ReadAt(magic, 0)
	if readErr != nil && readErr != io.EOF {
		return readErr
	}
	if n == 16 && string(magic) == "SQLite format 3\x00" {
		return archiveT3SQLite(ctx, writer, header, path, scratch)
	}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	_, err = io.CopyN(writer, file, header.Size)
	return err
}

func archiveT3SQLite(ctx context.Context, writer *tar.Writer, header *tar.Header, path, scratch string) error {
	file, err := os.CreateTemp(scratch, "snapshot-*.sqlite")
	if err != nil {
		return err
	}
	snapshot := file.Name()
	file.Close()
	defer os.Remove(snapshot)
	command := exec.CommandContext(ctx, "sqlite3", "-readonly", path, ".timeout 5000", ".backup '"+strings.ReplaceAll(snapshot, "'", "''")+"'")
	if err := command.Run(); err != nil {
		return errors.New("SQLite snapshot failed; no complete archive created")
	}
	file, err = os.Open(snapshot)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	header.Size = info.Size()
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	_, err = io.CopyN(writer, file, header.Size)
	return err
}
