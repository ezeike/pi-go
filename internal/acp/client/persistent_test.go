package client

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPersistentSessionKeepsOneSessionAcrossPrompts(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	session, err := StartPersistentSession(context.Background(), PersistentRequest{
		Command:    []string{os.Args[0], "-test.run=TestACPClientHelperProcess", "--"},
		CWD:        t.TempDir(),
		RPCTimeout: time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("StartPersistentSession() error: %v", err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("Close() error: %v", err)
		}
	})

	first, err := session.Prompt(context.Background(), "/forge-status")
	if err != nil {
		t.Fatalf("first Prompt() error: %v", err)
	}
	second, err := session.Prompt(context.Background(), "/forge-pause api")
	if err != nil {
		t.Fatalf("second Prompt() error: %v", err)
	}
	if first.SessionID != "session-1" || second.SessionID != first.SessionID {
		t.Errorf("session IDs = %q, %q; want one reused ACP session", first.SessionID, second.SessionID)
	}
	if !strings.Contains(first.Result, "echo: /forge-status") {
		t.Errorf("first result = %q", first.Result)
	}
	if !strings.Contains(second.Result, "echo: /forge-pause api") {
		t.Errorf("second result = %q", second.Result)
	}
}
