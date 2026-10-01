package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// clipboardTexts runs cmd (flattening a tea.Batch) and returns every clipboard
// write it produced. Sub-commands that do not answer promptly, such as the
// flash timer, are abandoned rather than waited out.
func clipboardTexts(cmd tea.Cmd) []string {
	if cmd == nil {
		return nil
	}
	var cmds []tea.Cmd
	if batch, ok := cmd().(tea.BatchMsg); ok {
		cmds = batch
	} else {
		cmds = []tea.Cmd{cmd}
	}
	results := make(chan tea.Msg, len(cmds))
	for _, c := range cmds {
		if c == nil {
			continue
		}
		go func() { results <- c() }()
	}
	var out []string
	deadline := time.After(250 * time.Millisecond)
	for range cmds {
		select {
		case msg := <-results:
			if fmt.Sprintf("%T", msg) == "tea.setClipboardMsg" {
				out = append(out, fmt.Sprint(msg))
			}
		case <-deadline:
			return out
		}
	}
	return out
}

// copyOffsetModel shows a panel card on the same rows as the chat text, so a
// copy that forgets chatXOffset() would pick up panel text.
func copyOffsetModel(t *testing.T) *model {
	t.Helper()
	m := historyModel(t, "first")
	m.width, m.height = 200, 40
	m.applyResize()
	m.running = true
	m.chatModel.Messages = append(m.chatModel.Messages,
		message{role: "user", content: "SELECT-ME-ONE"},
		panelCard("subagent", "pi", "worker",
			agentEv{kind: "tool_call", content: "read one"},
			agentEv{kind: "tool_call", content: "read two"},
			agentEv{kind: "tool_call", content: "read three"},
			agentEv{kind: "tool_call", content: "read four"}))
	m.View()
	return m
}

func TestCopyOnRelease_UsesChatXOffset(t *testing.T) {
	m := copyOffsetModel(t)
	row, col := findRow(t, m, "SELECT-ME-ONE")
	if strings.TrimSpace(panelColumns(strings.Split(m.lastFrame, "\n")[row])) == "" {
		t.Fatalf("setup: the panel is blank on row %d, so a dropped offset would go unnoticed", row)
	}
	end := col + len("SELECT-ME-ONE") - 1
	top := m.frameTop()
	m = updateWith(t, m, tea.MouseClickMsg(tea.Mouse{X: col, Y: row + top, Button: tea.MouseLeft}))
	m = updateWith(t, m, tea.MouseMotionMsg(tea.Mouse{X: end, Y: row + top, Button: tea.MouseLeft}))
	_, cmd := m.Update(tea.MouseReleaseMsg(tea.Mouse{X: end, Y: row + top, Button: tea.MouseLeft}))

	got := clipboardTexts(cmd)
	if len(got) != 1 || got[0] != "SELECT-ME-ONE" {
		t.Errorf("release copied %q, want exactly [\"SELECT-ME-ONE\"]", got)
	}
}

func TestClickInsideHighlightRecopies_UsesChatXOffset(t *testing.T) {
	m := copyOffsetModel(t)
	row, col := findRow(t, m, "SELECT-ME-ONE")
	m = drag(t, m, col, row, col+len("SELECT-ME-ONE")-1, row)
	if !m.sel.present {
		t.Fatal("setup: the drag made no selection")
	}

	_, cmd := m.Update(tea.MouseClickMsg(tea.Mouse{X: col + 2, Y: row + m.frameTop(), Button: tea.MouseLeft}))
	got := clipboardTexts(cmd)
	if len(got) != 1 || got[0] != "SELECT-ME-ONE" {
		t.Errorf("re-copy copied %q, want exactly [\"SELECT-ME-ONE\"]", got)
	}
}
