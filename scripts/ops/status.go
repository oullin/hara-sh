package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// level grades one check: pass, warn (works, but needs a look), fail (broken) or skip (not checked,
// because a link it depends on is broken).
type level int

const (
	pass level = iota
	warn
	fail
	skip
)

func (l level) String() string {
	return [...]string{"✓", "!", "✗", "-"}[l]
}

// check is one line of `make status`: what was checked, how it went, and how to fix it.
type check struct {
	label  string
	level  level
	detail string
}

// status checks every link: the containers, the three addresses, the panel, the keys, the accounts.
type status struct {
	client  *http.Client
	keys    keys
	local   string
	network string
	project string
	sh      shell
}

func (s status) checks() []check {
	out := s.containers()
	local := s.address("this computer", s.local, "the proxy does not answer; run: make logs proxy")
	network := s.address("your network", s.network, "portless does not reach the proxy; run: make up")
	out = append(out, local, network, s.tailnet())

	// The rest follows from the first broken address, so it would only repeat that failure.
	out = append(out, after(network, "panel", s.panel))

	return append(out, after(local, "client key", s.clientKey), after(local, "accounts", s.accounts))
}

// after runs next only when the address it goes through answered.
func after(address check, label string, next func() check) check {
	if address.level == fail {
		return check{label, skip, "not checked: " + address.label + " fails"}
	}

	return next()
}

// report prints the checks and returns whether none failed.
func report(w io.Writer, checks []check) bool {
	ok := true

	for _, c := range checks {
		fmt.Fprintf(w, "%s %-19s %s\n", c.level, c.label, c.detail)
		ok = ok && c.level != fail
	}

	return ok
}

func (s status) containers() []check {
	var out []check

	for _, service := range []string{"proxy", "tailscale", "quota"} {
		name := fmt.Sprintf("%s-%s-1", s.project, service)
		label := service + " container"
		out = append(out, s.container(service, label, name))
	}

	return out
}

// container reads the state and, for the proxy, the health check result: "running/healthy".
func (s status) container(service, label, name string) check {
	out, err := s.sh("docker", "inspect", "-f", "{{.State.Status}}/{{if .State.Health}}{{.State.Health.Status}}{{end}}", name)

	if err != nil {
		return check{label, fail, "not found (is Docker running?); run: make up"}
	}

	state, health, _ := strings.Cut(strings.TrimSpace(string(out)), "/")

	switch {
	case state != "running":
		return check{label, fail, state + "; run: make up, then make logs " + service}
	case health == "unhealthy":
		return check{label, fail, "running but unhealthy; run: make logs " + service}
	case health == "starting":
		return check{label, warn, "starting"}
	case health != "":
		return check{label, pass, "running, " + health}
	default:
		return check{label, pass, "running"}
	}
}

// address checks that base answers /healthz.
func (s status) address(label, base, hint string) check {
	start := time.Now()
	_, code, err := proxy{client: s.short(), base: base}.get("/healthz", "")
	elapsed := time.Since(start).Round(time.Millisecond)

	switch {
	case err != nil:
		return check{label, fail, base + "  " + hint}
	case code != http.StatusOK:
		return check{label, fail, fmt.Sprintf("%s  %d; %s", base, code, hint)}
	default:
		return check{label, pass, fmt.Sprintf("%s  %s", base, elapsed)}
	}
}

// tailscaleStatus is the part of `tailscale status --json` that status reads.
type tailscaleStatus struct {
	BackendState string `json:"BackendState"`
	AuthURL      string `json:"AuthURL"`
	Self         struct {
		DNSName string `json:"DNSName"`
	} `json:"Self"`
}

func (s status) tailnet() check {
	const label = "your tailnet"
	out, err := s.sh("docker", "exec", s.project+"-tailscale-1", "tailscale", "status", "--json", "--peers=false")
	var ts tailscaleStatus

	if err != nil || json.Unmarshal(out, &ts) != nil {
		return check{label, warn, "Tailscale does not answer; run: make logs tailscale"}
	}

	switch ts.BackendState {
	case "Running":
		dns := strings.TrimSuffix(ts.Self.DNSName, ".")
		c := s.address(label, "https://"+dns, "the tailnet does not reach the proxy; run: make logs tailscale")

		if c.level == pass {
			c.detail += "  (base URL for other tools: https://" + dns + "/v1)"
		}

		return c
	case "NeedsLogin":
		if ts.AuthURL == "" {
			return check{label, warn, "not signed in; run: make status again in a few seconds for the link"}
		}

		return check{label, warn, "not signed in: open " + ts.AuthURL + " to add this device, then run: make status"}
	default:
		return check{label, warn, "Tailscale is " + ts.BackendState + "; run: make logs tailscale"}
	}
}

func (s status) panel() check {
	const label = "panel"
	url := s.network + "/management.html"
	_, code, err := proxy{client: s.short(), base: s.network}.get("/management.html", "")

	if err != nil || code != http.StatusOK {
		return check{label, fail, fmt.Sprintf("%s  %d; is panel/management.html built? run: make code panel, then make up", url, code)}
	}

	return check{label, pass, url}
}

func (s status) clientKey() check {
	const label = "client key"

	if s.keys.api == "" {
		return check{label, warn, "API_KEY is not set; run it through make status"}
	}

	ids, err := proxy{client: s.short(), base: s.local, keys: s.keys}.models()

	switch {
	case err != nil && strings.Contains(err.Error(), ": 401"):
		return check{label, fail, "the proxy rejects it; run: make ops keys, then make up"}
	case err != nil:
		return check{label, fail, err.Error()}
	default:
		return check{label, pass, fmt.Sprintf("accepted, %d models", len(ids))}
	}
}

func (s status) accounts() check {
	const label = "accounts"

	if s.keys.mgmt == "" {
		return check{label, warn, "MGMT_KEY is not set; run it through make status"}
	}

	accounts, err := proxy{client: s.short(), base: s.local, keys: s.keys}.accounts()

	switch {
	case err != nil && strings.Contains(err.Error(), ": 401"):
		return check{label, fail, "the management password is rejected; run: make ops keys, then make up"}
	case err != nil:
		return check{label, fail, err.Error()}
	case len(accounts) == 0:
		return check{label, warn, "none connected; see README, Add an account"}
	}

	counts := map[string]int{}
	var order []string

	for _, a := range accounts {
		state := a.state()

		if counts[state] == 0 {
			order = append(order, state)
		}

		counts[state]++
	}

	parts := make([]string, 0, len(order))
	lvl := pass

	for _, state := range order {
		parts = append(parts, fmt.Sprintf("%d %s", counts[state], state))

		if state != "active" {
			lvl = warn
		}
	}

	detail := strings.Join(parts, ", ")

	if lvl == warn {
		detail += "; details: make ops accounts"
	}

	return check{label, lvl, detail}
}

// short bounds each status call, so a dead address fails fast.
func (s status) short() *http.Client {
	c := *s.client
	c.Timeout = 5 * time.Second

	return &c
}
