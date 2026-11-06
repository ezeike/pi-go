package piagent

import (
	"slices"
	"testing"
)

func TestBuiltinToolsAllowListFiltersOnlyBuiltins(t *testing.T) {
	isolate(t)

	all := toolNames(newTestAgent(t, &fakeLLM{name: "fake", reply: "ok"}).tools)
	for _, want := range []string{"read", "bash", "session-stats"} {
		if !slices.Contains(all, want) {
			t.Fatalf("control: default tools %v lack %q", all, want)
		}
	}

	ag := newTestAgent(t, &fakeLLM{name: "fake", reply: "ok"},
		WithBuiltinTools("read", "bash", "no-such-tool"),
		WithTools(namedTool{name: "custom"}),
	)
	got := toolNames(ag.tools)
	slices.Sort(got)
	if want := []string{"bash", "custom", "read"}; !slices.Equal(got, want) {
		t.Errorf("tools = %v, want %v", got, want)
	}
}

func TestBuiltinToolsEmptyListRemovesEveryBuiltin(t *testing.T) {
	isolate(t)

	ag := newTestAgent(t, &fakeLLM{name: "fake", reply: "ok"},
		WithBuiltinTools(), WithTools(namedTool{name: "custom"}))
	if got := toolNames(ag.tools); !slices.Equal(got, []string{"custom"}) {
		t.Errorf("tools = %v, want only the embedder's", got)
	}
}
