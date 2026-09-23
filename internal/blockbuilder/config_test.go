package blockbuilder

import "testing"

// TestResolveConfig_TrimsWhitespaceAndReadsBoth pins resolveConfig's contract:
// both BLOCK_BUILDER_URL and BLOCK_BUILDER_API_KEY are read via
// config.LookupEnv (decision D4 — never os.Getenv), and the reported
// urlSource distinguishes "read from the process environment" from a future
// .env-file-sourced value for a wiring-time log line that never includes the
// key itself.
func TestResolveConfig_TrimsWhitespaceAndReadsBoth(t *testing.T) {
	t.Setenv("BLOCK_BUILDER_URL", "  https://bb.example.com  ")
	t.Setenv("BLOCK_BUILDER_API_KEY", "secret")

	got := resolveConfig()

	if got.baseURL != "https://bb.example.com" {
		t.Fatalf("baseURL = %q, want trimmed URL", got.baseURL)
	}
	if got.apiKey != "secret" {
		t.Fatalf("apiKey = %q, want %q", got.apiKey, "secret")
	}
	if got.urlSource != "environment" {
		t.Fatalf("urlSource = %q, want %q", got.urlSource, "environment")
	}
}

// TestResolveConfig_UnsetIsEmpty documents the "not configured" input shape
// NewClient's ErrNotConfigured branch (Task 3) keys off: both vars empty is
// the zero-value resolvedConfig, not a resolution error.
func TestResolveConfig_UnsetIsEmpty(t *testing.T) {
	t.Setenv("BLOCK_BUILDER_URL", "")
	t.Setenv("BLOCK_BUILDER_API_KEY", "")

	got := resolveConfig()

	if got.baseURL != "" {
		t.Fatalf("baseURL = %q, want empty when unset", got.baseURL)
	}
	if got.apiKey != "" {
		t.Fatalf("apiKey = %q, want empty when unset", got.apiKey)
	}
}

// TestValidateBaseURL covers the accept/reject boundary NewClient (Task 3)
// relies on to turn a malformed-but-present URL into a clear wiring-time
// error instead of a confusing per-RPC dial failure.
func TestValidateBaseURL(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"valid https", "https://bb.example.com", false},
		{"valid http with port", "http://localhost:8080", false},
		{"relative path", "/no-host", true},
		{"non-http scheme", "ftp://bb.example.com", true},
		{"unparseable", "://bad", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBaseURL(tc.raw)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateBaseURL(%q) err = %v, wantErr %v", tc.raw, err, tc.wantErr)
			}
		})
	}
}
