# pi-go Architecture (codebase map)

Evidence: `.gograph` import graph, package source, ARCHITECTURE.md (partly stale; code wins), AGENTS.md. Edges below are derived from actual `internal/...` imports and call sites checked in source.

## Component boundaries

pi-go is a single Go module producing one main binary (`cmd/pi`) plus helpers. Everything is layered around the Google ADK Go agent runtime.

- **Entry / composition root: `internal/cli`.** The largest fan-out in the repo. It resolves config, model role and provider, wraps the LLM with `guardrail`, builds tools and toolsets, composes the before/after-tool callback chains, then dispatches by mode (interactive TUI, print, json, socket, rpc) or by subcommand (`serve`, `acp-server`, `a2a`, `memory`, `plugin`, ...). Wiring happens here and in `piagent` (the public SDK) and `acp/server/runtime.go`; these three are parallel composition roots that duplicate the wiring.
- **Agent runtime: `agent`, `session`, `autocompact`, `atif`.** `agent` wraps ADK runner/llmagent and exposes a `PreTurnHook`. `session.FileService` is the ADK session service backed by `~/.pi-go/sessions/<id>/` (`meta.json`, `events.jsonl`, `trajectory.atif.json`, `branches.json`). `autocompact` is the pre-turn hook that calls `session.AutoCompact`.
- **Model pipeline: `provider`, `guardrail`, `retry`, `ratelimit`.** Providers implement ADK `model.LLM`; guardrail wraps the LLM for token accounting.
- **Tools: `tools`.** Registry plus sandboxed (`os.Root`) tools and post-processing (dedup, compactor). Depends on `subagent`, `acp/client/forgecontroller`, `kagentapi`, `lsp`, `memory`, `config`. It does not depend on `agent`.
- **Extensions: `extension`, `plugin`, `lsp`.** `extension` owns hooks, skills, MCP, tracing callbacks and `ComposeAfterToolChain`. `lsp` contributes an after-tool callback and LSP tools.
- **Subagents: `subagent`.** Orchestrator with a concurrency pool, a `pi`-subprocess spawner, ACP/Codex dispatch, and a git `WorktreeManager`.
- **Protocol edges: `acp/server`, `acp/client`, `a2a/server`, `pirpc`, `webserver`.** `acp/server` hosts pi as an ACP agent via `piagent`; `a2a/server` is a thin A2A executor over the ACP server runtime; `acp/client/*` drive external agents; `webserver` serves a pairing-gated web terminal that runs `pi` in a PTY (it does not import `agent`).
- **UI: `tui`.** Bubble Tea v2; imports agent, session, subagent, tools, palace, sop, provider, extension. Must never write to stdout/stderr (AGENTS.md).
- **Memory: `memory`, `palace`.** `memory.Worker` captures tool observations asynchronously and compresses via a subagent spawner; `palace` (SQLite, embeddings, KG) receives observations through `ObservationBridge`.
- **Public SDK: `piagent`, `pimodels`.** Embeddable facade that imports most internals.

Notable boundary smells (from imports): `guardrail` imports `autocompact`; `palace` imports `memory`; `acp/server` imports `acp/client` and `piagent`; three composition roots (cli, piagent, acp/server) re-implement similar wiring. ARCHITECTURE.md lists `guardrail -> cli` and `logger -> cli`, which the current imports do not show.

## Component Relationships

