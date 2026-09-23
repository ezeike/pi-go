// Package blockbuilder wires block-builder's SDK (github.com/machina/block-builder/sdk) into
// pi-go as in-process ADK tools — see docs/superpowers/specs/2026-09-23-block-builder-sdk-tools-design.md.
package blockbuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/machina/block-builder/sdk"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	defaultProgressBufferEntries = 500
	registryCapacity             = 64
	progressIdleTTL              = 10 * time.Minute
	progressReapInterval         = 1 * time.Minute
)

// progressEntry is one already-marshaled message retained in a progressBuffer, addressed by
// absolute index so a caller asking "everything after index N" can be told honestly that
// earlier entries were discarded rather than silently skipped forward.
type progressEntry struct {
	Index int64
	Data  json.RawMessage
}

// progressBuffer is the retained tail of one server-stream's messages, keyed externally (by
// conversation_id / run_id) in a progressRegistry. Mirrors internal/tools/bash_stream.go's
// stream type, entry-addressed instead of byte-addressed.
type progressBuffer struct {
	mu    sync.Mutex
	limit int

	entries []progressEntry // retained tail, at most limit entries
	base    int64           // absolute index of entries[0]
	next    int64           // absolute index the next appended entry will get

	done      bool
	doneAt    time.Time
	streamErr error

	// notify is closed and replaced on every append or finish, so any number of
	// readers can wait for "something changed" without a per-reader channel.
	notify chan struct{}
}

func newProgressBuffer(limit int) *progressBuffer {
	if limit <= 0 {
		limit = defaultProgressBufferEntries
	}
	return &progressBuffer{limit: limit, notify: make(chan struct{})}
}

// append records one more message. No-op after finish has been called (a drain loop that races
// a late message against its own finish call — should not happen given how drainStream is
// written, but append is defensively idempotent-safe here rather than trusting the caller).
func (b *progressBuffer) append(data json.RawMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done {
		return
	}
	b.entries = append(b.entries, progressEntry{Index: b.next, Data: data})
	b.next++
	if over := len(b.entries) - b.limit; over > 0 {
		b.entries = b.entries[over:]
		b.base += int64(over)
	}
	b.wake()
}

// finish marks the stream ended — cleanly if err is nil, or in-error otherwise. Idempotent:
// only the first call has effect, matching "the terminal message" being observed exactly once.
func (b *progressBuffer) finish(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done {
		return
	}
	b.done = true
	b.doneAt = time.Now()
	b.streamErr = err
	b.wake()
}

// wake closes and replaces notify under the caller's held lock — see bash_stream.go's stream.Write
// for the same pattern and its rationale.
func (b *progressBuffer) wake() {
	close(b.notify)
	b.notify = make(chan struct{})
}

// since returns retained entries at or after off, the offset to pass next time, how many
// entries between off and the returned data were dropped because they aged out of the buffer,
// and the terminal state (done, streamErr) as of this call.
func (b *progressBuffer) since(off int64) (entries []progressEntry, next int64, dropped int64, done bool, streamErr error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if off < b.base {
		dropped = b.base - off
		off = b.base
	}
	if off >= b.next {
		return nil, b.next, dropped, b.done, b.streamErr
	}
	out := make([]progressEntry, len(b.entries[off-b.base:]))
	copy(out, b.entries[off-b.base:])
	return out, b.next, dropped, b.done, b.streamErr
}

// isDone reports whether finish has been called, and when — used by progressRegistry's idle
// reaper to age out completed buffers.
func (b *progressBuffer) isDone() (done bool, doneAt time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.done, b.doneAt
}

// wait returns a channel closed on the next append or finish — see bash_stream.go's stream.wait
// for why taking it under the same lock that publishes writes matters (avoids a missed-wakeup
// race between a reader's check and its wait).
func (b *progressBuffer) wait() <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.notify
}

// progressRegistry tracks one progressBuffer per active/recently-finished stream, keyed by
// conversation_id or run_id. Bounded per registryCapacity, self-cleaning via a background
// idle reaper bound to the ctx passed at construction.
type progressRegistry struct {
	mu       sync.Mutex
	buffers  map[string]*progressBuffer
	order    []string // insertion order, oldest first — evictLocked's tie-breaker
	capacity int
	idleTTL  time.Duration
}

func newProgressRegistry(ctx context.Context) *progressRegistry {
	r := &progressRegistry{
		buffers:  make(map[string]*progressBuffer),
		capacity: registryCapacity,
		idleTTL:  progressIdleTTL,
	}
	go r.reapLoop(ctx)
	return r
}

