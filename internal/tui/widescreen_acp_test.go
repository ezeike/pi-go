package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestWidescreenPanel_ACPEventSequence(t *testing.T) {
	m := historyModel(t, "first")
	m.width, m.height = 200, 40
	m.applyResize()
	m.running = true // the panel shows only a running turn's subagents
	m.chatModel.Messages = append(m.chatModel.Messages,
		message{role: "user", content: "plan the migration"},
		message{role: "tool", tool: "subagent", agentType: "forge-controller", agentTitle: "Plan the migration"})

	const id = "forge-controller-1760000000000000000"
	seq := []agentSubEventMsg{
		{agentID: id, kind: "spawn", pipelineID: "p1", pipelineMode: "single", pipelineStep: 1, pipelineTotal: 1},
		{agentID: id, kind: "message_start"},
		{agentID: id, kind: "text_delta", content: "Reading "},
		{agentID: id, kind: "text_delta", content: "the plan"},
		{agentID: id, kind: "tool_call", content: "read plan.md"},
		{agentID: id, kind: "stderr", content: "WS frame dump 0x42"},
		{agentID: id, kind: "message_end"},
	}
	for _, ev := range seq {
		next, _ := m.Update(ev)
		m = next.(*model)
	}
	if got := m.chatModel.Messages[1].agentID; got != id {
		t.Fatalf("spawn did not bind the card: agentID = %q, want %q", got, id)
	}

	rows := strings.Split(m.View().Content, "\n")
	var panel []string
	for row, line := range rows {
		if got := ansi.StringWidth(line); got != m.width {
			t.Fatalf("row %d is %d cells wide, want %d", row, got, m.width)
		}
		if row < m.topSectionRows() {
			panel = append(panel, panelColumns(line))
		}
	}
	text := strings.Join(panel, "\n")

	header := "agent[" + agentBracketLabel("forge-controller") + "] Plan the migration"
	for _, want := range []string{header, "text: Reading the plan", "tool_call: read plan.md"} {
		if !strings.Contains(text, want) {
			t.Errorf("panel columns are missing %q:\n%s", want, text)
		}
	}
	for _, hidden := range []string{"message_start", "message_end", "stderr", "WS frame dump", "spawn"} {
		if strings.Contains(text, hidden) {
			t.Errorf("panel columns show %q, which the tail must hide:\n%s", hidden, text)
		}
	}
}
