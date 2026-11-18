package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// streamT3Archive writes a gzip tar of sources to output. Files removed while the walk runs are reported to warnings and skipped.
func streamT3Archive(ctx context.Context, output io.Writer, sources []backupSource, warnings io.Writer) error {
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
		if err := archiveT3Source(ctx, tarWriter, source, scratch, warnings); err != nil {
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

func archiveT3Source(ctx context.Context, writer *tar.Writer, source backupSource, scratch string, warnings io.Writer) error {
	return filepath.WalkDir(source.Path, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := archiveT3Entry(ctx, writer, source, scratch, path, entry, walkErr)

		if errors.Is(err, errVanished) {
			relative, _ := filepath.Rel(source.Path, path)
			fmt.Fprintf(warnings, "Warning: skipped %s; it was removed during the backup.\n", filepath.ToSlash(filepath.Join(source.Name, relative)))

			return nil
		}

		return err
	})
}

// archiveT3Entry writes one walked entry, returning errVanished when it disappeared before anything was written.
func archiveT3Entry(ctx context.Context, writer *tar.Writer, source backupSource, scratch, path string, entry fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		if path != source.Path && errors.Is(walkErr, fs.ErrNotExist) {
			return errVanished
		}

		return errors.New("could not read a backup source; no complete archive created")
	}

	if backupTransient(path) {
		return nil
	}

	header, err := archiveT3Header(source, path, entry)

	if err != nil || header == nil {
		return err
	}

	if header.Typeflag == tar.TypeReg {
		return archiveT3File(ctx, writer, header, path, scratch)
	}

	return writer.WriteHeader(header)
}

// archiveT3Header describes a directory, regular file or symlink under its archive name; other types get no header.
func archiveT3Header(source backupSource, path string, entry fs.DirEntry) (*tar.Header, error) {
	info, err := entry.Info()

	if errors.Is(err, fs.ErrNotExist) {
		return nil, errVanished
	}

	if err != nil {
		return nil, err
	}

	mode := info.Mode()

	if !mode.IsRegular() && !mode.IsDir() && mode&os.ModeSymlink == 0 {
		return nil, nil
	}

	target := ""

	if mode&os.ModeSymlink != 0 {
		target, err = os.Readlink(path)

		if errors.Is(err, fs.ErrNotExist) {
			return nil, errVanished
		}

		if err != nil {
			return nil, err
		}
	}

	relative, err := filepath.Rel(source.Path, path)

	if err != nil {
		return nil, err
	}

	header, err := tar.FileInfoHeader(info, target)

	if err != nil {
		return nil, err
	}

	header.Name = filepath.ToSlash(filepath.Join(source.Name, relative))

	return header, nil
}

// errVanished marks a source file removed before its header was written, which is safe to skip.
var errVanished = errors.New("backup file vanished")

func archiveT3File(ctx context.Context, writer *tar.Writer, header *tar.Header, path, scratch string) error {
	file, err := os.Open(path)

	if errors.Is(err, fs.ErrNotExist) {
		return errVanished
	}

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

	return copyArchived(writer, file, header.Size)
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

	return copyArchived(writer, file, header.Size)
}

// copyArchived copies exactly size bytes after the header is written; a file truncated in place cannot be skipped any more.
func copyArchived(writer io.Writer, file io.Reader, size int64) error {
	_, err := io.CopyN(writer, file, size)

	if errors.Is(err, io.EOF) {
		return errors.New("a file shrank while it was copied; close applications using provider homes and retry")
	}

	return err
}
