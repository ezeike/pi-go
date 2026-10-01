package tui

import (
	"reflect"
	"testing"
)

// panelCard builds a subagent card the way handleToolCall leaves it while the
// subagent is still running: role "tool", no result content yet.
func panelCard(tool, agentType, title string, evs ...agentEv) message {
	return message{role: "tool", tool: tool, agentType: agentType, agentTitle: title, agentEvents: evs}
}

// finishedCard is a card whose result has arrived.
func finishedCard(c message) message {
	c.content = "done"
	return c
}

func TestSubagentPanelConstants(t *testing.T) {
	if SubagentPanelWidth != 44 {
		t.Errorf("SubagentPanelWidth = %d, want 44", SubagentPanelWidth)
	}
	if SubagentPanelMinTotalWidth != 147 {
		t.Errorf("SubagentPanelMinTotalWidth = %d, want 147", SubagentPanelMinTotalWidth)
	}
}

func TestPendingSubagentCards_NewestFirstAndFiltered(t *testing.T) {
	a2a := panelCard("a2a", "", "check mesh")
	a2a.agentLabel = "istio-agent"
	refreshed := panelCard("agent", "pi", "polling")
	refreshed.content = "previous poll"
	refreshed.pendingRefresh = true

	msgs := []message{
		{role: "user", content: "go"},                            // 0
		panelCard("agent", "claude", "alpha"),                    // 1 pending
		{role: "assistant", content: "working"},                  // 2
		panelCard("subagent", "pi", "beta"),                      // 3 pending
		finishedCard(panelCard("agent", "pi", "finished")),       // 4 result arrived
		{role: "tool", tool: "bash", toolIn: `{"command":"ls"}`}, // 5 pending, not a subagent
		a2a,       // 6 pending
		refreshed, // 7 pending (refresh in flight)
	}

	got := pendingSubagentCards(msgs)
	want := []int{7, 6, 3, 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pendingSubagentCards = %v, want %v (newest first; finished and non-subagent cards excluded)", got, want)
	}
}

func TestPendingSubagentCards_NoneAndEmpty(t *testing.T) {
	if got := pendingSubagentCards(nil); len(got) != 0 {
		t.Fatalf("nil messages: got %v, want none", got)
	}
	msgs := []message{
		{role: "user", content: "hi"},
		finishedCard(panelCard("subagent", "pi", "old")),
	}
	if got := pendingSubagentCards(msgs); len(got) != 0 {
		t.Fatalf("no pending cards: got %v, want none", got)
	}
}

func TestPendingSubagentCards_DoesNotMutate(t *testing.T) {
	build := func() []message {
		return []message{
			panelCard("agent", "pi", "one", agentEv{kind: "text", content: "a"}, agentEv{kind: "text", content: "b"}),
			panelCard("subagent", "claude", "two"),
		}
	}
	msgs := build()
	_ = pendingSubagentCards(msgs)
	if !reflect.DeepEqual(msgs, build()) {
		t.Fatal("pendingSubagentCards mutated its input")
	}
}
