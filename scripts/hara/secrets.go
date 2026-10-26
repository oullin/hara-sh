package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// localCredential avoids starting Docker for every new provider session. The
// initializer already writes these owner-only files to the host's private state.
// Missing files retain the existing container fallback; invalid files fail closed.
func localCredential(state, field string) (string, error) {
	if err := validateCredentialField(field); err != nil {
		return "", err
	}

	file, err := os.Open(filepath.Join(state, "secrets", field))

	if err != nil {
		return "", fmt.Errorf("read credential file: %w", err)
	}

	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, 74))

	if err != nil {
		return "", fmt.Errorf("read credential file: %w", err)
	}

	value := strings.TrimSuffix(string(content), "\n")

	if len(value) < 20 || len(value) > 72 || !utf8.ValidString(value) || strings.ContainsAny(value, "\r\n\x00") {
		return "", errors.New("credential file must contain 20–72 UTF-8 bytes without line breaks")
	}

	return value, nil
}

func validateCredentialField(field string) error {
	switch field {
	case apiField, managementField:
		return nil
	case "--refresh", "--clear":
		return errors.New("use make ops import-op to sync 1Password, or make ops keys to rotate local keys")
	default:
		return errors.New("unknown credential field")
	}
}
