package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func wideSelectionModel(t *testing.T) *model {
	t.Helper()
	m := historyModel(t, "first")
	m.width, m.height = 200, 40
	m.applyResize()
	m.chatModel.Messages = append(m.chatModel.Messages,
		message{role: "user", content: "SELECT-ME-ONE"},
		message{role: "assistant", content: "SELECT-ME-TWO"})
	return m
}

func click(t *testing.T, m *model, x, frameRow int) *model {
	t.Helper()
	next, _ := m.Update(tea.MouseClickMsg(tea.Mouse{X: x, Y: frameRow + m.frameTop(), Button: tea.MouseLeft}))
	return next.(*model)
}

func TestClampToChat_SubtractsPanelOffset(t *testing.T) {
	m := wideSelectionModel(t)
	m.View() // sets the msg viewport the Y clamp reads
	off := m.chatXOffset()
	if off != SubagentPanelWidth {
		t.Fatalf("chatXOffset = %d, want %d", off, SubagentPanelWidth)
	}
	for _, c := range []struct{ x, want int }{
		{-3, 0},
		{0, 0},                           // inside the panel: pins to chat column 0
		{off - 1, 0},                     // last panel column
		{off, 0},                         // first chat column
		{off + 6, 6},                     // maps to the chat column under the pointer
		{m.width - 1, m.chatWidth() - 1}, // far right (sidebar): pins to the chat's last column
	} {
		if got, _ := m.clampToChat(c.x, 5); got != c.want {
			t.Errorf("clampToChat(%d) X = %d, want %d", c.x, got, c.want)
		}
	}
}

func TestClickLeftOfChatClearsSelectionAndStartsNone(t *testing.T) {
	m := wideSelectionModel(t)
	m.View()
	row, col := findRow(t, m, "SELECT-ME-ONE")
	m = drag(t, m, col, row, col+4, row)
	if !m.sel.present {
		t.Fatal("setup: the drag made no selection")
	}

	for _, x := range []int{0, 10, SubagentPanelWidth - 1} {
		m = drag(t, m, col, row, col+4, row) // re-establish a selection
		m = click(t, m, x, row)
		if m.sel.present || m.sel.dragging {
			t.Errorf("click at X=%d (inside the panel) left sel=%+v, want it cleared with no drag started", x, m.sel)
		}
	}
}

func TestClickInsideChatMapsToChatColumn(t *testing.T) {
	m := wideSelectionModel(t)
	m.View()
	row, col := findRow(t, m, "SELECT-ME-ONE")
	if col < m.chatXOffset() {
		t.Fatalf("message at frame column %d is left of the chat offset %d", col, m.chatXOffset())
	}

	m = click(t, m, col, row)
	if !m.sel.dragging || m.sel.anchorX != col-m.chatXOffset() {
		t.Fatalf("click at frame column %d gave sel=%+v, want a drag anchored at chat column %d",
			col, m.sel, col-m.chatXOffset())
	}

	m = click(t, m, m.chatXOffset(), row) // the chat's first column is selectable
	if !m.sel.dragging || m.sel.anchorX != 0 {
		t.Errorf("click at the chat's first column gave sel=%+v, want anchorX 0", m.sel)
	}
}

func TestClickRightOfChatClearsSelection(t *testing.T) {
	m := wideSelectionModel(t)
	m.View()
	row, col := findRow(t, m, "SELECT-ME-ONE")
	m = drag(t, m, col, row, col+4, row)

	m = click(t, m, m.chatXOffset()+m.chatWidth(), row) // the rail column
	if m.sel.present || m.sel.dragging {
		t.Errorf("click on the rail left sel=%+v, want it cleared", m.sel)
	}
}

func TestDragStrayingIntoPanelPinsToChatColumnZero(t *testing.T) {
	m := wideSelectionModel(t)
	m.View()
	row, col := findRow(t, m, "SELECT-ME-ONE")
	m = drag(t, m, col, row, 5, row) // release in the panel

	if m.sel.cursorX != 0 {
		t.Errorf("cursorX = %d after dragging into the panel, want 0", m.sel.cursorX)
	}
}
