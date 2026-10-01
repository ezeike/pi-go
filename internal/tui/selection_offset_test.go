package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// A frame with an 8-column panel, an 11-column chat, a 1-column rail and a
// 4-column sidebar: 24 columns.
const offsetFrame = "PANEL---hello world|SIDE\nPANEL---second line|SIDE"

func TestSelectedText_OffsetSkipsPanel(t *testing.T) {
	one := selection{present: true, anchorX: 0, anchorY: 0, cursorX: 4, cursorY: 0}
	if got := selectedText(offsetFrame, one, 11, 8); got != "hello" {
		t.Errorf("single row = %q, want %q (chat columns 0-4, not the panel's)", got, "hello")
	}

	two := selection{present: true, anchorX: 6, anchorY: 0, cursorX: 5, cursorY: 1}
	if got := selectedText(offsetFrame, two, 11, 8); got != "world\nsecond" {
		t.Errorf("two rows = %q, want %q", got, "world\nsecond")
	}

	whole := selection{present: true, anchorX: 0, anchorY: 0, cursorX: 10, cursorY: 1}
	got := selectedText(offsetFrame, whole, 11, 8)
	for _, leak := range []string{"PANEL", "SIDE", "|"} {
		if strings.Contains(got, leak) {
			t.Errorf("whole-row selection %q leaked %q from outside the chat", got, leak)
		}
	}
}

func TestSelectedText_ZeroOffsetIsTheOldBehaviour(t *testing.T) {
	sel := selection{present: true, anchorX: 0, anchorY: 0, cursorX: 4, cursorY: 0}
	if got := selectedText("hello world|SIDE", sel, 11, 0); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestHighlight_OffsetKeepsPanelAndSidebarIntact(t *testing.T) {
	sel := selection{present: true, anchorX: 0, anchorY: 0, cursorX: 4, cursorY: 0}
	out := highlight(offsetFrame, sel, 11, 24, 8)

	lines := strings.Split(out, "\n")
	if got := ansi.Strip(lines[0]); got != "PANEL---hello world|SIDE" {
		t.Errorf("row 0 text changed: %q", got)
	}
	if !strings.HasPrefix(lines[0], "PANEL---") {
		t.Errorf("the panel was restyled or shifted: %q", lines[0])
	}
	if !strings.Contains(lines[0], selectionStyle.Render("hello")) {
		t.Errorf("the chat's first five cells are not reversed: %q", lines[0])
	}
	if !strings.HasSuffix(ansi.Strip(lines[0]), "|SIDE") {
		t.Errorf("rail and sidebar were lost: %q", ansi.Strip(lines[0]))
	}
	if lines[1] != "PANEL---second line|SIDE" {
		t.Errorf("an unselected row was modified: %q", lines[1])
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got != 24 {
			t.Errorf("row %d is %d cells wide, want 24", i, got)
		}
	}
}

// With the panel actually rendering text on the selected row, a drag in the
// chat copies exactly the chat text under it.
func TestDragCopiesOnlyChatTextWithPanelShown(t *testing.T) {
	m := historyModel(t, "first")
	m.width, m.height = 200, 40
	m.applyResize()
	m.running = true // the panel shows only a running turn's subagents
	m.chatModel.Messages = append(m.chatModel.Messages,
		message{role: "user", content: "SELECT-ME-ONE"},
		panelCard("subagent", "pi", "worker",
			agentEv{kind: "tool_call", content: "read one"},
			agentEv{kind: "tool_call", content: "read two"},
			agentEv{kind: "tool_call", content: "read three"},
			agentEv{kind: "tool_call", content: "read four"}))
	m.View()

	row, col := findRow(t, m, "SELECT-ME-ONE")
	frameRows := strings.Split(m.lastFrame, "\n")
	if strings.TrimSpace(panelColumns(frameRows[row])) == "" {
		t.Fatalf("setup: the panel is blank on row %d, so this test could not catch a leak", row)
	}

	m = drag(t, m, col, row, col+len("SELECT-ME-ONE")-1, row)
	if got := selectedText(m.lastFrame, m.sel, m.chatWidth(), m.chatXOffset()); got != "SELECT-ME-ONE" {
		t.Errorf("copied %q, want %q — nothing from the panel, rail or sidebar", got, "SELECT-ME-ONE")
	}
}

func TestHighlightPreservesFrameWidthAndPanelWithPanelShown(t *testing.T) {
	m := wideSelectionModel(t)
	frame := m.View().Content
	row, col := findRow(t, m, "SELECT-ME-ONE")

	sel := selection{present: true, anchorX: col - m.chatXOffset(), anchorY: row, cursorX: col - m.chatXOffset() + 8, cursorY: row + 2}
	out := highlight(frame, sel, m.chatWidth(), m.width, m.chatXOffset())

	orig := strings.Split(frame, "\n")
	for i, line := range strings.Split(out, "\n") {
		if got := ansi.StringWidth(line); got != m.width {
			t.Fatalf("row %d is %d wide after highlighting, want %d", i, got, m.width)
		}
		if panelColumns(line) != panelColumns(orig[i]) {
			t.Fatalf("row %d: the panel columns changed under the highlight", i)
		}
		sidebar := func(s string) string { return ansi.Strip(ansi.Cut(s, m.width-SidebarWidth, m.width)) }
		if sidebar(line) != sidebar(orig[i]) {
			t.Fatalf("row %d: the sidebar columns changed under the highlight", i)
		}
	}
	if !strings.Contains(out, "\x1b[7m") {
		t.Error("nothing was drawn in reverse video; the selection is invisible")
	}
}
