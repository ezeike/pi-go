# block-builder SDK tools — design

## Context

block-builder (`~/src/machina/block-builder`) recently extracted its
Connect-RPC client port into a standalone Go submodule,
`github.com/machina/block-builder/sdk`, specifically so a second consumer
outside that repo could depend on it without pulling in the server's full
dependency graph. `sdk.Adapter` implements `SpecClient`/`RunClient`/
`ProjectClient`/`ConversationClient`/`UserClient`/`AssetClient`/
`DeployClient`/`ChatClient` against block-builder's plugin-generation +
product API. block-builder-mcp already consumes it as an MCP tool server
(`internal/mcpserver/tools/`); this design makes pi-go the second consumer,
calling the same API as in-process ADK tool handlers — no subprocess, no
MCP hop.

pi-go has no existing integration with block-builder today (zero
references anywhere in the repo). This is new subsystem work: a new
package, a new toolset registered into `internal/tools`' `CoreTools`, new
config surface, and — because two of the RPCs are server-streaming and
ADK's `tool.Tool.Run` is a single synchronous call with no push/progress
primitive — a new "long-running tool + buffered reader" mechanism that
has exactly one existing precedent in this codebase (`bash_stream.go`).

## Goal

pi-go can drive block-builder's plugin-authoring and codegen-run workflow
(propose a spec change, accept/validate it, start a run, watch it to
completion, fetch the manifest) as native agent tool calls, configured via
`internal/config` the same way other external integrations (e.g. kagent)
already are.

## Non-goals

- **The rest of the SDK's surface.** `ProjectClient`, `ConversationClient`,
  `UserClient`, `AssetClient`, `DeployClient`, `ChatClient` are not wired
  up here. Only the plugin-authoring + codegen-run loop
  (`SpecClient` + `RunClient`) ships in this pass; the rest is future
  follow-up if a use case appears.
- **A generic/reusable "long-running RPC stream" abstraction.** The
  buffered-reader mechanism below is built for these two RPCs. Do not
  generalize it into a shared helper package speculatively — if a third
  streaming RPC needs the same treatment later, extract the shared part
  then, informed by two real call sites instead of one imagined one.
- **A published `sdk/vX.Y.Z` release dependency.** pi-go depends on the
  `sdk/` module via a local relative `replace` directive
  (`~/src/coding-agents/pi-go` → `~/src/machina/block-builder/sdk`, i.e.
  `../../machina/block-builder/sdk`), not a tagged version — both repos
  are under active development and a tag adds release-process overhead
  neither side is ready to commit to yet. Revisit once the integration
  stabilizes.
- **ADK's `agenttool`/custom-`Agent` delegation pattern.** Investigated and
  ruled out: `agenttool.New`'s `Run` fully drains its wrapped agent's event
  channel internally before returning only the final text — it gives no
  live-progress benefit over a plain synchronous call, at the cost of a
  from-scratch custom `Agent` implementation, a private in-memory
  `runner`/`SessionService`, and a synthetic RPC-message-to-`session.Event`
  mapping layer with no existing precedent in this codebase. Not used.

## Architecture

```
pi-go/
├── internal/
│   ├── blockbuilder/                  (NEW package)
│   │   ├── client.go                  config → authenticated sdk.Adapter
│   │   ├── toolset.go                 BlockBuilderToolset (tool.Toolset)
│   │   ├── progress.go                bounded ring buffer + background
│   │   │                              drain goroutine (bash_stream.go
│   │   │                              pattern), keyed by
│   │   │                              conversation_id / run_id
│   │   └── *_test.go                  built against sdk/sdktest fakes
│   ├── config/                        + BlockBuilder{BaseURL,APIKey} fields
│   └── tools/
│       └── registry.go                CoreTools registers the new toolset
└── go.mod                             + require .../sdk, replace → local path
```

`BlockBuilderToolset` follows the existing `A2AToolset` shape exactly
(`Name() string` + `Tools(ctx agent.ReadonlyContext) ([]tool.Tool, error)`),
holding one `*sdk.Adapter` built once at construction from
`internal/config`. No per-call reconnection, no hidden session state:
every tool's arguments are self-contained (`plugin_id`, `conversation_id`,
`run_id`), mirroring block-builder-mcp's own MCP tool argument shapes —
anyone who has used the MCP surface already knows these tool names and
argument shapes.

## Auth

`sdk.Adapter` takes a plain `*http.Client` and does no authentication of
its own — block-builder's `X-API-KEY` forwarding intentionally stayed
behind in block-builder-mcp's own `apikeyfwd` package, not in the sdk. So
`internal/blockbuilder/client.go` constructs an `http.Client` whose
`Transport` is a small `http.RoundTripper` that sets `X-API-KEY` from the
new config field on every outbound request, then passes that client to
`sdk.New`.

## Config

