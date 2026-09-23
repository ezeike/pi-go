package blockbuilder

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pluginv1 "github.com/machina/block-builder/sdk/gen/block-builder/plugin/v1"
	"github.com/machina/block-builder/sdk/sdktest"
	"google.golang.org/protobuf/encoding/protojson"
)

// ---------------------------------------------------------------------------
// Task 1: progressBuffer — bounded, index-addressed per-stream buffer.
// ---------------------------------------------------------------------------

func TestProgressBuffer_AppendAndSince(t *testing.T) {
	b := newProgressBuffer(10)
	b.append(json.RawMessage(`{"a":1}`))
	b.append(json.RawMessage(`{"a":2}`))
	b.append(json.RawMessage(`{"a":3}`))

	entries, next, dropped, done, streamErr := b.since(0)
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}
	if entries[0].Index != 0 || entries[2].Index != 2 {
		t.Fatalf("entries[0].Index=%d entries[2].Index=%d, want 0 and 2", entries[0].Index, entries[2].Index)
	}
	if string(entries[1].Data) != `{"a":2}` {
		t.Fatalf("entries[1].Data = %s, want {\"a\":2}", entries[1].Data)
	}
	if next != 3 {
		t.Fatalf("next = %d, want 3", next)
	}
	if dropped != 0 {
		t.Fatalf("dropped = %d, want 0", dropped)
	}
	if done {
		t.Fatalf("done = true, want false")
	}
	if streamErr != nil {
		t.Fatalf("streamErr = %v, want nil", streamErr)
	}

	// Reading again from next should return nothing new yet.
	entries2, next2, _, _, _ := b.since(next)
	if len(entries2) != 0 {
		t.Fatalf("len(entries2) = %d, want 0", len(entries2))
	}
	if next2 != 3 {
		t.Fatalf("next2 = %d, want 3", next2)
	}
}

func TestProgressBuffer_Eviction(t *testing.T) {
	b := newProgressBuffer(2) // retain only the last 2 entries
	for i := 0; i < 5; i++ {
		b.append(json.RawMessage(`{}`))
	}
	entries, next, dropped, _, _ := b.since(0)
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2 (only the tail retained)", len(entries))
	}
	if entries[0].Index != 3 || entries[1].Index != 4 {
		t.Fatalf("retained indices = %d,%d, want 3,4", entries[0].Index, entries[1].Index)
	}
	if dropped != 3 {
		t.Fatalf("dropped = %d, want 3", dropped)
	}
	if next != 5 {
		t.Fatalf("next = %d, want 5", next)
	}
}

func TestProgressBuffer_FinishClean(t *testing.T) {
	b := newProgressBuffer(10)
	b.append(json.RawMessage(`{}`))
	b.finish(nil)

	_, _, _, done, streamErr := b.since(0)
	if !done {
		t.Fatalf("done = false, want true after finish(nil)")
	}
	if streamErr != nil {
		t.Fatalf("streamErr = %v, want nil", streamErr)
	}
	doneFlag, doneAt := b.isDone()
	if !doneFlag || doneAt.IsZero() {
		t.Fatalf("isDone() = (%v, %v), want (true, non-zero)", doneFlag, doneAt)
	}
}

func TestProgressBuffer_FinishError(t *testing.T) {
	b := newProgressBuffer(10)
	wantErr := errors.New("stream boom")
	b.finish(wantErr)

	_, _, _, done, streamErr := b.since(0)
	if !done {
		t.Fatalf("done = false, want true after finish(err)")
	}
	if streamErr == nil || streamErr.Error() != wantErr.Error() {
		t.Fatalf("streamErr = %v, want %v", streamErr, wantErr)
	}
}

func TestProgressBuffer_Wait(t *testing.T) {
	b := newProgressBuffer(10)
	waitCh := b.wait()

	done := make(chan struct{})
	go func() {
		b.append(json.RawMessage(`{}`))
		close(done)
	}()

	select {
	case <-waitCh:
	case <-time.After(2 * time.Second):
		t.Fatal("wait() channel never closed after append")
	}
	<-done
}

// ---------------------------------------------------------------------------
// Task 2: progressRegistry — keyed lookup, capped with evict-completed-first
// + idle reaper.
// ---------------------------------------------------------------------------

