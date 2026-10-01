package tui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// SubagentPanelWidth is the fixed width of the left subagent panel on a
// widescreen terminal. The panel keeps this width for the whole session, with
// or without a pending subagent, so the layout never reflows mid-turn.
const SubagentPanelWidth = 44

// SubagentPanelMinTotalWidth is the widest terminal that still renders two
// columns (chat + sidebar). One column wider and the panel appears: 44 for the
// panel, 23 for the sidebar and 80 for a chat that stays at least that wide
// (the 80th column belongs to the rail, so the chat itself is 80 at width 148).
const SubagentPanelMinTotalWidth = SubagentPanelWidth + SidebarWidth + 80

// maxSubagentBlocks caps how many subagents the panel stacks; more pending
// subagents collapse into a "+N more" footer row.
const maxSubagentBlocks = 4

// SubagentPanelRenderInput is everything RenderSubagentPanel reads. The panel is
// a pure function of it, derived fresh every frame from existing model state.
type SubagentPanelRenderInput struct {
	Width    int
	Height   int
	Palette  Palette
	Messages []message
}

// isSubagentCardTool reports whether a tool message is a subagent card.
func isSubagentCardTool(tool string) bool {
	return tool == "agent" || tool == "subagent" || tool == "a2a"
}

// pendingSubagentCards returns the indices of the subagent cards still waiting
// on a result, newest first. A later index is the more recent card; no
// timestamp is involved. It reads messages and never writes them.
func pendingSubagentCards(messages []message) []int {
	var out []int
	for i := len(messages) - 1; i >= 0; i-- {
		if isSubagentCardTool(messages[i].tool) && messages[i].toolPending() {
			out = append(out, i)
		}
	}
	return out
}

// subagentPanelPlaceholder is row 0 of the panel while no subagent is pending.
const subagentPanelPlaceholder = "No active subagents"

// RenderSubagentPanel draws the left subagent panel: one block per pending
// subagent, newest first, capped at maxSubagentBlocks with a "+N more" footer.
// It always returns exactly Height rows of exactly Width cells (empty for a
// non-positive Height) and reads in.Messages without writing it.
func RenderSubagentPanel(in SubagentPanelRenderInput) string {
	if in.Height <= 0 {
		return ""
	}
	if in.Width <= 0 {
		return strings.Repeat("\n", in.Height-1)
	}
	pal := paletteOrDark(in.Palette)
	dim := lipgloss.NewStyle().Foreground(pal.Dim)

	rows := make([]string, 0, in.Height)
	pending := pendingSubagentCards(in.Messages)
	if len(pending) == 0 {
		rows = append(rows, subagentPanelRow(subagentPanelPlaceholder, dim, in.Width))
	} else {
		header := lipgloss.NewStyle().Bold(true).Foreground(pal.Mauve)
		shown, footer := subagentPanelPlan(in.Height, len(pending))
		avail := in.Height
		if footer {
			avail--
		}
		for i, h := range subagentBlockHeights(avail, shown) {
			rows = append(rows, renderSubagentBlock(in.Messages[pending[i]], h, in.Width, header)...)
		}
		if footer {
			more := "+" + strconv.Itoa(len(pending)-shown) + " more"
			rows = append(rows, subagentPanelRow(more, dim, in.Width))
		}
	}

	blank := padLine("", in.Width)
	for len(rows) < in.Height {
		rows = append(rows, blank)
	}
	return strings.Join(rows[:in.Height], "\n")
}

// renderSubagentBlock renders one subagent as height rows: its header, then
// blank rows reserved for the event tail.
func renderSubagentBlock(msg message, height, width int, header lipgloss.Style) []string {
	if height <= 0 {
		return nil
	}
	rows := make([]string, 0, height)
	rows = append(rows, subagentPanelRow(subagentHeaderText(msg), header, width))
	blank := padLine("", width)
	for len(rows) < height {
		rows = append(rows, blank)
	}
	return rows
}

// subagentPanelRow lays out one panel row: a one-cell left indent, the text
// clipped to Width-2 cells before styling (no ellipsis), and padding that
// leaves the last column blank as a right gutter.
func subagentPanelRow(text string, style lipgloss.Style, width int) string {
	inner := max(0, width-2)
	clipped := ansi.Truncate(sanitizeCells(text), inner, "")
	if clipped == "" {
		return padLine("", width)
	}
	return padLine(" "+style.Render(clipped), width)
}

// subagentPanelPlan decides how many blocks fit in height rows for pending
// subagents, and whether a "+N more" footer row is needed. When not even one
// block fits beside the footer, the panel is the footer alone (shown == 0).
func subagentPanelPlan(height, pending int) (shown int, footer bool) {
	shown = min(pending, maxSubagentBlocks)
	for shown > 0 {
		need := shown
		if pending > shown {
			need++
		}
		if need <= height {
			break
		}
		shown--
	}
	return shown, pending > shown
}

// subagentBlockHeights splits avail rows evenly over shown blocks; the last
// block absorbs the remainder.
func subagentBlockHeights(avail, shown int) []int {
	if shown <= 0 {
		return nil
	}
	hs := make([]int, shown)
	for i := range hs {
		hs[i] = avail / shown
	}
	hs[shown-1] += avail % shown
	return hs
}

// subagentHeaderText is a block's plain, single-line header, e.g.
// "agent[claude] review the diff". Clipping happens in the row layout.
func subagentHeaderText(msg message) string {
	label := msg.agentLabel
	if label == "" {
		label = agentBracketLabel(msg.agentType)
	}
	var b strings.Builder
	b.WriteString("agent")
	if label != "" {
		b.WriteString("[" + label + "]")
	}
	if title := collapseToSingleLine(msg.agentTitle); title != "" {
		b.WriteString(" " + title)
	}
	return b.String()
}
