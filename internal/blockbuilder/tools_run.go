package blockbuilder

import (
	"github.com/machina/block-builder/sdk"
	pluginv1 "github.com/machina/block-builder/sdk/gen/block-builder/plugin/v1"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
)

// Task 7a (impl-specs/toolset.md): start_run, get_run, get_manifest — 3 plain-unary run tools,
// same round-trip + protojson-marshal-response convention as tools_spec.go's task 5 tools.
// watch_run/get_run_progress (the long-running/buffered pair) are the streaming component's
// ground (internal/blockbuilder/streaming_tools.go) — NOT built here.

// StartRunInput is start_run's argument shape — field names confirmed directly against
// sdk/gen/block-builder/plugin/v1/run.pb.go's StartRunRequest.
type StartRunInput struct {
	PluginID            string   `json:"plugin_id"`
	Only                []string `json:"only,omitempty"`
	DeterministicOnly   bool     `json:"deterministic_only,omitempty"`
	SynthModel          string   `json:"synth_model,omitempty"`
	ReviewModel         string   `json:"review_model,omitempty"`
	VerifyOnly          bool     `json:"verify_only,omitempty"`
	SamplingProfile     string   `json:"sampling_profile,omitempty"`
	ImplSamplingProfile string   `json:"impl_sampling_profile,omitempty"`
}

// startRunTool wraps RunClient.StartRun.
func startRunTool(run sdk.RunClient) (tool.Tool, error) {
	return newTool("start_run", "Starts a codegen run for a plugin.",
		func(ctx agent.Context, in StartRunInput) (map[string]any, error) {
			resp, err := run.StartRun(ctx, &pluginv1.StartRunRequest{
				PluginId:            in.PluginID,
				Only:                in.Only,
				DeterministicOnly:   in.DeterministicOnly,
				SynthModel:          in.SynthModel,
				ReviewModel:         in.ReviewModel,
				VerifyOnly:          in.VerifyOnly,
				SamplingProfile:     in.SamplingProfile,
				ImplSamplingProfile: in.ImplSamplingProfile,
			})
			if err != nil {
				return nil, err
			}
			return protoResult(resp)
		})
}

// GetRunInput is get_run's argument shape.
type GetRunInput struct {
	PluginID string `json:"plugin_id"`
	RunID    string `json:"run_id"`
}

// getRunTool wraps RunClient.GetRun.
func getRunTool(run sdk.RunClient) (tool.Tool, error) {
	return newTool("get_run", "Fetches a codegen run's current status.",
		func(ctx agent.Context, in GetRunInput) (map[string]any, error) {
			resp, err := run.GetRun(ctx, &pluginv1.GetRunRequest{
				PluginId: in.PluginID,
				RunId:    in.RunID,
			})
			if err != nil {
				return nil, err
			}
			return protoResult(resp)
		})
}

// GetManifestInput is get_manifest's argument shape.
type GetManifestInput struct {
	PluginID string `json:"plugin_id"`
}

// getManifestTool wraps RunClient.GetManifest.
func getManifestTool(run sdk.RunClient) (tool.Tool, error) {
	return newTool("get_manifest", "Fetches a plugin's build manifest (resolves the plugin's current/latest run server-side).",
		func(ctx agent.Context, in GetManifestInput) (map[string]any, error) {
			resp, err := run.GetManifest(ctx, &pluginv1.GetManifestRequest{PluginId: in.PluginID})
			if err != nil {
				return nil, err
			}
			return protoResult(resp)
		})
}
