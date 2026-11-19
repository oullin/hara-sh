package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"time"
)

func serveInference() error {
	server := &http.Server{
		Addr:              ":8318",
		Handler:           inferenceIngress(&url.URL{Scheme: "http", Host: "proxy:8317"}),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	return server.ListenAndServe()
}

func inferenceIngress(upstream *url.URL) http.Handler {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(upstream)
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !inferencePath(r.URL.Path) {
			http.NotFound(w, r)

			return
		}

		proxy.ServeHTTP(w, r)
	})
}

func inferencePath(requestPath string) bool {
	if requestPath == "/healthz" {
		return true
	}

	if requestPath != path.Clean(requestPath) || strings.Contains(requestPath, `\`) ||
		requestPath == "/v1/tunnel" || strings.HasPrefix(requestPath, "/v1/tunnel/") {
		return false
	}

	for _, prefix := range []string{"/v1/", "/v1beta/", "/openai/v1/", "/backend-api/codex/", "/muse-code/"} {
		if strings.HasPrefix(requestPath, prefix) {
			return true
		}
	}

	return false
}
