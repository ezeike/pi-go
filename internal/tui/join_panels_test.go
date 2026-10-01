package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// legacyJoinPanelSidebar is the body of the function joinPanels generalizes,
// copied verbatim so the two-column equivalence stays checkable after the
// original is deleted.
func legacyJoinPanelSidebar(left, sidebar string, leftWidth, sidebarWidth int) string {
	leftRows := strings.Split(left, "\n")
	sideRows := strings.Split(sidebar, "\n")
	rows := max(len(leftRows), len(sideRows))

	out := make([]string, rows)
	for i := range out {
		var l, s string
		if i < len(leftRows) {
			l = leftRows[i]
		}
		if i < len(sideRows) {
			s = sideRows[i]
		}
		out[i] = padLine(l, leftWidth) + padLine(s, sidebarWidth)
	}
	return strings.Join(out, "\n")
}

func TestJoinPanels_TwoColumnsMatchLegacy(t *testing.T) {
	cases := []struct {
		name   string
		left   string
		side   string
		lw, sw int
	}{
		{"equal heights", "aaa\nbbb\nccc", "111\n222\n333", 5, 3},
		{"sidebar taller", "aaa", "1\n2\n3", 5, 2},
		{"panel taller", "a\nb\nc", "1", 4, 3},
		{"stray wide row", "ok\n" + strings.Repeat("W", 30) + "\nok", "SIDE\nSIDE\nSIDE", 6, 4},
		{"control characters", "a\tb\nc\rd\ne\x08f", "xx\nxx\nxx", 10, 2},
		{"styled and wide runes", "\x1b[31mred\x1b[0m\n漢字漢字漢字\nplain", "ss\nss\nss", 5, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := legacyJoinPanelSidebar(c.left, c.side, c.lw, c.sw)
			got := joinPanels(panelColumn{c.left, c.lw}, panelColumn{c.side, c.sw})
			if got != want {
				t.Fatalf("joinPanels differs from joinPanelSidebar\n got: %q\nwant: %q", got, want)
			}
		})
	}
}

// A single row that renders wider than its column — one tab is enough — must not
// move any other column: every column starts at the same cell on every row.
func TestJoinPanels_ThreeColumnsStrayRowKeepsColumnStarts(t *testing.T) {
	left := panelColumn{"AAAAA\nAAAAA\nAAAAA", 5}
	mid := panelColumn{"BBBBBBBBBB\nB\tBBBBBBBBBBBBBBBBBBBB\nBBBB", 10}
	side := panelColumn{"SSSS\nSSSS\nSSSS", 4}

	rows := strings.Split(joinPanels(left, mid, side), "\n")
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	for i, row := range rows {
		if got := ansi.StringWidth(row); got != 19 {
			t.Fatalf("row %d is %d cells wide, want 19: %q", i, got, row)
		}
		if !strings.HasPrefix(row, "AAAAA") || !strings.HasSuffix(row, "SSSS") {
			t.Fatalf("row %d moved a column start: %q", i, row)
		}
		if strings.ContainsAny(row, "\t\r\b") {
			t.Fatalf("row %d carries a control character: %q", i, row)
		}
	}
}

func TestJoinPanels_SkipsZeroWidthAndHandlesNone(t *testing.T) {
	if got := joinPanels(); got != "" {
		t.Fatalf("no columns: got %q, want empty", got)
	}
	got := joinPanels(panelColumn{"hidden", 0}, panelColumn{"ab\ncd", 4})
	if want := "ab  \ncd  "; got != want {
		t.Fatalf("zero-width column was not skipped: got %q, want %q", got, want)
	}
}

func TestJoinPanels_SingleColumnMatchesPadLinesTo(t *testing.T) {
	in := "short\n" + strings.Repeat("x", 40) + "\nz"
	if got, want := joinPanels(panelColumn{in, 12}), padLinesTo(in, 12); got != want {
		t.Fatalf("single column differs from padLinesTo\n got: %q\nwant: %q", got, want)
	}
}
