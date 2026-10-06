package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// sample is one timed request.
type sample struct {
	firstToken time.Duration // until the first text delta
	total      time.Duration // until the stream ended
	input      int           // prompt tokens, cached ones included
	cached     int           // prompt tokens served from the provider's cache
}

// api is one wire protocol: how to build its request and read its stream.
type api struct {
	name  string
	path  string
	build func(model, prefix, nonce string) any
	auth  func(h http.Header, key string)
	// event reads one SSE data payload: whether it is the first text, and any usage it carries.
	event func(data []byte, s *sample) (text bool)
}

const prompt = "Reply with exactly: pong"

// messages is the Anthropic Messages API, as Claude Code speaks it. The prefix is marked for
// caching the way Claude Code marks its system prompt.
var messages = api{
	name: "claude",
	path: "/v1/messages",
	build: func(model, prefix, _ string) any {
		return map[string]any{
			"model":      model,
			"max_tokens": 16,
			"stream":     true,
			"system":     []any{map[string]any{"type": "text", "text": prefix, "cache_control": map[string]string{"type": "ephemeral"}}},
			"messages":   []any{map[string]any{"role": "user", "content": prompt}},
		}
	},
	auth: func(h http.Header, key string) {
		h.Set("x-api-key", key)
		h.Set("anthropic-version", "2023-06-01")
	},
	event: func(data []byte, s *sample) bool {
		var e struct {
			Type    string `json:"type"`
			Message struct {
				Usage struct {
					Input         int `json:"input_tokens"`
					CacheRead     int `json:"cache_read_input_tokens"`
					CacheCreation int `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Delta struct {
				Type string `json:"type"`
			} `json:"delta"`
		}

		if json.Unmarshal(data, &e) != nil {
			return false
		}

		if e.Type == "message_start" {
			u := e.Message.Usage
			s.input, s.cached = u.Input+u.CacheRead+u.CacheCreation, u.CacheRead
		}

		return e.Type == "content_block_delta" && e.Delta.Type == "text_delta"
	},
}

// responses is the OpenAI Responses API, as Codex speaks it over HTTP. prompt_cache_key keeps the
// run on one account, the way Codex's session does.
var responses = api{
	name: "codex",
	path: "/v1/responses",
	build: func(model, prefix, nonce string) any {
		return map[string]any{
			"model":            model,
			"stream":           true,
			"store":            false,
			"instructions":     prefix,
			"prompt_cache_key": "bench-" + nonce,
			"input":            []any{map[string]any{"role": "user", "content": []any{map[string]string{"type": "input_text", "text": prompt}}}},
		}
	},
	auth: func(h http.Header, key string) { h.Set("Authorization", "Bearer "+key) },
	event: func(data []byte, s *sample) bool {
		var e struct {
			Type     string `json:"type"`
			Response struct {
				Usage struct {
					Input   int `json:"input_tokens"`
					Details struct {
						Cached int `json:"cached_tokens"`
					} `json:"input_tokens_details"`
				} `json:"usage"`
			} `json:"response"`
		}

		if json.Unmarshal(data, &e) != nil {
			return false
		}

		if e.Type == "response.completed" {
			s.input, s.cached = e.Response.Usage.Input, e.Response.Usage.Details.Cached
		}

		return e.Type == "response.output_text.delta"
	},
}

// run sends one streaming request and times it.
func (a api) run(client *http.Client, base, key, model, prefix, nonce string) (sample, error) {
	body, err := json.Marshal(a.build(model, prefix, nonce))

	if err != nil {
		return sample{}, err
	}

	req, err := http.NewRequest(http.MethodPost, base+a.path, bytes.NewReader(body))

	if err != nil {
		return sample{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	a.auth(req.Header, key)

	start := time.Now()
	resp, err := client.Do(req)

	if err != nil {
		return sample{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))

		return sample{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	return a.read(resp.Body, start)
}

// read consumes the SSE stream, noting when the first text arrived.
func (a api) read(r io.Reader, start time.Time) (sample, error) {
	var s sample

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)

	for scanner.Scan() {
		data, ok := strings.CutPrefix(scanner.Text(), "data:")

		if !ok {
			continue
		}

		if a.event([]byte(strings.TrimSpace(data)), &s) && s.firstToken == 0 {
			s.firstToken = time.Since(start)
		}
	}

	if err := scanner.Err(); err != nil {
		return s, err
	}

	if s.firstToken == 0 {
		return s, fmt.Errorf("the stream ended without any text")
	}

	s.total = time.Since(start)

	return s, nil
}
