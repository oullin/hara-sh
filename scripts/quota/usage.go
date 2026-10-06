package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
)

const (
	claudeUsageURL = "https://api.anthropic.com/api/oauth/usage"
	codexUsageURL  = "https://chatgpt.com/backend-api/wham/usage"
)

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
