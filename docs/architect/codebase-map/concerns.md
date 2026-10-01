# Concerns (codebase map, Agent 4)

Tags: **[V]** verified against the tree on 2026-10-01; **[U]** unverified (taken from a doc/digest, not re-checked).

## 1. Build and dependency hygiene

- **[V] Out-of-tree `replace` breaks clean clones and CI.** `go.mod:208` `replace github.com/machina/block-builder/sdk => ../../machina/block-builder/sdk` (required at `go.mod:27`, imported by `internal/blockbuilder/*.go`). The comment at `go.mod:203-207` admits it only resolves from the primary checkout (`~/src/machina`) or via a manual symlink `ln -s ~/src/machina ~/src/coding-agents/machina`. `.github/workflows/ci.yml` has no step that checks out or symlinks the sibling repo (grep for machina/block-builder in ci.yml, release.yml, Makefile: no hits), and no `vendor/` exists. On a GitHub runner `../../machina` resolves to a nonexistent path, so `go build`/`go test ./...` should fail. Not executed in CI here (could not run it), so the failure itself is inferred. Also affects the Nix flake, GoReleaser and Docker image builds (**[U]**, not inspected).
- **[V] Vendored forks in `third_party/`** (`go.mod:192,201`): `acp-go-sdk` (replace, no removal note) and `go-ordered-map` (removal condition: ollama drops/changes its wrapper, `go.mod:197-200`). Both need manual upstream tracking.
- **[V] `ci.yml:~37` installs `govulncheck@latest`** unpinned, while other actions are SHA-pinned; non-reproducible gate.
- **[V] `ci.yml` header comment**: no `GOEXPERIMENT` because gomlx/compute amd64 kernels do not compile against Go 1.27 simd API; a dependency/toolchain compat workaround, revisit on gomlx upgrade.

## 2. CI and test coverage gaps

- **[V] `internal/acp/server` is excluded from `-race`** (`ci.yml:76`), with no comment explaining why. Unknown whether it is a known race or flakiness; the package is the ACP server surface.
- **[V] Windows job has no `-race`** (comment in `ci.yml` test-windows: needs CGO + C toolchain). Windows-only files (`internal/tui`, `internal/codex`, `internal/subagent`) are never race-tested.
- **[V] Coverage excludes `cmd/` and `hack/`** (`ci.yml` coverage job, `codecov.yml`); `cmd/pi/main.go` startup (GC defaults, dotenv, OTEL probe) is untested by coverage. Codecov upload has `fail_ci_if_error: false`, so upload failures are silent.
- **[V] Test job needs uv + rustup** for LSP fixtures (`hack/test/lsp/{python,rust}`); heavy and a source of CI flake. Several pkgs are gated behind e2e/integration build tags (`make test-e2e`), not run by plain CI `go test` (**[U]** which ones).
- **[V] `TEST_COVERAGE_REVIEW.md`** (335 lines) lists open gaps for `attest`, `voice`, `eval`, `memory`, `palace`, `procs` (e.g. `attest.Verifier.Verify`, `memory.SubagentCompressor`, `procs.terminate` timer branch). Not re-measured against current coverage (**[U]** whether closed).
- **[V] AGENTS.md "Windows-only failures"**: tests holding `tools.NewSandbox` must `Close()`, and must not hardcode `ripgrep`. Rule is enforced by convention only; no lint/test guard found (**[U]** not searched exhaustively).

## 3. Gaps, TODOs, unimplemented

- **[V] `internal/otel/otel.go:126`** `TODO: console exporter – for now fall through to no-op` (silent no-op when console exporter configured).
- **[V] `internal/codex/protocol.go:8,23,166,198`** future protocol methods not implemented: `OutputSchema` for structured turn results, `Branch` for `baseBranch` target.
- **[V] `hack/test/mcp/main.go:305`** TODO: does not wait for elicitation form submission (test helper only).
- **[V] `cmd/pi/main.go:20`, `internal/tui/tui.go:769`** reference "TODO §28/§30/§42", which live in gitignored `TODO.md` (`AGENTS.md` repo notes). Provenance is invisible to teammates and fresh clones.
- **[V] `ROADMAP.md`** has many unchecked items (62-line file): Kotlin LSP (l.19), per-phase model switching (l.20), Kanban (l.21), OAuth for web UI (l.22), SSH agent sandbox (l.26), ACP registry submission (l.27), terminal-bench eval (l.28), Temporal workflows (l.29), distributed tracing (l.30), `.cursorignore`-style ignore (l.33), ssh/docker/k8s sandboxes (l.34), Vertex/Amazon/OpenRouter/Fireworks/Ollama(anthropic) providers (l.55-63). Roadmap, not defects; note that distributed tracing is listed open though `internal/otel` exists (**[U]** scope difference).

