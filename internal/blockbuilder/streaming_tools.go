package blockbuilder

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/machina/block-builder/sdk"
	pluginv1 "github.com/machina/block-builder/sdk/gen/block-builder/plugin/v1"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
)

type ProposeSpecChangeArgs struct {
	PluginID             string `json:"plugin_id" jsonschema:"the plugin whose spec is being drafted"`
	ConversationID       string `json:"conversation_id,omitempty" jsonschema:"existing conversation to continue; omit or empty to start a new one"`
	Message              string `json:"message" jsonschema:"natural-language request describing the desired spec change"`
	SpecProposalsEnabled bool   `json:"spec_proposals_enabled" jsonschema:"true to allow the model to propose spec YAML this turn; false keeps the turn to free-form discussion"`
}

// ProposeSpecChangeResult.ConversationID is the caller-supplied value if ConversationID was
// non-empty on the request (continuing an existing conversation); otherwise a locally-generated
// placeholder key used to poll get_spec_progress. The real server-assigned conversation_id for a
// brand-new conversation appears inside a later buffered event's own payload instead (see
// streaming.md Task 4 decision 1 — the Partial event carries no conversation_id at all).
type ProposeSpecChangeResult struct {
	ConversationID string `json:"conversation_id" jsonschema:"a handle for polling get_spec_progress with this turn. For a NEW conversation (no conversation_id was given) this is a locally generated polling handle, not the server-assigned conversation id — the real one appears inside a later buffered event's own payload; do not assume this value round-trips through accept_spec or another service call, only through get_spec_progress."`
}

type ProgressPollArgs struct {
	ConversationID string `json:"conversation_id" jsonschema:"the conversation_id returned by propose_spec_change"`
	Offset         int64  `json:"offset,omitempty" jsonschema:"absolute index to read from; 0 or omitted reads from the start of the retained buffer"`
}

// Events is typed []any, not []json.RawMessage: functiontool's automatic OutputSchema
// inference (google.golang.org/jsonschema-go, no InputSchema/OutputSchema override in
// newTool/newLongRunningTool) reflects json.RawMessage as a plain byte slice and infers an
// "array of int, or null" item schema, which then fails validation against the actual decoded
// JSON object each entry marshals to. []any infers an unconstrained item schema instead, which
// is what "one already-decoded JSON value of any shape" actually needs — confirmed by running
// the stage-9 tests against the live vendored adk/jsonschema-go versions.
type ProgressPollResult struct {
	Events     []any  `json:"events" jsonschema:"buffered event messages (protojson, proto field names) since offset, in order"`
	NextOffset int64  `json:"next_offset" jsonschema:"pass as offset on the next poll"`
	Dropped    int64  `json:"dropped,omitempty" jsonschema:"count of entries aged out of the buffer before offset — the caller missed these"`
	Done       bool   `json:"done" jsonschema:"true once the stream has ended (cleanly or in error)"`
	Error      string `json:"error,omitempty" jsonschema:"set iff done and the stream ended in error, or the key is unknown"`
}

// NewProposeSpecChangeTool starts (or continues) a propose_spec_change conversation. The RPC
// stream is opened with a context detached from this call's own (context.WithoutCancel) since it
// must keep draining into the registry after Run returns.
func NewProposeSpecChangeTool(spec sdk.SpecClient, reg *progressRegistry) (tool.Tool, error) {
	handler := func(ctx agent.Context, args ProposeSpecChangeArgs) (ProposeSpecChangeResult, error) {
		key := args.ConversationID
		if key == "" {
			key = uuid.NewString()
		}
		buf, started := reg.reserve(key)
		if !started {
			// Already draining (or drained) for this conversation_id — this call is a
			// re-invocation of the same turn, not a new one. Do not open a second RPC
			// stream: it would never be drained (a reserved/started key is a no-op),
			// leaking the connection and silently discarding its events.
			return ProposeSpecChangeResult{ConversationID: key}, nil
		}
		stream, err := spec.ProposeSpecChange(context.WithoutCancel(ctx), &pluginv1.ProposeSpecChangeRequest{
			PluginId:             args.PluginID,
			ConversationId:       args.ConversationID,
			Message:              args.Message,
			SpecProposalsEnabled: args.SpecProposalsEnabled,
		})
		if err != nil {
			buf.finish(fmt.Errorf("propose_spec_change: %w", err))
			return ProposeSpecChangeResult{}, fmt.Errorf("propose_spec_change: %w", err)
		}
		go drainStream[pluginv1.ProposeSpecChangeEvent](stream, buf)
		return ProposeSpecChangeResult{ConversationID: key}, nil
	}
	return newLongRunningTool(
		"propose_spec_change",
		"Start or continue a natural-language conversation proposing a change to a plugin's spec. Returns immediately with a conversation_id; poll get_spec_progress with it to observe the model's response as it streams in.",
		handler,
	)
}

