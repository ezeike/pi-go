package tui

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// stalePanelModel is a width-200 model mid-turn (running), so the panel is
// live until a test ends the turn.
func stalePanelModel(t *testing.T) *model {
	t.Helper()
	m := historyModel(t, "first")
	m.width, m.height = 200, 40
	m.applyResize()
	m.running = true
	m.chatModel.Messages = append(m.chatModel.Messages, message{role: "user", content: "go"})
	return m
}

func updateWith(t *testing.T, m *model, msg any) *model {
	t.Helper()
	next, _ := m.Update(msg)
	mm, ok := next.(*model)
	if !ok {
		t.Fatalf("Update returned %T, want *model", next)
	}
	return mm
}

// panelText is the text of the panel columns across the top section of the
// real View frame.
func panelText(t *testing.T, m *model) string {
	t.Helper()
	var panel []string
	for row, line := range strings.Split(m.View().Content, "\n") {
		if got := ansi.StringWidth(line); got != m.width {
			t.Fatalf("row %d is %d cells wide, want %d", row, got, m.width)
		}
		if row < m.topSectionRows() {
			panel = append(panel, panelColumns(line))
		}
	}
	return strings.Join(panel, "\n")
}

func subagentCall(id, key string, tasks ...string) agentToolCallMsg {
	list := make([]any, 0, len(tasks))
	for _, task := range tasks {
		list = append(list, map[string]any{"agent": "pi", "task": task})
	}
	return agentToolCallMsg{id: id, name: "subagent", args: map[string]any{key: list}}
}

func wantPlaceholder(t *testing.T, m *model, titles ...string) {
	t.Helper()
	text := panelText(t, m)
	if !strings.Contains(text, subagentPanelPlaceholder) {
		t.Errorf("panel is missing %q:\n%s", subagentPanelPlaceholder, text)
	}
	for _, title := range titles {
		if strings.Contains(text, title) {
			t.Errorf("panel still shows the finished card %q:\n%s", title, text)
		}
	}
}

func TestStalePanel_SingleResultClearsParallelCards(t *testing.T) {
	for _, key := range []string{"tasks", "chain"} {
		t.Run(key, func(t *testing.T) {
			m := stalePanelModel(t)
			m = updateWith(t, m, subagentCall("call-1", key, "alpha-job", "beta-job", "gamma-job"))

			text := panelText(t, m)
			for _, title := range []string{"alpha-job", "beta-job", "gamma-job"} {
				if !strings.Contains(text, title) {
					t.Fatalf("setup: running call must show %q:\n%s", title, text)
				}
			}

			m = updateWith(t, m, agentToolResultMsg{id: "call-1", name: "subagent", content: "all done"})
			wantPlaceholder(t, m, "alpha-job", "beta-job", "gamma-job")
		})
	}
}

func TestStalePanel_RunningParallelCallKeepsEveryCard(t *testing.T) {
	m := stalePanelModel(t)
	m = updateWith(t, m, subagentCall("call-1", "tasks", "alpha-job", "beta-job", "gamma-job"))
	text := panelText(t, m)
	for _, title := range []string{"alpha-job", "beta-job", "gamma-job"} {
		if !strings.Contains(text, title) {
			t.Errorf("panel lost %q while the call is still running:\n%s", title, text)
		}
	}
	if strings.Contains(text, subagentPanelPlaceholder) {
		t.Errorf("panel shows the placeholder while subagents run:\n%s", text)
	}
}

func TestStalePanel_AnotherCallsResultKeepsThisCall(t *testing.T) {
	m := stalePanelModel(t)
	m = updateWith(t, m, subagentCall("call-1", "tasks", "alpha-job", "beta-job"))
	m = updateWith(t, m, subagentCall("call-2", "tasks", "gamma-job"))
	m = updateWith(t, m, agentToolResultMsg{id: "call-2", name: "subagent", content: "done"})
	text := panelText(t, m)
	for _, title := range []string{"alpha-job", "beta-job"} {
		if !strings.Contains(text, title) {
			t.Errorf("panel dropped %q although only call-2 was answered:\n%s", title, text)
		}
	}
	if strings.Contains(text, "gamma-job") {
		t.Errorf("panel still shows the answered call-2 card:\n%s", text)
	}
}

func TestStalePanel_CancelledTurnShowsPlaceholder(t *testing.T) {
	m := stalePanelModel(t)
	m = updateWith(t, m, subagentCall("call-1", "tasks", "alpha-job", "beta-job"))
	m = updateWith(t, m, agentDoneMsg{err: context.Canceled})
	if m.running {
		t.Fatal("setup: agentDoneMsg must end the turn")
	}
	wantPlaceholder(t, m, "alpha-job", "beta-job")
}

func TestStalePanel_FollowUpPromptHidesStaleCard(t *testing.T) {
	m := stalePanelModel(t)
	m = updateWith(t, m, subagentCall("call-1", "tasks", "alpha-job"))
	// A steer or queued prompt: a new user message while the model is running.
	m.chatModel.Messages = append(m.chatModel.Messages, message{role: "user", content: "never mind"})
	wantPlaceholder(t, m, "alpha-job")
}

func TestStalePanel_ReusedToolIDInLaterTurnStaysLive(t *testing.T) {
	m := stalePanelModel(t)
	m = updateWith(t, m, subagentCall("call_0", "tasks", "old-job"))
	m = updateWith(t, m, agentToolResultMsg{id: "call_0", name: "subagent", content: "done"})
	m.chatModel.Messages = append(m.chatModel.Messages, message{role: "user", content: "again"})
	m = updateWith(t, m, subagentCall("call_0", "tasks", "new-job-a", "new-job-b"))

	text := panelText(t, m)
	for _, title := range []string{"new-job-a", "new-job-b"} {
		if !strings.Contains(text, title) {
			t.Errorf("an older answered call_0 hid the live card %q:\n%s", title, text)
		}
	}
	if strings.Contains(text, "old-job") {
		t.Errorf("panel shows the answered turn-1 card:\n%s", text)
	}
}

