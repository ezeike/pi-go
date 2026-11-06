package piagent

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"google.golang.org/genai"
)

// factoryRecorder is a CommandFactory that records what it was asked to run
// and then runs it unchanged, so a tool call still completes.
type factoryRecorder struct {
	mu    sync.Mutex
	calls []string // "name arg arg ..."
	dirs  []string
}

func (r *factoryRecorder) factory(ctx context.Context, dir, name string, args ...string) (*exec.Cmd, error) {
	r.mu.Lock()
	r.calls = append(r.calls, strings.TrimSpace(name+" "+strings.Join(args, " ")))
	r.dirs = append(r.dirs, dir)
	r.mu.Unlock()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd, nil
}

func (r *factoryRecorder) saw(prefix string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

// runToolCall drives one turn in which the model calls call, against an agent
// built with the recorder installed as its command factory.
func runToolCall(t *testing.T, call *genai.FunctionCall, rec *factoryRecorder) {
	t.Helper()
	isolate(t)
	workDir := t.TempDir()

	ag := newTestAgent(t, &fakeLLM{name: "fake", reply: "done", toolCall: call},
		WithWorkingDir(workDir), WithCommandFactory(rec.factory))

	sessionID, err := ag.NewSession(t.Context())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if _, err := ag.Ask(t.Context(), sessionID, "go"); err != nil {
		t.Fatalf("Ask: %v", err)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, d := range rec.dirs {
		if d != workDir {
			t.Errorf("factory asked to run in %q, want the agent's working directory %q", d, workDir)
		}
	}
}

func TestCommandFactoryRunsTheBashTool(t *testing.T) {
	rec := &factoryRecorder{}

	runToolCall(t, &genai.FunctionCall{Name: "bash", Args: map[string]any{"command": "echo hi"}}, rec)

	if !rec.saw("bash -c echo hi") {
		t.Errorf("the bash tool did not go through the factory; factory saw %v", rec.calls)
	}
}

func TestCommandFactoryRunsTheGitTools(t *testing.T) {
	rec := &factoryRecorder{}

	runToolCall(t, &genai.FunctionCall{Name: "git-overview", Args: map[string]any{}}, rec)

	if !rec.saw("git ") {
		t.Errorf("the git tools did not go through the factory; factory saw %v", rec.calls)
	}
}
