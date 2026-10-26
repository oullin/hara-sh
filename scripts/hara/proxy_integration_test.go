package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

type proxyLab struct {
	mu     sync.Mutex
	mode   string
	failID string
	calls  []string
	base   string
	client *http.Client
}

// Opt-in image contract tests use synthetic credentials and a local upstream only.
// Run on a Docker host: HARA_PROXY_INTEGRATION=1 go test -count=1 -run TestProxyImage -v .
// Ordinary unit tests (including the containerised suite) never start Docker.
func TestProxyImage(t *testing.T) {
	if os.Getenv("HARA_PROXY_INTEGRATION") != "1" {
		t.Skip("set HARA_PROXY_INTEGRATION=1 to test the pinned Docker image")
	}

	t.Run("transient_failover_preserves_warm_binding", func(t *testing.T) {
		lab := startProxyLab(t)
		first := lab.request(t, false, "standard")
		lab.failNext(first)

		if got := lab.request(t, false, "standard"); got == first {
			t.Fatal("transient failure did not fail over")
		}

		// Exercise the production five-second circuit breaker, not a shorter fixture.
		time.Sleep(6 * time.Second)

		if got := lab.request(t, false, "standard"); got != first {
			t.Fatalf("session moved from its warm account %q to %q", first, got)
		}
	})

	t.Run("fast_credit_refusal_does_not_drain_pool", func(t *testing.T) {
		lab := startProxyLab(t)
		lab.setMode("credits")
		status, body := lab.rawRequest(t, false, "fast")

		if status != 429 || !strings.Contains(body, "Usage credits are required") {
			t.Fatalf("credit refusal: HTTP %d %s", status, body)
		}

		if got := len(lab.attempts()); got != 1 {
			t.Fatalf("credit refusal rotated through %d accounts", got)
		}

		lab.request(t, false, "standard")

		if got := lab.attempts(); len(got) != 2 || got[0] != got[1] {
			t.Fatalf("credit refusal cooled or rebound the account: %v", got)
		}
	})

	t.Run("quota_failover_rebinds_session", func(t *testing.T) {
		lab := startProxyLab(t)
		lab.setMode("quota")
		first := lab.request(t, false, "standard")
		lab.failNext(first)
		next := lab.request(t, false, "standard")

		if next == first {
			t.Fatal("quota refusal did not fail over")
		}

		time.Sleep(6 * time.Second)

		if got := lab.request(t, false, "standard"); got != next {
			t.Fatalf("quota failover did not rebind: got %q, want %q", got, next)
		}
	})

	t.Run("outage_retries_are_bounded", func(t *testing.T) {
		lab := startProxyLab(t)
		lab.setMode("outage")
		status, _ := lab.rawRequest(t, false, "standard")

		if status < 500 {
			t.Fatalf("outage reported HTTP %d", status)
		}

		if got := len(lab.attempts()); got != 4 {
			t.Fatalf("outage made %d attempts, want two accounts in two rounds", got)
		}
	})

	t.Run("empty_stream_recovers_before_output", func(t *testing.T) {
		lab := startProxyLab(t)
		lab.setMode("empty")
		status, body := lab.rawRequest(t, true, "standard")

		if status != 200 || !strings.Contains(body, "message_stop") {
			t.Fatalf("empty stream did not recover: HTTP %d %s", status, body)
		}

		if got := len(lab.attempts()); got < 2 || got > 4 {
			t.Fatalf("unexpected bootstrap attempt count: %d", got)
		}
	})

	t.Run("partial_stream_is_never_replayed", func(t *testing.T) {
		lab := startProxyLab(t)
		lab.setMode("partial")
		_, body := lab.rawRequest(t, true, "standard")

		if !strings.Contains(body, "partial") || strings.Contains(body, "message_stop") {
			t.Fatalf("partial stream was lost or reported complete: %s", body)
		}

		if got := len(lab.attempts()); got != 1 {
			t.Fatalf("partially delivered turn was replayed %d times", got)
		}
	})

	t.Run("established_stream_gets_heartbeats", func(t *testing.T) {
		lab := startProxyLab(t)
		lab.setMode("heartbeat")
		status, body := lab.rawRequest(t, true, "standard")

		if status != 200 || !strings.Contains(body, ": keep-alive") || !strings.Contains(body, "message_stop") {
			t.Fatalf("idle stream did not stay alive: HTTP %d %s", status, body)
		}
	})
}

func (lab *proxyLab) setMode(mode string) {
	lab.mu.Lock()

	defer lab.mu.Unlock()

	lab.mode = mode
}

func (lab *proxyLab) failNext(id string) {
	lab.mu.Lock()

	defer lab.mu.Unlock()

	lab.failID = id
}

func (lab *proxyLab) attempts() []string {
	lab.mu.Lock()

	defer lab.mu.Unlock()

	return slices.Clone(lab.calls)
}

