package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mockProxy serves the management endpoints: claude-one is cooling down with weekly capacity
// expiring in 10h, claude-two's usage call fails, codex-one has nothing expiring but a stale boost,
// claude-three is already boosted correctly, and gemini is disabled and unsupported.
type mockProxy struct {
	calls   []apiCall
	patches []map[string]any
}

var now = time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)

func TestClaudeWindows(t *testing.T) {
	body := `{
		"seven_day_sonnet": {"utilization": 3, "resets_at": null},
		"five_hour": {"utilization": 42.5, "resets_at": "2026-10-06T11:00:00.123456+00:00"},
		"seven_day": {"utilization": 61, "resets_at": "2026-10-07T05:00:00Z"},
		"seven_day_opus": {"utilization": 10, "resets_at": "2026-10-09T05:00:00Z"},
		"extra_usage": {"is_enabled": false},
		"note": "ignored"
	}`
	windows, err := claudeWindows([]byte(body))

	if err != nil {
		t.Fatal(err)
	}

	var labels []string

	for _, w := range windows {
		labels = append(labels, w.Label)
	}

	if got := strings.Join(labels, ","); got != "5h,7d,7d opus,7d sonnet" {
		t.Fatalf("labels = %s", got)
	}

	if windows[0].Used != 42.5 || windows[0].Reset.Sub(now) != 2*time.Hour+123456*time.Microsecond {
		t.Fatalf("5h window = %+v", windows[0])
	}

	if windows[3].Reset != nil {
		t.Fatalf("an unstarted window has no reset: %+v", windows[3])
	}
}

func TestCodexWindows(t *testing.T) {
	body := `{
		"rate_limit": {
			"primary_window": {"used_percent": 30, "limit_window_seconds": 18000, "reset_after_seconds": 3600},
			"secondary_window": {"used_percent": 80, "limit_window_seconds": 604800, "reset_at": 1792000000}
		},
		"additional_rate_limits": [
			{"limit_name": "GPT-5.3-Codex-Spark", "rate_limit": {"primary_window": {"used_percent": 5, "limit_window_seconds": 604800}}},
			{"rate_limit": {"primary_window": {"used_percent": 1, "limit_window_seconds": 90}}}
		]
	}`
	windows, err := codexWindows([]byte(body), now)

	if err != nil {
		t.Fatal(err)
	}

	want := []string{"5h", "7d", "GPT-5.3-Codex-Spark 7d", "extra primary"}

	if len(windows) != len(want) {
		t.Fatalf("windows = %+v", windows)
	}

	for i, label := range want {
		if windows[i].Label != label {
			t.Fatalf("window %d label = %q, want %q", i, windows[i].Label, label)
		}
	}

	if !windows[0].Reset.Equal(now.Add(time.Hour)) || windows[1].Reset.Unix() != 1792000000 {
		t.Fatalf("resets = %v, %v", windows[0].Reset, windows[1].Reset)
	}
}

func TestIsExpiring(t *testing.T) {
	soon, later := now.Add(20*time.Hour), now.Add(30*time.Hour)
	cases := []struct {
		w    window
		want bool
	}{
		{window{"7d", 50, &soon}, true},
		{window{"GPT-5.3-Codex-Spark 7d", 5, &soon}, true},
		{window{"7d opus", 10, &soon}, true},
		{window{"7d", 100, &soon}, false},
		{window{"7d", 50, &later}, false},
		{window{"7d", 50, nil}, false},
		{window{"7d", 50, ptr(now.Add(-time.Hour))}, false},
		{window{"5h", 0, &soon}, false},
	}

	for _, c := range cases {
		if got := isExpiring(c.w, now); got != c.want {
			t.Errorf("isExpiring(%+v) = %v, want %v", c.w, got, c.want)
		}
	}
}

func TestUntil(t *testing.T) {
	if got := until(now.Add(2*time.Hour+5*time.Minute), now); got != "2h05m" {
		t.Errorf("until = %s", got)
	}

	if got := until(now.Add(50*time.Hour), now); got != "2d02h" {
		t.Errorf("until = %s", got)
	}

	if got := until(now.Add(-time.Hour), now); got != "0h00m" {
		t.Errorf("until = %s", got)
	}
}

func (m *mockProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer test" {
		w.WriteHeader(http.StatusUnauthorized)

		return
	}

	switch r.URL.Path {
	case "/v0/management/auth-files":
		json.NewEncoder(w).Encode(map[string]any{"files": []map[string]any{
			{"provider": "claude", "name": "claude-one.json", "auth_index": "a1", "next_retry_after": now.Add(3 * time.Hour)},
			{"provider": "claude", "name": "claude-two.json", "auth_index": "a2", "priority": 90},
			{"provider": "codex", "name": "codex-one.json", "auth_index": "a3", "priority": 95, "id_token": map[string]any{"chatgpt_account_id": "acct"}},
			{"provider": "claude", "name": "claude-three.json", "auth_index": "a5", "priority": 98},
			{"provider": "gemini", "name": "gemini.json", "auth_index": "a4", "disabled": true},
		}})
	case "/v0/management/api-call":
		var call apiCall

		json.NewDecoder(r.Body).Decode(&call)
		m.calls = append(m.calls, call)

		switch call.AuthIndex {
		case "a1":
			json.NewEncoder(w).Encode(apiCallResult{StatusCode: 200, Body: `{"five_hour":{"utilization":100,"resets_at":"2026-10-06T12:00:00Z"},"seven_day":{"utilization":55,"resets_at":"2026-10-06T19:00:00Z"}}`})
		case "a2":
			json.NewEncoder(w).Encode(apiCallResult{StatusCode: 401, Body: "{}"})
		case "a5":
			json.NewEncoder(w).Encode(apiCallResult{StatusCode: 200, Body: `{"seven_day":{"utilization":20,"resets_at":"2026-10-06T11:30:00Z"}}`})
		default:
			json.NewEncoder(w).Encode(apiCallResult{StatusCode: 200, Body: `{"rate_limit":{"primary_window":{"used_percent":12,"limit_window_seconds":18000,"reset_after_seconds":7000}}}`})
		}
	case "/v0/management/auth-files/fields":
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)

			return
		}

		var patch map[string]any

		json.NewDecoder(r.Body).Decode(&patch)
		m.patches = append(m.patches, patch)
		w.Write([]byte(`{"status":"ok"}`))
	}
}

