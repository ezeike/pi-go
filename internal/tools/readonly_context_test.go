package tools

import (
	"context"

	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

// mockReadonlyContext is a minimal agent.ReadonlyContext for toolset tests.
type mockReadonlyContext struct {
	context.Context
}

func (m *mockReadonlyContext) UserContent() *genai.Content          { return nil }
func (m *mockReadonlyContext) InvocationID() string                 { return "" }
func (m *mockReadonlyContext) AgentName() string                    { return "test" }
func (m *mockReadonlyContext) ReadonlyState() session.ReadonlyState { return nil }
func (m *mockReadonlyContext) UserID() string                       { return "" }
func (m *mockReadonlyContext) AppName() string                      { return "" }
func (m *mockReadonlyContext) SessionID() string                    { return "" }
func (m *mockReadonlyContext) Branch() string                       { return "" }