| From | To | Via | Sync/Async | Purpose |
|---|---|---|---|---|
| cmd/pi | cli | `cli.Execute` | Sync | Process entry point |
| cli | config | `config` load / `ResolveRole` | Sync | Load global and project config, resolve model role |
| cli | provider | `provider` resolve and build `model.LLM` | Sync | Select backend for model |
| cli | guardrail | `guardrail.WrapModel` | Sync | Wrap LLM with token tracking and daily limit |
| cli | agent | agent construction and `SetPreTurnHook` | Sync | Build ADK agent and run it |
| cli | tools | `CoreTools`, `AgentTool`, dedup/compactor builders | Sync | Register tools and post-processing callbacks |
| cli | extension | `BuildBeforeToolCallbacks`, `BuildAfterToolCallbacks`, `ComposeAfterToolChain` | Sync | Compose hook/tracing callback chain |
| cli | lsp | `lsp.NewManager`, `BuildLSPAfterToolCallback` | Sync | LSP manager and after-tool callback |
| cli | session | `session.NewFileService` | Sync | Persist session events |
| cli | autocompact | `autocompact.BuildHook` | Sync | Install pre-turn compaction hook |
| cli | subagent | `subagent.NewOrchestrator` | Sync | Create subagent orchestrator |
| cli | memory | memory worker and store setup | Sync | Observation capture |
| cli | palace | `setupPalace`, `NewObservationBridge` | Sync | Palace tools, context and observation sink |
| cli | tui | interactive mode entry (`interactive.go`) | Sync | Run TUI |
| cli | pirpc | rpc/socket mode dispatch | Sync | JSON-RPC mode |
| cli | acp/server | `acp-server` subcommand | Sync | Serve pi over ACP |
| cli | a2a/server | `a2a` subcommand | Sync | Serve pi over A2A |
| cli | webserver | `serve` subcommand | Sync | Serve web terminal |
| cli | blockbuilder | block-builder tool wiring | Sync | Optional block-builder tools |
| cli | acp/client/forgecontroller | forge controller manager | Sync | Forge controller tool support |
| cli | auth | `login` subcommand | Sync | OAuth/device login |
| cli | plugin | `plugin` subcommand | Sync | Marketplace install/update |
| cli | audit | `audit` subcommand | Sync | Hidden-character scan |
| cli | otel / logger / httplog | tracer init and session log | Sync | Observability |
| agent | provider | ADK `model.LLM.GenerateContent` | Sync (streamed iterator) | LLM call |
| agent | tools | ADK function tool invocation | Sync | Execute tool calls |
| agent | session | ADK session service `AppendEvent` | Sync | Persist events |
| agent | retry | `retry` wrapper | Sync | Transient error backoff |
| agent | extension | before/after tool and model callbacks | Sync | Hooks and tracing |
| agent | subagent | via `tools` agent tool and attribution | Sync | Spawn child work |
| autocompact | session | `FileService.AutoCompact` | Sync | Shed / summarize old events |
| autocompact | agent | `agent.PreTurnHook` type | Sync | Hook runs before each turn |
| autocompact | tools | `ResultDeduper.Reset` | Sync | Reset dedup after summarize |
| guardrail | autocompact | token tracker coupling (`BodyTokens`) | Sync | Feed context size to compaction |
| session | atif | `atif.Writer.AppendEvent` | Sync | Write trajectory beside events |
| tools | subagent | `AgentTool` -> `Orchestrator.Spawn` | Sync (event channel) | LLM-initiated subagents |
| tools | acp/client/forgecontroller | `Manager.Run` | Sync | `forge_controller` tool |
| tools | kagentapi | gRPC client | Sync | kagent agent-instance tool |
| tools | a2a (a2a-go client) | `ClientCache.SendMessage` | Sync / streaming | Remote A2A agent dispatch |
| tools | memory | `mem-search`, `mem-timeline`, `mem-get` | Sync | Memory query tools |
| tools | lsp | LSP tools | Sync | Language intelligence |
| extension | tools / plugin / auth / browser / audit | skills, MCP OAuth, plugin skill dirs | Sync | Extension loading |
| extension | MCP servers | MCP go-sdk over subprocess/HTTP | Sync | Bridge external tools |
| lsp | language servers | stdio LSP | Sync | Diagnostics and format after edit |
| subagent | acp/client/{claudecode,copilot,cursor,gemini,agy,forgecontroller} | `dispatchACP` | Async (event pump) | Run ACP-backed subagents |
| subagent | codex | `spawner_codex` | Async | Run Codex subagent |
| subagent | pi subprocess | `exec.Cmd`, JSONL stdout | Async | Run child `pi` in json mode |
| subagent | git (worktree) | `WorktreeManager` git CLI | Sync | Isolated worktree per agent |
| subagent | session | `AgentContext` attribution | Sync | Link subagent to parent session |
| acp/client/* | external agent CLIs | ACP over stdio | Async | Drive vendor agents |
| acp/server | piagent | `NewPromptHandler`, `runPromptTurn` | Sync | Run pi turn per ACP prompt |
| acp/server | session / autocompact / guardrail / provider / tools / extension / lsp | `buildSessionResources` | Sync | Per-session runtime |
| acp/server | subagent | `newSessionOrchestrator` | Sync | Subagents inside ACP sessions |
| a2a/server | acp/server | `piExecutor` reuses prompt handler and adapter | Sync | A2A task execution |
| webserver | `pi` child process (PTY) | `exec.Command` + `creack/pty` | Async | Web terminal |
| webserver | voicegemini | Gemini voice WS | Async | Voice sessions |
| pirpc | agent / provider | RPC turns | Sync | RPC mode |
| memory | subagent | compressor spawner | Async | Compress observations |
| memory | SQLite | `modernc.org/sqlite` | Sync | Observation store |
| palace | memory | observation types | Sync | Convert observations to drawers |
| tui | agent / session / subagent / tools / palace / sop / provider / extension | model update loop and init goroutine | Async (channels) | Interactive UI |
| piagent | agent / provider / tools / session / autocompact / guardrail / memory / palace / subagent / forgecontroller / blockbuilder | `piagent.Build` | Sync | SDK composition root |
| sop | subagent | SOP execution | Sync | Run SOP steps |
| provider | auth / httplog / ratelimit / retry | backend request path | Sync | Credentials and resilience |

## Key Flows


### Flow: Interactive/print prompt turn
1. User -> cli: prompt (via cobra root `pi [prompt]`)
2. cli -> agent: Run(sessionID, message) (via ADK runner)
3. agent -> autocompact: pre-turn hook (via PreTurnHook)
4. agent -> provider: GenerateContent (via model.LLM through guardrail wrapper)
5. provider -> agent: text or tool call (via iter.Seq2 stream)
6. agent -> tools: execute tool (via ADK functiontool in os.Root sandbox)
7. tools -> agent: tool result (via after-tool chain)
8. agent -> session: AppendEvent (via FileService)
9. agent -> tui: events (via iter.Seq2 and agentMsg channel)

### Flow: After-tool chain
1. agent -> extension: tool finished (via ADK AfterToolCallback)
2. extension -> extension: user after-hooks then tracing (via ComposeAfterTool fold)
3. extension -> lsp: format and diagnostics on write/edit (via BuildLSPAfterToolCallback)
4. extension -> tools: dedup against prior result (via BuildDedupCallback)
5. extension -> tools: compact result (via BuildCompactorCallback)
6. extension -> memory: enqueue observation (via memoryObservationCallback)
7. extension -> agent: single composed result (via ComposeAfterToolChain one-element slice)

### Flow: Auto-compaction
1. agent -> autocompact: before turn (via PreTurnHook)
2. autocompact -> guardrail: read body tokens and window (via Tracker)
3. autocompact -> session: AutoCompact (via FileService)
4. session -> session: stage 1 shed superseded tool results (via ShedSupersededToolResults)
5. session -> provider: stage 2 summarize (via LLMSummarizer, SimpleSummarizer fallback)
6. autocompact -> tools: reset deduper after summarize (via ResultDeduper.Reset)
7. autocompact -> tui: outcome notice (via report / SystemNoticeCh)

### Flow: Subagent spawn
1. agent -> tools: agent tool call (via ADK function call)
2. tools -> subagent: Spawn (via Orchestrator)
3. subagent -> subagent: acquire slot (via Pool.Acquire)
4. subagent -> subagent: create worktree when agent needs isolation (via WorktreeManager.CreateAt, stash/pop around git worktree add)
5. subagent -> pi child: start process in json mode (via Spawner exec.Cmd) or ACP/Codex agent (via dispatchACP / spawner_codex)
6. pi child -> subagent: JSONL events (via stdout line scanner)
7. subagent -> tools: Event channel (via Process.Events)
8. tools -> agent: summary result (via tool return)
9. subagent -> subagent: release slot, cleanup/merge worktree (via Cleanup, MergeBack)

### Flow: Session persistence
1. agent -> session: AppendEvent (via FileService)
2. session -> disk: append events.jsonl (via file append under session dir)
3. session -> atif: AppendEvent (via atifWriter)
4. atif -> disk: trajectory.atif.json (via Writer)
5. session -> atif: link subagent trajectories (via LinkSubagentTrajectories)

### Flow: ACP server prompt
1. ACP client -> acp/server: Prompt (via ACP JSON-RPC over stdio)
2. acp/server -> acp/server: bind or resume session (via bindSession, openPiSession)
3. acp/server -> piagent: build agent on first turn (via initPiSessionState / buildSessionResources)
4. acp/server -> piagent: run turn (via runPromptTurn)
5. piagent -> acp/server: tool and text events (via adapter.Stream, streamProxy OnToolStart/OnToolEnd)
6. acp/server -> ACP client: session updates (via SessionUpdater)
7. acp/server -> ACP client: PromptResponse (via ACP)

### Flow: A2A task
1. A2A client -> a2a/server: message (via a2a-go server)
2. a2a/server -> acp/server: execute prompt (via piExecutor.Execute using ACP prompt handler)
3. acp/server -> a2a/server: streamed events (via adapter)
4. a2a/server -> A2A client: task events with heartbeat (via iter.Seq2 a2a.Event)

### Flow: Outbound A2A call
1. agent -> tools: a2a tool call (via function tool)
2. tools -> remote A2A agent: SendMessage (via ClientCache, a2aclient, optional streaming)
3. remote A2A agent -> tools: text (via stream collector)

### Flow: Forge controller
1. agent -> tools: forge_controller call (via function tool)
2. tools -> forgecontroller: Manager.Run (via resolveContext and Command)
3. forgecontroller -> acp/client: start or reuse persistent session per project (via startSession, projectKey)
4. acp/client -> external agent: prompt (via ACP stdio)
5. forgecontroller -> tools: RunResult and events (via onEvent callback)

### Flow: Web terminal
1. Browser -> webserver: GET /pair, POST /api/pair/submit (via HTTP pairing code)
2. webserver -> webserver: create session (via SessionManager.CreateSession)
3. Browser -> webserver: GET /ws/ upgrade (via WebSocket, same-origin and pairing token check)
4. webserver -> pi child: spawn in PTY (via exec.Command and pty.StartWithSize)
5. Browser -> webserver: keys/resize (via WebSocket)
6. webserver -> Browser: terminal output (via WebSocket)

### Flow: Memory and palace capture
1. extension -> memory: Enqueue raw observation (via after-tool callback, buffered channel)
2. memory -> subagent: compress (via memory-compressor spawner, fallback observation on failure)
3. memory -> SQLite: store observation (via Store)
4. memory -> palace: OnAfterStore hook (via ObservationBridge.ConvertAndStore)
5. palace -> SQLite: write drawer with wing/room/hall (via palace store)

### Flow: Palace/memory retrieval
1. cli -> palace: setupPalace (via startup)
2. palace -> cli: context text (via palaceContext appended to system instruction)
3. agent -> tools: memory or palace tool call (via function tool)
4. tools -> memory: query store (via mem-search/timeline/get)

### Flow: Deferred TUI init
1. tui -> cli: start init goroutine (via Bubble Tea Cmd)
2. cli -> tools: create sandbox and core tools (via CoreTools)
3. cli -> lsp, memory, extension, subagent: parallel init (via goroutines)
4. cli -> agent: build agent (via builder)
5. cli -> tui: InitEvent (via channel)

### Candidate flows considered and excluded
- MCP tool bridging: seen in `extension/mcp.go` but not traced end to end.
- OAuth login and plugin install: CLI-only side flows, covered in ARCHITECTURE.md; not cross-component runtime.
- Eval harness (`eval`), SOP execution (`sop`), voice sessions, `pirpc` RPC turns, block-builder streaming tools: handlers not traced in this pass.
- Provider retry/rate-limit internals: internal to `provider`/`retry`.
- LSP tool calls (explicit): single-hop tool to language server.
- Guardrail daily-limit enforcement: single-component.
- Item to confirm: the Deferred TUI init steps are taken from ARCHITECTURE.md, not re-verified in `tui`/`cli/interactive.go`.
