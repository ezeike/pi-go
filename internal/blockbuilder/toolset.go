package blockbuilder

import (
	"context"
	"errors"

	"github.com/machina/block-builder/sdk"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
)

// BlockBuilderToolset exposes block-builder's plugin-generation API (proto/block-builder/plugin/v1)
// as ADK tools: 7 plain-unary tools (this component: tools_spec.go, tools_run.go) plus 6
// long-running/buffered-poll tool pairs (the streaming component's streaming_tools.go, built on
// its progressRegistry) — 13 tools total. Assembled here (Task 8, impl-specs/toolset.md; the chat
// pair added for PluginChatService.SendMessage).
type BlockBuilderToolset struct {
	spec sdk.SpecClient
	run  sdk.RunClient
	chat sdk.ChatClient

	specBuf *progressRegistry // keyed by conversation_id — propose_spec_change / get_spec_progress
	runBuf  *progressRegistry // keyed by run_id — watch_run / get_run_progress
	chatBuf *progressRegistry // keyed by conversation_id — send_message / get_chat_progress

	// specBuf and chatBuf are deliberately separate registries even though a conversation_id can
	// be shared between propose_spec_change and send_message (chat.proto's own doc comment: both
	// RPCs write through the identical ConversationRepo/MessageRepo, so a conversation_id started
	// via one is valid on the other). Each RPC call still opens its own stream and drains into its
	// own buffer — sharing a registry would let a send_message call silently no-op against a
	// buffer a propose_spec_change call already reserved for the same key, discarding the turn.
}

// NewBlockBuilderToolset builds a toolset over caller-supplied client interfaces. Production
// code (buildToolsets, both binaries) passes the triple returned by the sibling client
// component's blockbuilder.NewClient(); tests pass sdktest fakes directly (D2,
// impl-specs/toolset.md correction 2).
//
// The three progress registries are given context.Background() as their own lifetime context —
// per golang-context, a long-lived structure (their idle reapers run for the process's life)
// holds only its own lifetime context, never a per-call/request-scoped one; buildToolsets has no
// narrower lifetime to offer at construction time, matching how the other toolsets it builds
// (tools.NewA2AToolset, tools.NewLLMSCachedToolset) are also constructed without a context.
func NewBlockBuilderToolset(spec sdk.SpecClient, run sdk.RunClient, chat sdk.ChatClient) *BlockBuilderToolset {
	ctx := context.Background()
	return &BlockBuilderToolset{
		spec:    spec,
		run:     run,
		chat:    chat,
		specBuf: newProgressRegistry(ctx),
		runBuf:  newProgressRegistry(ctx),
		chatBuf: newProgressRegistry(ctx),
	}
}

// Name implements adktool.Toolset.
func (t *BlockBuilderToolset) Name() string { return "block-builder" }

// Tools implements adktool.Toolset — constructs all 13 tools, collecting any construction errors
// with errors.Join rather than failing on the first one, so a caller sees every problem at once.
func (t *BlockBuilderToolset) Tools(_ agent.ReadonlyContext) ([]tool.Tool, error) {
	var tools []tool.Tool
	var errs []error

	add := func(tl tool.Tool, err error) {
		if err != nil {
			errs = append(errs, err)
			return
		}
		tools = append(tools, tl)
	}

	// Task 5 — plain-unary spec tools.
	add(acceptSpecTool(t.spec))
	add(validateSpecTool(t.spec))
	add(getSpecTool(t.spec))
	add(listPrimitivesTool(t.spec))

	// Task 7a — plain-unary run tools.
	add(startRunTool(t.run))
	add(getRunTool(t.run))
	add(getManifestTool(t.run))

	// Streaming component's long-running/buffered-poll tool pairs.
	add(NewProposeSpecChangeTool(t.spec, t.specBuf))
	add(NewGetSpecProgressTool(t.specBuf))
	add(NewWatchRunTool(t.run, t.runBuf))
	add(NewGetRunProgressTool(t.runBuf))
	add(NewSendMessageTool(t.chat, t.chatBuf))
	add(NewGetChatProgressTool(t.chatBuf))

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return tools, nil
}
