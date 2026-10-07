// Command ops talks to the running proxy: `make status` checks every link from the containers to
// the accounts and says how to fix what is broken; `make ops models|accounts|smoke|logs` list the
// models and accounts, send one test message and print the server log.
//
//	ops status   [-local URL] [-network URL] [-project NAME]
//	ops models   [-url URL]
//	ops accounts [-url URL]
//	ops smoke    [-url URL] [-model MODEL]
//	ops logs     [-url URL] [-n LINES]
//
// Env: API_KEY (the client key), MGMT_KEY (the management password).
package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// keys holds the client key and the management password.
type keys struct {
	api  string
	mgmt string
}

// shell runs a command and returns its standard output; tests replace it.
type shell func(name string, args ...string) ([]byte, error)

const usage = "usage: ops status|models|accounts|smoke|logs [flags] (run it through make)"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}

	env := keys{api: os.Getenv("API_KEY"), mgmt: os.Getenv("MGMT_KEY")}

	err := run(os.Args[1], os.Args[2:], env, newClient(), docker, os.Stdout)

	// A failed status already said what is broken.
	if err != nil && !errors.Is(err, errFailed) {
		fmt.Fprintln(os.Stderr, "error:", err)
	}

	if err != nil {
		os.Exit(1)
	}
}

// errFailed reports that status found a broken link; the checks already say which.
var errFailed = errors.New("some checks failed")

func run(cmd string, args []string, k keys, client *http.Client, sh shell, w io.Writer) error {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	url := fs.String("url", "https://hara.local", "base URL of the proxy")

	switch cmd {
	case "status":
		local := fs.String("local", "http://localhost:8317", "the proxy on this computer")
		network := fs.String("network", "https://hara.local", "the proxy on your network (portless)")
		project := fs.String("project", "cli-proxy-api-local", "the Docker Compose project")

		if err := fs.Parse(args); err != nil {
			return err
		}

		s := status{client: client, keys: k, local: trim(*local), network: trim(*network), project: *project, sh: sh}

		if !report(w, s.checks()) {
			return errFailed
		}

		return nil
	case "models", "accounts", "logs":
		n := fs.Int("n", 200, "server log lines (logs)")

		if err := fs.Parse(args); err != nil {
			return err
		}

		p := proxy{client: client, base: trim(*url), keys: k}

		switch cmd {
		case "models":
			return p.printModels(w)
		case "accounts":
			return p.printAccounts(w)
		default:
			return p.printLogs(w, *n)
		}
	case "smoke":
		model := fs.String("model", "claude-haiku-4-5-20251001", "the Claude model to ask")

		if err := fs.Parse(args); err != nil {
			return err
		}

		return proxy{client: client, base: trim(*url), keys: k}.smoke(w, *model)
	default:
		return errors.New(usage)
	}
}

func trim(url string) string {
	return strings.TrimRight(strings.TrimSpace(url), "/")
}

// newClient trusts the portless certificate authority, which signs hara.local.
func newClient() *http.Client {
	client := &http.Client{Timeout: time.Minute}
	home, _ := os.UserHomeDir()
	pem, err := os.ReadFile(filepath.Join(home, ".portless", "ca.pem"))

	if err != nil {
		return client
	}

	pool, err := x509.SystemCertPool()

	if err != nil {
		pool = x509.NewCertPool()
	}

	pool.AppendCertsFromPEM(pem)
	client.Transport = &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}

	return client
}

func docker(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}
