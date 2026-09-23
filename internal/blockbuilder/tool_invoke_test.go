package blockbuilder

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/adk/v2/agent"
	adkmemory "google.golang.org/adk/v2/memory"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool/toolconfirmation"
	"google.golang.org/genai"
)

// mockToolCtx is a minimal agent.Context backed by a real context.Context, letting tests invoke
// tools through the tool.Tool Run interface (exercising the functiontool factory closures the
// streaming/progress tools are built from) without a live ADK agent/session. Adapted from
// internal/tools/tool_invoke_test.go's mockToolCtx — that helper is package-private to
// internal/tools and not importable from here, so this component keeps its own copy. Full v2
// method set implemented so it stays forward-compatible with agent.Context surface growth.
type mockToolCtx struct {
	context.Context
}

func (mockToolCtx) FunctionCallID() string         { return "" }
func (mockToolCtx) Actions() *session.EventActions { return &session.EventActions{} }
func (mockToolCtx) SearchMemory(context.Context, string) (*adkmemory.SearchResponse, error) {
	return nil, nil
}
func (mockToolCtx) ToolConfirmation() *toolconfirmation.ToolConfirmation { return nil }
func (mockToolCtx) RequestConfirmation(string, any) error                { return nil }
func (mockToolCtx) AgentName() string                                    { return "" }
func (mockToolCtx) ReadonlyState() session.ReadonlyState                 { return nil }
func (mockToolCtx) State() session.State                                 { return nil }
func (mockToolCtx) Artifacts() agent.Artifacts                           { return nil }
func (mockToolCtx) InvocationID() string                                 { return "" }
func (mockToolCtx) UserContent() *genai.Content                          { return nil }
func (mockToolCtx) AppName() string                                      { return "" }
func (mockToolCtx) Branch() string                                       { return "" }
func (mockToolCtx) SessionID() string                                    { return "" }
func (mockToolCtx) UserID() string                                       { return "" }

// InvocationContext surface.
func (mockToolCtx) Agent() agent.Agent          { return nil }
func (mockToolCtx) Memory() agent.Memory        { return nil }
func (mockToolCtx) Session() session.Session    { return nil }
func (mockToolCtx) RunConfig() *agent.RunConfig { return nil }
func (mockToolCtx) EndInvocation()              {}
func (mockToolCtx) Ended() bool                 { return false }
func (mockToolCtx) WithContext(ctx context.Context) agent.InvocationContext {
	return mockToolCtx{Context: ctx}
}
func (mockToolCtx) IsolationScope() string { return "" }
func (mockToolCtx) ResumedInput(string) (any, bool) {
	return nil, false
}
func (mockToolCtx) WithICDelta(*agent.InvocationContextDelta) agent.InvocationContext {
	return mockToolCtx{}
}

// agent.Context-only surface (v2 additions).
func (mockToolCtx) Path() string                            { return "" }
func (mockToolCtx) RunID() string                           { return "" }
func (mockToolCtx) SubScheduler() agent.DynamicSubScheduler { return nil }
func (mockToolCtx) WithAgentContext(ctx context.Context) agent.Context {
	return mockToolCtx{Context: ctx}
}
func (mockToolCtx) WithAgentTimeout(time.Duration) (agent.Context, context.CancelFunc) {
	return mockToolCtx{}, func() {}
}
func (mockToolCtx) WithAgentCancel() (agent.Context, context.CancelFunc) {
	return mockToolCtx{}, func() {}
}
func (mockToolCtx) OutputForAncestors() []string { return nil }
func (mockToolCtx) WithDelta(*agent.CommonContextDelta) agent.Context {
	return mockToolCtx{}
}

var _ agent.Context = mockToolCtx{}

// runnableTool mirrors the (unexported) adk interface implemented by tools built via
// functiontool.New — see internal/tools/tool_invoke_test.go for the confirmed shape this copies.
type runnableTool interface {
	Run(ctx agent.Context, args any) (map[string]any, error)
}

// runTool invokes tl through the tool.Tool Run interface, marshaling args (a typed struct, using
// its json tags) to the map[string]any the ADK Run signature expects, and unmarshaling the
// returned map back into TResult. Unlike internal/tools' non-generic runTool helper (which Fatals
// on any Run error, since none of its callers need the error itself), this one RETURNS the error
// so streaming/progress-tool tests can assert on start-failure paths (e.g.
// TestProposeSpecChangeTool_StartError) as well as the success paths.
func runTool[TArgs, TResult any](t *testing.T, tl any, args TArgs) (TResult, error) {
	t.Helper()
	var result TResult

	argsJSON, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	var argsMap map[string]any
	if err := json.Unmarshal(argsJSON, &argsMap); err != nil {
		t.Fatalf("unmarshal args to map: %v", err)
	}

	r, ok := tl.(runnableTool)
	if !ok {
		t.Fatalf("tool %T does not implement Run", tl)
	}
	out, err := r.Run(mockToolCtx{Context: context.Background()}, argsMap)
	if err != nil {
		return result, err
	}

	outJSON, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal result map: %v", err)
	}
	if err := json.Unmarshal(outJSON, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return result, nil
}
