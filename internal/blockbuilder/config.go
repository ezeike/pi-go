// Package blockbuilder is pi-go's authenticated transport/construction layer
// for block-builder connectivity. It resolves connection settings from the
// environment and returns the two narrow SDK ports
// (github.com/machina/block-builder/sdk's SpecClient, RunClient) a caller's
// toolset wires up as tools — it holds no tool-registration or tool-calling
// logic itself.
package blockbuilder

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/dimetron/pi-go/internal/config"
	"github.com/dimetron/pi-go/internal/logger"
)

// envURL and envAPIKey name the environment/.env keys config.LookupEnv
// resolves for block-builder connectivity — see config.LookupEnv's doc
// comment for the .pi-go/.env / .env / ~/.pi-go/.env precedence this gives
// for free (process env wins, then nearest project file, then the home
// file).
const (
	envURL    = "BLOCK_BUILDER_URL"
	envAPIKey = "BLOCK_BUILDER_API_KEY"
)

// resolvedConfig is the outcome of resolving block-builder connection
// settings from the environment. A zero-value resolvedConfig (empty
// baseURL) means block-builder is not configured for this run.
type resolvedConfig struct {
	baseURL   string
	apiKey    string
	urlSource string // for a future wiring-time log line; never logged with apiKey
}

// resolveConfig reads BLOCK_BUILDER_URL and BLOCK_BUILDER_API_KEY via
// config.LookupEnv (decision D4: never os.Getenv directly).
func resolveConfig() resolvedConfig {
	rawURL, source := config.LookupEnv(envURL)
	rawKey, _ := config.LookupEnv(envAPIKey)
	return resolvedConfig{
		baseURL:   strings.TrimSpace(rawURL),
		apiKey:    strings.TrimSpace(rawKey),
		urlSource: source,
	}
}

// validateBaseURL rejects a configured-but-malformed base URL early. The
// generated *v1connect client constructors sdk.New wraps perform no
// validation of their own (confirmed by reading sdk/adapter.go's New doc
// comment), so a bad URL would otherwise surface as a confusing per-RPC
// dial error instead of a clear wiring-time one.
func validateBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		// Deliberately not %w-wrapping err: url.Error's own message embeds the raw
		// input verbatim, and this error can reach a log line (see the buildToolsets
		// wiring callers) — BLOCK_BUILDER_URL is user/environment supplied and could
		// carry userinfo/a query token even while failing to parse as a whole.
		return fmt.Errorf("blockbuilder: %s is not a valid URL", envURL)
	}
	if !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("blockbuilder: %s must be an absolute http(s) URL, got %q", envURL, logger.SafeBaseURL(raw))
	}
	return nil
}
