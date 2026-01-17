# Mob TUI Documentation

The TUI is a terminal dashboard built using **Bubble Tea** (charmbracelet/bubbletea) that provides an interactive interface for managing and monitoring the mob agent system. It's launched via the `mob tui` command.

## Layout Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│ ▌ [Chat] [Daemon] [Agent Output] [Agents]                              │
├─────────────────────────────────────────────────────────┬───────────────┤
│                                                         │               │
│  Content Viewport                                       │ Sidebar:      │
│  ┌──────────────────────────────────────────────────┐  │ • Status      │
│  │ Scrollable content area                          │  │ • Beads       │
│  │ (j/k, Page Up/Down, etc.)                        │  │ • Agents      │
│  │                                                  │  │ • Usage       │
│  └──────────────────────────────────────────────────┘  │               │
│                                                         │               │
│  Input Area (Chat tab only, dynamic height 3-24 lines) │               │
│  ┌───────────────────────────────────────────────────┐ │               │
│  │ Type message... Shift+Enter for newline           │ │               │
│  │ /new · Ctrl+N  — New chat                         │ │               │
│  └───────────────────────────────────────────────────┘ │               │
│                                                         │               │
├─────────────────────────────────────────────────────────┴───────────────┤
│ Help: Tab navigate  1-4 jump  S sidebar  :q quit                        │
└─────────────────────────────────────────────────────────────────────────┘
```

## Main Tabs

### Tab 1: Chat
- Interactive chat interface with the Underboss 
- Streaming response support with real-time thinking and tool use display
- Multiline input with dynamic height expansion (3-24 lines)
- Slash command support with autocomplete
- Shows user messages, assistant thinking, tool calls, and responses
- Scrollable viewport for conversation history

### Tab 2: Daemon
- Daemon process status (running/stopped) with PID
- Real-time daemon log viewer (last 500 lines)
- Color-coded log entries by severity (error, warning, info)
- Auto-scrolls to bottom on new logs

### Tab 3: Agent Output
- Live stream of agent execution output
- Timestamp and agent name for each line
- Color-coded by agent (deterministic colors based on agent name)
- Supports stderr vs stdout (different styling)

### Tab 4: Agents
- Tabular view of all active agents
- Shows: Name, Type, Status, Current Task, Last Ping
- Color-coded status (active=green, idle=muted, stuck=red)
- Relative time formatting for last activity

## Right Sidebar

Visible when terminal width >= 120 characters. Contains:

**Status Section:**
- Daemon status (running/stopped with PID)
- Current model being used
- Session duration

**Beads Section:**
- Count of beads by status:
  - "In progress" (●)
  - "Pending approval" (⚠)
  - "Open" (○)
  - "Closed" (✓)

**Agents Section:**
- List of active soldati (agents)
- Status indicator (active/idle)

**Usage Section:**
- Session duration
- Token usage (input/output split)
- Total session cost in dollars

## Key Bindings

### Global Shortcuts

| Key | Action |
|-----|--------|
| `Tab` / `Right` / `]` | Next tab |
| `Shift+Tab` / `Left` / `[` | Previous tab |
| `1`, `2`, `3`, `4` | Jump to specific tab |
| `Ctrl+C` | Quit |
| `:q`, `:qa`, `:qa!` | Vim-style quit |

### Chat Tab (Input Focused)

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `Shift+Enter` / `Ctrl+Enter` / `Alt+Enter` | Insert newline |
| `Esc` | Unfocus input |
| `Ctrl+N` | Scroll viewport down 1 line |
| `Ctrl+P` | Scroll viewport up 1 line |
| `Ctrl+D` | Half-page down |
| `Ctrl+U` | Half-page up |
| `Ctrl+F` | Page down |
| `Ctrl+B` | Page up |
| `Up` / `Down` | Navigate slash commands (when visible) |

### Chat Tab (Input Not Focused)

| Key | Action |
|-----|--------|
| `i` / `Enter` | Focus input |
| `j` | Scroll down 1 line |
| `k` | Scroll up 1 line |
| `Ctrl+D` | Half-page down |
| `Ctrl+U` | Half-page up |
| `Ctrl+F` | Page down |
| `Ctrl+B` | Page up |
| `g` | Jump to top |
| `G` | Jump to bottom |

### Other Tabs (Daemon, Agent Output, Agents)

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll down/up |
| `Ctrl+D` / `Ctrl+U` | Half-page navigation |
| `Ctrl+F` / `Ctrl+B` | Page navigation |
| `g` / `G` | Top/bottom navigation |

### Mouse Support
- Mouse wheel scrolling in all viewports

## Slash Commands

| Command | Shortcut | Description |
|---------|----------|-------------|
| `/new` | `Ctrl+N` | Clear chat history, reset session |
| `/status` | - | Show daemon status |

Commands show in a popover below input as you type `/`. Use Up/Down to navigate, Enter to select.

## Visual Elements

### Tool Icons
| Icon | Tool |
|------|------|
| `$` | Bash |
| `→` | Read |
| `✱` | Grep/Glob search |
| `←` | Write/Edit |
| `◉` | Todo/Task |
| `◐` | Thinking |

### Status Symbols
| Symbol | Meaning |
|--------|---------|
| `●` | In progress |
| `○` | Open |
| `✓` | Closed |
| `⚠` | Pending approval |

## Color Scheme

OpenCode-inspired dark theme:

- **Background**: #0a0a0a (main), #141414 (panels)
- **Primary**: #fab283 (warm peach/orange for labels)
- **Secondary**: #5c9cf5 (blue for accents)
- **Accent**: #9d7cd8 (purple)
- **Text**: #eeeeee (primary), #808080 (muted)
- **Error**: #e06c75 (red)
- **Warning**: #f5a742 (orange)
- **Success**: #7fd88f (green)
- **Info**: #56b6c2 (cyan)

## Layout Constants

| Constant | Value | Description |
|----------|-------|-------------|
| Sidebar width | 42 chars | Fixed when visible |
| Min width for sidebar | 120 chars | Below this, sidebar is hidden |
| Min input height | 3 lines | Minimum textarea height |
| Max input height | 24 lines | Maximum textarea height |
| Max daemon log lines | 500 | Memory optimization |
| Sidebar refresh rate | 2 seconds | Periodic update interval |

## Chat Message Display

### User Messages
- Blue left border (┃)
- Muted background panel
- Word-wrapped to viewport width

### Assistant Messages
- **Thinking blocks**: Reasoning with expandable summary
- **Tool use blocks**: Icon + tool name with styled input/output boxes
- **Tool result blocks**: Execution output in bordered boxes
- **Text blocks**: Normal message content
- Footer showing model name and response duration

## Data Integration

The TUI connects to:
- **Underboss**: Claude AI interactions
- **Daemon Manager**: Status and logs
- **Bead Storage**: Task statuses
- **Soldati Manager**: Active agents
- **Agent Registry**: Agent metadata

## File Structure

| File | Description |
|------|-------------|
| `internal/tui/tui.go` | Main TUI model and rendering |
| `internal/tui/styles.go` | Color scheme and style definitions |
| `internal/tui/slash_commands.go` | Slash command parsing |
| `internal/tui/message_parts.go` | Assistant response processing |
| `cmd/tui.go` | CLI command entry point |