// NewGetSpecProgressTool reads back buffered propose_spec_change events for a conversation_id.
func NewGetSpecProgressTool(reg *progressRegistry) (tool.Tool, error) {
	handler := func(_ agent.Context, args ProgressPollArgs) (ProgressPollResult, error) {
		buf, ok := reg.get(args.ConversationID)
		if !ok {
			return ProgressPollResult{
				Done:  true,
				Error: fmt.Sprintf("no such conversation_id %q (never started via propose_spec_change, or its buffer was evicted)", args.ConversationID),
			}, nil
		}
		return pollResult(buf, args.Offset), nil
	}
	return newTool(
		"get_spec_progress",
		"Read buffered propose_spec_change events for a conversation_id since a given offset. Call repeatedly (passing the previous next_offset) until done is true.",
		handler,
	)
}

type WatchRunArgs struct {
	PluginID string `json:"plugin_id" jsonschema:"the plugin the run belongs to"`
	RunID    string `json:"run_id" jsonschema:"the run to watch, from start_run"`
}

type WatchRunResult struct {
	RunID string `json:"run_id"`
}

type RunProgressArgs struct {
	RunID  string `json:"run_id" jsonschema:"the run_id passed to watch_run"`
	Offset int64  `json:"offset,omitempty" jsonschema:"absolute index to read from; 0 or omitted reads from the start of the retained buffer"`
}

// NewWatchRunTool starts watching a run's node-by-node progress. Unlike propose_spec_change,
// run_id is never ambiguous (always supplied, per WatchRunRequest), so the registry key is
// always args.RunID directly.
func NewWatchRunTool(run sdk.RunClient, reg *progressRegistry) (tool.Tool, error) {
	handler := func(ctx agent.Context, args WatchRunArgs) (WatchRunResult, error) {
		buf, started := reg.reserve(args.RunID)
		if !started {
			// Already draining (or drained) for this run_id — see the matching comment in
			// NewProposeSpecChangeTool for why opening a second stream here would leak it.
			return WatchRunResult{RunID: args.RunID}, nil
		}
		stream, err := run.WatchRun(context.WithoutCancel(ctx), &pluginv1.WatchRunRequest{
			PluginId: args.PluginID,
			RunId:    args.RunID,
		})
		if err != nil {
			buf.finish(fmt.Errorf("watch_run: %w", err))
			return WatchRunResult{}, fmt.Errorf("watch_run: %w", err)
		}
		go drainStream[pluginv1.WatchRunResponse](stream, buf)
		return WatchRunResult{RunID: args.RunID}, nil
	}
	return newLongRunningTool(
		"watch_run",
		"Start watching a codegen run's node-by-node progress. Returns immediately with run_id; poll get_run_progress with it to observe events as they stream in.",
		handler,
	)
}

// NewGetRunProgressTool reads back buffered watch_run events for a run_id.
func NewGetRunProgressTool(reg *progressRegistry) (tool.Tool, error) {
	handler := func(_ agent.Context, args RunProgressArgs) (ProgressPollResult, error) {
		buf, ok := reg.get(args.RunID)
		if !ok {
			return ProgressPollResult{
				Done:  true,
				Error: fmt.Sprintf("no such run_id %q (never started via watch_run, or its buffer was evicted)", args.RunID),
			}, nil
		}
		return pollResult(buf, args.Offset), nil
	}
	return newTool(
		"get_run_progress",
		"Read buffered watch_run events for a run_id since a given offset. Call repeatedly (passing the previous next_offset) until done is true.",
		handler,
	)
}

// pollResult reads buf since off and shapes it into the poll tools' shared response type.
func pollResult(buf *progressBuffer, off int64) ProgressPollResult {
	entries, next, dropped, done, streamErr := buf.since(off)
	events := make([]any, len(entries))
	for i, e := range entries {
		events[i] = e.Data
	}
	res := ProgressPollResult{Events: events, NextOffset: next, Dropped: dropped, Done: done}
	if streamErr != nil {
		res.Error = streamErr.Error()
	}
	return res
}
