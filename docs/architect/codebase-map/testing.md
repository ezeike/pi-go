# Testing

## Tiers (build tags)
| Tier | Selector | Command |
|---|---|---|
| Unit (default, no tag) | none | `make test` / `make test-unit` = `go test ./...` |
| Integration | `//go:build integration` (3 files) | `make test-integration` = `go test -tags integration ./...` |
| E2E | `//go:build e2e` (18 files; one `e2e && !windows`) | `make test-e2e` (alias `make e2e`) = `go test -tags e2e ./...` |
| All | | `make test-all` (Makefile:159) = unit + integration + e2e |
| Other tags | `embedding`, `race` / `!race`, `ORT` (build-accel only) | |
- OS tags: `windows`, `!windows`, `unix` test/impl files (e.g. `internal/tui/*_windows_test.go`).
- Coverage: `make test-coverage` (`-coverpkg=./internal/...`). CI coverage runs every package except `cmd/` and `hack/` with `-coverpkg` set to that same list, uploads to Codecov (`fail_ci_if_error: false`). `codecov.yml` ignores `cmd/`, `internal/testenv/`, `internal/kagentapi/`. Not `-coverpkg=./...` because instrumenting cmd/hack trips the idle-timeout assertion in internal/tools bash supervisor tests.
- Other gates: `make lint` (golangci-lint v2.13 in CI), `make vet`, `make vulncheck` (govulncheck piped to `hack/vulngate`; fails only on findings with a fixed version), `make check-cve` (mutates go.mod via `go mod tidy`; don't use as a check), `make scan` (grype), `make sbom`.

## CI (`.github/workflows/ci.yml`)
Jobs: Lint, Vulncheck, Test (`go test -count=1 ./...` then `-race` on all except `internal/acp/server`; needs uv + rustup for LSP fixtures under `hack/test/lsp/*`), Test (Windows, no `-race`, no rg; the only job executing `//go:build windows` files), Coverage, Build matrix (linux/windows/darwin x amd64/arm64, no windows/arm64).

## Helpers
- `testSandbox(t, dir)`: `internal/tools/tools_test.go:23` and `internal/agent/agent_test.go:25`; builds a `Sandbox` and registers `t.Cleanup(sb.Close)`.
- `internal/testenv`: `SetHome` (sets HOME and USERPROFILE on Windows), `SetUnwritableHome`, other OS-portability helpers.
- `rgAvailable` package var (`internal/tools/grep.go:97`) toggled in tests: `TestCompactorRouting_WithRipgrep` / `_WithoutRipgrep` (`internal/tools/compactor_test.go:1334,1372`); `grep_rg_test.go`.
- `buildProdResult(t, v)` (`compactor_test.go:1051`): marshal real output struct -> `map[string]any`, like ADK.
- Guards: `TestCompactor_EveryToolCompacts` (`compactor_all_tools_test.go:107`), `TestCompactorRouting_EveryRegisteredTool` (`:1265`), `compactor_preservation_test.go`, `TestDedup_ReachableFieldsAreHonest` (`dedup_compaction_test.go:183`).
- LLM mocking: `github.com/kagent-dev/mockllm` in `internal/agent/mock_e2e_test.go` (`TestE2EMockLLMAnthropic`, mock Anthropic API server); `cmd/pi-acp-mock` is a mock ACP agent; `toolCallingLLM` pattern in `internal/agent/after_chain_compose_test.go` (real agent end-to-end for the after-tool chain; must use a real `agent.Context`, the tracing stage panics on nil).
- Tests rely heavily on `t.TempDir()`/`t.Setenv` (~1.8k uses); `t.Parallel` in ~57 files.
- `.gograph/graph-tests.md` maps test funcs to production symbols (very large; query with `gograph`, don't read whole).

## Windows pitfalls
- Open directory handle blocks `t.TempDir()` cleanup: close `tools.NewSandbox` roots (`t.Cleanup(func(){ _ = sb.Close() })`); failure shows in testing.go cleanup, not your assertion.
- `rg` absent on the Windows runner: never assert tool name `"ripgrep"`; assert exactly one of ripgrep/grep registered, cover both via `rgAvailable`.
- HOME vs USERPROFILE: use `testenv.SetHome`.
- Windows CI lacks `-race`; `internal/acp/server` excluded from race CI; `cmd/` coverage invisible.

## Eval harness (`internal/eval`)
- `make eval-run` (`TestEvalRun`, e2e tag, `./internal/tui/`, 45m; needs built binary + LLM key, `PI_EVAL_RUN=1`, `PI_BINARY`); `eval-pin` (tag `eval/base`, saves golden); `eval-judge` (baseline `eval/golden`, `PI_EVAL_JUDGE_MODEL`, default claude-sonnet-4-6).
- `make eval-tools` / `eval-tools-judge`: `TestEvalTools` in `internal/eval/scenarios/`, one headless `pi --mode print` scenario per tool family, coverage matrix; knobs `PI_EVAL_MODEL`, `PI_EVAL_SCENARIO`, `PI_EVAL_THINKING`, `PI_EVAL_TIMEOUT`, `PI_EVAL_STRICT=1`, `PI_EVAL_SERIAL=1`.
- `make record-pgo` merges eval + TUI render benchmark profiles into `cmd/pi/default.pgo`. Docs: `internal/eval/eval.md`, `internal/eval/scenarios/README.md`. Metrics: `AvgResultBytes` in `metrics.go` (reported, asserted in one unit test, not gated vs a baseline).
- `make test-ollama` runs `scripts/test-ollama-e2e.sh`.

## Verification practice
Reintroduce the bug to confirm a guard fails; build fixtures from real types; measure at the producer's cap; label unverified numbers (AGENTS.md "Never trust a big win").
