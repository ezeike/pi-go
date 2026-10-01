package blockbuilder

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/machina/block-builder/sdk"
	pluginv1 "github.com/machina/block-builder/sdk/gen/block-builder/plugin/v1"
	"github.com/machina/block-builder/sdk/sdktest"
)

// ---------------------------------------------------------------------------
// Task 4: propose_spec_change + get_spec_progress tools.
// ---------------------------------------------------------------------------

// TestProposeSpecChangeTool_IsLongRunning pins the design doc's requirement that
// propose_spec_change reports tool.Tool.IsLongRunning() == true (confirmed against the vendored
// google.golang.org/adk/v2@v2.4.0 tool.Tool interface and functiontool.Config.IsLongRunning —
// see the sibling "toolset" component's tool.go / newLongRunningTool helper, which this
// component's constructors are expected to route through rather than calling
// functiontool.New directly).
func TestProposeSpecChangeTool_IsLongRunning(t *testing.T) {
	spec := &sdktest.FakeSpecClient{}
	reg := newProgressRegistry(context.Background())
	tl, err := NewProposeSpecChangeTool(spec, reg)
	if err != nil {
		t.Fatalf("NewProposeSpecChangeTool: %v", err)
	}
	if !tl.IsLongRunning() {
		t.Fatal("IsLongRunning() = false, want true for propose_spec_change")
	}
}

// TestGetSpecProgressTool_IsNotLongRunning: the poll companion is a normal, quickly-returning
// tool — only the stream-starting tool is long-running.
func TestGetSpecProgressTool_IsNotLongRunning(t *testing.T) {
	reg := newProgressRegistry(context.Background())
	tl, err := NewGetSpecProgressTool(reg)
	if err != nil {
		t.Fatalf("NewGetSpecProgressTool: %v", err)
	}
	if tl.IsLongRunning() {
		t.Fatal("IsLongRunning() = true, want false for get_spec_progress")
	}
}

