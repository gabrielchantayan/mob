# Mob TUI Design

Date: 2026-01-17

## Summary

Build a Bubble Tea–based TUI that recreates the OpenCode chat experience 1:1, scoped to `TUI.md` plus toasts and the inline ask-user-question chooser. The TUI includes four tabs (Chat, Daemon, Agent Output, Agents), a conditional right sidebar, a help bar, and a dynamic input area on the Chat tab.

## Goals

- Match the OpenCode chat UI look, layout, and interaction patterns from `TUI.md`.
- Provide full tab set + sidebar with consistent styling.
- Include inline ask-user-question chooser in chat and toast notifications.
- Keep architecture modular for future daemon/underboss integration.

## Non-Goals

- Full OpenCode command palette, session/theme/model pickers.
- Server-driven TUI API endpoints.
- Non-macOS platform support beyond what Bubble Tea already affords.

## Architecture

### Root Model

- `tui.Model` owns global UI state: active tab, sidebar visibility, window size, help mode, toasts, and shared data snapshots.
- Each tab has its own `viewport.Model` with independent scroll state.
- Chat tab includes a textarea input and a message renderer pipeline.
- Sidebar is conditionally rendered (width ≥ 120, or toggled).

### State/Data

- `tui.State` aggregates data for rendering:
  - daemon status + logs
  - agent output stream
  - agents table data
  - bead counts
  - usage/session metrics
  - chat conversation stream
- Event loop ingests streaming updates via channels and maps them into state.
- Each viewport supports auto-follow unless the user scrolls away.

## Layout

- Top tab bar: `[Chat] [Daemon] [Agent Output] [Agents]`.
- Main content: per-tab viewport.
- Chat-only input area: dynamic height (3–24 lines) with slash command popover.
- Right sidebar (fixed width 42): Status, Beads, Agents, Usage.
- Bottom help bar: keybind hints (context-aware for input/chooser).

## UI Components

### Chat Message Parts

- User message block (blue border, muted panel).
- Assistant blocks:
  - thinking blocks
  - tool-use blocks (icon + name + input/output)
  - tool-result blocks
  - plain text blocks
- Inline ask-user-question chooser:
  - Renders directly after the question block
  - Highlights selected option
  - Enter submits, Esc cancels

### Tabs

- **Daemon**: status line + log viewport (severity colors, cap 500 lines).
- **Agent Output**: timestamped lines, deterministic agent colors.
- **Agents**: table view with colored status badges.

### Toasts

- Overlay box in bottom-right of content area.
- Queue with auto-dismiss after duration.

## Commands & Keybinds

- Global: Tab/Shift+Tab, `[`, `]`, number keys, `Ctrl+C`, `:q` variants.
- Viewports: j/k, g/G, Ctrl+D/U/F/B, Page Up/Down.
- Chat: `i`/Enter focus, Esc blur, Shift+Enter newline.
- Slash command popover: Up/Down select, Enter accept.
- Inline chooser: Up/Down select, Enter submit, Esc cancel.

## Error Handling

- Stream errors shown via toasts and status warnings.
- Renderer falls back to plain text for unknown parts.
- Reconnect attempts for streams with backoff.

## Testing

- Unit tests for:
  - message-part rendering
  - chooser navigation logic
  - textarea height calculation
- Model tests for keybind routing and focus/chooser overrides.

## Integration Points

- Data sources for daemon, logs, agent output, and underboss chat are modeled as interfaces so they can be wired up later without touching UI logic.

## Milestones

1. Build TUI layout + styles.
2. Implement chat renderer + inline chooser.
3. Add other tabs + sidebar data wiring.
4. Add toasts and keybind map.
5. Add tests for core behaviors.