func TestPendingSubagentCards_OlderAnsweredCardDoesNotRetireNewerCall(t *testing.T) {
	old := finishedCard(message{role: "tool", tool: "subagent", toolID: "call_0"})
	live := message{role: "tool", tool: "subagent", toolID: "call_0"}
	if got, want := pendingSubagentCards([]message{old, live}), []int{1}; !slices.Equal(got, want) {
		t.Errorf("older answered card retired a newer call: got %v, want %v", got, want)
	}
}

func TestPendingSubagentCards_AnsweredSiblingsAreNotLive(t *testing.T) {
	a := message{role: "tool", tool: "subagent", toolID: "x", agentTitle: "a"}
	b := message{role: "tool", tool: "subagent", toolID: "x", agentTitle: "b"}
	done := finishedCard(message{role: "tool", tool: "subagent", toolID: "x", agentTitle: "c"})
	other := message{role: "tool", tool: "subagent", toolID: "y", agentTitle: "d"}

	if got := pendingSubagentCards([]message{a, b, done}); len(got) != 0 {
		t.Errorf("siblings of an answered call = %v, want none", got)
	}
	// Cards after an answered card with the same toolID are a newer call.
	if got, want := pendingSubagentCards([]message{done, a, b}), []int{2, 1}; !slices.Equal(got, want) {
		t.Errorf("answered card first = %v, want %v", got, want)
	}
	if got, want := pendingSubagentCards([]message{a, b, other}), []int{2, 1, 0}; !slices.Equal(got, want) {
		t.Errorf("unanswered call = %v, want %v", got, want)
	}
	if got, want := pendingSubagentCards([]message{done, other}), []int{1}; !slices.Equal(got, want) {
		t.Errorf("a different toolID must stay live: got %v, want %v", got, want)
	}
}

func TestPendingSubagentCards_EmptyToolIDNeverGroups(t *testing.T) {
	a := message{role: "tool", tool: "subagent", agentTitle: "a"}
	done := finishedCard(message{role: "tool", tool: "subagent", agentTitle: "b"})
	if got, want := pendingSubagentCards([]message{a, done}), []int{0}; !slices.Equal(got, want) {
		t.Errorf("empty toolID cards = %v, want %v", got, want)
	}
}

func TestPendingSubagentCards_LaterUserMessageEndsTheTurn(t *testing.T) {
	card := message{role: "tool", tool: "subagent", toolID: "x"}
	user := message{role: "user", content: "next"}
	if got := pendingSubagentCards([]message{user, card}); !slices.Equal(got, []int{1}) {
		t.Errorf("user message before the card = %v, want [1]", got)
	}
	if got := pendingSubagentCards([]message{card, user}); len(got) != 0 {
		t.Errorf("user message after the card = %v, want none", got)
	}
	if got := pendingSubagentCards([]message{card, user, card}); !slices.Equal(got, []int{2}) {
		t.Errorf("only the card after the user message is live: got %v, want [2]", got)
	}
}

func TestRenderSubagentPanel_IdleShowsPlaceholder(t *testing.T) {
	msgs := []message{panelCard("subagent", "pi", "worker")}
	in := SubagentPanelRenderInput{Width: SubagentPanelWidth, Height: 8, Messages: msgs}
	if out := ansi.Strip(RenderSubagentPanel(in)); !strings.Contains(out, "worker") {
		t.Fatalf("zero-value Idle must keep the card:\n%s", out)
	}
	in.Idle = true
	out := ansi.Strip(RenderSubagentPanel(in))
	if !strings.Contains(out, subagentPanelPlaceholder) || strings.Contains(out, "worker") {
		t.Errorf("Idle panel must show only the placeholder:\n%s", out)
	}
	if rows := strings.Split(out, "\n"); len(rows) != in.Height {
		t.Errorf("Idle panel has %d rows, want %d", len(rows), in.Height)
	}
}

func TestSubagentPanelInput_IdleMirrorsRunning(t *testing.T) {
	m := historyModel(t, "first")
	for _, running := range []bool{true, false} {
		m.running = running
		if got := m.subagentPanelInput(SubagentPanelWidth, 10).Idle; got != !running {
			t.Errorf("running=%v: Idle = %v, want %v", running, got, !running)
		}
	}
}

// A turn that starts without a user message (/run, create_skill) after a
// canceled subagent turn must not bring the canceled turn's cards back.
func TestStalePanel_TurnWithoutUserMessageDoesNotRevive(t *testing.T) {
	m := stalePanelModel(t)
	m.beginTurn()
	m = updateWith(t, m, subagentCall("c1", "tasks", "revive-me-a", "revive-me-b"))
	m = updateWith(t, m, agentDoneMsg{err: context.Canceled})
	wantPlaceholder(t, m, "revive-me-a", "revive-me-b")

	m.beginTurn() // what /run does: no user message is appended
	wantPlaceholder(t, m, "revive-me-a", "revive-me-b")

	m = updateWith(t, m, subagentCall("c2", "tasks", "fresh-card"))
	if text := panelText(t, m); !strings.Contains(text, "fresh-card") {
		t.Errorf("a card spawned in the new turn is missing:\n%s", text)
	}
}

// Every turn start goes through beginTurn so the marker cannot be skipped.
func TestBeginTurn_IsTheOnlyRunningSetter(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if f != "tui.go" && strings.Contains(string(src), "m.running = true") {
			t.Errorf("%s sets m.running = true directly; use m.beginTurn()", f)
		}
	}
}
