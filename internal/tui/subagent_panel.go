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
// It shows only live subagents: see [pendingSubagentCards] for which cards
// count, and Idle for the turn-ended case.
type SubagentPanelRenderInput struct {
	Width    int
	Height   int
	Palette  Palette
	Messages []message
	// Idle is true while no turn is running. Nothing is live then, so the panel
	// shows the placeholder whatever the cards say.
	Idle bool
}

// isSubagentCardTool reports whether a tool message is a subagent card.
func isSubagentCardTool(tool string) bool {
	return tool == "agent" || tool == "subagent" || tool == "a2a"
}

// pendingSubagentCards returns the indices of the live subagent cards, newest
// first. A later index is the more recent card; no timestamp is involved. It
// reads messages and never writes them.
//
// A card is live when it is still waiting on a result and neither rule below
// retires it:
//  1. Its toolID is shared with a card that has a result. A tasks[]/chain[] call
//     fans out into N cards under one toolID, but its single result fills only
//     one of them, so the call is answered and its siblings are done.
//  2. A user message follows it. A new turn (a steer or a queued prompt) began,
//     so the turn that spawned the card ended.
//
// A turn canceled with nothing after it has no later user message; that case
// is [SubagentPanelRenderInput.Idle].
func pendingSubagentCards(messages []message) []int {
	answered := make(map[string]bool)
	for i := range messages {
		if isSubagentCardTool(messages[i].tool) && messages[i].toolID != "" && !messages[i].toolPending() {
			answered[messages[i].toolID] = true
		}
	}
	var out []int
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].role == "user" {
			break
		}
		if !isSubagentCardTool(messages[i].tool) || !messages[i].toolPending() {
			continue
		}
		if id := messages[i].toolID; id != "" && answered[id] {
			continue
		}
		out = append(out, i)
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
	var pending []int
	if !in.Idle {
		pending = pendingSubagentCards(in.Messages)
	}
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
			rows = append(rows, renderSubagentBlock(in.Messages[pending[i]], h, in.Width, header, dim)...)
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
// the newest event-tail lines top-aligned beneath it, then blank padding.
func renderSubagentBlock(msg message, height, width int, headerStyle, tailStyle lipgloss.Style) []string {
	if height <= 0 {
		return nil
	}
	rows := make([]string, 0, height)
	rows = append(rows, subagentPanelRow(subagentHeaderText(msg), headerStyle, width))
	for _, line := range subagentTailLines(msg.agentEvents, height-1) {
		rows = append(rows, subagentPanelRow(line, tailStyle, width))
	}
	blank := padLine("", width)
	for len(rows) < height {
		rows = append(rows, blank)
	}
	return rows
}

// subagentTailLines returns the newest n rendered lines of the event stream,
// oldest first, as plain unclipped text. It walks newest to oldest so older
// events are never expanded once n lines are in hand; it never writes evs.
func subagentTailLines(evs []agentEv, n int) []string {
	if n <= 0 {
		return nil
	}
	renderable := renderableAgentEvents(evs)
	var chunks [][]string // newest event first
	count := 0
	for i := len(renderable) - 1; i >= 0 && count < n; i-- {
		lines := subagentEventLines(renderable[i])
		chunks = append(chunks, lines)
		count += len(lines)
	}
	out := make([]string, 0, count)
	for i := len(chunks) - 1; i >= 0; i-- {
		out = append(out, chunks[i]...)
	}
	return out[max(0, len(out)-n):]
}

// subagentEventLines renders one event as "kind: first line" followed by
// two-space-indented continuation lines; blank lines are dropped.
func subagentEventLines(ev agentEv) []string {
	kind := ev.kind
	if kind == "text_delta" {
		kind = "text"
	}
	content := strings.ReplaceAll(ansi.Strip(ev.content), "\r\n", "\n")
	var lines []string
	for part := range strings.SplitSeq(content, "\n") {
		part = strings.TrimRight(part, " \t\r")
		if strings.TrimSpace(part) == "" {
			continue
		}
		if lines == nil {
			lines = append(lines, kind+": "+part)
			continue
		}
		lines = append(lines, "  "+part)
	}
	if lines == nil {
		return []string{kind + ":"}
	}
	return lines
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

// subagentPanelInput derives the panel's input fresh from the model every frame;
// the message slice is passed as is, not copied. Idle is true while no turn is
// running, so a turn canceled mid-subagent leaves the placeholder.
func (m *model) subagentPanelInput(width, height int) SubagentPanelRenderInput {
	return SubagentPanelRenderInput{Width: width, Height: height, Palette: m.palette, Messages: m.chatModel.Messages, Idle: !m.running}
}
