package blockbuilder

import (
	"errors"
	"testing"
)

// TestNewClient_NotConfigured pins decision D8: an empty BLOCK_BUILDER_URL
// means "absent from the tool registry" (ErrNotConfigured), never a hard
// error — the sibling toolset component's wiring checks this via errors.Is
// to decide whether to register any block-builder tools at all this run.
func TestNewClient_NotConfigured(t *testing.T) {
	t.Setenv("BLOCK_BUILDER_URL", "")
	t.Setenv("BLOCK_BUILDER_API_KEY", "")

	spec, run, err := NewClient()

	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
	if spec != nil || run != nil {
		t.Fatalf("expected nil clients when not configured, got spec=%v run=%v", spec, run)
	}
}

// TestNewClient_InvalidURL pins the "malformed-but-present URL is a hard
// error, not a silent skip" design decision — distinct from
// ErrNotConfigured so a misconfiguration is visible instead of silently
// disabling the tools.
func TestNewClient_InvalidURL(t *testing.T) {
	t.Setenv("BLOCK_BUILDER_URL", "not-a-url")
	t.Setenv("BLOCK_BUILDER_API_KEY", "")

	_, _, err := NewClient()

	if err == nil {
		t.Fatal("expected an error for an invalid BLOCK_BUILDER_URL, got nil")
	}
	if errors.Is(err, ErrNotConfigured) {
		t.Fatal("an invalid-but-present URL must not report ErrNotConfigured")
	}
}

// TestNewClient_ConfiguredReturnsNonNilClients pins "no network I/O at
// construction": sdk.New / the generated *v1connect client constructors
// dial lazily per RPC, so this assertion needs no live server and performs
// no network I/O — a regression that made NewClient dial eagerly would hang
// or fail this test against bb.example.com.
func TestNewClient_ConfiguredReturnsNonNilClients(t *testing.T) {
	t.Setenv("BLOCK_BUILDER_URL", "https://bb.example.com")
	t.Setenv("BLOCK_BUILDER_API_KEY", "secret")

	spec, run, err := NewClient()

	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if spec == nil {
		t.Fatal("expected non-nil SpecClient")
	}
	if run == nil {
		t.Fatal("expected non-nil RunClient")
	}
}
