// Command quota prints each pooled account's usage windows, when they reset, and the weekly
// capacity about to expire unused. Run it through `make quota` (URL and MGMT_KEY from the env).
//
// The provider usage endpoints are called through the proxy's /v0/management/api-call, which
// substitutes $TOKEN$ with the account's OAuth token server-side: tokens never reach this program.
package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	claudeUsageURL = "https://api.anthropic.com/api/oauth/usage"
	codexUsageURL  = "https://chatgpt.com/backend-api/wham/usage"
	// A weekly window resetting this soon with capacity left is reported as expiring.
	expiringWithin = 24 * time.Hour
)

func main() {
	baseURL := os.Getenv("URL")
	if baseURL == "" {
		baseURL = "https://hara.local"
	}
	key := os.Getenv("MGMT_KEY")
	if key == "" {
		fmt.Fprintln(os.Stderr, "error: MGMT_KEY is not set (run it through `make quota`)")
		os.Exit(1)
	}
	if err := run(os.Stdout, newClient(), strings.TrimRight(baseURL, "/"), key, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// newClient trusts portless's own certificate authority, which signs https://hara.local.
func newClient() *http.Client {
	client := &http.Client{Timeout: 30 * time.Second}
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

type account struct {
	Provider       string     `json:"provider"`
	Name           string     `json:"name"`
	AuthIndex      string     `json:"auth_index"`
	Disabled       bool       `json:"disabled"`
	NextRetryAfter *time.Time `json:"next_retry_after"`
	IDToken        struct {
		ChatGPTAccountID string `json:"chatgpt_account_id"`
	} `json:"id_token"`
}

type apiCall struct {
	AuthIndex string            `json:"auth_index"`
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Header    map[string]string `json:"header"`
}

type apiCallResult struct {
	StatusCode int    `json:"status_code"`
	Body       string `json:"body"`
}

// window is one usage limit: Reset is nil while the window has not started (it starts on first use).
type window struct {
	Label string
	Used  float64
	Reset *time.Time
}

func run(out io.Writer, client *http.Client, baseURL, key string, now time.Time) error {
	var listing struct {
		Files []account `json:"files"`
	}
	if err := management(client, baseURL, key, http.MethodGet, "auth-files", nil, &listing); err != nil {
		return err
	}

	var expiring []string
	for _, acct := range listing.Files {
		fmt.Fprintf(out, "%-8s %s\n", acct.Provider, acct.Name)
		if acct.Disabled {
			fmt.Fprintln(out, "    disabled in the proxy")
		} else if acct.NextRetryAfter != nil && acct.NextRetryAfter.After(now) {
			fmt.Fprintf(out, "    proxy cooldown until %s (in %s)\n", clock(*acct.NextRetryAfter), until(*acct.NextRetryAfter, now))
		}

		call, ok := usageCall(acct)
		if !ok {
			fmt.Fprintln(out, "    usage not available for this provider")
			continue
		}
		var result apiCallResult
		if err := management(client, baseURL, key, http.MethodPost, "api-call", call, &result); err != nil {
			fmt.Fprintf(out, "    usage request failed: %v\n", err)
			continue
		}
		if result.StatusCode != http.StatusOK {
			fmt.Fprintf(out, "    usage request failed: HTTP %d from %s\n", result.StatusCode, call.URL)
			continue
		}
		windows, err := parseWindows(acct.Provider, []byte(result.Body), now)
		if err != nil {
			fmt.Fprintf(out, "    unreadable usage response: %v\n", err)
			continue
		}
		expiring = append(expiring, report(out, acct.Name, windows, now)...)
	}

	if len(expiring) > 0 {
		fmt.Fprintf(out, "\nExpiring unused within %dh:\n", int(expiringWithin.Hours()))
		for _, line := range expiring {
			fmt.Fprintf(out, "  %s\n", line)
		}
	}
	return nil
}

func management(client *http.Client, baseURL, key, method, path string, payload, into any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, baseURL+"/v0/management/"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from the proxy (%s)", resp.StatusCode, path)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

// usageCall is the api-call request that reads this account's usage; false for unsupported providers.
func usageCall(acct account) (apiCall, bool) {
	switch acct.Provider {
	case "claude":
		return apiCall{
			AuthIndex: acct.AuthIndex,
			Method:    http.MethodGet,
			URL:       claudeUsageURL,
			Header:    map[string]string{"Authorization": "Bearer $TOKEN$", "anthropic-beta": "oauth-2025-04-20"},
		}, true
	case "codex":
		header := map[string]string{"Authorization": "Bearer $TOKEN$", "User-Agent": "codex_cli_rs"}
		if acct.IDToken.ChatGPTAccountID != "" {
			header["ChatGPT-Account-Id"] = acct.IDToken.ChatGPTAccountID
		}
		return apiCall{AuthIndex: acct.AuthIndex, Method: http.MethodGet, URL: codexUsageURL, Header: header}, true
	}
	return apiCall{}, false
}

func parseWindows(provider string, body []byte, now time.Time) ([]window, error) {
	if provider == "claude" {
		return claudeWindows(body)
	}
	return codexWindows(body, now)
}

// claudeWindows reads every window of /api/oauth/usage (five_hour, seven_day, seven_day_opus, ...).
func claudeWindows(body []byte) ([]window, error) {
	var usage map[string]json.RawMessage
	if err := json.Unmarshal(body, &usage); err != nil {
		return nil, err
	}
	var windows, extra []window
	for _, key := range []string{"five_hour", "seven_day"} {
		if w, ok := claudeWindow(key, usage[key]); ok {
			windows = append(windows, w)
		}
	}
	for key, raw := range usage {
		if key == "five_hour" || key == "seven_day" {
			continue
		}
		if w, ok := claudeWindow(key, raw); ok {
			extra = append(extra, w)
		}
	}
	// Model-specific windows follow 5h and 7d, sorted by label (map order is random).
	slices.SortFunc(extra, func(a, b window) int { return strings.Compare(a.Label, b.Label) })
	return append(windows, extra...), nil
}

func claudeWindow(key string, raw json.RawMessage) (window, bool) {
	var w struct {
		Utilization *float64 `json:"utilization"`
		ResetsAt    *string  `json:"resets_at"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &w) != nil || w.Utilization == nil {
		return window{}, false
	}
	label := strings.NewReplacer("five_hour", "5h", "seven_day", "7d", "_", " ").Replace(key)
	result := window{Label: label, Used: *w.Utilization}
	if w.ResetsAt != nil {
		if reset, err := time.Parse(time.RFC3339Nano, *w.ResetsAt); err == nil {
			result.Reset = &reset
		}
	}
	return result, true
}

type codexWindow struct {
	UsedPercent        float64  `json:"used_percent"`
	LimitWindowSeconds float64  `json:"limit_window_seconds"`
	ResetAfterSeconds  *float64 `json:"reset_after_seconds"`
	ResetAt            *float64 `json:"reset_at"`
}

type codexRateLimit struct {
	Primary   *codexWindow `json:"primary_window"`
	Secondary *codexWindow `json:"secondary_window"`
}

// codexWindows reads the main and any additional (per-model) limits of /backend-api/wham/usage.
func codexWindows(body []byte, now time.Time) ([]window, error) {
	var usage struct {
		RateLimit  codexRateLimit `json:"rate_limit"`
		Additional []struct {
			LimitName      string         `json:"limit_name"`
			MeteredFeature string         `json:"metered_feature"`
			RateLimit      codexRateLimit `json:"rate_limit"`
		} `json:"additional_rate_limits"`
	}
	if err := json.Unmarshal(body, &usage); err != nil {
		return nil, err
	}
	windows := codexRateLimitWindows(usage.RateLimit, "", now)
	for _, extra := range usage.Additional {
		name := extra.LimitName
		if name == "" {
			name = extra.MeteredFeature
		}
		if name == "" {
			name = "extra"
		}
		windows = append(windows, codexRateLimitWindows(extra.RateLimit, name+" ", now)...)
	}
	return windows, nil
}

func codexRateLimitWindows(limit codexRateLimit, prefix string, now time.Time) []window {
	var windows []window
	for _, slot := range []struct {
		w        *codexWindow
		fallback string
	}{{limit.Primary, "primary"}, {limit.Secondary, "secondary"}} {
		if slot.w == nil {
			continue
		}
		result := window{Label: prefix + codexLabel(slot.w.LimitWindowSeconds, slot.fallback), Used: slot.w.UsedPercent}
		if slot.w.ResetAt != nil {
			reset := time.Unix(int64(*slot.w.ResetAt), 0)
			result.Reset = &reset
		} else if slot.w.ResetAfterSeconds != nil {
			reset := now.Add(time.Duration(*slot.w.ResetAfterSeconds * float64(time.Second)))
			result.Reset = &reset
		}
		windows = append(windows, result)
	}
	return windows
}

func codexLabel(seconds float64, fallback string) string {
	s := int64(seconds)
	switch {
	case s > 0 && s%86400 == 0:
		return fmt.Sprintf("%dd", s/86400)
	case s > 0 && s%3600 == 0:
		return fmt.Sprintf("%dh", s/3600)
	}
	return fallback
}

// report prints one account's windows and returns the weekly ones expiring with capacity left.
func report(out io.Writer, name string, windows []window, now time.Time) []string {
	if len(windows) == 0 {
		fmt.Fprintln(out, "    no usage windows in the response")
	}
	var expiring []string
	for _, w := range windows {
		when := "no active window (starts on first use)"
		if w.Reset != nil {
			when = fmt.Sprintf("resets %s (in %s)", clock(*w.Reset), until(*w.Reset, now))
		}
		fmt.Fprintf(out, "    %-14s %5.1f%% used  %5.1f%% left  %s\n", w.Label, w.Used, max(0, 100-w.Used), when)
		if isExpiring(w, now) {
			expiring = append(expiring, fmt.Sprintf("%s %s: %.0f%% left, resets in %s", name, w.Label, 100-w.Used, until(*w.Reset, now)))
		}
	}
	return expiring
}

func isExpiring(w window, now time.Time) bool {
	weekly := false
	for _, field := range strings.Fields(w.Label) {
		weekly = weekly || field == "7d"
	}
	return weekly && w.Reset != nil && w.Used < 100 && w.Reset.Sub(now) <= expiringWithin
}

func clock(t time.Time) string {
	return t.Local().Format("Mon 02 Jan 15:04")
}

func until(t, now time.Time) string {
	d := max(0, t.Sub(now))
	days := int(d / (24 * time.Hour))
	hours := int(d/time.Hour) % 24
	minutes := int(d/time.Minute) % 60
	if days > 0 {
		return fmt.Sprintf("%dd%02dh", days, hours)
	}
	return fmt.Sprintf("%dh%02dm", hours, minutes)
}
