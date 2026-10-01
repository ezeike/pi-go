package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// View must draw the selection at the chat's frame column, not at its
// chat-relative column, and leave the panel columns untouched.
func TestWidescreenView_HighlightIsOffsetByThePanel(t *testing.T) {
	m := wideSelectionModel(t)
	baseline := strings.Split(m.View().Content, "\n")

	row, col := findRow(t, m, "SELECT-ME-ONE")
	off := m.chatXOffset()
	if col < off {
		t.Fatalf("message at frame column %d is left of the chat offset %d", col, off)
	}
	lo := col - off
	const span = len("SELECT-ME-ONE")
	m.sel = selection{present: true, anchorX: lo, anchorY: row, cursorX: lo + span - 1, cursorY: row}

	got := strings.Split(m.View().Content, "\n")
	if len(got) != len(baseline) {
		t.Fatalf("selection changed the frame height: %d rows, want %d", len(got), len(baseline))
	}

	want := selectionStyle.Render("SELECT-ME-ONE")
	idx := strings.Index(got[row], want)
	if idx < 0 {
		t.Fatalf("row %d has no reverse-video span %q:\n%q", row, want, got[row])
	}
	if start := ansi.StringWidth(got[row][:idx]); start != off+lo {
		t.Errorf("selection starts at frame column %d, want chatXOffset+lo = %d", start, off+lo)
	}

	for i := range got {
		if w := ansi.StringWidth(got[i]); w != m.width {
			t.Errorf("row %d is %d cells wide with a selection, want %d", i, w, m.width)
		}
		if a, b := panelColumns(got[i]), panelColumns(baseline[i]); a != b {
			t.Errorf("row %d panel columns changed under a selection:\n got %q\nwant %q", i, a, b)
		}
	}
}