Two new fields, following the existing pattern other integrations (e.g.
kagent's A2A URL) already use in `internal/config`:

- `block_builder.base_url` (env `BLOCK_BUILDER_URL`) — e.g.
  `http://localhost:8080`.
- `block_builder.api_key` (env `BLOCK_BUILDER_API_KEY`).

Both empty (the default) disables the toolset entirely — `CoreTools`
skips registering it rather than registering tools that would just fail
every call. This matches "presented credential always verified, no
credential means pass through/absent" being block-builder's own auth
posture, applied here as "no config means the feature isn't offered."

## Tools (11)

All plugin-authoring/codegen-run RPCs. `plugin_id`/`conversation_id`/
`run_id` argument names and semantics match block-builder-mcp's existing
MCP tools (`internal/mcpserver/tools/spec_tools.go`, `run_tools.go`)
exactly.

| Tool | RPC | Shape |
|---|---|---|
| `propose_spec_change` | `SpecClient.ProposeSpecChange` | long-running — starts the call, drains `ProposeSpecChangeEvent`s into a buffer keyed by `conversation_id`, returns immediately with the `conversation_id` |
| `get_spec_progress` | (buffer read) | poll companion — returns buffered events for a `conversation_id` since a given offset |
| `accept_spec` | `SpecClient.AcceptSpec` | plain unary |
| `validate_spec` | `SpecClient.ValidateSpec` | plain unary |
| `get_spec` | `SpecClient.GetSpec` | plain unary |
| `list_primitives` | `SpecClient.ListPrimitives` | plain unary |
| `start_run` | `RunClient.StartRun` | plain unary — returns `run_id` |
| `watch_run` | `RunClient.WatchRun` | long-running — drains `WatchRunResponse`s into a buffer keyed by `run_id`, returns immediately |
| `get_run_progress` | (buffer read) | poll companion — returns buffered events for a `run_id` since a given offset |
| `get_run` | `RunClient.GetRun` | plain unary — current run state (distinct from the buffered event log) |
| `get_manifest` | `RunClient.GetManifest` | plain unary — fetches `manifest.json` once a run completes |

### Long-running / buffered-reader mechanism

`propose_spec_change` and `watch_run` set `tool.Tool.IsLongRunning() ==
true`. Each, on first call for a given key (`conversation_id` /
`run_id`), starts the underlying `sdk.Stream[T]` and a background
goroutine that copies each received message into a bounded ring buffer
(same shape as `internal/tools/bash_stream.go`'s `stream` type — retained
tail, absolute byte/index offsets so a caller asking "everything after
offset N" can be told honestly that earlier messages were discarded
rather than silently skipped forward), then returns immediately. The
companion poll tool (`get_spec_progress` / `get_run_progress`) reads
from that buffer by key and offset. The buffer is dropped once the
stream's terminal message is observed and has been read at least once,
or after an idle TTL — exact eviction policy is an implementation
decision, not a frozen contract, since nothing else depends on it.

### Error handling

RPC errors (from `sdk.Adapter`, itself surfacing Connect-RPC errors)
propagate as a plain Go `error` return from each tool's `Run`, following
existing pi-go tool convention — no bespoke error-formatting layer.
Stream errors (an error value on `Stream[T].Err()`) are recorded into the
same buffer as a terminal error entry, so a poll tool sees "the stream
ended in error" the same way it would see any other terminal message,
rather than needing a separate error-checking path.

## Testing

`internal/blockbuilder`'s tests are built directly against
`sdk/sdktest`'s `FakeSpecClient`/`FakeRunClient` (already shipped by the
sdk module for exactly this purpose — block-builder-mcp's own tools use
the same fakes, see `internal/mcpserver/tools/fake_clients_test.go` in
that repo). No new pi-go-local mock layer for the sdk client interfaces.

## Success criteria (measurable)

- `go build ./... && go vet ./...` green in pi-go with the new package and
  toolset wired into `CoreTools`.
- With `block_builder.base_url`/`api_key` unset, `CoreTools` registers no
  block-builder tools (verified by a test asserting the toolset is absent
  from the returned tool list).
- With both set, all 11 tools are present and callable; each plain-unary
  tool round-trips correctly against `sdktest`'s fakes (request in,
  response out, `X-API-KEY` header present on the underlying HTTP call).
- `propose_spec_change`/`watch_run` return immediately (do not block until
  the stream completes); `get_spec_progress`/`get_run_progress` observe
  buffered messages appearing incrementally as the fake stream emits them,
  and the terminal message once the stream completes.
- A stream error surfaces through the poll tool as a terminal error entry,
  not a panic or a silently-empty buffer.

## Out of scope (explicit)

- Publishing a tagged `sdk/` version (see Non-goals).
- Any tool beyond the 11 listed (see Non-goals).
- UI/TUI-level live rendering of buffered progress — the poll tools exist
  for the *model* to call; whether/how pi-go's TUI additionally surfaces
  this to the human is a separate concern this design doesn't address.
- Changes to block-builder or the `sdk/` module itself — pure pi-go-side
  consumer work.
