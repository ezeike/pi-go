# Stack

Sources: go.mod, Makefile, .goreleaser.yaml, .github/workflows/{ci,release}.yml, AGENTS.md, .gograph/GRAPH_REPORT.md. Items marked (unverified) were not confirmed in code.

## Language and runtime
- Go 1.27.0 (`go.mod`); module `github.com/dimetron/pi-go`. CI uses `go-version-file: go.mod`.
- Release builds are `CGO_ENABLED=0` (`.goreleaser.yaml`); linux/darwin/windows, amd64/arm64 (windows/arm64 excluded).
- No `GOEXPERIMENT=simd` anywhere: Go 1.27 changed simd/archsimd and gomlx/compute amd64 kernels do not compile against it (comment in ci.yml and .goreleaser.yaml).
- Size (gograph): 77 packages, 1027 files, ~15.5k symbols.
- Secondary languages: TypeScript VS Code extension in `vscode/` (bun compile, vsce package; own Makefile); shell/YAML in `hack/agentgateway/`; Nix `flake.nix`; Python/Rust LSP fixtures under `hack/test/lsp/`.

## Binaries (`cmd/`)
`pi` (main CLI/agent), `pi-sandbox`, `pi-acp-mock`, `pi-mermaid`. Tools in `hack/`: `session-merge`, `sopcheck`, `vulngate`, `test/mcp`. Public embeddable packages: `piagent/`, `pimodels/`.

## Key dependencies (direct, go.mod)
| Dependency | Role |
|---|---|
| google.golang.org/adk/v2 v2.4.0 | Agent loop / tool runtime (Google ADK Go v2); after-tool callback chain semantics (see AGENTS.md) |
| google.golang.org/genai v1.71.0 | Gemini client |
| anthropic-sdk-go v1.71.0, openai-go/v3 v3.66.0 | Anthropic / OpenAI providers |
| ollama/ollama v0.33.3 | Ollama types/client |
| charm.land/bubbletea/v2, bubbles/v2, lipgloss/v2; charmbracelet/glamour, ultraviolet, x/ansi, colorprofile; chroma/v2; muesli/reflow, termenv; mattn/go-runewidth | Terminal UI and rendering |
| spf13/cobra v1.10.2 | CLI |
| modelcontextprotocol/go-sdk v1.7.0 | MCP client (and mock server in hack/test/mcp) |
| coder/acp-go-sdk v0.13.5 (replaced) | Agent Client Protocol |
| a2aproject/a2a-go/v2 v2.5.0 | A2A protocol |
| google.golang.org/grpc v1.83.2, protobuf v1.36.12 | gRPC (kagentapi, A2A gRPC) |
| gorilla/websocket | Web terminal / webserver |
| go.opentelemetry.io/otel v1.46.0 + otlptrace(grpc,http), sdk, proto/otlp | Tracing |
| modernc.org/sqlite v1.58.0 | Pure-Go SQLite (palace/memory) |
| knights-analytics/hugot v0.7.8 (+ indirect gomlx/*, onnx) | Local embeddings (palace) |
| sigstore/sigstore-go v1.3.0, in-toto/attestation v1.2.0 | Release attestation verification (`internal/attest`) |
| golang.org/x/oauth2, creack/pty, google/jsonschema-go, go-cmp, testify, kagent-dev/mockllm, yaml.v3 | Auth, PTY, schema, tests, config |
| github.com/machina/block-builder/sdk | block-builder SDK (see replaces) |

## go.mod replaces
1. `github.com/coder/acp-go-sdk => ./third_party/acp-go-sdk` (vendored fork).
2. `github.com/wk8/go-ordered-map/v2 => ./third_party/go-ordered-map` (pb33f fork keeping module path; avoids mailru/easyjson supply-chain risk, per go.mod comment; remove when ollama drops it).
3. `github.com/machina/block-builder/sdk => ../../machina/block-builder/sdk` (path outside the repo; go.mod comment says worktrees need a sibling symlink `~/src/coding-agents/machina`). A fresh clone cannot build without that sibling; how CI satisfies it is unverified (ci.yml shows no checkout/symlink step for it).

## Layout
`internal/` has 43 packages (largest: tui, tools, provider, cli, palace, subagent, webserver, session, extension). Other dirs: `third_party/`, `specs/` (incl. `specs/kagent/Dockerfile`), `docs/`, `scripts/`, `flake.nix`, `.githooks/`, devcontainer (unverified contents).

## Build and tooling (Makefile)
Targets: build, install, build-accel/deps-accel, test/test-unit/test-integration/test-e2e/test-all/test-coverage/test-ollama, eval-run/eval-pin/eval-judge/eval-tools/eval-tools-judge, record-pgo, lint (golangci-lint v2), vet, vulncheck, check-cve (runs `go mod tidy`, mutates tree), scan, sbom, hooks, sandbox-run/sandbox-log, fetch-models, fetch-modelsdev-pricing, fetch-ollama-pricing. Version stamped via `-X internal/cli.Version` and `BuildTag`.
Git hooks (`.githooks`): large-file check, gofmt/goimports, golangci-lint, sign-off, pre-push signature check.

## CI (.github/workflows/ci.yml; actions pinned by SHA)
Jobs: Lint (golangci-lint v2.13), Vulncheck (`govulncheck -format json ./... | go run ./hack/vulngate`; fails only on findings with a fix), Test (ubuntu; `go test ./...` plus `-race` excluding `internal/acp/server`; installs uv and rust toolchain for LSP fixtures), Test (Windows; no -race), Coverage (all pkgs except cmd/ and hack/, to Codecov), Build matrix (linux/windows/darwin x amd64/arm64 minus windows/arm64).

## Release (release.yml, on `v*` tags)
lint, race tests, Docker image `ghcr.io/<repo>-kagent` (linux/amd64+arm64, `specs/kagent/Dockerfile`, digest exported), GoReleaser release with `id-token`/`attestations` permissions (Sigstore build provenance + SBOM), then a "Publish the draft" step. VSIX built via `vscode/` `make release` (per AGENTS.md; not re-checked in the workflow).
