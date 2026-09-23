package blockbuilder

import (
	"testing"

	"google.golang.org/adk/v2/agent"
)

// ---------------------------------------------------------------------------
// Task 4 (impl-specs/toolset.md): newTool / newLongRunningTool — the local
// functiontool-wrapping helper this package uses in place of
// internal/tools' unexported newTool (that package's helper is unreachable
// from here: Go visibility, sibling packages — see toolset.md correction 5).
// ---------------------------------------------------------------------------

type trivialArgs struct {
	Name string `json:"name"`
}

type trivialResult struct {
	Greeting string `json:"greeting"`
}

func trivialHandler(_ agent.Context, in trivialArgs) (trivialResult, error) {
	return trivialResult{Greeting: "hello " + in.Name}, nil
}

func TestNewTool_DeclarationRoundTripsNameAndDescription(t *testing.T) {
	// tool.Tool (google.golang.org/adk/v2@v2.4.0/tool/tool.go) exposes Name()/Description()
	// directly — there is no Declaration() method/type on the interface (confirmed against the
	// vendored source; this test originally guessed at a Declaration() accessor that does not
	// exist in this adk version).
	tl, err := newTool("greet", "says hello", trivialHandler)
	if err != nil {
		t.Fatalf("newTool: %v", err)
	}
	if tl.Name() != "greet" {
		t.Fatalf("Name() = %q, want %q", tl.Name(), "greet")
	}
	if tl.Description() != "says hello" {
		t.Fatalf("Description() = %q, want %q", tl.Description(), "says hello")
	}
}

func TestNewTool_IsNotLongRunning(t *testing.T) {
	tl, err := newTool("greet", "says hello", trivialHandler)
	if err != nil {
		t.Fatalf("newTool: %v", err)
	}
	lr, ok := tl.(interface{ IsLongRunning() bool })
	if !ok {
		t.Fatalf("tool %T does not expose IsLongRunning()", tl)
	}
	if lr.IsLongRunning() {
		t.Fatal("IsLongRunning() = true, want false for a plain tool built via newTool")
	}
}

func TestNewLongRunningTool_IsLongRunning(t *testing.T) {
	tl, err := newLongRunningTool("greet_async", "says hello, eventually", trivialHandler)
	if err != nil {
		t.Fatalf("newLongRunningTool: %v", err)
	}
	lr, ok := tl.(interface{ IsLongRunning() bool })
	if !ok {
		t.Fatalf("tool %T does not expose IsLongRunning()", tl)
	}
	if !lr.IsLongRunning() {
		t.Fatal("IsLongRunning() = false, want true for a tool built via newLongRunningTool")
	}
}

func TestNewTool_HandlerInvokedThroughRun(t *testing.T) {
	tl, err := newTool("greet", "says hello", trivialHandler)
	if err != nil {
		t.Fatalf("newTool: %v", err)
	}
	got, callErr := runTool[trivialArgs, trivialResult](t, tl, trivialArgs{Name: "world"})
	if callErr != nil {
		t.Fatalf("runTool: %v", callErr)
	}
	if got.Greeting != "hello world" {
		t.Fatalf("Greeting = %q, want %q", got.Greeting, "hello world")
	}
}
