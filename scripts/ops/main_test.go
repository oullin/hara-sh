package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var testKeys = keys{api: "client-key", mgmt: "mgmt-key"}

// mockProxy answers like the proxy: healthz, the panel, models for the client key, auth-files and
// logs for the management password, and messages for the client key in x-api-key.
func mockProxy(t *testing.T, accounts string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	bearer := func(r *http.Request, key string) bool { return r.Header.Get("Authorization") == "Bearer "+key }

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"ok"}`)) })
	mux.HandleFunc("GET /management.html", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) })
	mux.HandleFunc("GET /v1/models", func(w http.ResponseWriter, r *http.Request) {
		if !bearer(r, testKeys.api) {
			http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)

			return
		}

		w.Write([]byte(`{"data":[{"id":"claude-haiku-4-5-20251001"},{"id":"gpt-5.5"}]}`))
	})
	mux.HandleFunc("GET /v0/management/auth-files", func(w http.ResponseWriter, r *http.Request) {
		if !bearer(r, testKeys.mgmt) {
			http.Error(w, `{"error":"invalid management key"}`, http.StatusUnauthorized)

			return
		}

		w.Write([]byte(accounts))
	})
	mux.HandleFunc("GET /v0/management/logs", func(w http.ResponseWriter, r *http.Request) {
		if !bearer(r, testKeys.mgmt) || r.URL.Query().Get("limit") != "2" {
			http.Error(w, "bad request", http.StatusBadRequest)

			return
		}

		w.Write([]byte(`{"lines":["one","two"]}`))
	})
	mux.HandleFunc("POST /v1/messages", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string `json:"model"`
		}

		if r.Header.Get("X-Api-Key") != testKeys.api || json.NewDecoder(r.Body).Decode(&req) != nil || req.Model != "claude-haiku-4-5-20251001" {
			http.Error(w, `{"error":"no account for this model"}`, http.StatusBadGateway)

			return
		}

		w.Write([]byte(`{"content":[{"type":"text","text":"pong"}]}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv
}

// fakeDocker answers `docker inspect` with each container's state and `docker exec ... tailscale
// status` with the given JSON; a missing container fails like Docker does.
func fakeDocker(states map[string]string, tailscale string) shell {
	return func(name string, args ...string) ([]byte, error) {
		switch args[0] {
		case "inspect":
			if state, ok := states[args[len(args)-1]]; ok {
				return []byte(state + "\n"), nil
			}

			return nil, errors.New("no such object")
		case "exec":
			if tailscale == "" {
				return nil, errors.New("container is not running")
			}

			return []byte(tailscale), nil
		}

		return nil, errors.New("unexpected command")
	}
}

var allRunning = map[string]string{
	"p-proxy-1": "running/healthy", "p-tailscale-1": "running/", "p-quota-1": "running/",
}

func runStatus(t *testing.T, srv *httptest.Server, k keys, sh shell) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run("status", []string{"-local", srv.URL, "-network", srv.URL + "/", "-project", "p"}, k, srv.Client(), sh, &out)

	return out.String(), err
}