func TestProposeSpecChangeTool_ContinuingConversation_ReturnsGivenID(t *testing.T) {
	fs := &sdktest.FakeStream[pluginv1.ProposeSpecChangeEvent]{
		Msgs: []*pluginv1.ProposeSpecChangeEvent{
			{Event: &pluginv1.ProposeSpecChangeEvent_Partial{Partial: &pluginv1.Partial{ProposedSpecJson: "{}"}}},
			{Event: &pluginv1.ProposeSpecChangeEvent_Proposal{Proposal: &pluginv1.Proposal{ConversationId: "conv-existing"}}},
		},
	}
	spec := &sdktest.FakeSpecClient{
		ProposeSpecChangeFunc: func(ctx context.Context, req *pluginv1.ProposeSpecChangeRequest) (sdk.Stream[pluginv1.ProposeSpecChangeEvent], error) {
			if req.ConversationId != "conv-existing" {
				t.Fatalf("req.ConversationId = %q, want %q", req.ConversationId, "conv-existing")
			}
			return fs, nil
		},
	}
	reg := newProgressRegistry(context.Background())
	tl, err := NewProposeSpecChangeTool(spec, reg)
	if err != nil {
		t.Fatalf("NewProposeSpecChangeTool: %v", err)
	}

	res, err := runTool[ProposeSpecChangeArgs, ProposeSpecChangeResult](t, tl, ProposeSpecChangeArgs{
		PluginID: "p1", ConversationID: "conv-existing", Message: "add a transfer tx",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ConversationID != "conv-existing" {
		t.Fatalf("ConversationID = %q, want %q", res.ConversationID, "conv-existing")
	}

	// The tool must not itself drain — the buffer becomes done asynchronously.
	waitBufDone(t, reg, "conv-existing")
	if !fs.Closed() {
		t.Fatal("stream not Closed() after drain")
	}
}

func TestProposeSpecChangeTool_NewConversation_GeneratesKey(t *testing.T) {
	fs := &sdktest.FakeStream[pluginv1.ProposeSpecChangeEvent]{
		Msgs: []*pluginv1.ProposeSpecChangeEvent{
			{Event: &pluginv1.ProposeSpecChangeEvent_Proposal{Proposal: &pluginv1.Proposal{ConversationId: "conv-server-assigned"}}},
		},
	}
	spec := &sdktest.FakeSpecClient{
		ProposeSpecChangeFunc: func(ctx context.Context, req *pluginv1.ProposeSpecChangeRequest) (sdk.Stream[pluginv1.ProposeSpecChangeEvent], error) {
			if req.ConversationId != "" {
				t.Fatalf("req.ConversationId = %q, want empty (new conversation)", req.ConversationId)
			}
			return fs, nil
		},
	}
	reg := newProgressRegistry(context.Background())
	tl, err := NewProposeSpecChangeTool(spec, reg)
	if err != nil {
		t.Fatalf("NewProposeSpecChangeTool: %v", err)
	}

	res, err := runTool[ProposeSpecChangeArgs, ProposeSpecChangeResult](t, tl, ProposeSpecChangeArgs{
		PluginID: "p1", Message: "start fresh",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ConversationID == "" {
		t.Fatal("ConversationID is empty, want a generated placeholder key")
	}
	waitBufDone(t, reg, res.ConversationID)
}

func TestProposeSpecChangeTool_StartError(t *testing.T) {
	wantErr := errors.New("connect: unavailable")
	spec := &sdktest.FakeSpecClient{
		ProposeSpecChangeFunc: func(ctx context.Context, req *pluginv1.ProposeSpecChangeRequest) (sdk.Stream[pluginv1.ProposeSpecChangeEvent], error) {
			return nil, wantErr
		},
	}
	reg := newProgressRegistry(context.Background())
	tl, err := NewProposeSpecChangeTool(spec, reg)
	if err != nil {
		t.Fatalf("NewProposeSpecChangeTool: %v", err)
	}

	_, err = runTool[ProposeSpecChangeArgs, ProposeSpecChangeResult](t, tl, ProposeSpecChangeArgs{PluginID: "p1", Message: "x"})
	if err == nil {
		t.Fatal("Run: want error, got nil")
	}
}

// TestProposeSpecChangeTool_ExistingKey_DoesNotReopenStream: a second call for a conversation_id
// already registered (draining or drained) must not open a second RPC stream — it would never be
// drained (reserve is a no-op for an existing key), leaking the stream and silently discarding
// that turn's events. Regression test for that bug.
func TestProposeSpecChangeTool_ExistingKey_DoesNotReopenStream(t *testing.T) {
	calls := 0
	fs := &sdktest.FakeStream[pluginv1.ProposeSpecChangeEvent]{
		Msgs: []*pluginv1.ProposeSpecChangeEvent{
			{Event: &pluginv1.ProposeSpecChangeEvent_Proposal{Proposal: &pluginv1.Proposal{ConversationId: "conv-existing"}}},
		},
	}
	spec := &sdktest.FakeSpecClient{
		ProposeSpecChangeFunc: func(ctx context.Context, req *pluginv1.ProposeSpecChangeRequest) (sdk.Stream[pluginv1.ProposeSpecChangeEvent], error) {
			calls++
			return fs, nil
		},
	}
	reg := newProgressRegistry(context.Background())
	tl, err := NewProposeSpecChangeTool(spec, reg)
	if err != nil {
		t.Fatalf("NewProposeSpecChangeTool: %v", err)
	}

	args := ProposeSpecChangeArgs{PluginID: "p1", ConversationID: "conv-existing", Message: "first turn"}
	if _, err := runTool[ProposeSpecChangeArgs, ProposeSpecChangeResult](t, tl, args); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	waitBufDone(t, reg, "conv-existing")

	res, err := runTool[ProposeSpecChangeArgs, ProposeSpecChangeResult](t, tl, args)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if res.ConversationID != "conv-existing" {
		t.Fatalf("ConversationID = %q, want %q", res.ConversationID, "conv-existing")
	}
	if calls != 1 {
		t.Fatalf("ProposeSpecChange called %d times, want 1 — a re-invocation for an already-registered key must not open a second stream", calls)
	}
}

func TestGetSpecProgressTool_IncrementalThenDone(t *testing.T) {
	fs := &sdktest.FakeStream[pluginv1.ProposeSpecChangeEvent]{
		Msgs: []*pluginv1.ProposeSpecChangeEvent{
			{Event: &pluginv1.ProposeSpecChangeEvent_Partial{Partial: &pluginv1.Partial{ProposedSpecJson: "{}"}}},
			{Event: &pluginv1.ProposeSpecChangeEvent_Proposal{Proposal: &pluginv1.Proposal{ConversationId: "conv-1"}}},
		},
	}
	spec := &sdktest.FakeSpecClient{
		ProposeSpecChangeFunc: func(ctx context.Context, req *pluginv1.ProposeSpecChangeRequest) (sdk.Stream[pluginv1.ProposeSpecChangeEvent], error) {
			return fs, nil
		},
	}
	reg := newProgressRegistry(context.Background())
	proposeT, _ := NewProposeSpecChangeTool(spec, reg)
	progressT, _ := NewGetSpecProgressTool(reg)

	if _, err := runTool[ProposeSpecChangeArgs, ProposeSpecChangeResult](t, proposeT, ProposeSpecChangeArgs{
		PluginID: "p1", ConversationID: "conv-1", Message: "x",
	}); err != nil {
		t.Fatalf("propose Run: %v", err)
	}
	waitBufDone(t, reg, "conv-1")

	res, err := runTool[ProgressPollArgs, ProgressPollResult](t, progressT, ProgressPollArgs{ConversationID: "conv-1"})
	if err != nil {
		t.Fatalf("progress Run: %v", err)
	}
	if len(res.Events) != 2 {
		t.Fatalf("len(Events) = %d, want 2", len(res.Events))
	}
	if !res.Done || res.Error != "" {
		t.Fatalf("Done=%v Error=%q, want Done=true Error=\"\"", res.Done, res.Error)
	}
	if res.NextOffset != 2 {
		t.Fatalf("NextOffset = %d, want 2", res.NextOffset)
	}
}

func TestGetSpecProgressTool_StreamError_SurfacesInBand(t *testing.T) {
	fs := &sdktest.FakeStream[pluginv1.ProposeSpecChangeEvent]{FailWith: errors.New("upstream boom")}
	spec := &sdktest.FakeSpecClient{
		ProposeSpecChangeFunc: func(ctx context.Context, req *pluginv1.ProposeSpecChangeRequest) (sdk.Stream[pluginv1.ProposeSpecChangeEvent], error) {
			return fs, nil
		},
	}
	reg := newProgressRegistry(context.Background())
	proposeT, _ := NewProposeSpecChangeTool(spec, reg)
	progressT, _ := NewGetSpecProgressTool(reg)

	if _, err := runTool[ProposeSpecChangeArgs, ProposeSpecChangeResult](t, proposeT, ProposeSpecChangeArgs{
		PluginID: "p1", ConversationID: "conv-err", Message: "x",
	}); err != nil {
		t.Fatalf("propose Run: %v", err)
	}
	waitBufDone(t, reg, "conv-err")

	res, err := runTool[ProgressPollArgs, ProgressPollResult](t, progressT, ProgressPollArgs{ConversationID: "conv-err"})
	if err != nil {
		t.Fatalf("progress Run: %v (want nil error — stream errors surface in-band, not as a Go error)", err)
	}
	if !res.Done || res.Error == "" {
		t.Fatalf("Done=%v Error=%q, want Done=true and a non-empty Error", res.Done, res.Error)
	}
}

func TestGetSpecProgressTool_UnknownKey(t *testing.T) {
	reg := newProgressRegistry(context.Background())
	progressT, _ := NewGetSpecProgressTool(reg)

	res, err := runTool[ProgressPollArgs, ProgressPollResult](t, progressT, ProgressPollArgs{ConversationID: "never-started"})
	if err != nil {
		t.Fatalf("Run: %v, want nil error", err)
	}
	if !res.Done || res.Error == "" {
		t.Fatalf("Done=%v Error=%q, want Done=true and a non-empty explanatory Error", res.Done, res.Error)
	}
}

// waitBufDone polls the registry until key's buffer reports done, or fails the test after 2s.
func waitBufDone(t *testing.T, reg *progressRegistry, key string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if buf, ok := reg.get(key); ok {
			if done, _ := buf.isDone(); done {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("buffer %q never reached done", key)
}

// ---------------------------------------------------------------------------
// Task 5: watch_run + get_run_progress tools.
// ---------------------------------------------------------------------------

// TestWatchRunTool_IsLongRunning: see TestProposeSpecChangeTool_IsLongRunning above — the same
// design-doc requirement applies to watch_run.
func TestWatchRunTool_IsLongRunning(t *testing.T) {
	run := &sdktest.FakeRunClient{}
	reg := newProgressRegistry(context.Background())
	tl, err := NewWatchRunTool(run, reg)
	if err != nil {
		t.Fatalf("NewWatchRunTool: %v", err)
	}
	if !tl.IsLongRunning() {
		t.Fatal("IsLongRunning() = false, want true for watch_run")
	}
}

func TestGetRunProgressTool_IsNotLongRunning(t *testing.T) {
	reg := newProgressRegistry(context.Background())
	tl, err := NewGetRunProgressTool(reg)
	if err != nil {
		t.Fatalf("NewGetRunProgressTool: %v", err)
	}
	if tl.IsLongRunning() {
		t.Fatal("IsLongRunning() = true, want false for get_run_progress")
	}
}

func TestWatchRunTool_ReturnsImmediately(t *testing.T) {
	fs := &sdktest.FakeStream[pluginv1.WatchRunResponse]{
		Msgs: []*pluginv1.WatchRunResponse{
			{RunStatus: pluginv1.RunStatus_RUN_STATUS_RUNNING},
			{RunStatus: pluginv1.RunStatus_RUN_STATUS_SUCCEEDED},
		},
	}
	run := &sdktest.FakeRunClient{
		WatchRunFunc: func(ctx context.Context, req *pluginv1.WatchRunRequest) (sdk.Stream[pluginv1.WatchRunResponse], error) {
			if req.RunId != "run-1" {
				t.Fatalf("req.RunId = %q, want %q", req.RunId, "run-1")
			}
			return fs, nil
		},
	}
	reg := newProgressRegistry(context.Background())
	tl, err := NewWatchRunTool(run, reg)
	if err != nil {
		t.Fatalf("NewWatchRunTool: %v", err)
	}

	res, err := runTool[WatchRunArgs, WatchRunResult](t, tl, WatchRunArgs{PluginID: "p1", RunID: "run-1"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.RunID != "run-1" {
		t.Fatalf("RunID = %q, want %q", res.RunID, "run-1")
	}
	waitBufDone(t, reg, "run-1")
	if !fs.Closed() {
		t.Fatal("stream not Closed() after drain")
	}
}

func TestWatchRunTool_StartError(t *testing.T) {
	wantErr := errors.New("run not found")
	run := &sdktest.FakeRunClient{
		WatchRunFunc: func(ctx context.Context, req *pluginv1.WatchRunRequest) (sdk.Stream[pluginv1.WatchRunResponse], error) {
			return nil, wantErr
		},
	}
	reg := newProgressRegistry(context.Background())
	tl, err := NewWatchRunTool(run, reg)
	if err != nil {
		t.Fatalf("NewWatchRunTool: %v", err)
	}
	if _, err := runTool[WatchRunArgs, WatchRunResult](t, tl, WatchRunArgs{PluginID: "p1", RunID: "run-x"}); err == nil {
		t.Fatal("Run: want error, got nil")
	}
}

// TestWatchRunTool_ExistingKey_DoesNotReopenStream: same regression coverage as
// TestProposeSpecChangeTool_ExistingKey_DoesNotReopenStream, for watch_run's run_id key (always
// caller-supplied, never generated, but the reserve-before-open bug applied identically).
func TestWatchRunTool_ExistingKey_DoesNotReopenStream(t *testing.T) {
	calls := 0
	fs := &sdktest.FakeStream[pluginv1.WatchRunResponse]{
		Msgs: []*pluginv1.WatchRunResponse{{}},
	}
	run := &sdktest.FakeRunClient{
		WatchRunFunc: func(ctx context.Context, req *pluginv1.WatchRunRequest) (sdk.Stream[pluginv1.WatchRunResponse], error) {
			calls++
			return fs, nil
		},
	}
	reg := newProgressRegistry(context.Background())
	tl, err := NewWatchRunTool(run, reg)
	if err != nil {
		t.Fatalf("NewWatchRunTool: %v", err)
	}

	args := WatchRunArgs{PluginID: "p1", RunID: "run-existing"}
	if _, err := runTool[WatchRunArgs, WatchRunResult](t, tl, args); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	waitBufDone(t, reg, "run-existing")

	res, err := runTool[WatchRunArgs, WatchRunResult](t, tl, args)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if res.RunID != "run-existing" {
		t.Fatalf("RunID = %q, want %q", res.RunID, "run-existing")
	}
	if calls != 1 {
		t.Fatalf("WatchRun called %d times, want 1 — a re-invocation for an already-registered key must not open a second stream", calls)
	}
}

func TestGetRunProgressTool_IncrementalThenDone(t *testing.T) {
	fs := &sdktest.FakeStream[pluginv1.WatchRunResponse]{
		Msgs: []*pluginv1.WatchRunResponse{
			{RunStatus: pluginv1.RunStatus_RUN_STATUS_RUNNING},
			{RunStatus: pluginv1.RunStatus_RUN_STATUS_SUCCEEDED},
		},
	}
	run := &sdktest.FakeRunClient{
		WatchRunFunc: func(ctx context.Context, req *pluginv1.WatchRunRequest) (sdk.Stream[pluginv1.WatchRunResponse], error) {
			return fs, nil
		},
	}
	reg := newProgressRegistry(context.Background())
	watchT, _ := NewWatchRunTool(run, reg)
	progressT, _ := NewGetRunProgressTool(reg)

	if _, err := runTool[WatchRunArgs, WatchRunResult](t, watchT, WatchRunArgs{PluginID: "p1", RunID: "run-2"}); err != nil {
		t.Fatalf("watch Run: %v", err)
	}
	waitBufDone(t, reg, "run-2")

	res, err := runTool[RunProgressArgs, ProgressPollResult](t, progressT, RunProgressArgs{RunID: "run-2"})
	if err != nil {
		t.Fatalf("progress Run: %v", err)
	}
	if len(res.Events) != 2 || !res.Done || res.Error != "" {
		t.Fatalf("Events=%d Done=%v Error=%q, want 2/true/\"\"", len(res.Events), res.Done, res.Error)
	}
}

func TestGetRunProgressTool_StreamError_SurfacesInBand(t *testing.T) {
	fs := &sdktest.FakeStream[pluginv1.WatchRunResponse]{FailWith: errors.New("run crashed")}
	run := &sdktest.FakeRunClient{
		WatchRunFunc: func(ctx context.Context, req *pluginv1.WatchRunRequest) (sdk.Stream[pluginv1.WatchRunResponse], error) {
			return fs, nil
		},
	}
	reg := newProgressRegistry(context.Background())
	watchT, _ := NewWatchRunTool(run, reg)
	progressT, _ := NewGetRunProgressTool(reg)

	if _, err := runTool[WatchRunArgs, WatchRunResult](t, watchT, WatchRunArgs{PluginID: "p1", RunID: "run-err"}); err != nil {
		t.Fatalf("watch Run: %v", err)
	}
	waitBufDone(t, reg, "run-err")

	res, err := runTool[RunProgressArgs, ProgressPollResult](t, progressT, RunProgressArgs{RunID: "run-err"})
	if err != nil {
		t.Fatalf("progress Run: %v, want nil", err)
	}
	if !res.Done || res.Error == "" {
		t.Fatalf("Done=%v Error=%q, want Done=true and non-empty Error", res.Done, res.Error)
	}
}

// ---------------------------------------------------------------------------
// send_message + get_chat_progress tools — same long-running/buffered-poll
// shape as propose_spec_change/get_spec_progress, exercised the same way.
// ---------------------------------------------------------------------------

func TestSendMessageTool_IsLongRunning(t *testing.T) {
	chat := &sdktest.FakeChatClient{}
	reg := newProgressRegistry(context.Background())
	tl, err := NewSendMessageTool(chat, reg)
	if err != nil {
		t.Fatalf("NewSendMessageTool: %v", err)
	}
	if !tl.IsLongRunning() {
		t.Fatal("IsLongRunning() = false, want true for send_message")
	}
}

func TestGetChatProgressTool_IsNotLongRunning(t *testing.T) {
	reg := newProgressRegistry(context.Background())
	tl, err := NewGetChatProgressTool(reg)
	if err != nil {
		t.Fatalf("NewGetChatProgressTool: %v", err)
	}
	if tl.IsLongRunning() {
		t.Fatal("IsLongRunning() = true, want false for get_chat_progress")
	}
}

func TestSendMessageTool_ContinuingConversation_ReturnsGivenID(t *testing.T) {
	fs := &sdktest.FakeStream[pluginv1.ChatEvent]{
		Msgs: []*pluginv1.ChatEvent{
			{Event: &pluginv1.ChatEvent_Delta{Delta: &pluginv1.TextDelta{Text: "hi"}}},
			{Event: &pluginv1.ChatEvent_Done{Done: &pluginv1.Done{ConversationId: "conv-existing"}}},
		},
	}
	chat := &sdktest.FakeChatClient{
		SendMessageFunc: func(ctx context.Context, req *pluginv1.SendMessageRequest) (sdk.Stream[pluginv1.ChatEvent], error) {
			if req.ConversationId != "conv-existing" {
				t.Fatalf("req.ConversationId = %q, want %q", req.ConversationId, "conv-existing")
			}
			return fs, nil
		},
	}
	reg := newProgressRegistry(context.Background())
	tl, err := NewSendMessageTool(chat, reg)
	if err != nil {
		t.Fatalf("NewSendMessageTool: %v", err)
	}

	res, err := runTool[SendMessageArgs, SendMessageResult](t, tl, SendMessageArgs{
		PluginID: "p1", ConversationID: "conv-existing", Message: "hello",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ConversationID != "conv-existing" {
		t.Fatalf("ConversationID = %q, want %q", res.ConversationID, "conv-existing")
	}

	waitBufDone(t, reg, "conv-existing")
	if !fs.Closed() {
		t.Fatal("stream not Closed() after drain")
	}
}

func TestSendMessageTool_NewConversation_GeneratesKey(t *testing.T) {
	fs := &sdktest.FakeStream[pluginv1.ChatEvent]{
		Msgs: []*pluginv1.ChatEvent{
			{Event: &pluginv1.ChatEvent_Done{Done: &pluginv1.Done{ConversationId: "conv-server-assigned"}}},
		},
	}
	chat := &sdktest.FakeChatClient{
		SendMessageFunc: func(ctx context.Context, req *pluginv1.SendMessageRequest) (sdk.Stream[pluginv1.ChatEvent], error) {
			if req.ConversationId != "" {
				t.Fatalf("req.ConversationId = %q, want empty (new conversation)", req.ConversationId)
			}
			return fs, nil
		},
	}
	reg := newProgressRegistry(context.Background())
	tl, err := NewSendMessageTool(chat, reg)
	if err != nil {
		t.Fatalf("NewSendMessageTool: %v", err)
	}

	res, err := runTool[SendMessageArgs, SendMessageResult](t, tl, SendMessageArgs{
		PluginID: "p1", Message: "start fresh",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ConversationID == "" {
		t.Fatal("ConversationID is empty, want a generated placeholder key")
	}
	waitBufDone(t, reg, res.ConversationID)
}

func TestSendMessageTool_StartError(t *testing.T) {
	wantErr := errors.New("connect: unavailable")
	chat := &sdktest.FakeChatClient{
		SendMessageFunc: func(ctx context.Context, req *pluginv1.SendMessageRequest) (sdk.Stream[pluginv1.ChatEvent], error) {
			return nil, wantErr
		},
	}
	reg := newProgressRegistry(context.Background())
	tl, err := NewSendMessageTool(chat, reg)
	if err != nil {
		t.Fatalf("NewSendMessageTool: %v", err)
	}

	_, err = runTool[SendMessageArgs, SendMessageResult](t, tl, SendMessageArgs{PluginID: "p1", Message: "x"})
	if err == nil {
		t.Fatal("Run: want error, got nil")
	}
}

// TestSendMessageTool_ExistingKey_DoesNotReopenStream: same regression coverage as
// TestProposeSpecChangeTool_ExistingKey_DoesNotReopenStream, for send_message.
func TestSendMessageTool_ExistingKey_DoesNotReopenStream(t *testing.T) {
	calls := 0
	fs := &sdktest.FakeStream[pluginv1.ChatEvent]{
		Msgs: []*pluginv1.ChatEvent{
			{Event: &pluginv1.ChatEvent_Done{Done: &pluginv1.Done{ConversationId: "conv-existing"}}},
		},
	}
	chat := &sdktest.FakeChatClient{
		SendMessageFunc: func(ctx context.Context, req *pluginv1.SendMessageRequest) (sdk.Stream[pluginv1.ChatEvent], error) {
			calls++
			return fs, nil
		},
	}
	reg := newProgressRegistry(context.Background())
	tl, err := NewSendMessageTool(chat, reg)
	if err != nil {
		t.Fatalf("NewSendMessageTool: %v", err)
	}

	args := SendMessageArgs{PluginID: "p1", ConversationID: "conv-existing", Message: "first turn"}
	if _, err := runTool[SendMessageArgs, SendMessageResult](t, tl, args); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	waitBufDone(t, reg, "conv-existing")

	res, err := runTool[SendMessageArgs, SendMessageResult](t, tl, args)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if res.ConversationID != "conv-existing" {
		t.Fatalf("ConversationID = %q, want %q", res.ConversationID, "conv-existing")
	}
	if calls != 1 {
		t.Fatalf("SendMessage called %d times, want 1 — a re-invocation for an already-registered key must not open a second stream", calls)
	}
}

func TestGetChatProgressTool_IncrementalThenDone(t *testing.T) {
	fs := &sdktest.FakeStream[pluginv1.ChatEvent]{
		Msgs: []*pluginv1.ChatEvent{
			{Event: &pluginv1.ChatEvent_Delta{Delta: &pluginv1.TextDelta{Text: "hi"}}},
			{Event: &pluginv1.ChatEvent_Done{Done: &pluginv1.Done{ConversationId: "conv-1"}}},
		},
	}
	chat := &sdktest.FakeChatClient{
		SendMessageFunc: func(ctx context.Context, req *pluginv1.SendMessageRequest) (sdk.Stream[pluginv1.ChatEvent], error) {
			return fs, nil
		},
	}
	reg := newProgressRegistry(context.Background())
	sendT, _ := NewSendMessageTool(chat, reg)
	progressT, _ := NewGetChatProgressTool(reg)

	if _, err := runTool[SendMessageArgs, SendMessageResult](t, sendT, SendMessageArgs{
		PluginID: "p1", ConversationID: "conv-1", Message: "x",
	}); err != nil {
		t.Fatalf("send Run: %v", err)
	}
	waitBufDone(t, reg, "conv-1")

	res, err := runTool[ChatProgressArgs, ProgressPollResult](t, progressT, ChatProgressArgs{ConversationID: "conv-1"})
	if err != nil {
		t.Fatalf("progress Run: %v", err)
	}
	if len(res.Events) != 2 {
		t.Fatalf("len(Events) = %d, want 2", len(res.Events))
	}
	if !res.Done || res.Error != "" {
		t.Fatalf("Done=%v Error=%q, want Done=true Error=\"\"", res.Done, res.Error)
	}
	if res.NextOffset != 2 {
		t.Fatalf("NextOffset = %d, want 2", res.NextOffset)
	}
}

func TestGetChatProgressTool_StreamError_SurfacesInBand(t *testing.T) {
	fs := &sdktest.FakeStream[pluginv1.ChatEvent]{FailWith: errors.New("upstream boom")}
	chat := &sdktest.FakeChatClient{
		SendMessageFunc: func(ctx context.Context, req *pluginv1.SendMessageRequest) (sdk.Stream[pluginv1.ChatEvent], error) {
			return fs, nil
		},
	}
	reg := newProgressRegistry(context.Background())
	sendT, _ := NewSendMessageTool(chat, reg)
	progressT, _ := NewGetChatProgressTool(reg)

	if _, err := runTool[SendMessageArgs, SendMessageResult](t, sendT, SendMessageArgs{
		PluginID: "p1", ConversationID: "conv-err", Message: "x",
	}); err != nil {
		t.Fatalf("send Run: %v", err)
	}
	waitBufDone(t, reg, "conv-err")

	res, err := runTool[ChatProgressArgs, ProgressPollResult](t, progressT, ChatProgressArgs{ConversationID: "conv-err"})
	if err != nil {
		t.Fatalf("progress Run: %v (want nil error — stream errors surface in-band, not as a Go error)", err)
	}
	if !res.Done || res.Error == "" {
		t.Fatalf("Done=%v Error=%q, want Done=true and a non-empty Error", res.Done, res.Error)
	}
}

func TestGetChatProgressTool_UnknownKey(t *testing.T) {
	reg := newProgressRegistry(context.Background())
	progressT, _ := NewGetChatProgressTool(reg)

	res, err := runTool[ChatProgressArgs, ProgressPollResult](t, progressT, ChatProgressArgs{ConversationID: "never-started"})
	if err != nil {
		t.Fatalf("Run: %v, want nil error", err)
	}
	if !res.Done || res.Error == "" {
		t.Fatalf("Done=%v Error=%q, want Done=true and a non-empty explanatory Error", res.Done, res.Error)
	}
}
