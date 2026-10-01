// internal/tui/subagent_panel_tail_test.go
package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// acpShapedEvents is the stream an ACP-backed subagent produces: lifecycle
// events, a tool call, a multi-line message, stderr chatter.
func acpShapedEvents() []agentEv {
	return []agentEv{
		{kind: "spawn"},
		{kind: "message_start"},
		{kind: "tool_call", content: "read alpha"},
		{kind: "text", content: "line1\n\nline2\r\nline3\n"},
		{kind: "stderr", content: "WS frame dump"},
		{kind: "tool_call", content: "edit beta"},
		{kind: "message_end"},
		{kind: "done"},
	}
}

func TestSubagentTailLines(t *testing.T) {
	all := subagentTailLines(acpShapedEvents(), 100)
	want := []string{"tool_call: read alpha", "text: line1", "  line2", "  line3", "tool_call: edit beta"}
	if !reflect.DeepEqual(all, want) {
		t.Fatalf("tail = %q\nwant   %q", all, want)
	}

	if got := subagentTailLines(acpShapedEvents(), 2); !reflect.DeepEqual(got, want[3:]) {
		t.Fatalf("newest 2 lines = %q, want %q", got, want[3:])
	}
	if got := subagentTailLines(acpShapedEvents(), 0); len(got) != 0 {
		t.Fatalf("n=0: got %q, want none", got)
	}
}

func TestSubagentTailLines_Rendering(t *testing.T) {
	cases := []struct {
		name string
		ev   agentEv
		want []string
	}{
		{"text_delta is shown as text", agentEv{kind: "text_delta", content: "hi"}, []string{"text: hi"}},
		{"ansi stripped", agentEv{kind: "text", content: "\x1b[31mred\x1b[0m"}, []string{"text: red"}},
		{"empty tool call keeps its kind", agentEv{kind: "tool_call"}, []string{"tool_call:"}},
		{"blank text event dropped", agentEv{kind: "text", content: "  \n "}, nil},
		{"lifecycle and stderr dropped", agentEv{kind: "stderr", content: "boom"}, nil},
	}
	for _, c := range cases {
		got := subagentTailLines([]agentEv{c.ev}, 10)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestRenderSubagentPanel_ShowsNewestTailLines(t *testing.T) {
	card := panelCard("subagent", "pi", "worker", acpShapedEvents()...)

	plain := panelPlain(t, panelInput(40, 6, card)) // header + 5 tail rows: all five lines fit
	for row, want := range []string{
		"agent[pi] worker", "tool_call: read alpha", "text: line1", "line2", "line3", "tool_call: edit beta",
	} {
		if got := strings.TrimSpace(plain[row]); got != want {
			t.Errorf("row %d = %q, want %q", row, got, want)
		}
	}

	plain = panelPlain(t, panelInput(40, 3, card)) // header + 2 rows: the newest two lines only
	for row, want := range []string{"agent[pi] worker", "line3", "tool_call: edit beta"} {
		if got := strings.TrimSpace(plain[row]); got != want {
			t.Errorf("short panel row %d = %q, want %q", row, got, want)
		}
	}
}

func TestRenderSubagentPanel_HidesLifecycleAndStderr(t *testing.T) {
	card := panelCard("subagent", "pi", "worker",
		agentEv{kind: "spawn"}, agentEv{kind: "message_start"},
		agentEv{kind: "stderr", content: "WS frame dump"},
		agentEv{kind: "message_end"}, agentEv{kind: "done"},
		agentEv{kind: "text", content: "   "})
	plain := panelPlain(t, panelInput(40, 6, card))
	all := strings.Join(plain, "\n")
	for _, hidden := range []string{"spawn", "message_start", "message_end", "stderr", "WS frame dump", "done", "text:"} {
		if strings.Contains(all, hidden) {
			t.Errorf("%q leaked into the panel:\n%s", hidden, all)
		}
	}
}

func TestRenderSubagentPanel_ClipsLongLinesToInnerWidth(t *testing.T) {
	card := panelCard("agent", "pi", "w", agentEv{kind: "text", content: strings.Repeat("x", 200)})
	plain := panelPlain(t, panelInput(20, 3, card))
	// 20 cells = 1 indent + 18 inner + 1 gutter.
	if got := strings.TrimSpace(plain[1]); got != "text: "+strings.Repeat("x", 12) {
		t.Fatalf("row 1 = %q, want the line clipped to 18 cells", got)
	}
	if !strings.HasSuffix(plain[1], " ") {
		t.Errorf("the right gutter column is not blank: %q", plain[1])
	}
}

// Subagent output is whatever the subagent printed: tabs, carriage returns,
// backspaces, escape sequences, wide runes. None may change a row's width.
func TestRenderSubagentPanel_HostileEventContentKeepsRowWidth(t *testing.T) {
	card := panelCard("subagent", "pi", "hostile",
		agentEv{kind: "tool_call", content: "go test\t./...\r\x08\x08"},
		agentEv{kind: "text", content: "\x1b[31mred\x1b[0m\ttabbed\tcolumns\nPulling\r  50%\rPulling  100%"},
		agentEv{kind: "text", content: "漢字漢字漢字漢字漢字漢字漢字漢字漢字漢字"})
	for _, w := range []int{1, 2, 3, 10, 21, 44, 60} {
		out := RenderSubagentPanel(panelInput(w, 8, card))
		for i, r := range strings.Split(out, "\n") {
			if got := ansi.StringWidth(r); got != w {
				t.Fatalf("width %d row %d: %d cells wide: %q", w, i, got, ansi.Strip(r))
			}
		}
		if strings.ContainsAny(ansi.Strip(out), "\t\r\b\x1b") {
			t.Fatalf("width %d: a control character survived into the panel", w)
		}
	}
}

// Rendering reads agentEvents and never writes them: two adjacent text events
// stay two events (handleAgentSubEvent does the merging, not the renderer).
func TestRenderSubagentPanel_DoesNotMutateMessages(t *testing.T) {
	build := func() []message {
		return []message{
			panelCard("agent", "pi", "one",
				agentEv{kind: "text", content: "Reading "}, agentEv{kind: "text", content: "the plan"}),
			panelCard("subagent", "claude", "two", agentEv{kind: "stderr", content: "noise"}),
		}
	}
	msgs := build()
	_ = RenderSubagentPanel(panelInput(40, 12, msgs...))
	if !reflect.DeepEqual(msgs, build()) {
		t.Fatal("RenderSubagentPanel mutated the messages it was given")
	}
}
