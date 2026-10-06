package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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

func TestRun(t *testing.T) {
	cooldown := now.Add(3 * time.Hour)
	var calls []apiCall
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v0/management/auth-files":
			json.NewEncoder(w).Encode(map[string]any{"files": []map[string]any{
				{"provider": "claude", "name": "claude-one.json", "auth_index": "a1", "next_retry_after": cooldown},
				{"provider": "claude", "name": "claude-two.json", "auth_index": "a2"},
				{"provider": "codex", "name": "codex-one.json", "auth_index": "a3", "id_token": map[string]any{"chatgpt_account_id": "acct"}},
				{"provider": "gemini", "name": "gemini.json", "auth_index": "a4", "disabled": true},
			}})
		case "/v0/management/api-call":
			var call apiCall
			json.NewDecoder(r.Body).Decode(&call)
			calls = append(calls, call)
			switch call.AuthIndex {
			case "a1":
				json.NewEncoder(w).Encode(apiCallResult{StatusCode: 200, Body: `{"five_hour":{"utilization":100,"resets_at":"2026-10-06T12:00:00Z"},"seven_day":{"utilization":55,"resets_at":"2026-10-06T19:00:00Z"}}`})
			case "a2":
				json.NewEncoder(w).Encode(apiCallResult{StatusCode: 401, Body: "{}"})
			default:
				json.NewEncoder(w).Encode(apiCallResult{StatusCode: 200, Body: `{"rate_limit":{"primary_window":{"used_percent":12,"limit_window_seconds":18000,"reset_after_seconds":7000}}}`})
			}
		}
	}))
	defer server.Close()

	var out strings.Builder
	if err := run(&out, server.Client(), server.URL, "test", now); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"proxy cooldown until",
		"5h             100.0% used    0.0% left",
		"usage request failed: HTTP 401 from " + claudeUsageURL,
		"disabled in the proxy",
		"usage not available for this provider",
		"Expiring unused within 24h:\n  claude-one.json 7d: 45% left, resets in 10h00m",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if len(calls) != 3 || calls[0].Header["Authorization"] != "Bearer $TOKEN$" || calls[2].Header["ChatGPT-Account-Id"] != "acct" {
		t.Errorf("api-calls = %+v", calls)
	}

	if err := run(&out, server.Client(), server.URL, "wrong", now); err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Errorf("a rejected management key must fail, got %v", err)
	}
}
