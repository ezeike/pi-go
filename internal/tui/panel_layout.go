package tui

// panelWidths is the single source of the frame's column widths: the subagent
// panel on the left, the chat body plus its rail in the middle, and the sidebar
// on the right. The three always sum to width for any width above 80.
//
//	width <= 80                          -> no panel, no sidebar
//	80 < width <= SubagentPanelMinTotalWidth -> sidebar only (today's layout)
//	width >  SubagentPanelMinTotalWidth  -> panel, chat, sidebar
func panelWidths(width int) (left, main, sidebar int) {
	switch {
	case width <= 80:
		return 0, width, 0
	case width <= SubagentPanelMinTotalWidth:
		return 0, width - SidebarWidth, SidebarWidth
	default:
		return SubagentPanelWidth, width - SubagentPanelWidth - SidebarWidth, SidebarWidth
	}
}

// chatXOffset is the frame column the chat body starts at: the subagent panel's
// width when it is shown, else 0. Selection coordinates are chat-relative, so
// every translation between a frame column and a chat column goes through this.
func (m *model) chatXOffset() int {
	left, _, _ := panelWidths(m.width)
	return left
}
