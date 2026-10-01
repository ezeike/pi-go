package tui

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