## 4. Known defects and open issues

- **[V, open per doc] `ISSUES.md:71+` Priority 1: ADK validates tool args against JSON schema before `coercingTool.Run`**, producing 454 of the logged errors (253 additional-properties, 201 missing-required). Status "Investigating". Priority 2: subagent worktree paths like `../../go.mod` rejected by sandbox (`path escapes from parent`, 11 occurrences). Counts come from 30-day logs and are not reproduced (**[U]**). The fix status of the later priorities was not rechecked.
- **[V] `internal/eval/report.go:204-245` `sparkline`**: `TEST_COVERAGE_REVIEW.md` claims it indexes a string by byte and emits invalid UTF-8; current code uses `levels := []rune(...)` (l.235), so that claim appears fixed. Stale doc item.
- **[V] 5 non-merge commits on `main` with invalid signatures** (`60cbdfa`, `6ce61d9`, `4aa99b8`, `369cfbd`, `de6481b`, per `AGENTS.md`); removal requires force-push, deliberately left. Also 111 GitHub merge commits fail local verify by design. Commit hashes not re-verified (**[U]**).
- **[V] `internal/provider/stream_cancel.go:13-15`**: provider emits a `TODO: last event is not final` string that once reached the user on Esc; guarded, but a string-match fix, so it can regress (**[U]** test coverage of it).

## 5. Large files / complexity

- **[V]** `internal/tui/run.go` 2147 lines, `internal/cli/cli.go` 2214 lines. `DESIGN-REVIEW.md` named these as decomposition targets; `docs/complexity-refactor.md` exists. `cli.go` ~821-940 builds the agent loop inline (digest, **[U]**). `internal/tui` is ~61k LOC across 47 files (digest).

## 6. Stale or misleading docs

- **[V] `DESIGN-REVIEW.md` is stale** (last validated 2026-04-23, last commit 2026-04-23). Its top finding, a nil panic at `internal/acp/client/cursor/cursor.go:393-410` (`s.stderr.String()`), no longer matches the code: `cursor.go` is now 197 lines and the cited lines do not exist. The "repo does not pass `go test ./...`" headline should not be trusted.
- **[V] `AGENTS.md:514` cites `internal/typeutil/convert.go`**, but `internal/typeutil` does not exist in this repo. `internal/tools/compactor.go:78` correctly says it is `adk v2.4.0 internal/typeutil/convert.go` (the ADK module's internal package). AGENTS.md wording is misleading; it should say "in the ADK module".
- **[V] Counts drift**: digest says 43 `internal/` packages; gograph reports 77 packages overall (`.gograph/GRAPH_REPORT.md`, includes cmd/hack/third_party). `README.md` lags the package set (**[U]**, not diffed).
- **[V] `AGENTS.md` carries macOS-specific paths** (`/opt/homebrew/bin/rtk`, 1Password `op-ssh-sign` on `/Applications/...`); this dev machine is Linux, so signing/rtk guidance is platform-specific. `ollama-pr-draft.md` sits at repo root (**[U]** whether still relevant).
- **[V] `ISSUES.md` mixes "Implemented Fixes (v1.2.1)" with open items**; version labels not tied to any release (**[U]**).
- **[V] `TODO.md` and `MEM_PPROF.md` are gitignored** yet AGENTS.md and code comments depend on them for findings.

## 7. Undocumented tradeoffs / shortcuts

- **[V] Compaction/dedup reachability**: `dedupTools` lists tools whose results the deduper cannot actually reach (only `read`, `read_image`, `git-file-diff` work; `AGENTS.md` "The deduper reaches fewer tools than it lists"). Latent: extending requires a deliberate feature change.
- **[V] After-tool chain fragility**: ADK runs only the first non-nil after-tool callback, so correctness depends on always going through `extension.ComposeAfterToolChain`. Prior silent-failure history (dedup/compactor/LSP/memory dead for months). No compile-time guard against passing a slice (**[U]**).
- **[V] Sandbox via `os.Root` in `tools.NewSandbox`** holds an open handle that must be closed (see section 2).
- **[V] `eval-*` metrics (`AvgResultBytes`) reported but not gated** against a baseline (`AGENTS.md` "Never trust a big win").
- **[V] Git worktrees stash and pop uncommitted changes** around `git worktree add` (`internal/subagent/worktree.go:117`): safe but a data-loss window if the pop fails (**[U]** failure handling).
