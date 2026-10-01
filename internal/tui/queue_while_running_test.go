package tui

import (
	"context"
	"testing"
)

// Queueing: a prompt submitted while a turn runs waits for that turn to end. It
// must not cancel the turn, so subagents and tool calls in flight finish.

func TestSubmitWhileRunning_QueuesWithoutCancelingTurn(t *testing.T) {
	m := newTestModel(t)
	m.running = true
	m.agentCh = make(chan agentMsg, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.agentCancel = cancel

	_, cmd, handled := m.handleInputSubmit(InputSubmitMsg{Text: "also do X"})
	if !handled {
		t.Fatal("submit was not handled")
	}
	if cmd != nil {
		t.Fatal("a prompt submitted mid-turn started immediately; it must wait")
	}
	if ctx.Err() != nil {
		t.Fatal("submitting a prompt canceled the running turn")
	}
	if m.agentCancel == nil {
		t.Fatal("agentCancel was cleared; Esc could no longer reach the running turn")
	}
	if !m.running {
		t.Fatal("running was cleared")
	}
	if got := len(m.pendingPrompts); got != 1 {
		t.Fatalf("pending = %d, want 1", got)
	}
}

// The turn's own done starts the queued prompt, and the prompt keeps running.
func TestSubmitWhileRunning_DoneStartsQueuedPrompt(t *testing.T) {
	m := newTestModel(t)
	m.running = true
	m.agentCh = make(chan agentMsg, 4)
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.agentCancel = cancel

	m.handleInputSubmit(InputSubmitMsg{Text: "next up"})

	close(m.agentCh)
	m.agentCh = nil
	_, cmd := m.Update(agentDoneMsg{})
	if cmd == nil {
		t.Fatal("the finished turn's done did not start the queued prompt")
	}
	if !m.running {
		t.Fatal("queued turn is not running")
	}
	if got := len(m.pendingPrompts); got != 0 {
		t.Fatalf("pending = %d after start, want 0", got)
	}
	if got := m.chatModel.Messages[len(m.chatModel.Messages)-2].content; got != "next up" {
		t.Fatalf("queued prompt = %q, want %q", got, "next up")
	}
	if m.lastPromptFailed {
		t.Error("a clean turn end offered a retry")
	}
}

// A full queue refuses the prompt and leaves the running turn alone.
func TestSubmitWhileRunning_FullQueueLeavesTurnRunning(t *testing.T) {
	m := newTestModel(t)
	m.running = true
	m.agentCh = make(chan agentMsg, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.agentCancel = cancel
	m.pendingPrompts = make([]queuedPrompt, maxPendingPrompts)

	_, cmd, _ := m.handleInputSubmit(InputSubmitMsg{Text: "won't fit"})
	if cmd != nil {
		t.Fatal("submit returned a command with a full queue")
	}
	if ctx.Err() != nil || !m.running {
		t.Error("a refused prompt disturbed the running turn")
	}
	if m.flash != "Prompt queue full" {
		t.Errorf("flash = %q, want the queue-full notice", m.flash)
	}
}

// An Esc cancel is still a cancellation and keeps its error and retry offer.
func TestCancel_EscStillReportsFailure(t *testing.T) {
	m := newTestModel(t)
	m.running = true
	m.agentCh = make(chan agentMsg, 4)
	m.lastPrompt = "do something long"

	m.Update(agentDoneMsg{err: context.Canceled})

	if !m.lastPromptFailed {
		t.Error("an Esc cancel stopped offering the retry it used to offer")
	}
}

// Esc during a turn must not strand a prompt the user already queued.
// cancelAgent drains the channel, and startNextPrompt is otherwise only reached
// from handleAgentDone, so cancelAgent has to start it itself.
func TestCancel_StartsQueuedPrompt(t *testing.T) {
	m := newTestModel(t)
	m.running = true
	m.agentCh = make(chan agentMsg, 4)

	if _, cmd := m.enqueuePrompt("queued while running", nil); cmd != nil {
		t.Fatal("queued prompt started while a turn was running")
	}
	m.cancelAgent()

	if len(m.pendingPrompts) != 0 {
		t.Error("pending prompt stranded after Esc: nothing will ever start it")
	}
	if !m.running {
		t.Error("queued prompt did not start after Esc")
	}
}
