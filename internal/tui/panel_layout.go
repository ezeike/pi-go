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

// composeTopSection joins the top section's columns: [subagent panel |] chat
// body plus rail | sidebar. Below the sidebar threshold it is today's padded
// left panel alone.
func (m *model) composeTopSection(leftPanel string, panelRows int) string {
	left, main, sidebar := panelWidths(m.width)
	if sidebar == 0 {
		return padLinesTo(leftPanel, m.width)
	}
	cols := make([]panelColumn, 0, 3)
	if left > 0 {
		cols = append(cols, panelColumn{RenderSubagentPanel(m.subagentPanelInput(left, panelRows)), left})
	}
	cols = append(cols,
		panelColumn{leftPanel, main},
		panelColumn{RenderSidebar(m.sidebarRenderInput(sidebar, panelRows)), sidebar})
	return joinPanels(cols...)
}
