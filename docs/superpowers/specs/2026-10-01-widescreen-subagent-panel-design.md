# Widescreen subagent panel

## Goal

On very wide terminals, add a third column to the left of the chat transcript that shows a live tail of running subagents. Narrower terminals are unchanged.

## Decisions

- D1. The panel is a passive viewer. No input, no focus, no scrollback, no mouse or key handling inside it.
- D2. Subagent cards stay inline in the chat transcript. The panel is an additional view, not a replacement.
- D3. Concurrent subagents are stacked, most recently active first, capped at 4. A `+N more` footer line appears only when more than 4 are pending.
- D4. When no subagent is pending the panel keeps its width and shows a dim `No active subagents` placeholder, so the layout never reflows mid-session.
- D5. Activation is automatic by terminal width only. No toggle command or keybind.
- D6. The panel is rendered fresh each frame from existing state. No new event channel, message type, or persisted model state.

## Architecture

Layout becomes three columns when wide enough:

```
| subagent panel | chat body + rail | sidebar |
        status bar + input (full width, unchanged)
```

### New file `internal/tui/subagent_panel.go`

Modeled on `sidebar.go`: a `SubagentPanelRenderInput` struct (width, height, palette, messages) and a pure `RenderSubagentPanel(in) string`.

Data source is `chatModel.Messages`. A subagent is a card where `tool` is `agent`, `subagent`, or `a2a` and `toolPending()` is true. A2A cards use `agentLabel` in the header (for example `agent[istio-agent]`), matching the chat. `handleAgentSubEvent` already appends live events to `agentEvents` on these cards, so no routing changes are needed.

Recency order: later message index is treated as more recent. No timestamp is added.

Per subagent block: header `agent[<type or label>] <title>`, then the tail of `agentEvents` (kind and content), plain clipped text, no glamour. Panel height is divided evenly among shown subagents, the last absorbing the remainder. A card with empty `agentEvents` renders the header only.

### Layout changes in `tui.go` / `minimap.go`

- `panelWidths(width) (left, main, sidebar int)` becomes the single source of width thresholds:
  - `width <= 80`: `(0, width, 0)`
  - `80 < width <= SubagentPanelMinTotalWidth`: `(0, width-SidebarWidth, SidebarWidth)` (today's behavior)
  - `width > SubagentPanelMinTotalWidth`: `(SubagentPanelWidth, width-SubagentPanelWidth-SidebarWidth, SidebarWidth)`
- Constants: `SubagentPanelWidth = 44`, `SubagentPanelMinTotalWidth = SubagentPanelWidth + SidebarWidth + 80`.
- `mainWidth()` and `chatWidth()` keep their signatures and delegate to `panelWidths`. `View()` stops computing `sidebarWidth := m.width - mainWidth` inline.
- `joinPanelSidebar` is generalized to `joinPanels(cols ...panelColumn)` with `panelColumn{content string; width int}`. Each row of each column is clamped to its width, preserving the existing stray-wide-row protection. `View()` calls it once with the subagent panel, the chat body+rail, and the sidebar.
- Panel height equals `panelRows`, the same rule the sidebar follows.

### Mouse and selection

- New `chatXOffset()` returns the left panel width when shown, else 0.
- `clampToChat` computes X as `min(max(mouseX-offset, 0), chatWidth-1)`.
- `handleMouseClick` treats `mouse.X < offset || mouse.X >= offset+chatWidth()` as off-panel: clears the selection, does nothing else.
- `hitContextClear` is unchanged. The gauge row is full width.
- `selectedText` and `highlight` take columns relative to the frame. Both must be checked and adjusted for the offset (see Open items).

## Edge cases

- Resize across the threshold mid-turn: the panel appears or disappears next frame. It holds no state, so there is no teardown.
- A session with no subagents on a wide terminal reserves the panel width for the whole session. This is the accepted cost of D4.
- Matrix bar sizing calls `m.mainWidth()`. That value shrinks by the panel width on widescreen; verify the matrix bar still fits `chatWidth`.

## Testing

- T1. `panelWidths` at 80, 81, `SubagentPanelMinTotalWidth`, and `SubagentPanelMinTotalWidth+1`.
- T2. Selection of pending subagent cards: ordering, cap of 4, `+N more` footer, empty-events card, no pending cards (placeholder).
- T3. `joinPanels` with two columns reproduces current `joinPanelSidebar` output. Three columns with a stray wide row keeps all column starts fixed.
- T4. `clampToChat` and `handleMouseClick` with the panel shown: click left of the offset clears selection, click inside maps to the correct chat column.
- T5. Frame-level: with width above the threshold every frame row has width `m.width`, and the panel text appears in the left columns.
- T6. Fixtures are built from real `message` values, not hand-written maps.
- T7. A2A cards appear in the panel with the `agentLabel` header.
- T8. Frame-level: a subagent spawned through the ACP spawner shows its live events in the panel tail.

## Open items to resolve in the plan

- O1. `selectedText(m.lastFrame, m.sel, m.chatWidth())` and `highlight(final, m.sel, m.chatWidth(), m.width)` index into the full frame. Confirm whether they assume the chat starts at column 0 and add the offset.
- O2. `railColumn` and `renderMinimap` take chat-relative rows only, expected unaffected. Confirm.
- O4. Confirm `internal/subagent/spawner_acp.go` emits live events onto `AgentEventCh`. If it does not, ACP-driven subagents would show a header with no tail. Either fix the emission in this work or split it into a follow-up.
- O3. Windows CI: tests touching `NewSandbox` or `rg` are not involved here, but frame tests must not depend on terminal state.
