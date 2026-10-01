# CLAUDE.md — pi-go

Terminal coding agent in Go (module `github.com/dimetron/pi-go`, Go 1.27.0) built on
Google ADK Go v2. Bubble Tea v2 TUI, plus print/json/socket/rpc modes, ACP/A2A servers,
and a web terminal. Embeddable via `piagent/` and `pimodels/`.

**Read `AGENTS.md` first.** It is authoritative for worktrees, signed commits, PR/review
flow, TUI output safety, and tool-compaction invariants. This file is a quick map only.

## Commands (use the Makefile)
make build | install | test | test-integration | test-e2e | test-all | lint | vet | vulncheck
make hooks   # enable .githooks (large-file check, gofmt, golangci-lint, sign-off, signature check)

## Repo layout
cmd/{pi,pi-sandbox,pi-acp-mock,pi-mermaid}; internal/ (43 packages; largest: tui, tools,
provider, cli); piagent/, pimodels/; third_party/ (ACP SDK, ordered-map forks); vscode/
(extension, own Makefile); hack/ (vulngate, agentgateway, test fixtures); docs/.

## Rules that bite
- Work in a git worktree (`.worktrees/<name>`), never leave edits in the primary checkout.
- Every commit: `git commit -s -S`. Never `--no-verify`. No GIFs/recordings in git.
- Never write to stdout/stderr from TUI code; use `logger.Logger` or `SystemNoticeCh`.
- After-tool callbacks must go through `extension.ComposeAfterToolChain`, never a bare slice.
- Tests: close `tools.Sandbox` (`t.Cleanup`); don't hard-code `ripgrep` (toggle `rgAvailable`).
- Build needs the sibling checkout `../../machina/block-builder/sdk` (go.mod replace).
- Never put agent session links in PRs or commit messages.

## Architecture / knowledge docs
docs/architect/codebase-map/ (stack, architecture, conventions, testing, concerns),
ARCHITECTURE.md, MODELS.md, ROADMAP.md.
