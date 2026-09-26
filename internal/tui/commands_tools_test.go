package tui

import (
	"errors"
	"strings"
	"testing"

	adkagent "google.golang.org/adk/v2/agent"
	adktool "google.golang.org/adk/v2/tool"
)

// fakeToolsTool is a minimal adktool.Tool test double.
type fakeToolsTool struct {
	name string
	desc string
	long bool
}

func (f fakeToolsTool) Name() string        { return f.name }
func (f fakeToolsTool) Description() string { return f.desc }
func (f fakeToolsTool) IsLongRunning() bool { return f.long }

var _ adktool.Tool = fakeToolsTool{}

// fakeToolset is a minimal adktool.Toolset test double.
type fakeToolset struct {
	name  string
	tools []adktool.Tool
	err   error
}

func (f fakeToolset) Name() string { return f.name }
func (f fakeToolset) Tools(_ adkagent.ReadonlyContext) ([]adktool.Tool, error) {
	return f.tools, f.err
}

var _ adktool.Toolset = fakeToolset{}

func TestHandleToolsCommand_Empty(t *testing.T) {
	m := &model{
		chatModel: ChatModel{Messages: make([]message, 0)},
		cfg:       Config{},
	}
	m.handleToolsCommand()
	if len(m.chatModel.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(m.chatModel.Messages))
	}
	if !strings.Contains(m.chatModel.Messages[0].content, "No tools registered") {
		t.Errorf("expected 'No tools registered', got %q", m.chatModel.Messages[0].content)
	}
}

func TestHandleToolsCommand_CoreTools(t *testing.T) {
	m := &model{
		chatModel: ChatModel{Messages: make([]message, 0)},
		cfg: Config{
			Tools: []adktool.Tool{
				fakeToolsTool{name: "read", desc: "Read a file"},
				fakeToolsTool{name: "watch_run", desc: "Watch a run", long: true},
			},
		},
	}
	m.handleToolsCommand()
	content := m.chatModel.Messages[0].content
	if !strings.Contains(content, "Core tools") {
		t.Errorf("expected 'Core tools' heading, got %q", content)
	}
	if !strings.Contains(content, "`read`") || !strings.Contains(content, "Read a file") {
		t.Errorf("expected read tool listed, got %q", content)
	}
	if !strings.Contains(content, "`watch_run` (long-running)") {
		t.Errorf("expected watch_run flagged long-running, got %q", content)
	}
}

func TestHandleToolsCommand_NonMCPToolsets(t *testing.T) {
	m := &model{
		chatModel: ChatModel{Messages: make([]message, 0)},
		cfg: Config{
			NonMCPToolsets: []adktool.Toolset{
				fakeToolset{name: "block-builder", tools: []adktool.Tool{
					fakeToolsTool{name: "send_message", desc: "Send a chat message"},
				}},
				fakeToolset{name: "a2a", err: errors.New("boom")},
			},
		},
	}
	m.handleToolsCommand()
	content := m.chatModel.Messages[0].content
	if !strings.Contains(content, "block-builder") {
		t.Errorf("expected block-builder section, got %q", content)
	}
	if !strings.Contains(content, "`send_message`") {
		t.Errorf("expected send_message tool listed, got %q", content)
	}
	// The erroring toolset must be skipped, not surfaced as an error message
	// or a panic — matching the poll tools' "stream errors surface in-band"
	// convention elsewhere in this codebase would be the wrong model here;
	// a listing command has no in-band error channel, so it just omits it.
	if strings.Contains(content, "boom") {
		t.Errorf("expected erroring toolset's error not to leak into output, got %q", content)
	}
}

func TestHandleToolsCommand_MCPTools(t *testing.T) {
	// extension.LoadedToolsetTools only recognizes its own concrete
	// *resilientToolset type, so a fake toolset here contributes nothing —
	// this pins that MCP-empty behavior rather than exercising the MCP path
	// itself (covered indirectly wherever extension.LoadedToolsetTools is
	// tested against a real resilientToolset).
	m := &model{
		chatModel: ChatModel{Messages: make([]message, 0)},
		cfg: Config{
			MCPToolsets: []adktool.Toolset{
				fakeToolset{name: "unrelated", tools: []adktool.Tool{fakeToolsTool{name: "x"}}},
			},
		},
	}
	m.handleToolsCommand()
	content := m.chatModel.Messages[0].content
	if strings.Contains(content, "MCP tools") {
		t.Errorf("expected no MCP tools section when nothing is a *resilientToolset, got %q", content)
	}
}
