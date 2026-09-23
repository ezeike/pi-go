package blockbuilder

import (
	"encoding/json"
	"fmt"

	"github.com/machina/block-builder/sdk"
	pluginv1 "github.com/machina/block-builder/sdk/gen/block-builder/plugin/v1"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/protobuf/encoding/protojson"
)

// Task 5 (impl-specs/toolset.md): 4 plain-unary spec tools — accept_spec, validate_spec,
// get_spec, list_primitives. Each wraps one sdk.SpecClient method: build the request from the
// tool's typed input, call the client, return protojson.Marshal(resp) (UseProtoNames, to match
// this package's json:"plugin_id"-style snake_case convention) as the tool's output. RPC errors
// propagate unwrapped (spec.md "Error handling": no bespoke formatting).

// AcceptSpecInput is accept_spec's argument shape — field names/semantics match
// block-builder-mcp's AcceptSpecInput exactly (Global Constraints naming rule).
type AcceptSpecInput struct {
	PluginID       string `json:"plugin_id"`
	ConversationID string `json:"conversation_id"`
}

// acceptSpecTool wraps SpecClient.AcceptSpec: commits the conversation's proposed spec as the
// plugin's spec.
func acceptSpecTool(spec sdk.SpecClient) (tool.Tool, error) {
	return newTool("accept_spec", "Accepts a plugin spec conversation's proposal, committing it as the plugin's spec.",
		func(ctx agent.Context, in AcceptSpecInput) (map[string]any, error) {
			resp, err := spec.AcceptSpec(ctx, &pluginv1.AcceptSpecRequest{
				PluginId:       in.PluginID,
				ConversationId: in.ConversationID,
			})
			if err != nil {
				return nil, err
			}
			return protoResult(resp)
		})
}

// ValidateSpecInput is validate_spec's argument shape. Spec is optional: when omitted, the
// currently-committed spec is validated instead of an unaccepted draft (spec.pb.go's own doc
// comment on ValidateSpecRequest.Spec).
type ValidateSpecInput struct {
	PluginID string          `json:"plugin_id"`
	Spec     json.RawMessage `json:"spec,omitempty"`
}

// validateSpecTool wraps SpecClient.ValidateSpec.
func validateSpecTool(spec sdk.SpecClient) (tool.Tool, error) {
	return newTool("validate_spec", "Validates a plugin spec draft, or the currently committed spec when none is given.",
		func(ctx agent.Context, in ValidateSpecInput) (map[string]any, error) {
			req := &pluginv1.ValidateSpecRequest{PluginId: in.PluginID}
			if len(in.Spec) > 0 {
				var draft pluginv1.PluginSpec
				if err := protojson.Unmarshal(in.Spec, &draft); err != nil {
					return nil, fmt.Errorf("validate_spec: unmarshal spec: %w", err)
				}
				req.Spec = &draft
			}
			resp, err := spec.ValidateSpec(ctx, req)
			if err != nil {
				return nil, err
			}
			return protoResult(resp)
		})
}

// GetSpecInput is get_spec's argument shape.
type GetSpecInput struct {
	PluginID string `json:"plugin_id"`
}

// getSpecTool wraps SpecClient.GetSpec.
func getSpecTool(spec sdk.SpecClient) (tool.Tool, error) {
	return newTool("get_spec", "Fetches a plugin's currently committed spec.",
		func(ctx agent.Context, in GetSpecInput) (map[string]any, error) {
			resp, err := spec.GetSpec(ctx, &pluginv1.GetSpecRequest{PluginId: in.PluginID})
			if err != nil {
				return nil, err
			}
			return protoResult(resp)
		})
}

// ListPrimitivesInput is list_primitives' argument shape — confirmed empty against
// sdk/gen/block-builder/plugin/v1/spec.pb.go's ListPrimitivesRequest (no data fields),
// resolving impl-specs/toolset.md's OQ-A for this tool.
type ListPrimitivesInput struct{}

// listPrimitivesTool wraps SpecClient.ListPrimitives.
func listPrimitivesTool(spec sdk.SpecClient) (tool.Tool, error) {
	return newTool("list_primitives", "Lists the transaction primitives available for plugin specs.",
		func(ctx agent.Context, _ ListPrimitivesInput) (map[string]any, error) {
			resp, err := spec.ListPrimitives(ctx, &pluginv1.ListPrimitivesRequest{})
			if err != nil {
				return nil, err
			}
			return protoResult(resp)
		})
}
