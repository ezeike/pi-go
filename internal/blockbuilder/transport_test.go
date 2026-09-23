package blockbuilder

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNewHTTPClient_SetsAPIKeyHeader pins the one behavior block-builder's
// caller-auth interceptor requires: every outbound request carries
// X-API-KEY when a key is configured.
func TestNewHTTPClient_SetsAPIKeyHeader(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-API-KEY")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := newHTTPClient("secret-key").Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	resp.Body.Close()

	if got != "secret-key" {
		t.Fatalf("X-API-KEY header = %q, want %q", got, "secret-key")
	}
}

// TestNewHTTPClient_NoKeyNoHeader documents the anonymous-caller path: an
// empty apiKey must not send an empty X-API-KEY header (block-builder's auth
// interceptor treats a *presented* credential as always-verified, so an
// empty-but-present header would be rejected rather than treated as
// anonymous).
func TestNewHTTPClient_NoKeyNoHeader(t *testing.T) {
	var got string
	sawHeader := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, sawHeader = r.Header.Get("X-API-KEY"), r.Header.Get("X-API-KEY") != ""
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := newHTTPClient("").Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	resp.Body.Close()

	if sawHeader {
		t.Fatalf("X-API-KEY header = %q, want no header sent", got)
	}
}

// TestApiKeyTransport_DoesNotMutateOriginalRequest pins http.RoundTripper's
// contract ("RoundTrip should not modify the request") — a violation here
// would corrupt a caller's request object across retries/redirects.
func TestApiKeyTransport_DoesNotMutateOriginalRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	tr := &apiKeyTransport{base: http.DefaultTransport, apiKey: "secret-key"}

	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	resp.Body.Close()

	if req.Header.Get("X-API-KEY") != "" {
		t.Fatalf("original request was mutated: X-API-KEY = %q, want empty (RoundTripper must clone)",
			req.Header.Get("X-API-KEY"))
	}
}
