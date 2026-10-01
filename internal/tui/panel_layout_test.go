package tui

import "testing"

func TestPanelWidths(t *testing.T) {
	cases := []struct {
		width               int
		left, main, sidebar int
	}{
		{80, 0, 80, 0},
		{81, 0, 58, 23},
		{120, 0, 97, 23},
		{SubagentPanelMinTotalWidth, 0, 124, 23},     // 147: still two columns
		{SubagentPanelMinTotalWidth + 1, 44, 81, 23}, // 148: panel appears, chat is 80 wide
		{200, 44, 133, 23},
	}
	for _, c := range cases {
		l, m, s := panelWidths(c.width)
		if l != c.left || m != c.main || s != c.sidebar {
			t.Errorf("panelWidths(%d) = (%d,%d,%d), want (%d,%d,%d)",
				c.width, l, m, s, c.left, c.main, c.sidebar)
		}
	}
}

func TestPanelWidths_ColumnsSumToWidth(t *testing.T) {
	for w := 81; w <= 400; w++ {
		l, m, s := panelWidths(w)
		if l+m+s != w {
			t.Fatalf("width %d: %d+%d+%d = %d, columns must tile the terminal", w, l, m, s, l+m+s)
		}
		if l > 0 && m-railWidth < 80 {
			t.Fatalf("width %d: chat is %d wide with the panel shown, want at least 80", w, m-railWidth)
		}
	}
}

func TestChatXOffset(t *testing.T) {
	for _, c := range []struct{ width, want int }{
		{0, 0}, {60, 0}, {120, 0}, {SubagentPanelMinTotalWidth, 0},
		{SubagentPanelMinTotalWidth + 1, SubagentPanelWidth}, {200, SubagentPanelWidth},
	} {
		m := &model{width: c.width}
		if got := m.chatXOffset(); got != c.want {
			t.Errorf("chatXOffset at width %d = %d, want %d", c.width, got, c.want)
		}
	}
}