func newMock(t *testing.T) (*mockProxy, proxy) {
	mock := &mockProxy{}
	server := httptest.NewServer(mock)
	t.Cleanup(server.Close)

	return mock, proxy{client: server.Client(), baseURL: server.URL, key: "test"}
}

func TestShow(t *testing.T) {
	mock, p := newMock(t)

	var out strings.Builder

	if err := show(&out, p, now); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"claude   claude-one.json\n    proxy cooldown until",
		"5h             100.0% used    0.0% left",
		"claude-two.json  (priority 90)\n    usage request failed: HTTP 401 from " + claudeUsageURL,
		"disabled in the proxy\n    usage not available for this provider",
		"Expiring unused within 24h:\n  claude-one.json 7d: 45% left, resets in 10h00m\n  claude-three.json 7d: 80% left, resets in 2h30m",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}

	if len(mock.calls) != 4 || mock.calls[0].Header["Authorization"] != "Bearer $TOKEN$" || mock.calls[2].Header["ChatGPT-Account-Id"] != "acct" {
		t.Errorf("api-calls = %+v", mock.calls)
	}

	if len(mock.patches) != 0 {
		t.Errorf("show must not change priorities: %+v", mock.patches)
	}

	p.key = "wrong"

	if err := show(&out, p, now); err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Errorf("a rejected management key must fail, got %v", err)
	}
}

func TestRoute(t *testing.T) {
	mock, p := newMock(t)

	var logs []string
	logf := func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) }

	if err := route(logf, p, now); err != nil {
		t.Fatal(err)
	}
	// claude-one is boosted (10h to reset -> 90), codex-one's stale boost is cleared, claude-two
	// (unreadable) and claude-three (already 98) are left alone, gemini is never touched.
	want := []map[string]any{
		{"name": "claude-one.json", "priority": float64(90)},
		{"name": "codex-one.json", "priority": float64(0)},
	}

	if fmt.Sprint(mock.patches) != fmt.Sprint(want) {
		t.Errorf("patches = %v, want %v", mock.patches, want)
	}

	joined := strings.Join(logs, "\n")

	for _, line := range []string{
		"claude-one.json: priority 0 -> 90 (7d: 45% left, resets in 10h00m)",
		"claude-two.json: usage request failed: HTTP 401 from " + claudeUsageURL + "; priority stays 90",
		"codex-one.json: priority 95 -> 0 (no weekly capacity expiring)",
		"checked 4 accounts: 2 boosted, 2 changed",
	} {
		if !strings.Contains(joined, line) {
			t.Errorf("logs lack %q:\n%s", line, joined)
		}
	}

	var out strings.Builder

	if err := routeEvery(&out, p, 0); err != nil || !strings.Contains(out.String(), "quota: ") {
		t.Errorf("a single routed run: err %v, output %q", err, out.String())
	}
}

func TestTargetPriority(t *testing.T) {
	soon, sooner := now.Add(20*time.Hour), now.Add(90*time.Minute)
	priority, reason := targetPriority([]window{{"7d", 50, &soon}, {"7d opus", 10, &sooner}, {"5h", 0, &sooner}}, now)

	if priority != 99 || reason != "7d opus: 90% left, resets in 1h30m" {
		t.Errorf("targetPriority = %d, %q", priority, reason)
	}

	if priority, _ := targetPriority([]window{{"5h", 10, &soon}}, now); priority != normalPriority {
		t.Errorf("no weekly window expiring must keep the normal priority, got %d", priority)
	}
}

func TestManagementKey(t *testing.T) {
	t.Setenv("MGMT_KEY", "")
	t.Setenv("MGMT_KEY_FILE", "")

	if _, err := managementKey(); err == nil {
		t.Error("a missing key must fail")
	}

	path := filepath.Join(t.TempDir(), "key")
	os.WriteFile(path, []byte("from-file\n"), 0o600)
	t.Setenv("MGMT_KEY_FILE", path)

	if key, err := managementKey(); key != "from-file" || err != nil {
		t.Errorf("file key = %q, %v", key, err)
	}

	t.Setenv("MGMT_KEY", "from-env")

	if key, _ := managementKey(); key != "from-env" {
		t.Errorf("env key = %q", key)
	}
}

func ptr(t time.Time) *time.Time { return &t }
