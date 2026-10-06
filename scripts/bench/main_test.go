package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeProxy streams both APIs like the proxy does. A prefix it has seen before is reported as
// cached; badStatus answers every request with that status, and noText ends streams without text.
type fakeProxy struct {
	mu        sync.Mutex
	seen      map[string]bool
	bodies    []map[string]any
	badStatus int
	noText    bool
}

const goodKey = "client-key"

func (f *fakeProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.badStatus != 0 {
		http.Error(w, `{"error":"limited"}`, f.badStatus)

		return
	}

	authorized := r.Header.Get("x-api-key") == goodKey || r.Header.Get("Authorization") == "Bearer "+goodKey

	if !authorized {
		http.Error(w, "unauthorized", http.StatusUnauthorized)

		return
	}

	var body map[string]any

	json.NewDecoder(r.Body).Decode(&body)

	f.mu.Lock()
	f.bodies = append(f.bodies, body)
	prefix := fmt.Sprint(body["system"], body["instructions"])
	hit := f.seen[prefix]
	f.seen[prefix] = true
	f.mu.Unlock()

	cached := 0

	if hit {
		cached = 4500
	}

	w.Header().Set("Content-Type", "text/event-stream")

	for _, event := range f.events(r.URL.Path, cached) {
		fmt.Fprintf(w, "event: x\ndata: %s\n\n", event)
		w.(http.Flusher).Flush()
	}
}

func (f *fakeProxy) events(path string, cached int) []string {
	if path == messages.path {
		events := []string{fmt.Sprintf(`{"type":"message_start","message":{"usage":{"input_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}}`, 5000-cached, cached, map[bool]int{true: 0, false: 5000}[cached > 0])}

		if !f.noText {
			events = append(events, `{"type":"content_block_delta","delta":{"type":"text_delta","text":"pong"}}`)
		}

		return append(events, `{"type":"message_stop"}`)
	}

	events := []string{`{"type":"response.created"}`}

	if !f.noText {
		events = append(events, `{"type":"response.output_text.delta","delta":"pong"}`)
	}

	return append(events, fmt.Sprintf(`{"type":"response.completed","response":{"usage":{"input_tokens":5000,"input_tokens_details":{"cached_tokens":%d}}}}`, cached))
}

func newBench(key string, n int) bench {
	return bench{client: &http.Client{Timeout: 5 * time.Second}, key: key, n: n, claudeModel: "claude-haiku", codexModel: "gpt-5.5", apis: []api{messages, responses}}
}

func start(f *fakeProxy) *httptest.Server {
	f.seen = map[string]bool{}

	return httptest.NewServer(f)
}

func TestColdThenWarm(t *testing.T) {
	f := &fakeProxy{}
	srv := start(f)

	defer srv.Close()

	var out strings.Builder

	if !newBench(goodKey, 3).run(&out, []string{srv.URL}) {
		t.Fatalf("run failed:\n%s", out.String())
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")

	if len(lines) != 3 || !strings.HasPrefix(lines[0], "address") {
		t.Fatalf("want a header and one line per API:\n%s", out.String())
	}

	for i, name := range []string{"claude", "codex"} {
		line := lines[i+1]

		if !strings.Contains(line, " "+name+" ") || !strings.Contains(line, "  3  ") || !strings.HasSuffix(line, "2/2 warm hits, 90% of prompt cached") {
			t.Fatalf("line %q", line)
		}
	}

	// The models, the cache marker and the per-run cache key reach the wire as the clients send them.
	claude, codex := f.bodies[0], f.bodies[3]

	if claude["model"] != "claude-haiku" || !strings.Contains(fmt.Sprint(claude["system"]), "cache_control:map[type:ephemeral]") {
		t.Fatalf("claude body = %v", claude["system"])
	}

	if codex["model"] != "gpt-5.5" || !strings.HasPrefix(fmt.Sprint(codex["prompt_cache_key"]), "bench-") || codex["store"] != false {
		t.Fatalf("codex body = %v", codex)
	}

	// A series repeats one prefix; the two APIs of a run do not share it.
	if fmt.Sprint(f.bodies[0]["system"]) != fmt.Sprint(f.bodies[2]["system"]) || strings.Contains(fmt.Sprint(f.bodies[0]["system"]), fmt.Sprint(codex["instructions"])) {
		t.Fatal("want one prefix per series")
	}
}

func TestFailures(t *testing.T) {
	cases := []struct {
		name  string
		proxy *fakeProxy
		key   string
		want  string
	}{
		{"wrong key", &fakeProxy{}, "wrong", "claude  FAIL HTTP 401: unauthorized"},
		{"rate limited", &fakeProxy{badStatus: http.StatusTooManyRequests}, goodKey, `FAIL HTTP 429: {"error":"limited"}`},
		{"no text", &fakeProxy{noText: true}, goodKey, "FAIL the stream ended without any text"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := start(tc.proxy)

			defer srv.Close()

			var out strings.Builder

			if newBench(tc.key, 2).run(&out, []string{srv.URL}) || !strings.Contains(out.String(), tc.want) {
				t.Fatalf("want %q:\n%s", tc.want, out.String())
			}

			// One failing request ends its series: no further requests for that API.
			if strings.Count(out.String(), "FAIL") != 2 {
				t.Fatalf("each API reports one failure:\n%s", out.String())
			}
		})
	}
}

func TestUnreachableAddress(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	var out strings.Builder

	if newBench(goodKey, 1).run(&out, []string{url}) || !strings.Contains(out.String(), "connection refused") {
		t.Fatalf("want a dial failure:\n%s", out.String())
	}
}

func TestSummaries(t *testing.T) {
	ms := time.Millisecond

	if got := spread([]time.Duration{300 * ms, 100 * ms, 900 * ms, 200 * ms}); got != "200ms / 900ms" {
		t.Fatalf("spread = %s", got)
	}

	if got := cacheUse([]sample{{input: 5000}}); got != "n/a (needs -n 2 or more)" {
		t.Fatalf("one sample = %s", got)
	}

	cold := []sample{{input: 5000}, {input: 5000}, {input: 5000, cached: 2500}}

	if got := cacheUse(cold); got != "1/2 warm hits, 25% of prompt cached" {
		t.Fatalf("partial = %s", got)
	}

	if got := cacheUse([]sample{{}, {}}); got != "0/1 warm hits, 0% of prompt cached" {
		t.Fatalf("no usage = %s", got)
	}
}

func TestCachePrefixIsLargeAndFresh(t *testing.T) {
	a, b := cachePrefix(newNonce()), cachePrefix(newNonce())

	// About 4 characters per token: comfortably above Haiku 4.5's 4,096-token minimum.
	if len(a) < 4*4500 || a == b {
		t.Fatalf("prefix of %d characters; distinct = %v", len(a), a != b)
	}
}

func TestAddresses(t *testing.T) {
	got := addresses([]string{" https://hara.local/ ", "", "http://localhost:8317", "https://hara.local"})

	if !slices.Equal(got, []string{"https://hara.local", "http://localhost:8317"}) {
		t.Fatalf("addresses = %v", got)
	}

	if got := addresses(nil); !slices.Equal(got, []string{"http://localhost:8317"}) {
		t.Fatalf("default = %v", got)
	}
}
