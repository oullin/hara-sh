package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// proxy calls one address of the proxy.
type proxy struct {
	client *http.Client
	base   string
	keys   keys
}

// account is one entry of the management API's auth-files listing.
type account struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Disabled bool   `json:"disabled"`
}

// get fetches path with the given Authorization bearer token and returns the body and status code.
func (p proxy) get(path, token string) ([]byte, int, error) {
	header := http.Header{}

	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}

	return p.do(http.MethodGet, path, header, nil)
}

func (p proxy) do(method, path string, header http.Header, body []byte) ([]byte, int, error) {
	req, err := http.NewRequest(method, p.base+path, bytes.NewReader(body))

	if err != nil {
		return nil, 0, err
	}

	req.Header = header

	res, err := p.client.Do(req)

	if err != nil {
		return nil, 0, err
	}

	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)

	return data, res.StatusCode, err
}

// getJSON fetches path and decodes a 200 answer into out.
func (p proxy) getJSON(path, token string, out any) error {
	data, code, err := p.get(path, token)

	if err != nil {
		return err
	}

	if code != http.StatusOK {
		return fmt.Errorf("GET %s: %d %s", path, code, firstLine(data))
	}

	return json.Unmarshal(data, out)
}

func (p proxy) models() ([]string, error) {
	var listing struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := p.getJSON("/v1/models", p.keys.api, &listing); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(listing.Data))

	for _, m := range listing.Data {
		ids = append(ids, m.ID)
	}

	return ids, nil
}

func (p proxy) accounts() ([]account, error) {
	var listing struct {
		Files []account `json:"files"`
	}

	err := p.getJSON("/v0/management/auth-files", p.keys.mgmt, &listing)

	return listing.Files, err
}

func (p proxy) printModels(w io.Writer) error {
	ids, err := p.models()

	for _, id := range ids {
		fmt.Fprintln(w, id)
	}

	return err
}

func (p proxy) printAccounts(w io.Writer) error {
	accounts, err := p.accounts()

	for _, a := range accounts {
		fmt.Fprintf(w, "%-8s %-8s %s\n", a.Provider, a.state(), a.Name)
	}

	return err
}

func (a account) state() string {
	if a.Disabled {
		return "disabled"
	}

	return a.Status
}

func (p proxy) printLogs(w io.Writer, n int) error {
	var logs struct {
		Lines []string `json:"lines"`
	}

	if err := p.getJSON(fmt.Sprintf("/v0/management/logs?limit=%d", n), p.keys.mgmt, &logs); err != nil {
		return err
	}

	for _, line := range logs.Lines {
		fmt.Fprintln(w, line)
	}

	return nil
}

// smoke asks for "pong" through the Messages API and prints the answer.
func (p proxy) smoke(w io.Writer, model string) error {
	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 16,
		"messages":   []map[string]string{{"role": "user", "content": "Reply with exactly: pong"}},
	})
	header := http.Header{
		"X-Api-Key":         {p.keys.api},
		"Anthropic-Version": {"2023-06-01"},
		"Content-Type":      {"application/json"},
	}
	data, code, err := p.do(http.MethodPost, "/v1/messages", header, body)

	if err != nil {
		return err
	}

	var answer struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if code != http.StatusOK || json.Unmarshal(data, &answer) != nil || len(answer.Content) == 0 {
		return fmt.Errorf("%s: %d %s", model, code, firstLine(data))
	}

	fmt.Fprintln(w, answer.Content[0].Text)

	return nil
}

// firstLine shortens an error body for one line of output.
func firstLine(data []byte) string {
	line, _, _ := strings.Cut(strings.TrimSpace(string(data)), "\n")

	if len(line) > 160 {
		return line[:160] + "…"
	}

	return line
}
