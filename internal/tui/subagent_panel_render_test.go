// internal/tui/subagent_panel_render_test.go
package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func panelInput(width, height int, msgs ...message) SubagentPanelRenderInput {
	return SubagentPanelRenderInput{Width: width, Height: height, Palette: darkPalette, Messages: msgs}
}

// panelPlain renders in, enforces the sizing contract (exactly Height rows, each
// exactly Width cells) and returns the rows with ANSI stripped.
func panelPlain(t *testing.T, in SubagentPanelRenderInput) []string {
	t.Helper()
	rows := strings.Split(RenderSubagentPanel(in), "\n")
	if len(rows) != in.Height {
		t.Fatalf("panel has %d rows, want %d", len(rows), in.Height)
	}
	plain := make([]string, len(rows))
	for i, r := range rows {
		if got := ansi.StringWidth(r); got != in.Width {
			t.Fatalf("row %d is %d cells wide, want %d: %q", i, got, in.Width, ansi.Strip(r))
		}
		plain[i] = ansi.Strip(r)
	}
	return plain
}

func TestSubagentPanelPlan(t *testing.T) {
	cases := []struct {
		height, pending int
		wantShown       int
		wantFooter      bool
	}{
		{10, 1, 1, false},
		{10, 4, 4, false},
		{10, 5, 4, true},
		{10, 9, 4, true},
		{5, 5, 4, true}, // 4 blocks + footer = 5 rows, fits exactly
		{4, 5, 3, true}, // 4 blocks + footer = 5 > 4: show 3, N grows to 2
		{2, 3, 1, true}, // one block + footer
		{3, 2, 2, false},
		{1, 1, 1, false},
		{1, 3, 0, true}, // not even one block plus a footer: footer only
	}
	for _, c := range cases {
		shown, footer := subagentPanelPlan(c.height, c.pending)
		if shown != c.wantShown || footer != c.wantFooter {
			t.Errorf("subagentPanelPlan(height=%d, pending=%d) = (%d, %v), want (%d, %v)",
				c.height, c.pending, shown, footer, c.wantShown, c.wantFooter)
		}
	}
}

func TestSubagentBlockHeights(t *testing.T) {
	cases := []struct {
		avail, shown int
		want         []int
	}{
		{10, 4, []int{2, 2, 2, 4}}, // the last block absorbs the remainder
		{9, 3, []int{3, 3, 3}},
		{7, 2, []int{3, 4}},
		{4, 4, []int{1, 1, 1, 1}},
		{5, 1, []int{5}},
	}
	for _, c := range cases {
		if got := subagentBlockHeights(c.avail, c.shown); !reflect.DeepEqual(got, c.want) {
			t.Errorf("subagentBlockHeights(%d, %d) = %v, want %v", c.avail, c.shown, got, c.want)
		}
	}
}