func TestProgressRegistry_GetOrStart_SingleStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := newProgressRegistry(ctx)

	var starts int32
	run := func(buf *progressBuffer) {
		atomic.AddInt32(&starts, 1)
		buf.finish(nil)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.getOrStart("conv-1", run)
		}()
	}
	wg.Wait()

	// wg only guards the 10 getOrStart calls themselves; run executes in a separately spawned
	// goroutine (getOrStart's own "go run(buf)"), so starts must not be read until that
	// goroutine's buf.finish(nil) is observed — otherwise this races run's atomic.AddInt32.
	buf, ok := r.get("conv-1")
	if !ok || buf == nil {
		t.Fatalf("get(\"conv-1\") = (%v, %v), want a buffer", buf, ok)
	}
	waitDone(t, r, "conv-1")

	if got := atomic.LoadInt32(&starts); got != 1 {
		t.Fatalf("run invoked %d times, want exactly 1", got)
	}
}

func TestProgressRegistry_EvictCompletedFirst(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := newProgressRegistry(ctx)
	r.capacity = 2 // override the package default for a deterministic test

	block := make(chan struct{})
	r.getOrStart("running", func(buf *progressBuffer) { <-block }) // never finishes until we close(block)
	r.getOrStart("completed", func(buf *progressBuffer) { buf.finish(nil) })

	// Wait for "completed" to actually reach done before inserting the third key, so eviction
	// ordering is deterministic rather than racing the goroutine above.
	waitDone(t, r, "completed")

	r.getOrStart("newcomer", func(buf *progressBuffer) { <-block })

	if _, ok := r.get("completed"); ok {
		t.Fatal("\"completed\" should have been evicted (evict-completed-first)")
	}
	if _, ok := r.get("running"); !ok {
		t.Fatal("\"running\" should NOT have been evicted (still in flight, capacity satisfied by evicting completed)")
	}
	if _, ok := r.get("newcomer"); !ok {
		t.Fatal("\"newcomer\" should be present")
	}
	close(block)
}

// waitDone polls until buf's isDone() reports true or the test times out — a background
// goroutine's finish() call is inherently async from the calling test's perspective.
func waitDone(t *testing.T, r *progressRegistry, key string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if buf, ok := r.get(key); ok {
			if done, _ := buf.isDone(); done {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("buffer %q never reached done", key)
}

func TestProgressRegistry_ReapIdle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := newProgressRegistry(ctx)
	r.idleTTL = 10 * time.Millisecond

	r.getOrStart("gone-soon", func(buf *progressBuffer) { buf.finish(nil) })
	waitDone(t, r, "gone-soon")

	time.Sleep(50 * time.Millisecond) // past idleTTL
	r.reapIdle(time.Now())

	if _, ok := r.get("gone-soon"); ok {
		t.Fatal("\"gone-soon\" should have been reaped after idleTTL")
	}
}

// ---------------------------------------------------------------------------
// Task 3: drainStream — sdk.Stream[T] into a progressBuffer.
// ---------------------------------------------------------------------------

func TestDrainStream_CleanEnd(t *testing.T) {
	fs := &sdktest.FakeStream[pluginv1.WatchRunResponse]{
		Msgs: []*pluginv1.WatchRunResponse{
			{RunStatus: pluginv1.RunStatus_RUN_STATUS_RUNNING},
			{RunStatus: pluginv1.RunStatus_RUN_STATUS_SUCCEEDED},
		},
	}
	buf := newProgressBuffer(10)
	drainStream[pluginv1.WatchRunResponse](fs, buf)

	entries, _, _, done, streamErr := buf.since(0)
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	var got pluginv1.WatchRunResponse
	if err := protojson.Unmarshal(entries[1].Data, &got); err != nil {
		t.Fatalf("Unmarshal entries[1]: %v", err)
	}
	if got.RunStatus != pluginv1.RunStatus_RUN_STATUS_SUCCEEDED {
		t.Fatalf("entries[1].RunStatus = %v, want SUCCEEDED", got.RunStatus)
	}
	if !done || streamErr != nil {
		t.Fatalf("done=%v streamErr=%v, want done=true streamErr=nil", done, streamErr)
	}
	if !fs.Closed() {
		t.Fatal("stream was not Closed()")
	}
}

func TestDrainStream_StreamError(t *testing.T) {
	wantErr := errors.New("boom")
	fs := &sdktest.FakeStream[pluginv1.WatchRunResponse]{FailWith: wantErr}
	buf := newProgressBuffer(10)
	drainStream[pluginv1.WatchRunResponse](fs, buf)

	_, _, _, done, streamErr := buf.since(0)
	if !done {
		t.Fatal("done = false, want true")
	}
	if streamErr == nil || streamErr.Error() != wantErr.Error() {
		t.Fatalf("streamErr = %v, want %v", streamErr, wantErr)
	}
	if !fs.Closed() {
		t.Fatal("stream was not Closed() even on error")
	}
}