func TestStatusAllGood(t *testing.T) {
	srv := mockProxy(t, `{"files":[{"provider":"claude","name":"a.json","status":"active"},{"provider":"codex","name":"b.json","status":"active"}]}`)
	ts := `{"BackendState":"NeedsLogin","AuthURL":"https://login.tailscale.com/a/abc","Self":{"DNSName":""}}`
	out, err := runStatus(t, srv, testKeys, fakeDocker(allRunning, ts))

	if err != nil {
		t.Fatalf("status failed: %v\n%s", err, out)
	}

	for _, want := range []string{
		"✓ proxy container     running, healthy",
		"✓ quota container     running\n",
		"✓ this computer",
		"! your tailnet        not signed in: open https://login.tailscale.com/a/abc",
		"✓ panel",
		"✓ client key          accepted, 2 models",
		"✓ accounts            2 active",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestStatusFindsTheBrokenLinks(t *testing.T) {
	srv := mockProxy(t, `{"files":[{"provider":"claude","name":"a.json","status":"active"},{"provider":"codex","name":"b.json","status":"error"},{"provider":"claude","name":"c.json","disabled":true}]}`)
	states := map[string]string{"p-proxy-1": "running/unhealthy", "p-tailscale-1": "restarting/"}
	out, err := runStatus(t, srv, keys{api: "wrong", mgmt: testKeys.mgmt}, fakeDocker(states, ""))

	if !errors.Is(err, errFailed) {
		t.Fatalf("err = %v, want errFailed\n%s", err, out)
	}

	for _, want := range []string{
		"✗ proxy container     running but unhealthy; run: make logs proxy",
		"✗ tailscale container restarting; run: make up, then make logs tailscale",
		"✗ quota container     not found",
		"! your tailnet        Tailscale does not answer",
		"✗ client key          the proxy rejects it; run: make ops keys, then make up",
		"! accounts            1 active, 1 error, 1 disabled; details: make ops accounts",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestStatusDeadAddresses(t *testing.T) {
	srv := mockProxy(t, `{"files":[]}`)
	srv.Close()
	out, err := runStatus(t, srv, keys{}, fakeDocker(allRunning, `{"BackendState":"Stopped"}`))

	if !errors.Is(err, errFailed) {
		t.Fatalf("err = %v, want errFailed", err)
	}

	for _, want := range []string{
		"✗ this computer       " + srv.URL + "  the proxy does not answer; run: make logs proxy",
		"✗ your network        " + srv.URL + "  portless does not reach the proxy; run: make up",
		"! your tailnet        Tailscale is Stopped",
		"- panel               not checked: your network fails",
		"- client key          not checked: this computer fails",
		"- accounts            not checked: this computer fails",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestTailnetRunning(t *testing.T) {
	srv := mockProxy(t, `{"files":[]}`)
	ts := `{"BackendState":"Running","Self":{"DNSName":"cliproxy.tail.ts.net."}}`
	s := status{client: srv.Client(), sh: fakeDocker(allRunning, ts), project: "p"}
	c := s.tailnet()

	// The tailnet name does not resolve in the test, so the address check fails, naming it.
	if c.level != fail || !strings.Contains(c.detail, "https://cliproxy.tail.ts.net  the tailnet does not reach the proxy") {
		t.Fatalf("tailnet = %+v", c)
	}
}

func TestCommands(t *testing.T) {
	srv := mockProxy(t, `{"files":[{"provider":"claude","name":"a.json","status":"active"},{"provider":"codex","name":"b.json","disabled":true}]}`)
	cases := []struct {
		cmd  string
		args []string
		want string
	}{
		{"models", nil, "claude-haiku-4-5-20251001\ngpt-5.5\n"},
		{"accounts", nil, "claude   active   a.json\ncodex    disabled b.json\n"},
		{"logs", []string{"-n", "2"}, "one\ntwo\n"},
		{"smoke", nil, "pong\n"},
	}

	for _, c := range cases {
		var out bytes.Buffer

		if err := run(c.cmd, append([]string{"-url", srv.URL}, c.args...), testKeys, srv.Client(), nil, &out); err != nil {
			t.Fatalf("%s: %v", c.cmd, err)
		}

		if out.String() != c.want {
			t.Errorf("%s = %q, want %q", c.cmd, out.String(), c.want)
		}
	}
}

func TestCommandErrors(t *testing.T) {
	srv := mockProxy(t, `{"files":[]}`)
	var out bytes.Buffer

	if err := run("models", []string{"-url", srv.URL}, keys{api: "wrong"}, srv.Client(), nil, &out); err == nil || !strings.Contains(err.Error(), `401 {"error":"invalid api key"}`) {
		t.Errorf("models with a wrong key: %v", err)
	}

	if err := run("smoke", []string{"-url", srv.URL, "-model", "nope"}, testKeys, srv.Client(), nil, &out); err == nil || !strings.Contains(err.Error(), "nope: 502") {
		t.Errorf("smoke with an unserved model: %v", err)
	}

	if err := run("nope", nil, testKeys, srv.Client(), nil, &out); err == nil || err.Error() != usage {
		t.Errorf("unknown command: %v", err)
	}
}

func TestFirstLine(t *testing.T) {
	if got := firstLine([]byte("  first\nsecond")); got != "first" {
		t.Errorf("firstLine = %q", got)
	}

	if got := firstLine([]byte(strings.Repeat("x", 200))); len([]rune(got)) != 161 {
		t.Errorf("long line not cut: %d runes", len([]rune(got)))
	}
}

func TestContainerStarting(t *testing.T) {
	s := status{project: "p", sh: fakeDocker(map[string]string{"p-proxy-1": "running/starting"}, "")}

	if c := s.container("proxy", "proxy container", "p-proxy-1"); c.level != warn || c.detail != "starting" {
		t.Fatalf("starting proxy = %+v", c)
	}
}

func TestStatusWithoutKeys(t *testing.T) {
	srv := mockProxy(t, `{"files":[]}`)
	out, err := runStatus(t, srv, keys{}, fakeDocker(allRunning, `{"BackendState":"NeedsLogin"}`))

	if err != nil {
		t.Fatalf("warnings alone must not fail: %v\n%s", err, out)
	}

	for _, want := range []string{
		"! your tailnet        not signed in; run: make status again in a few seconds for the link",
		"! client key          API_KEY is not set",
		"! accounts            MGMT_KEY is not set",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
