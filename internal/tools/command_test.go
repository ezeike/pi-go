package tools

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// factoryCall is one invocation a recording factory saw.
type factoryCall struct {
	dir  string
	name string
	args []string
}

// recordingFactory returns a factory that records its calls and builds each
// command with build. A nil build runs the requested program unchanged.
func recordingFactory(build func(ctx context.Context, c factoryCall) *exec.Cmd) (CommandFactory, func() []factoryCall) {
	var (
		mu    sync.Mutex
		calls []factoryCall
	)
	factory := func(ctx context.Context, dir, name string, args ...string) (*exec.Cmd, error) {
		c := factoryCall{dir: dir, name: name, args: append([]string(nil), args...)}
		mu.Lock()
		calls = append(calls, c)
		mu.Unlock()
		if build != nil {
			return build(ctx, c), nil
		}
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		return cmd, nil
	}
	snapshot := func() []factoryCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]factoryCall(nil), calls...)
	}
	return factory, snapshot
}

func TestNewCommand_DefaultRunsOnHost(t *testing.T) {
	sb := testSandbox(t, t.TempDir())

	cmd, err := sb.newCommand(t.Context(), sb.Dir(), "git", "status", "--porcelain")
	if err != nil {
		t.Fatalf("newCommand: %v", err)
	}
	if cmd.Dir != sb.Dir() {
		t.Errorf("Dir = %q, want %q", cmd.Dir, sb.Dir())
	}
	if want := []string{"git", "status", "--porcelain"}; !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("Args = %v, want %v", cmd.Args, want)
	}
}

func TestNewCommand_FactoryReceivesTheRequest(t *testing.T) {
	sb := testSandbox(t, t.TempDir())
	factory, calls := recordingFactory(nil)
	sb.SetCommandFactory(factory)

	if _, err := sb.newCommand(t.Context(), "/some/dir", "git", "log", "-1"); err != nil {
		t.Fatalf("newCommand: %v", err)
	}

	want := []factoryCall{{dir: "/some/dir", name: "git", args: []string{"log", "-1"}}}
	if got := calls(); !reflect.DeepEqual(got, want) {
		t.Errorf("factory saw %+v, want %+v", got, want)
	}
}

func TestNewCommand_FactoryFailuresAreReported(t *testing.T) {
	sb := testSandbox(t, t.TempDir())

	sb.SetCommandFactory(func(context.Context, string, string, ...string) (*exec.Cmd, error) {
		return nil, errors.New("sandbox unavailable")
	})
	if _, err := sb.newCommand(t.Context(), sb.Dir(), "git"); err == nil || !strings.Contains(err.Error(), "sandbox unavailable") {
		t.Errorf("error = %v, want the factory's error", err)
	}

	sb.SetCommandFactory(func(context.Context, string, string, ...string) (*exec.Cmd, error) {
		return nil, nil
	})
	if _, err := sb.newCommand(t.Context(), sb.Dir(), "git"); err == nil {
		t.Error("a factory returning no command and no error was accepted")
	}
}

func TestSetCommandFactory_NilRestoresHost(t *testing.T) {
	sb := testSandbox(t, t.TempDir())
	factory, calls := recordingFactory(nil)

	sb.SetCommandFactory(factory)
	sb.SetCommandFactory(nil)
	if _, err := sb.newCommand(t.Context(), sb.Dir(), "git"); err != nil {
		t.Fatalf("newCommand: %v", err)
	}
	if got := calls(); len(got) != 0 {
		t.Errorf("a cleared factory was still called: %+v", got)
	}
}