// getOrStart returns the buffer for key, creating and starting one via run if none exists yet.
// started reports whether THIS call created it — the caller must invoke run only through
// getOrStart, never directly, so exactly one drain goroutine ever exists per key.
func (r *progressRegistry) getOrStart(key string, run func(buf *progressBuffer)) (buf *progressBuffer, started bool) {
	buf, started = r.reserve(key)
	if !started {
		return buf, false
	}
	go run(buf)
	return buf, true
}

// reserve atomically registers key if absent and reports whether THIS call did so. Unlike
// getOrStart, it does not start a drain goroutine — used when opening the underlying RPC stream
// must happen (and can fail) *between* the reservation and the drain starting, e.g. because the
// stream itself is the thing that has to be opened to know what to drain. A caller that reserves
// must either start draining the returned buffer or call its finish(err) — an RPC-open failure
// after a successful reserve still needs the buffer marked done, or a poller waits forever on an
// empty, never-finished buffer.
func (r *progressRegistry) reserve(key string) (buf *progressBuffer, started bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.buffers[key]; ok {
		return existing, false
	}
	buf = newProgressBuffer(defaultProgressBufferEntries)
	r.buffers[key] = buf
	r.order = append(r.order, key)
	r.evictLocked()
	return buf, true
}

func (r *progressRegistry) get(key string) (buf *progressBuffer, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	buf, ok = r.buffers[key]
	return buf, ok
}

// evictLocked drops entries once len(buffers) exceeds capacity: completed buffers first (oldest
// doneAt first), then — only if still over capacity — the oldest running buffer by insertion
// order. Caller must hold r.mu.
func (r *progressRegistry) evictLocked() {
	for len(r.buffers) > r.capacity {
		victim, ok := r.oldestCompletedLocked()
		if !ok {
			// Nothing completed left to evict; fall back to the oldest running one.
			if len(r.order) == 0 {
				return
			}
			victim = r.order[0]
		}
		delete(r.buffers, victim)
		r.removeFromOrderLocked(victim)
	}
}

func (r *progressRegistry) oldestCompletedLocked() (key string, ok bool) {
	var oldestAt time.Time
	found := false
	for _, k := range r.order {
		b, exists := r.buffers[k]
		if !exists {
			continue
		}
		done, doneAt := b.isDone()
		if !done {
			continue
		}
		if !found || doneAt.Before(oldestAt) {
			key, oldestAt, found = k, doneAt, true
		}
	}
	return key, found
}

func (r *progressRegistry) removeFromOrderLocked(key string) {
	for i, k := range r.order {
		if k == key {
			r.order = append(r.order[:i], r.order[i+1:]...)
			return
		}
	}
}

// reapIdle removes every completed buffer whose doneAt is older than idleTTL relative to now.
func (r *progressRegistry) reapIdle(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, k := range append([]string(nil), r.order...) {
		b, ok := r.buffers[k]
		if !ok {
			continue
		}
		done, doneAt := b.isDone()
		if done && now.Sub(doneAt) > r.idleTTL {
			delete(r.buffers, k)
			r.removeFromOrderLocked(k)
		}
	}
}

func (r *progressRegistry) reapLoop(ctx context.Context) {
	t := time.NewTicker(progressReapInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			r.reapIdle(now)
		}
	}
}

// drainStream copies every message from stream into buf, marshaling each via protojson (with
// proto field names, to match this component's json:"plugin_id"-style snake_case convention).
// stream.Close() always runs before buf.finish() (not deferred): finish() is what unblocks a
// waiter polling isDone()/wait(), so Close() must happen-before it — otherwise a waiter can
// observe done==true while Close() is still executing on this goroutine, racing FakeStream's
// internal state (caught by -race in TestWatchRunTool_ReturnsImmediately).
func drainStream[T any, PT interface {
	*T
	proto.Message
}](stream sdk.Stream[T], buf *progressBuffer) {
	var streamErr error
	for stream.Receive() {
		data, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(PT(stream.Msg()))
		if err != nil {
			streamErr = fmt.Errorf("marshal stream message: %w", err)
			break
		}
		buf.append(data)
	}
	if streamErr == nil {
		streamErr = stream.Err()
	}
	if closeErr := stream.Close(); closeErr != nil && streamErr == nil {
		streamErr = fmt.Errorf("close stream: %w", closeErr)
	}
	buf.finish(streamErr)
}
