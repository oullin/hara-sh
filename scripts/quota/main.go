// Command quota reads each pooled account's usage windows from the proxy and either prints them
// (`make quota`) or steers routing toward weekly capacity that would otherwise expire unused
// (-route, run by the quota service in local/compose.yaml whenever the proxy runs).
//
// The provider usage endpoints are called through the proxy's /v0/management/api-call, which
// substitutes $TOKEN$ with the account's OAuth token server-side: tokens never reach this program.
//
// Env: URL (default https://hara.local), and MGMT_KEY or MGMT_KEY_FILE (the management password).
package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	route := flag.Bool("route", false, "set account priorities so weekly capacity about to expire is used first")
	every := flag.Duration("every", 0, "with -route: repeat at this interval (0 runs once)")
	flag.Parse()

	key, err := managementKey()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	baseURL := os.Getenv("URL")
	if baseURL == "" {
		baseURL = "https://hara.local"
	}
	p := proxy{client: newClient(), baseURL: strings.TrimRight(baseURL, "/"), key: key}

	if *route {
		err = routeEvery(os.Stdout, p, *every)
	} else {
		err = show(os.Stdout, p, time.Now())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func managementKey() (string, error) {
	if key := os.Getenv("MGMT_KEY"); key != "" {
		return key, nil
	}
	if path := os.Getenv("MGMT_KEY_FILE"); path != "" {
		key, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if trimmed := strings.TrimSpace(string(key)); trimmed != "" {
			return trimmed, nil
		}
	}
	return "", errors.New("MGMT_KEY or MGMT_KEY_FILE must hold the management password (run it through `make quota`)")
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

// proxy calls the CLIProxyAPI management API.
type proxy struct {
	client  *http.Client
	baseURL string
	key     string
}

func (p proxy) call(method, path string, payload, into any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, p.baseURL+"/v0/management/"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from the proxy (%s)", resp.StatusCode, path)
	}
	if into == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

type account struct {
	Provider       string     `json:"provider"`
	Name           string     `json:"name"`
	AuthIndex      string     `json:"auth_index"`
	Disabled       bool       `json:"disabled"`
	Priority       int        `json:"priority"`
	NextRetryAfter *time.Time `json:"next_retry_after"`
	IDToken        struct {
		ChatGPTAccountID string `json:"chatgpt_account_id"`
	} `json:"id_token"`
}

// accountUsage is one account with its windows, or why they could not be read.
type accountUsage struct {
	account
	Supported bool
	Windows   []window
	Err       error
}

// inspect lists the pooled accounts and reads the usage windows of each supported one.
func inspect(p proxy, now time.Time) ([]accountUsage, error) {
	var listing struct {
		Files []account `json:"files"`
	}
	if err := p.call(http.MethodGet, "auth-files", nil, &listing); err != nil {
		return nil, err
	}
	usages := make([]accountUsage, 0, len(listing.Files))
	for _, acct := range listing.Files {
		usage := accountUsage{account: acct}
		if call, ok := usageCall(acct); ok {
			usage.Supported = true
			usage.Windows, usage.Err = fetchWindows(p, acct.Provider, call, now)
		}
		usages = append(usages, usage)
	}
	return usages, nil
}

func fetchWindows(p proxy, provider string, call apiCall, now time.Time) ([]window, error) {
	var result apiCallResult
	if err := p.call(http.MethodPost, "api-call", call, &result); err != nil {
		return nil, fmt.Errorf("usage request failed: %w", err)
	}
	if result.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("usage request failed: HTTP %d from %s", result.StatusCode, call.URL)
	}
	windows, err := parseWindows(provider, []byte(result.Body), now)
	if err != nil {
		return nil, fmt.Errorf("unreadable usage response: %w", err)
	}
	return windows, nil
}
