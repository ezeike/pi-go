package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// panelColumns returns the text of a frame row's subagent-panel columns.
func panelColumns(frameRow string) string {
	return strings.TrimRight(ansi.Strip(ansi.Cut(frameRow, 0, SubagentPanelWidth)), " ")
}

func TestMainAndChatWidthDelegateToPanelWidths(t *testing.T) {
	for _, c := range []struct{ width, main, chat int }{
		{0, 1, 1}, {1, 1, 1}, {60, 60, 59}, {80, 80, 79}, {81, 58, 57}, {120, 97, 96},
		{147, 124, 123}, {148, 81, 80}, {200, 133, 132},
	} {
		m := &model{width: c.width}
		if got := m.mainWidth(); got != c.main {
			t.Errorf("mainWidth at %d = %d, want %d", c.width, got, c.main)
		}
		if got := m.chatWidth(); got != c.chat {
			t.Errorf("chatWidth at %d = %d, want %d", c.width, got, c.chat)
		}
	}
}

// T5: above the threshold every row is the terminal width, the rail sits at the
// end of the middle column, and the sidebar owns the last SidebarWidth columns.
func TestWidescreenFrame_EveryRowFullWidthAndRailAligned(t *testing.T) {
	for _, width := range []int{148, 172, 200} {
		for _, matrix := range []bool{false, true} {
			m := historyModel(t, "first")
			m.width, m.height = width, 40
			m.matrix.active = matrix
			m.applyResize()
			m.chatModel.Messages = append(m.chatModel.Messages, realisticChat()...)

			if got := m.chatXOffset(); got != SubagentPanelWidth {
				t.Fatalf("width %d: chatXOffset = %d, want %d", width, got, SubagentPanelWidth)
			}
			maxScroll := m.chatModel.MaxScroll(m.messageViewportHeight())
			for _, scroll := range []int{0, maxScroll / 2, maxScroll} {
				m.chatModel.Scroll = scroll
				label := fmt.Sprintf("width=%d matrix=%v scroll=%d", width, matrix, scroll)
				rows := strings.Split(m.View().Content, "\n")

				railCol := m.chatXOffset() + m.mainWidth() - railWidth
				if railCol+1 != width-SidebarWidth {
					t.Fatalf("%s: rail at %d leaves the sidebar %d columns, want %d",
						label, railCol, width-railCol-1, SidebarWidth)
				}
				topRows := m.topSectionRows()
				for row, line := range rows {
					if got := ansi.StringWidth(line); got != width {
						t.Fatalf("%s: row %d is %d cells wide, want %d\n%q", label, row, got, width, ansi.Strip(line))
					}
					if row >= topRows {
						continue
					}
					plain := ansi.Strip(line)
					if g := runeAtCol(plain, railCol); !isRailCell(g) {
						t.Fatalf("%s: row %d has %q at the rail column %d\n%q", label, row, g, railCol, plain)
					}
					if g := runeAtCol(plain, SubagentPanelWidth-1); g != " " {
						t.Fatalf("%s: row %d has %q in the panel's gutter column, want a blank\n%q", label, row, g, plain)
					}
				}
			}
		}
	}
}

func TestWidescreenFrame_PlaceholderShownAtWidthAndAbsentBelow(t *testing.T) {
	wide := historyModel(t, "first")
	wide.width, wide.height = 200, 40
	wide.applyResize()
	rows := strings.Split(wide.View().Content, "\n")
	if got := panelColumns(rows[0]); !strings.Contains(got, "No active subagents") {
		t.Fatalf("row 0 panel columns = %q, want the placeholder", got)
	}

	narrow := historyModel(t, "first")
	narrow.width, narrow.height = SubagentPanelMinTotalWidth, 40
	narrow.applyResize()
	if strings.Contains(ansi.Strip(narrow.View().Content), "No active subagents") {
		t.Fatalf("panel rendered at width %d, want it only above %d", narrow.width, SubagentPanelMinTotalWidth)
	}
}

func TestWidescreenFrame_PendingCardAppearsInPanelColumns(t *testing.T) {
	m := historyModel(t, "first")
	m.width, m.height = 200, 40
	m.applyResize()
	m.chatModel.Messages = append(m.chatModel.Messages,
		panelCard("subagent", "pi", "Analyze internal/subagent",
			agentEv{kind: "tool_call", content: "read orchestrator"}))

	rows := strings.Split(m.View().Content, "\n")
	if got := panelColumns(rows[0]); !strings.Contains(got, "agent[pi] Analyze internal/subagent") {
		t.Errorf("row 0 panel columns = %q, want the card header", got)
	}
	if got := panelColumns(rows[1]); !strings.Contains(got, "tool_call: read orchestrator") {
		t.Errorf("row 1 panel columns = %q, want the event tail", got)
	}
}
