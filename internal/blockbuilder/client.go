package blockbuilder

import (
	"errors"

	"github.com/machina/block-builder/sdk"
)

// ErrNotConfigured is returned by NewClient when BLOCK_BUILDER_URL resolves
// to empty. Callers (the toolset wiring) check this via errors.Is to decide
// "register no block-builder tools this run" — decision D8: unconfigured
// means absent from the tool registry, never present-and-erroring.
var ErrNotConfigured = errors.New("blockbuilder: not configured (BLOCK_BUILDER_URL unset)")

// NewClient resolves block-builder connectivity from the environment and
// returns the two narrow SDK ports block-builder-mcp's own tool handlers
// use (sdk.SpecClient, sdk.RunClient — see sdk/client.go's doc comment in
// the block-builder repo). It performs no network I/O: the generated
// *v1connect client constructors sdk.New wraps dial lazily per RPC, so a
// successful return here means only "configuration looks usable", not
// "block-builder is reachable".
//
// The two returned values are the same *sdk.Adapter instance under two
// interface views — callers get interfaces, never *sdk.Adapter itself, so
// a caller's own tests can substitute github.com/machina/block-builder/sdk/sdktest's
// fakes without a real block-builder instance.
func NewClient() (sdk.SpecClient, sdk.RunClient, error) {
	cfg := resolveConfig()
	if cfg.baseURL == "" {
		return nil, nil, ErrNotConfigured
	}
	if err := validateBaseURL(cfg.baseURL); err != nil {
		return nil, nil, err
	}
	adapter := sdk.New(newHTTPClient(cfg.apiKey), cfg.baseURL)
	return adapter, adapter, nil
}
