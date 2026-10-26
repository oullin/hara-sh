package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalCredentialSkipsDocker(t *testing.T) {
	h := newTestHost(t, nil)

	if err := os.MkdirAll(filepath.Join(h.state, "secrets"), 0o700); err != nil {
		t.Fatal(err)
	}

	for _, field := range []string{apiField, managementField} {
		value := "synthetic-local-credential-" + field

		if err := os.WriteFile(filepath.Join(h.state, "secrets", field), []byte(value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		h.out.Reset()

		if err := h.dispatch([]string{"key", field}); err != nil {
			t.Fatal(err)
		}

		if h.out.String() != value+"\n" || len(h.fake.calls) != 0 {
			t.Fatal("local key retrieval spawned a process or changed credential output")
		}
	}
}

func TestInvalidLocalCredentialDoesNotFallBackOrLeak(t *testing.T) {
	for _, value := range []string{"", strings.Repeat("x", 19), strings.Repeat("x", 74), "synthetic-credential\nwith-newline", "synthetic-credential\x00null", "synthetic-credential\xff"} {
		h := newTestHost(t, nil)
		path := filepath.Join(h.state, "secrets", apiField)

		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := h.dispatch([]string{"key"}); err == nil || h.out.Len() != 0 || len(h.fake.calls) != 0 {
			t.Fatal("invalid local credential was accepted, leaked, or passed to Docker")
		}
	}
}