func TestSubagentHeaderText(t *testing.T) {
	a2a := panelCard("a2a", "", "check mesh")
	a2a.agentLabel = "istio-agent"
	cases := []struct {
		name string
		msg  message
		want string
	}{
		{"bundled ACP agent keeps its name", panelCard("agent", "claude", "alpha"), "agent[claude] alpha"},
		{"pi-based type collapses to pi", panelCard("subagent", "explore", "scan"), "agent[pi] scan"},
		{"a2a uses the verbatim label", a2a, "agent[istio-agent] check mesh"},
		{"no title", panelCard("agent", "pi", ""), "agent[pi]"},
		{"no label", panelCard("agent", "", "orphan"), "agent orphan"},
		{"neither", panelCard("agent", "", ""), "agent"},
		{"ansi and tab collapsed", panelCard("agent", "pi", "\x1b[31mred\x1b[0m\ttitle"), "agent[pi] red title"},
	}
	for _, c := range cases {
		if got := subagentHeaderText(c.msg); got != c.want {
			t.Errorf("%s: subagentHeaderText = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestRenderSubagentPanel_PlaceholderKeepsWidth(t *testing.T) {
	for _, msgs := range [][]message{
		nil,
		{{role: "user", content: "hi"}, finishedCard(panelCard("agent", "pi", "old"))},
	} {
		plain := panelPlain(t, panelInput(30, 5, msgs...))
		if got := strings.TrimSpace(plain[0]); got != "No active subagents" {
			t.Fatalf("row 0 = %q, want the placeholder", got)
		}
		if !strings.HasPrefix(plain[0], " No active subagents") {
			t.Errorf("placeholder is not indented one column: %q", plain[0])
		}
		for i := 1; i < len(plain); i++ {
			if strings.TrimSpace(plain[i]) != "" {
				t.Errorf("row %d = %q, want blank padding", i, plain[i])
			}
		}
	}
}

func TestRenderSubagentPanel_StacksNewestFirst(t *testing.T) {
	msgs := []message{
		panelCard("agent", "claude", "alpha"),
		{role: "assistant", content: "thinking"},
		panelCard("subagent", "pi", "beta"),
		panelCard("agent", "pi", "gamma"),
	}
	plain := panelPlain(t, panelInput(40, 12, msgs...)) // 3 blocks of 4 rows
	for row, want := range map[int]string{0: "agent[pi] gamma", 4: "agent[pi] beta", 8: "agent[claude] alpha"} {
		if got := strings.TrimSpace(plain[row]); got != want {
			t.Errorf("row %d = %q, want %q", row, got, want)
		}
	}
	if strings.Contains(strings.Join(plain, "\n"), "No active subagents") {
		t.Error("placeholder shown while subagents are pending")
	}
}

func sixCards() []message {
	return []message{
		panelCard("agent", "pi", "task-one"),
		panelCard("agent", "pi", "task-two"),
		panelCard("agent", "pi", "task-three"),
		panelCard("agent", "pi", "task-four"),
		panelCard("agent", "pi", "task-five"),
		panelCard("agent", "pi", "task-six"),
	}
}

func TestRenderSubagentPanel_CapsAtFourWithFooter(t *testing.T) {
	plain := panelPlain(t, panelInput(40, 20, sixCards()...))
	// 20 rows - 1 footer = 19 rows over 4 blocks: 4, 4, 4, 7 (last absorbs the remainder).
	for row, want := range map[int]string{
		0: "agent[pi] task-six", 4: "agent[pi] task-five", 8: "agent[pi] task-four", 12: "agent[pi] task-three",
		19: "+2 more",
	} {
		if got := strings.TrimSpace(plain[row]); got != want {
			t.Errorf("row %d = %q, want %q", row, got, want)
		}
	}
	all := strings.Join(plain, "\n")
	for _, gone := range []string{"task-one", "task-two"} {
		if strings.Contains(all, gone) {
			t.Errorf("%q is beyond the cap of 4 but was rendered", gone)
		}
	}
}

func TestRenderSubagentPanel_NoFooterAtExactlyFour(t *testing.T) {
	plain := panelPlain(t, panelInput(40, 8, sixCards()[2:]...)) // exactly 4 pending
	if strings.Contains(strings.Join(plain, "\n"), "more") {
		t.Error("footer shown with exactly 4 pending subagents")
	}
}

func TestRenderSubagentPanel_ShortPanelReducesShownAndGrowsFooter(t *testing.T) {
	plain := panelPlain(t, panelInput(40, 4, sixCards()[1:]...)) // 5 pending, 4 rows
	for row, want := range map[int]string{
		0: "agent[pi] task-six", 1: "agent[pi] task-five", 2: "agent[pi] task-four", 3: "+2 more",
	} {
		if got := strings.TrimSpace(plain[row]); got != want {
			t.Errorf("row %d = %q, want %q", row, got, want)
		}
	}
}

func TestRenderSubagentPanel_FooterOnlyWhenNoBlockFits(t *testing.T) {
	plain := panelPlain(t, panelInput(40, 1, sixCards()[:3]...))
	if got := strings.TrimSpace(plain[0]); got != "+3 more" {
		t.Fatalf("row 0 = %q, want %q", got, "+3 more")
	}
}

// T7: an A2A card is a subagent too, headed by its configured agent name.
func TestRenderSubagentPanel_A2AHeader(t *testing.T) {
	a2a := panelCard("a2a", "", "check mesh")
	a2a.agentLabel = "istio-agent"
	plain := panelPlain(t, panelInput(40, 6, a2a))
	if got := strings.TrimSpace(plain[0]); got != "agent[istio-agent] check mesh" {
		t.Fatalf("row 0 = %q, want the A2A header", got)
	}
}

func TestRenderSubagentPanel_HeaderOnlyWhenNoEvents(t *testing.T) {
	plain := panelPlain(t, panelInput(40, 6, panelCard("subagent", "pi", "quiet")))
	if got := strings.TrimSpace(plain[0]); got != "agent[pi] quiet" {
		t.Fatalf("row 0 = %q", got)
	}
	for i := 1; i < len(plain); i++ {
		if strings.TrimSpace(plain[i]) != "" {
			t.Errorf("row %d = %q, want blank (no events)", i, plain[i])
		}
	}
}

func TestRenderSubagentPanel_HeaderIsClippedAndControlSafe(t *testing.T) {
	long := panelCard("agent", "pi", "漢字漢字漢字漢字漢字漢字漢字漢字")
	bad := panelCard("agent", "pi", "\x1b[31mred\x1b[0m\ttitle\x08")
	for _, w := range []int{12, 21, 44} {
		plain := panelPlain(t, panelInput(w, 4, long))
		if strings.TrimSpace(plain[0]) == "" {
			t.Errorf("width %d: header vanished", w)
		}
	}
	plain := panelPlain(t, panelInput(40, 3, bad))
	if got := strings.TrimSpace(plain[0]); got != "agent[pi] red title" {
		t.Fatalf("header = %q, want ANSI stripped and control characters gone", got)
	}
	if strings.ContainsAny(strings.Join(plain, ""), "\t\r\b\x1b") {
		t.Error("control characters survived into the panel")
	}
}

// Rendering must never panic, whatever size it is given.
func TestRenderSubagentPanel_TinySizes(t *testing.T) {
	for _, pending := range [][]message{nil, sixCards()[:1], sixCards()} {
		for _, wh := range [][2]int{{0, 0}, {0, 3}, {-4, 3}, {3, 0}, {3, -1}, {1, 1}, {2, 1}, {3, 2}, {44, 1}, {44, 2}} {
			w, h := wh[0], wh[1]
			out := RenderSubagentPanel(panelInput(w, h, pending...))
			if h <= 0 {
				if out != "" {
					t.Errorf("%dx%d: got %q, want empty for non-positive height", w, h, out)
				}
				continue
			}
			rows := strings.Split(out, "\n")
			if len(rows) != h {
				t.Errorf("%dx%d (%d pending): %d rows, want %d", w, h, len(pending), len(rows), h)
			}
			if w > 0 {
				for i, r := range rows {
					if got := ansi.StringWidth(r); got != w {
						t.Errorf("%dx%d row %d: %d cells wide, want %d", w, h, i, got, w)
					}
				}
			}
		}
	}
}
