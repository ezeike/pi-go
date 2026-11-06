package a2atool

import (
	"context"
	"testing"
	"time"

	"google.golang.org/adk/v2/agent"
	adkmemory "google.golang.org/adk/v2/memory"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/toolconfirmation"
	"google.golang.org/genai"
)

// It lets tests invoke tools through the tool.Tool Run interface, exercising the
// functiontool factory closures that direct handler tests never reach. The full
// v2 method set is implemented so the mock stays forward-compatible with future
// agent.Context surface growth.
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

// runnableTool mirrors the (unexported) adk interface implemented by tools.
type runnableTool interface {
	Run(ctx agent.Context, args any) (map[string]any, error)
}

// runNamedTool invokes one of a tool set by name through the same Run path the
// ADK runner uses, so argument decoding is covered.
func runNamedTool(t *testing.T, ts []tool.Tool, name string, args map[string]any) (map[string]any, error) {
	t.Helper()
	for _, candidate := range ts {
		if candidate.Name() != name {
			continue
		}
		r, ok := candidate.(runnableTool)
		if !ok {
			t.Fatalf("tool %q is not runnable", name)
		}
		return r.Run(mockToolCtx{Context: context.Background()}, args)
	}
	t.Fatalf("tool %q not found", name)
	return nil, nil
}
