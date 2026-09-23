package blockbuilder

import (
	"encoding/json"

	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// protoResult renders a Connect-RPC response proto as a plain-unary tool's return value, using
// proto field names (UseProtoNames) to match this package's json:"plugin_id"-style snake_case
// convention — mirrors block-builder-mcp's internal/mcpserver/tools/spec_tools.go convention
// ("Pure mappers: MCP JSON args in, a Connect-RPC request proto out, the RPC response back as
// protojson").
//
// Returns map[string]any rather than json.RawMessage: functiontool.New infers a TResults
// handler's output JSON schema from the Go type (jsonschema.For[TResults]) and, when the
// produced value doesn't itself validate against that schema, Run silently wraps it as
// {"result": value} instead (google.golang.org/adk/v2@v2.4.0/tool/functiontool/function.go's
// Run — the "python impl allows basic types" fallback). json.RawMessage's inferred schema does
// not validate as the object it actually contains, so every tool consistently hit that fallback
// and callers saw an extra "result" wrapper key. map[string]any's inferred schema is an open
// object, so the conversion succeeds on the first path and the proto response's fields come
// back unwrapped, exactly as protojson produced them.
func protoResult(msg proto.Message) (map[string]any, error) {
	data, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// newTool builds a plain (non-long-running) ADK function tool. InputSchema/OutputSchema are left
// nil so functiontool.New infers them from the handler's argument/result struct tags (confirmed
// against the vendored google.golang.org/adk/v2@v2.4.0/tool/functiontool package: New falls back
// to jsonschema.For[T] whenever the corresponding Config field is nil).
//
// This package cannot reach internal/tools' unexported newTool/coercingTool/lenientSchema
// helpers (Go visibility, sibling packages — see impl-specs/toolset.md correction 5), so it
// keeps this minimal local equivalent instead. Unlike internal/tools' helper, arguments here are
// NOT given LLM-argument leniency (string->int/bool coercion, alias resolution) — a deliberate
// scope reduction, see impl-specs/toolset.md OQ-C.
func newTool[TArgs, TResults any](name, description string, handler functiontool.Func[TArgs, TResults]) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        name,
		Description: description,
	}, handler)
}

// newLongRunningTool is identical to newTool except IsLongRunning() reports true
// (functiontool.Config.IsLongRunning, confirmed present in the vendored adk v2.4.0). The handler
// itself must still return quickly — it starts a background drain and returns (see the streaming
// component's propose_spec_change/watch_run tools) — IsLongRunning only changes how the ADK
// agent loop treats the resulting FunctionCall/Response event pairing, not the handler's own
// blocking behavior.
func newLongRunningTool[TArgs, TResults any](name, description string, handler functiontool.Func[TArgs, TResults]) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:          name,
		Description:   description,
		IsLongRunning: true,
	}, handler)
}