func TestBashHandler_RunsThroughCommandFactory(t *testing.T) {
	sb := testSandbox(t, t.TempDir())
	var built *exec.Cmd
	// The factory runs something other than what was asked for, so the output
	// can only have come from the factory's command.
	factory, calls := recordingFactory(func(ctx context.Context, c factoryCall) *exec.Cmd {
		built = exec.CommandContext(ctx, "bash", "-c", "echo from-factory")
		built.Dir = c.dir
		return built
	})
	sb.SetCommandFactory(factory)

	out, err := bashHandler(sb, testSupervisor(t), nil, BashInput{Command: "echo requested"})
	if err != nil {
		t.Fatalf("bashHandler: %v", err)
	}
	if !strings.Contains(out.Stdout, "from-factory") || strings.Contains(out.Stdout, "requested") {
		t.Errorf("stdout = %q, want the factory command's output", out.Stdout)
	}

	want := []factoryCall{{dir: sb.Dir(), name: "bash", args: []string{"-c", "echo requested"}}}
	if got := calls(); !reflect.DeepEqual(got, want) {
		t.Errorf("factory saw %+v, want %+v", got, want)
	}
	// Isolate replaces SysProcAttr, Cancel and WaitDelay, which a sandbox
	// wrapper sets on purpose; the supervisor must leave a factory's command
	// as it was built.
	if built.SysProcAttr != nil || built.WaitDelay != 0 {
		t.Errorf("supervisor modified the factory's command: SysProcAttr=%v WaitDelay=%v", built.SysProcAttr, built.WaitDelay)
	}
}

func TestBashHandler_CommandFactoryErrorFailsTheCall(t *testing.T) {
	sb := testSandbox(t, t.TempDir())
	sb.SetCommandFactory(func(context.Context, string, string, ...string) (*exec.Cmd, error) {
		return nil, errors.New("sandbox unavailable")
	})
	sup := testSupervisor(t)

	_, err := bashHandler(sb, sup, nil, BashInput{Command: "echo hi"})
	if err == nil || !strings.Contains(err.Error(), "sandbox unavailable") {
		t.Fatalf("error = %v, want the factory's error", err)
	}
	if handles := sup.Handles(); len(handles) != 0 {
		t.Errorf("a command that never started left handles behind: %v", handles)
	}
}

func TestGitTools_RunThroughCommandFactory(t *testing.T) {
	sb := testSandbox(t, initGitRepo(t))
	factory, calls := recordingFactory(nil)
	sb.SetCommandFactory(factory)

	if _, err := gitOverviewHandler(sb, nil, GitOverviewInput{}); err != nil {
		t.Fatalf("gitOverviewHandler: %v", err)
	}

	got := calls()
	if len(got) == 0 {
		t.Fatal("git ran without going through the factory")
	}
	for _, c := range got {
		if c.name != "git" || c.dir != sb.Dir() {
			t.Errorf("factory saw %+v, want git in %s", c, sb.Dir())
		}
	}
}

func TestGitTools_FactoryFailureSurfaces(t *testing.T) {
	sb := testSandbox(t, initGitRepo(t))
	sb.SetCommandFactory(func(context.Context, string, string, ...string) (*exec.Cmd, error) {
		return nil, errors.New("sandbox unavailable")
	})

	_, err := gitOverviewHandler(sb, nil, GitOverviewInput{})
	if err == nil || !strings.Contains(err.Error(), "sandbox unavailable") {
		t.Errorf("error = %v, want the factory's error", err)
	}
}

func TestUseRipgrep_CommandFactoryKeepsRipgrepOffTheHost(t *testing.T) {
	old := rgAvailable
	rgAvailable = true
	t.Cleanup(func() { rgAvailable = old })

	sb := testSandbox(t, t.TempDir())
	if !useRipgrep(sb) {
		t.Fatal("ripgrep unused although it is available and no factory is set")
	}

	factory, _ := recordingFactory(nil)
	sb.SetCommandFactory(factory)
	if useRipgrep(sb) {
		t.Error("ripgrep would run on the host despite a command factory")
	}

	tool, err := newGrepTool(sb)
	if err != nil {
		t.Fatalf("newGrepTool: %v", err)
	}
	if tool.Name() != "grep" {
		t.Errorf("tool name = %q, want the built-in %q search", tool.Name(), "grep")
	}

	sb.SetCommandFactory(nil)
	if !useRipgrep(sb) {
		t.Error("clearing the factory did not restore ripgrep")
	}
}
