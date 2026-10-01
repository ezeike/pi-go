# Conventions (observed)

Sources: AGENTS.md, .golangci.yml, Makefile, .githooks/*, .github/workflows/ci.yml, sampled code. Module `github.com/dimetron/pi-go`, Go 1.27.

## Package layout
- Entry points under `cmd/` (pi, pi-sandbox, pi-acp-mock, pi-mermaid); private code under `internal/` (~43 pkgs); embeddable public API in `piagent/` and `pimodels/`; vendored forks in `third_party/`; ad-hoc probes/tools in `hack/` (vulngate, sopcheck, session-merge, test/*).
- Platform-specific files use `//go:build windows|!windows|unix` (e.g. `internal/tui/drain_windows.go`).
- `internal/testenv` is test-only (imported from `_test.go` only).

## Errors
- Wrap with `fmt.Errorf("context: %w", err)` (e.g. `internal/tools/sandbox.go:85-106`); inspect with `errors.Is/As` (lint: `errorlint`, `errname`, `nilerr`).
- Sentinels are `var ErrX = errors.New(...)` (e.g. `ErrNoDefaultRole`, `ErrSubagentTimeout`, `ErrOllamaUnavailable`); typed errors use the `XxxError` suffix (`guardrail.LimitExceededError`, `palace.DuplicateError`, `sop/exec.CycleBudgetError`; protocol ones `RPCError`, `ResponseError`).
- ADK tool handlers return failures inside result structs, not Go errors, so `nilerr` is excluded for `internal/tools/` and for callback-style pkgs `internal/(lsp|cli|memory|tui|palace)/` (`.golangci.yml`).
- Error strings: revive `error-strings`/`errorf`/`error-naming` enabled.

## Logging
- No slog in the TUI path. Custom session logger `internal/logger` (`Logger.Info/Error/Errorf/ToolCall/ToolResult/...`), writes to the session log file; plus `internal/httplog`, `internal/otel`.
- TUI rule (AGENTS.md): never `fmt.Print*`, `log.Print*`, `os.Stdout/Stderr` while the alt screen is active; use `m.cfg.Logger`, `SystemNoticeCh`, or TUI error rendering. Panic handler may write stderr as last resort.

## Context
- `ctx context.Context` first param (revive `context-as-argument`); ~520 production signatures take it. Cancellation propagates through agent/subagent/tool layers.

## Naming / style
- gofmt -s + goimports with `local-prefixes: github.com/dimetron/pi-go` (formatters in `.golangci.yml`, enforced by `.githooks/pre-commit`).
- US spelling (`misspell`), tool names register hyphenated (`git-file-diff`) or underscored per tool; test names `TestX_Y` / `TestCompactorRouting_WithRipgrep`.
- Linters: errcheck (check-type-assertions), govet enable-all minus fieldalignment/shadow, staticcheck all (ST1000 off), revive subset, bodyclose, copyloopvar, durationcheck, fatcontext, unconvert, wastedassign. Complexity linters (cyclop/gocyclo/gocognit) are intentionally disabled. Excluded paths: `tmp/`, `research/`, `hack/` (staticcheck); relaxed in `_test.go`.

## After-tool callback chain
- ADK runs only the first after-tool callback returning non-nil. Always pass the chain through `extension.ComposeAfterToolChain` (`internal/extension/hooks.go:245`); used at `internal/cli/cli.go:855`, `internal/cli/interactive.go:656`, `internal/acp/server/runtime.go:516`.
- Order: hooks -> LSP -> dedup -> compactor -> memory (tracing first). A composed stage returns `(nil,nil)` to continue, `(m,nil)` to replace, `(_,err)` to abort.

## Tool-output compaction (`internal/tools/compactor*.go`, `dedup.go`)
- Route on the registered name (`ripgrep`/`grep` via `grepToolName`; hyphenated git tools). `compactorPipelines` keyed by name.
- A pipeline writes only the field it read (`CompactResult` with `CompactWrite{Key,Value}`); keep value types (`[]any` of `map[string]any` after ADK JSON round-trip); cap arrays, set `truncated`, preserve `total_*`; return nil when no gain (never-worse); do not cap lists whose truncation misleads (`git-overview` staged/unstaged lists).
- Deduper hashes one string field (`primaryOutputField`); only `read`, `read_image`, `git-file-diff` reachable.
- rtk is a design reference, never delegated to.

## Sandbox / tools
- `tools.NewSandbox` uses `os.Root`; holds an open dir handle and must be `Close()`d.
- `rgAvailable` (`internal/tools/grep.go:97`) selects `ripgrep` vs `grep` registration.

## Config / env
- Config `~/.pi-go/config.json` + project `.pi-go/config.json`, `mcp.json`, `.env`; ~86 `os.Getenv` reads (see `.gograph/graph-config.md`). Secrets via `pass`/env, never hardcoded.

## Git workflow
- Worktree per task: `<repo>/.worktrees/<branch-with-dashes>` (humans), `.pi-go/tasks/<pathID>` (pi-go agent, branch `pi-agent-<id>`), `~/.arbor/worktrees/pi-go/<name>`. Check `git rev-parse --git-dir` vs `--git-common-dir` before creating one; never nest.
- Every commit `git commit -s -S`; SSH signing via 1Password; never `--no-verify`. Verify with `git verify-commit`, not the `gpgsig` header. Fix bad signatures with `git rebase --exec 'git commit --amend --no-edit -S'`.
- Hooks (`make hooks` sets `core.hooksPath=.githooks`): `pre-commit` (check-large-files staged, gofmt -s, goimports, `golangci-lint run ./cmd/... ./internal/... ./hack/...`), `commit-msg` (adds missing Signed-off-by), `post-commit` (warns if unsigned), `pre-push` (hard-fails unsigned/unsigned-off commits, oversized blobs).
- `.githooks/check-large-files`: 1 MiB for media/recordings (gif, mp4, cast, ...), 10 MiB else; `docs/screen/pi-go.gif` grandfathered. Never commit GIFs/recordings (use GitHub release / `vhs-e2e-gif`). Override only `PI_ALLOW_LARGE_FILES=1`.
- PR flow: open PR first (`gh pr create --fill --web`, include all pending changes), Codex review posted as formal GitHub review (`codex exec --sandbox danger-full-access`), resolve in-thread with commit refs, then `make vulncheck` and post result as PR comment. Never put agent session links in PR/commit/review text.
- Claims of large wins require proof: bound input by producer cap, real-struct fixtures, end-to-end measure (`AvgResultBytes` in `internal/eval/metrics.go`), verify the guard can fail.
- Profiling: `pi --pprof true` serves :6060; scripts in `.claude/skills/go-pprof/scripts/`. `TODO.md`, `MEM_PPROF.md` gitignored.
- Sessions live at `~/.pi-go/sessions/<id>/{meta.json,events.jsonl,trajectory.atif.json,branches.json}`.