func (lab *proxyLab) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Stream bool   `json:"stream"`
		Speed  string `json:"speed"`
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))

	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid synthetic request", 400)

		return
	}

	id := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")[0]
	lab.mu.Lock()
	lab.calls = append(lab.calls, id)
	mode := lab.mode
	first, failed := len(lab.calls) == 1, lab.failID == id

	if failed {
		lab.failID = ""
	}

	lab.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")

	if failed || mode == "outage" {
		if mode == "quota" {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"rate_limit_error","message":"synthetic quota limit"}}`)

			return
		}

		w.WriteHeader(503)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"overloaded_error","message":"synthetic overload"}}`)

		return
	}

	if mode == "credits" && payload.Speed == "fast" {
		w.WriteHeader(429)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"rate_limit_error","message":"Usage credits are required for fast mode."}}`)

		return
	}

	if !payload.Stream {
		_, _ = fmt.Fprintf(w, `{"id":"msg_test","type":"message","role":"assistant","model":"claude-opus-5-5","content":[{"type":"text","text":%q}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`, id)

		return
	}

	w.Header().Set("Content-Type", "text/event-stream")

	if mode == "empty" && first {
		dropProxyLabConnection(w)

		return
	}

	_, _ = io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-opus-5-5\",\"content\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":0}}}\n\n")
	_, _ = io.WriteString(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
	_, _ = io.WriteString(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"partial\"}}\n\n")
	w.(http.Flusher).Flush()

	if mode == "partial" {
		dropProxyLabConnection(w)

		return
	}

	if mode == "heartbeat" {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(17 * time.Second):
		}
	}

	_, _ = io.WriteString(w, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
}

func dropProxyLabConnection(w http.ResponseWriter) {
	w.(http.Flusher).Flush()
	conn, _, err := w.(http.Hijacker).Hijack()

	if err == nil {
		_ = conn.Close()
	}
}

func startProxyLab(t *testing.T) *proxyLab {
	t.Helper()

	lab := &proxyLab{client: &http.Client{Timeout: 30 * time.Second}}
	upstream := httptest.NewUnstartedServer(lab)
	listener, err := net.Listen("tcp", "0.0.0.0:0")

	if err != nil {
		t.Fatal(err)
	}

	upstream.Listener = listener
	upstream.Start()
	t.Cleanup(upstream.Close)
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	root := filepath.Join("..", "..")
	compose, err := os.ReadFile(filepath.Join(root, "local", "compose.yaml"))

	if err != nil {
		t.Fatal(err)
	}

	var image string

	for line := range strings.SplitSeq(string(compose), "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "image: eceasy/cli-proxy-api:"); ok {
			image = "eceasy/cli-proxy-api:" + value

			break
		}
	}

	if image == "" {
		t.Fatal("could not find the pinned proxy image")
	}

	template, err := os.ReadFile(filepath.Join(root, "config.yaml"))

	if err != nil {
		t.Fatal(err)
	}

	config := strings.NewReplacer(
		"__API_KEY__", "synthetic-client-key",
		"__MANAGEMENT_PASSWORD_BCRYPT__", "synthetic-management-password",
		"/data/auths", "/test/auths",
	).Replace(string(template))
	config += "\napi-keys:\n  claude:\n"

	for _, id := range []string{"a", "b"} {
		config += fmt.Sprintf("    - name: %s\n      base-url: http://host.docker.internal:%s/%s\n      models:\n        - name: claude-opus-5-5\n      keys:\n        - api-key: synthetic-upstream-%s\n", id, port, id, id)
	}

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	name := fmt.Sprintf("hara-proxy-test-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		defer cancel()

		if out, err := exec.CommandContext(ctx, "docker", "rm", "-f", name).CombinedOutput(); err != nil {
			t.Errorf("remove test proxy: %v: %s", err, out)
		}
	})
	labDocker(t, "run", "-d", "--name", name, "--add-host", "host.docker.internal:host-gateway", "-p", "127.0.0.1::8317", "-e", "WRITABLE_PATH=/test", "-v", dir+":/test", image, "./CLIProxyAPI", "-config", "/test/config.yaml", "--local-model")
	lab.base = "http://" + strings.TrimSpace(labDocker(t, "port", name, "8317/tcp"))
	deadline := time.After(25 * time.Second)

	for {
		resp, err := lab.client.Get(lab.base + "/healthz")

		if err == nil {
			_ = resp.Body.Close()

			if resp.StatusCode == 200 {
				return lab
			}
		}

		select {
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		case <-deadline:
			t.Fatalf("proxy failed to start: %s", labDocker(t, "logs", name))
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func labDocker(t *testing.T, args ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)

	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()

	if err != nil {
		t.Fatalf("docker %s: %v: %s", args[0], err, out)
	}

	return string(out)
}

func (lab *proxyLab) rawRequest(t *testing.T, stream bool, speed string) (int, string) {
	t.Helper()

	body := fmt.Sprintf(`{"model":"claude-opus-5-5","stream":%t,"speed":%q,"max_tokens":32,"messages":[{"role":"user","content":"synthetic test"}]}`, stream, speed)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, lab.base+"/v1/messages", strings.NewReader(body))

	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "synthetic-client-key")
	req.Header.Set("Anthropic-Version", "2023-06-01")
	req.Header.Set("X-Claude-Code-Session-Id", "hara-image-contract")
	resp, err := lab.client.Do(req)

	if err != nil {
		t.Fatal(err)
	}

	defer resp.Body.Close()

	content, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if err != nil {
		t.Fatalf("read proxy response: %v: %s", err, content)
	}

	return resp.StatusCode, string(content)
}

func (lab *proxyLab) request(t *testing.T, stream bool, speed string) string {
	t.Helper()

	status, body := lab.rawRequest(t, stream, speed)

	if status != 200 {
		t.Fatalf("HTTP %d: %s", status, body)
	}

	var message struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal([]byte(body), &message); err != nil || len(message.Content) == 0 {
		t.Fatalf("invalid response: %v: %s", err, body)
	}

	return message.Content[0].Text
}
