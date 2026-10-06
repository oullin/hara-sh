// Command bench measures the proxy the way the clients feel it (`make bench`): for each address
// and for Claude (Messages API) and Codex (Responses API), it sends N streaming requests that share
// a large prompt prefix and reports time to first token, total time and prompt-cache hits. The
// first request of a run starts cold; the others should be served from the provider's cache.
//
// Each run sends real requests: N per API per address, about 5k prompt tokens each, mostly cached.
//
// Env: API_KEY (the client key). Arguments: base URLs (default http://localhost:8317).
package main

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// bench configures one run.
type bench struct {
	client      *http.Client
	key         string
	n           int
	claudeModel string
	codexModel  string
	apis        []api
}

func main() {
	n := flag.Int("n", 5, "requests per API per address")
	claudeModel := flag.String("claude-model", "claude-haiku-4-5-20251001", "model for the Claude requests (empty skips them)")
	codexModel := flag.String("codex-model", "gpt-5.5", "model for the Codex requests (empty skips them)")
	flag.Parse()

	key := os.Getenv("API_KEY")

	if key == "" || *n < 1 {
		fmt.Fprintln(os.Stderr, "error: API_KEY must hold the client key and -n must be at least 1 (run it through `make bench`)")
		os.Exit(1)
	}

	b := bench{client: newClient(), key: key, n: *n, claudeModel: *claudeModel, codexModel: *codexModel}

	if *claudeModel != "" {
		b.apis = append(b.apis, messages)
	}

	if *codexModel != "" {
		b.apis = append(b.apis, responses)
	}

	if !b.run(os.Stdout, addresses(flag.Args())) {
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
		return []string{"http://localhost:8317"}
	}

	return out
}

// newClient trusts portless's own certificate authority, which signs https://hara.local.
func newClient() *http.Client {
	client := &http.Client{Timeout: 2 * time.Minute}
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

// run benchmarks every API at every address and reports whether every request succeeded.
func (b bench) run(w io.Writer, bases []string) bool {
	ok := true

	fmt.Fprintf(w, "%-28s %-7s %3s  %-22s %-22s %s\n", "address", "api", "n", "first token p50 / max", "total p50 / max", "prompt cache")

	for _, base := range bases {
		for _, a := range b.apis {
			if !b.series(w, base, a) {
				ok = false
			}
		}
	}

	return ok
}

// series sends n requests with one fresh prefix, so the first is cold and the rest can hit the cache.
func (b bench) series(w io.Writer, base string, a api) bool {
	nonce := newNonce()
	prefix := cachePrefix(nonce)
	model := map[string]string{messages.name: b.claudeModel, responses.name: b.codexModel}[a.name]

	var samples []sample

	for range b.n {
		s, err := a.run(b.client, base, b.key, model, prefix, nonce)

		if err != nil {
			fmt.Fprintf(w, "%-28s %-7s FAIL %v\n", base, a.name, err)

			return false
		}

		samples = append(samples, s)
	}

	fmt.Fprintln(w, summarize(base, a.name, samples))

	return true
}

// summarize prints one line: latency percentiles and how the warm requests used the cache.
func summarize(base, name string, samples []sample) string {
	first := make([]time.Duration, len(samples))
	total := make([]time.Duration, len(samples))

	for i, s := range samples {
		first[i], total[i] = s.firstToken, s.total
	}

	return fmt.Sprintf("%-28s %-7s %3d  %-22s %-22s %s", base, name, len(samples), spread(first), spread(total), cacheUse(samples))
}

// spread is "p50 / max", in milliseconds.
func spread(d []time.Duration) string {
	sorted := slices.Clone(d)

	slices.Sort(sorted)

	return fmt.Sprintf("%dms / %dms", sorted[(len(sorted)-1)/2].Milliseconds(), sorted[len(sorted)-1].Milliseconds())
}

// cacheUse reports how many warm requests (all but the first) read from the cache, and what share
// of their prompt it covered.
func cacheUse(samples []sample) string {
	if len(samples) < 2 {
		return "n/a (needs -n 2 or more)"
	}

	hits, cached, input := 0, 0, 0

	for _, s := range samples[1:] {
		if s.cached > 0 {
			hits++
		}

		cached += s.cached
		input += s.input
	}

	share := 0

	if input > 0 {
		share = cached * 100 / input
	}

	return fmt.Sprintf("%d/%d warm hits, %d%% of prompt cached", hits, len(samples)-1, share)
}

func newNonce() string {
	b := make([]byte, 6)

	rand.Read(b)

	return hex.EncodeToString(b)
}

// cachePrefix is about 5,000 tokens, above every model's minimum cacheable prompt (4,096 for
// Haiku 4.5). The nonce makes each run's prefix new, so its first request is always cold.
func cachePrefix(nonce string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Benchmark run %s. The reference lines below are context only; do not discuss them.\n", nonce)

	for i := range 420 {
		fmt.Fprintf(&sb, "Reference line %04d: the proxy forwards this request and measures how quickly the answer begins.\n", i)
	}

	return sb.String()
}
