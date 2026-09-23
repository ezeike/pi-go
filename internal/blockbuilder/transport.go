package blockbuilder

import "net/http"

// apiKeyTransport injects the X-API-KEY header block-builder's caller-auth
// interceptor expects (block-builder's own CLAUDE.md: "a Connect interceptor
// resolves an auth.Identity from an X-API-KEY header") on every outbound
// request. It implements http.RoundTripper so it can wrap
// http.DefaultTransport as the connect.HTTPClient sdk.New requires.
type apiKeyTransport struct {
	base   http.RoundTripper
	apiKey string
}

// RoundTrip clones the request before mutating headers — http.RoundTripper's
// contract forbids mutating the original request in place (net/http docs:
// "RoundTrip should not modify the request").
func (t *apiKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.apiKey == "" {
		return t.base.RoundTrip(req)
	}
	cloned := req.Clone(req.Context())
	cloned.Header.Set("X-API-KEY", t.apiKey)
	return t.base.RoundTrip(cloned)
}

// newHTTPClient builds the connect.HTTPClient sdk.New consumes. No
// client-side Timeout is set: PluginRunService.WatchRun and
// PluginSpecService.ProposeSpecChange are long-lived streams by design, and
// a blanket http.Client.Timeout would truncate a legitimate stream —
// per-call cancellation is the caller's (toolset's) responsibility via ctx.
func newHTTPClient(apiKey string) *http.Client {
	return &http.Client{Transport: &apiKeyTransport{base: http.DefaultTransport, apiKey: apiKey}}
}
