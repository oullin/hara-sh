// Command wssmoke checks that WebSockets work through each address of the proxy (`make ws-smoke`):
// the handshake, a ping/pong round trip, an optional idle hold, the closing handshake, and that a
// socket without the client key is refused. Ping/pong is answered by the proxy itself, so the
// checks need no provider account and send nothing upstream.
//
// Env: API_KEY (the client key). Arguments: base URLs (default https://hara.local).
package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// check configures one run over every address.
type check struct {
	path    string
	key     string
	idle    time.Duration
	timeout time.Duration
	tls     *tls.Config
}

// socketStep is one exchange on the open socket.
type socketStep struct {
	name string
	run  func() error
}

func main() {
	path := flag.String("path", "/v1/responses", "WebSocket route to check")
	idle := flag.Duration("idle", 0, "hold the socket idle this long, then ping again (finds idle timeouts)")
	timeout := flag.Duration("timeout", 10*time.Second, "limit for each step")
	flag.Parse()

	key := os.Getenv("API_KEY")

	if key == "" {
		fmt.Fprintln(os.Stderr, "error: API_KEY must hold the client key (run it through `make ws-smoke`)")
		os.Exit(1)
	}

	c := check{path: *path, key: key, idle: *idle, timeout: *timeout, tls: portlessTLS()}

	if !c.run(os.Stdout, addresses(flag.Args())) {
		os.Exit(1)
	}
}

// addresses trims and de-duplicates the base URLs, keeping their order.
func addresses(args []string) []string {
	var out []string

	for _, arg := range args {
		if arg = strings.TrimRight(strings.TrimSpace(arg), "/"); arg != "" && !slices.Contains(out, arg) {
			out = append(out, arg)
		}
	}

	if len(out) == 0 {
		return []string{"https://hara.local"}
	}

	return out
}

// portlessTLS trusts portless's own certificate authority, which signs https://hara.local.
func portlessTLS() *tls.Config {
	home, _ := os.UserHomeDir()
	pem, err := os.ReadFile(filepath.Join(home, ".portless", "ca.pem"))

	if err != nil {
		return nil
	}

	pool, err := x509.SystemCertPool()

	if err != nil {
		pool = x509.NewCertPool()
	}

	pool.AppendCertsFromPEM(pem)

	return &tls.Config{RootCAs: pool}
}

// run checks every address, prints one line per step and reports whether all passed.
func (c check) run(w io.Writer, bases []string) bool {
	ok := true

	for _, base := range bases {
		fmt.Fprintln(w, base+c.path)

		if !c.address(w, base) {
			ok = false
		}
	}

	return ok
}

// address runs the steps against one base URL. A failed step ends the socket's steps, but the
// key check always runs on its own connection.
func (c check) address(w io.Writer, base string) bool {
	ok := c.socket(w, base)

	return c.step(w, "no key", func() (string, error) {
		conn, err := dial(base, c.path, "", c.tls, c.timeout)

		if err == nil {
			conn.Close()

			return "", errors.New("101 without a client key: the route is open to anyone who reaches it")
		}

		if refused, isRefused := errors.AsType[errRefused](err); isRefused && refused.status == 401 {
			return "401 as expected", nil
		}

		return "", err
	}) && ok
}

func (c check) socket(w io.Writer, base string) bool {
	var conn *conn

	ok := c.step(w, "handshake", func() (string, error) {
		var err error

		conn, err = dial(base, c.path, c.key, c.tls, c.timeout)

		return "101 Switching Protocols", err
	})

	if !ok {
		return false
	}

	steps := []socketStep{{"ping/pong", func() error { return conn.ping(c.timeout) }}}

	if c.idle > 0 {
		steps = append(steps, socketStep{"idle " + c.idle.String(), func() error {
			time.Sleep(c.idle)

			return conn.ping(c.timeout)
		}})
	}

	steps = append(steps, socketStep{"close", func() error { return conn.close(c.timeout) }})

	for _, s := range steps {
		if !c.step(w, s.name, func() (string, error) { return "", s.run() }) {
			conn.Close()

			return false
		}
	}

	return true
}

// step times one check and prints "ok" or "FAIL" with its detail.
func (c check) step(w io.Writer, name string, run func() (string, error)) bool {
	start := time.Now()
	detail, err := run()
	took := time.Since(start).Round(time.Millisecond)

	if err != nil {
		fmt.Fprintf(w, "  FAIL  %-12s %v\n", name, err)

		return false
	}

	fmt.Fprintf(w, "  ok    %-12s %s\n", name, strings.TrimSpace(detail+" "+took.String()))

	return true
}
