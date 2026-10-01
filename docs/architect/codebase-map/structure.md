# pi-go Directory Structure

Module `github.com/dimetron/pi-go`, Go 1.27.0. Sources: `.gograph/GRAPH_REPORT.md` (77 packages, 1027 files), ARCHITECTURE.md, directory listing.

## Top level

| Path | Purpose |
|---|---|
| `cmd/pi/` | Main binary: GC defaults, dotenv, OTEL probe, `cli.Execute` |
| `cmd/pi-sandbox/` | Sandbox runner (seatbelt/entitlements) |
| `cmd/pi-acp-mock/` | Mock ACP server for tests |
| `cmd/pi-mermaid/` | Standalone Mermaid renderer |
| `piagent/` | Public embeddable SDK (Agent, Build, Turn, callbacks, options) |
| `pimodels/` | Public model list / metadata facade |
| `internal/` | All private packages (see below) |
| `third_party/` | Vendored `acp-go-sdk` and `go-ordered-map` (go.mod replace targets) |
| `vscode/` | TypeScript VS Code extension (own Makefile; bun + vsce) |
| `hack/` | Dev tooling: `agentgateway`, `session-merge`, `sopcheck`, `vulngate`, `test/*` |
| `scripts/` | Install, pricing/model fetch, e2e shell scripts |
| `docs/` | Design docs, `architect/` outputs, `superpowers/` specs |
| `specs/` | Feature/design specs (kagent, memory, harness, eval, etc.) |
| `.gograph/`, `graphify-out/` | Generated code graph / cache (not source) |
| `.githooks/`, `.github/` | Hooks (signing, large-file, lint) and CI/release workflows |
| `Makefile`, `flake.nix`, `Vibefile`, `codecov.yml`, `mempalace.yaml` | Build, Nix, coverage and memory-palace config |

## internal/ packages

### Runtime core
| Dir | Purpose |
|---|---|
| `cli/` | Cobra root and subcommands; builds agent, wires callbacks, dispatches modes (`interactive`, `print`, `json`, `socket`, `rpc`); `serve`, `acp-server`, `a2a`, `memory`, `plugin`, `login`, etc. |
| `agent/` | ADK agent wrapper: run loop, event stream, retry, grounding, `PreTurnHook` |
| `config/` | Global + project config, roles, env lookup |
| `session/` | JSONL file session store, branches, compaction (shed + summarize), merge, host env |
| `autocompact/` | Pre-turn compaction hook, config, token meter |
| `atif/` | ATIF trajectory writer and subagent trajectory linking |
| `ctxwindow/` | Context-window sizing |
| `guardrail/` | Token tracker / daily limit, model wrapper |
| `retry/`, `ratelimit/` | Transport retry and rate limiting |
| `provider/` | LLM backends (anthropic, openai, gemini, mistral, ollama, openrouter, xai, opencode, azure), catalog, pricing, `modeldata/` |
| `auth/`, `codex/` | OAuth/device login flows; Codex JSON-RPC client |

### Tools and extensions
| Dir | Purpose |
|---|---|
| `tools/` | Tool registry and one file per tool (file I/O, bash, grep/find/ls/tree, git, LSP, subagent, a2a, forge_controller, mem), `os.Root` sandbox, compactor/dedup/ledger/redact/truncate |
| `extension/` | Hooks, skills loader, MCP client/OAuth, tracing callbacks, `ComposeAfterToolChain`, read_image |
| `plugin/` | Marketplace manifests, registry, install/update |
| `lsp/` | LSP manager, clients, after-tool format/diagnostics hook |
| `sop/` | Standard operating procedures: plan/run/compile/validate/specdoc |
| `blockbuilder/` | Client/toolset for the external block-builder SDK |
| `kagentapi/` | Generated gRPC/protobuf client for kagent agent instances |

### Subagents and protocols
| Dir | Purpose |
|---|---|
| `subagent/` | Orchestrator, pool, `pi`-process spawner, ACP/Codex spawners, git worktree manager, agent definitions (`bundled/`) |
| `acp/` | ACP shared types, permissions, tool-call mapping |
| `acp/server/` (+`adapter/`) | ACP agent server (pi as an ACP agent), prompt runtime, session store/replay |
| `acp/client/` | ACP client runner/persistent session; per-vendor adapters `claudecode`, `copilot`, `cursor`, `gemini`, `agy`, `forgecontroller` |
| `a2a/server/` | A2A server executor over ACP server runtime |
| `jsonrpc/`, `pirpc/` | Generic JSON-RPC; pi's own RPC mode |
| `webserver/` | HTTP/WebSocket web terminal, pairing, PTY bridge, voice endpoints, embedded static assets |
| `voice/`, `voicegemini/`, `browser/` | Voice I/O, Gemini voice backend, browser launch |

### UI
| Dir | Purpose |
|---|---|
| `tui/` | Bubble Tea v2 TUI (~47 files): agent loop, chat, commands, sidebar/panels, themes, tool display |
| `mermaid/` | Native Mermaid parser/layout/renderer |

### Memory
| Dir | Purpose |
|---|---|
| `memory/` | Observation capture worker, compression, SQLite store, search, privacy |
| `palace/` | MemPalace: miner, embedders, drawers, KG, graph, palace tools, observation bridge |

### Cross-cutting and quality
| Dir | Purpose |
|---|---|
| `logger/`, `httplog/`, `otel/` | Session logger, HTTP span events, OpenTelemetry |
| `audit/`, `attest/`, `notice/` | Hidden-char scanner, provenance predicates, attribution notices |
| `eval/` | Scenario runner, judge, coverage metrics |
| `gitroot/`, `procs/`, `testenv/` | Repo root resolution, process listing, shared test env |
