# Integrations

Sources: internal/provider, internal/extension/mcp.go, internal/acp, internal/a2a, internal/otel, internal/lsp, internal/attest, internal/blockbuilder, internal/cli, hack/agentgateway, specs/kagent. Unverified items are marked.

## LLM providers (`internal/provider/`)
- Files per backend: anthropic.go (+anthropic_caching.go, cache_apply.go), openai*.go (completions, responses, azure, codex, reasoning), gemini.go, ollama.go, mistral.go, xai.go, openrouter.go, opencode.go, agentgateway.go; catalog.go/model_catalog.go/pricing.go for model metadata; retry.go, trace_transport.go.
- Credentials/endpoints from env (names seen in code): ANTHROPIC_API_KEY/ANTHROPIC_AUTH_TOKEN/ANTHROPIC_BASE_URL, OPENAI_API_KEY/OPENAI_BASE_URL, GEMINI_API_KEY/GOOGLE_API_KEY/GEMINI_BASE_URL, MISTRAL_*, XAI_*, OPENROUTER_*, OLLAMA_HOST/OLLAMA_API_KEY, AZURE_OPENAI_ENDPOINT, AGENTGATEWAY_API_KEY/AGENTGATEWAY_BASE_URL, CLAUDE_CODE_OAUTH_TOKEN, PI_MODEL/PI_BASE_URL. `config.LookupEnv` resolves process env, then `.pi-go/.env`/`.env`/`~/.pi-go/.env`.
- Default endpoints: Ollama `http://localhost:11434`; agentgateway `http://localhost:4000` (OpenAI-compatible); `internal/cli/ping.go` and `provider/list_models.go` know api.anthropic.com, api.openai.com, generativelanguage.googleapis.com.
- OAuth/login: `internal/auth/auth.go`, `pi login`; ChatGPT/Codex OAuth tokens detected via the `https://api.openai.com/auth` JWT claim (`provider/openai_codex.go`, `internal/codex`).
- Cross-cutting: `internal/retry`, `internal/ratelimit` (Gemini quota defaults), `internal/guardrail`, `internal/ctxwindow`.
- Ollama pricing and model data fetched by Makefile targets (`fetch-ollama-pricing`, `fetch-modelsdev-pricing`, `fetch-models`).

## agentgateway (`internal/provider/agentgateway.go`, `hack/agentgateway/`)
Local gateway fronting all providers; client wire format is chosen per model name/route (OpenAI-shaped by default; native Gemini and Ollama routes special-cased; placeholder key on Gemini routes). `hack/agentgateway/` holds config.yaml, docker-compose (with Postgres request log), run/install/update/backup scripts, pricing overlays. It also fronts Claude Code, Codex, Antigravity per its README.

## MCP (`internal/extension/mcp.go`, mcp_oauth_*.go)
Client via modelcontextprotocol/go-sdk; each configured server launched as a subprocess (`CommandTransport` wrapped by `respawnTransport`, `connTrackingTransport`). OAuth store and reauth handling present. Config from `mcp.json` (project/home). Test server in `hack/test/mcp`. Whether HTTP/SSE transports are supported was not checked.

## ACP (Agent Client Protocol)
- Server: `pi acp-server` (`internal/cli/acp_server.go` -> `internal/acp/server`).
- Client: `internal/acp/client` runner/session/persistent with adapters `claudecode`, `copilot`, `cursor`, `gemini`, `agy`, `forgecontroller`. Forge Controller is exposed as a tool via `internal/tools/forge_controller.go`.
- SDK: coder/acp-go-sdk replaced by `third_party/acp-go-sdk`. Mock agent: `cmd/pi-acp-mock`.

## A2A and kagent
`pi a2a` (`internal/cli/a2a_server.go`, `internal/a2a/server`): JSON-RPC and gRPC A2A server; agent card from KAGENT_AGENT_CARD_JSON; PORT env. `internal/kagentapi` holds generated protobuf/gRPC. Packaged as the kagent harness image (`specs/kagent/Dockerfile`, published to GHCR by release.yml). A2A client-side tool exists in `internal/tools` (per digest; unverified).

## Other serving modes
`--mode` interactive|print|json|socket|rpc (`internal/cli/cli.go`; default socket `/tmp/pi-go.sock`; `internal/jsonrpc`, `internal/pirpc`); `pi serve` web terminal (`internal/webserver`, gorilla/websocket); voice (`internal/voice`, `voicegemini`).

## OpenTelemetry (`internal/otel/otel.go`)
Exporter chosen by OTEL_TRACES_EXPORTER (otlp default, console, none; the console path carries a TODO near line 126); OTLP over gRPC or HTTP to OTEL_EXPORTER_OTLP_ENDPOINT. `cmd/pi/main.go` probes OTEL at startup. Provider HTTP traced via `provider/trace_transport.go`.

## LSP (`internal/lsp/`)
Built-in servers (languages.go): gopls (.go), typescript-language-server --stdio (.ts/.tsx/.js/.jsx), `uvx ty server` (.py/.pyi), rust-analyzer (.rs), jdtls (.java). Manager/client/hooks wire LSP into the after-tool chain and `internal/tools` LSP tools. Kotlin LSP is on the ROADMAP, not implemented.

## GitHub
- Release check/upgrade: `pi upgrade` -> `https://api.github.com/repos/dimetron/pi-go/releases/latest` (`internal/cli/upgrade.go`).
- Attestation lookup: `internal/attest` uses `https://api.github.com` (overridable for tests).
- CI/release on GitHub Actions; Codecov upload; GHCR image push.
- Review/PR workflow via `gh` and Codex per AGENTS.md (process, not code).

## Sigstore / in-toto (`internal/attest`, `pi verify`)
Verifies SLSA provenance and SBOM attestations produced by the release workflow; trust anchored to the public-good Sigstore TUF root (Fulcio/Rekor) via sigstore-go; statements parsed with in-toto attestation. GitHub-generated release-asset attestations are intentionally out of scope. Signing side: GoReleaser/GitHub OIDC in release.yml.

## block-builder (`internal/blockbuilder`)
Authenticated transport layer exposing SDK `SpecClient`/`RunClient` as tools (tool.go, toolset.go, tools_spec.go, tools_run.go, streaming_tools.go). Config: BLOCK_BUILDER_URL and BLOCK_BUILDER_API_KEY via `config.LookupEnv`; unset URL means disabled. SDK is the sibling-path replace described in stack.md. Where the toolset is registered in the CLI was not traced.

## Embeddings / memory
`internal/palace`, `internal/memory`: local hugot/ONNX embeddings (PALACE_MODEL_DIR, PI_ONNXRUNTIME_LIB, PI_NO_COREML) or Ollama embedder (`palace/embedder_ollama.go`); SQLite via modernc.

## Local LLM / eval
Eval harness (`internal/eval`, PI_EVAL_* env) drives `pi` against configured models; kagent-dev/mockllm used in tests.
