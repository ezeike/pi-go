package piagent

import (
	"sync/atomic"
	"testing"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
)

// TestEmbedderAfterModelCallbackRuns is the regression test for pi-go's own
// tracing callback returning the response: ADK stops at the first after-model
// callback that does, so the embedder's never ran.
func TestEmbedderAfterModelCallbackRuns(t *testing.T) {
	isolate(t)

	var ran atomic.Int32
	cb := llmagent.AfterModelCallback(func(agent.Context, *model.LLMResponse, error) (*model.LLMResponse, error) {
		ran.Add(1)
		return nil, nil
	})
	ag := newTestAgent(t, &fakeLLM{name: "fake", reply: "ok"}, WithAfterModelCallbacks(cb))

	sessionID, err := ag.NewSession(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ag.Ask(t.Context(), sessionID, "hi"); err != nil {
		t.Fatal(err)
	}
	if ran.Load() == 0 {
		t.Error("the embedder's after-model callback never ran")
	}
}
