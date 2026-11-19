package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestInferenceIngress(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s %s %s", r.Method, r.URL.RequestURI(), r.Header.Get("Authorization"))
	}))

	defer upstream.Close()

	target, err := url.Parse(upstream.URL)

	if err != nil {
		t.Fatal(err)
	}

	handler := inferenceIngress(target)

	for _, route := range []string{
		"/healthz", "/v1/models", "/v1/responses?stream=true", "/backend-api/codex/responses",
		"/v1beta/models/gemini:generateContent", "/openai/v1/videos", "/muse-code/v1/models",
	} {
		request := httptest.NewRequest(http.MethodGet, route, nil)
		request.Header.Set("Authorization", "Bearer synthetic-client-key")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if want := "GET " + route + " Bearer synthetic-client-key"; response.Code != http.StatusOK || response.Body.String() != want {
			t.Errorf("%s: HTTP %d %q, want 200 %q", route, response.Code, response.Body, want)
		}
	}

	for _, route := range []string{
		"/", "/management.html", "/v0/management/config", "/v8/management/config",
		"/codex/callback", "/v1/tunnel", "/v1/../management.html", "/v1/%2e%2e/management.html",
	} {
		request := httptest.NewRequest(http.MethodGet, route, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusNotFound {
			t.Errorf("%s: HTTP %d, want 404", route, response.Code)
		}
	}
}

func TestInferenceIngressRejectsArguments(t *testing.T) {
	if err := (toolRuntime{}).run([]string{"ingress", "extra"}, nil, io.Discard); err == nil {
		t.Fatal("ingress accepted unexpected arguments")
	}
}
