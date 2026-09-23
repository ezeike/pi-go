package blockbuilder

import (
	"testing"

	"github.com/machina/block-builder/sdk/sdktest"
)

// Task 8 (impl-specs/toolset.md): BlockBuilderToolset assembly. Tools() must construct all 11
// tools (7 plain-unary — this component's tools_spec.go/tools_run.go — plus the 4
// long-running/buffered-poll tools the streaming component builds), and propose_spec_change /
// watch_run specifically must report IsLongRunning() == true while every other tool reports
// false (tool.Tool exposes IsLongRunning() directly — confirmed against the vendored
// google.golang.org/adk/v2@v2.4.0/tool/tool.go, see tool_test.go's note on the same point).

func TestBlockBuilderToolset_Name(t *testing.T) {
	ts := NewBlockBuilderToolset(&sdktest.FakeSpecClient{}, &sdktest.FakeRunClient{})
	if got := ts.Name(); got != "block-builder" {
		t.Fatalf("Name() = %q, want %q", got, "block-builder")
	}
}

func TestBlockBuilderToolset_Tools_ExactSetOfElevenNames(t *testing.T) {
	ts := NewBlockBuilderToolset(&sdktest.FakeSpecClient{}, &sdktest.FakeRunClient{})
	tools, err := ts.Tools(nil)
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}

	want := map[string]bool{
		"propose_spec_change": true,
		"get_spec_progress":   true,
		"accept_spec":         true,
		"validate_spec":       true,
		"get_spec":            true,
		"list_primitives":     true,
		"start_run":           true,
		"watch_run":           true,
		"get_run_progress":    true,
		"get_run":             true,
		"get_manifest":        true,
	}
	if len(tools) != len(want) {
		t.Fatalf("len(tools) = %d, want %d", len(tools), len(want))
	}
	got := make(map[string]bool, len(tools))
	for _, tl := range tools {
		got[tl.Name()] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("missing tool %q", name)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("unexpected tool %q", name)
		}
	}
}

func TestBlockBuilderToolset_Tools_LongRunningFlags(t *testing.T) {
	ts := NewBlockBuilderToolset(&sdktest.FakeSpecClient{}, &sdktest.FakeRunClient{})
	tools, err := ts.Tools(nil)
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}

	wantLongRunning := map[string]bool{
		"propose_spec_change": true,
		"watch_run":           true,
	}
	for _, tl := range tools {
		if got, want := tl.IsLongRunning(), wantLongRunning[tl.Name()]; got != want {
			t.Errorf("%s.IsLongRunning() = %v, want %v", tl.Name(), got, want)
		}
	}
}
