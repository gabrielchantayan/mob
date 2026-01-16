# Multiline Input & Underboss Delegation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add multiline/paste support to the TUI and enforce that the Underboss never writes code directly - always plans and delegates via beads.

**Architecture:**
1. Replace `textinput` with `textarea` component from bubbles library for multiline support
2. Use Alt+Enter (or Ctrl+Enter) for newlines, plain Enter to submit (like opencode)
3. Handle paste with CRLF normalization
4. Update Underboss system prompt to enforce planning-and-delegation behavior

**Tech Stack:** Go, Bubbletea, Bubbles textarea component

---

## Task 1: Replace textinput with textarea in TUI

**Files:**
- Modify: `internal/tui/tui.go:12` (imports)
- Modify: `internal/tui/tui.go:131` (Model struct)
- Modify: `internal/tui/tui.go:142-174` (New function)

**Step 1: Update imports**

Replace:
```go
"github.com/charmbracelet/bubbles/textinput"
```

With:
```go
"github.com/charmbracelet/bubbles/textarea"
```

**Step 2: Update Model struct**

Replace:
```go
chatInput       textinput.Model
```

With:
```go
chatInput       textarea.Model
```

**Step 3: Update New() function**

Replace the textinput initialization (lines 146-149):
```go
// Initialize text input - minimal style
ti := textinput.New()
ti.Placeholder = "Type a message..."
ti.CharLimit = 2000
ti.Width = 80
```

With:
```go
// Initialize textarea for multiline input
ti := textarea.New()
ti.Placeholder = "Type a message... (Alt+Enter for newline)"
ti.CharLimit = 10000
ti.SetWidth(80)
ti.SetHeight(3)
ti.ShowLineNumbers = false
ti.KeyMap.InsertNewline.SetKeys("alt+enter", "ctrl+enter")
```

**Step 4: Run to verify compilation**

Run: `cd /Users/gabe/Documents/Programming/mob && go build ./...`
Expected: Build succeeds (may have errors to fix in next steps)

---

## Task 2: Update key handling for textarea

**Files:**
- Modify: `internal/tui/tui.go:313-333` (Update method key handling)

**Step 1: Update key handling in Update()**

Replace the chat input key handling block (lines 315-333):
```go
if m.activeTab == tabChat && m.chatInput.Focused() {
    switch msg.String() {
    case "ctrl+c":
        m.quitting = true
        return m, tea.Quit
    case "esc":
        m.chatInput.Blur()
        return m, nil
    case "enter":
        if !m.chatWaiting && m.chatInput.Value() != "" {
            return m, m.sendMessage()
        }
        return m, nil
    }
    // Update text input
    var cmd tea.Cmd
    m.chatInput, cmd = m.chatInput.Update(msg)
    return m, cmd
}
```

With:
```go
if m.activeTab == tabChat && m.chatInput.Focused() {
    switch msg.String() {
    case "ctrl+c":
        m.quitting = true
        return m, tea.Quit
    case "esc":
        m.chatInput.Blur()
        return m, nil
    case "enter":
        // Plain enter submits (unless empty)
        if !m.chatWaiting && strings.TrimSpace(m.chatInput.Value()) != "" {
            return m, m.sendMessage()
        }
        return m, nil
    }
    // Update textarea (handles alt+enter for newlines internally)
    var cmd tea.Cmd
    m.chatInput, cmd = m.chatInput.Update(msg)
    return m, cmd
}
```

**Step 2: Run to verify**

Run: `cd /Users/gabe/Documents/Programming/mob && go build ./...`
Expected: Build succeeds

---

## Task 3: Update layout calculations for textarea

**Files:**
- Modify: `internal/tui/tui.go:176-200` (updateLayout)
- Modify: `internal/tui/tui.go:358-362` (focus handling)

**Step 1: Update updateLayout() for textarea dimensions**

Replace line 199:
```go
m.chatInput.Width = mainWidth - 8
```

With:
```go
m.chatInput.SetWidth(mainWidth - 8)
```

**Step 2: Update focus handling**

Replace lines 358-362:
```go
case "i", "enter":
    if m.activeTab == tabChat && !m.chatWaiting {
        m.chatInput.Focus()
        return m, textinput.Blink
    }
```

With:
```go
case "i", "enter":
    if m.activeTab == tabChat && !m.chatWaiting {
        m.chatInput.Focus()
        return m, textarea.Blink
    }
```

**Step 3: Run to verify**

Run: `cd /Users/gabe/Documents/Programming/mob && go build ./...`
Expected: Build succeeds

---

## Task 4: Update sendMessage to handle multiline input

**Files:**
- Modify: `internal/tui/tui.go:400-438` (sendMessage)

**Step 1: Update sendMessage() to clear textarea properly**

Replace lines 401-402:
```go
message := m.chatInput.Value()
m.chatInput.SetValue("")
```

With:
```go
message := m.chatInput.Value()
m.chatInput.Reset()
```

**Step 2: Run to verify**

Run: `cd /Users/gabe/Documents/Programming/mob && go build ./...`
Expected: Build succeeds

---

## Task 5: Add paste handling with CRLF normalization

**Files:**
- Modify: `internal/tui/tui.go:263-378` (Update method)

**Step 1: Add paste message type at top of file**

Add after line 94 (after streamDoneMsg):
```go
// pasteMsg is sent when text is pasted
type pasteMsg struct {
    text string
}
```

**Step 2: Add paste handling in Update()**

Add a new case in the switch statement after the streamDoneMsg case (around line 312):
```go
case tea.PasteMsg:
    // Normalize line endings (Windows CRLF -> LF)
    normalized := strings.ReplaceAll(string(msg), "\r\n", "\n")
    normalized = strings.ReplaceAll(normalized, "\r", "\n")
    m.chatInput.InsertString(normalized)
    return m, nil
```

**Step 3: Enable paste in tea.Program**

Update the Run() function (line 1024):
```go
func Run() error {
    p := tea.NewProgram(New(), tea.WithAltScreen(), tea.WithMouseCellMotion())
    _, err := p.Run()
    return err
}
```

**Step 4: Run to verify**

Run: `cd /Users/gabe/Documents/Programming/mob && go build ./...`
Expected: Build succeeds

---

## Task 6: Update help text for new keybindings

**Files:**
- Modify: `internal/tui/tui.go:973-1020` (renderHelp)

**Step 1: Update help items when input is focused**

Replace lines 996-1003:
```go
if m.chatInput.Focused() {
    items = []struct {
        key  string
        desc string
    }{
        {"enter", "send"},
        {"esc", "cancel"},
    }
```

With:
```go
if m.chatInput.Focused() {
    items = []struct {
        key  string
        desc string
    }{
        {"enter", "send"},
        {"alt+enter", "newline"},
        {"esc", "cancel"},
    }
```

**Step 2: Run to verify**

Run: `cd /Users/gabe/Documents/Programming/mob && go build ./...`
Expected: Build succeeds

---

## Task 7: Update Underboss system prompt for delegation-only behavior

**Files:**
- Modify: `internal/underboss/prompts.go`

**Step 1: Replace the entire DefaultSystemPrompt**

Replace the entire const (lines 4-35):
```go
const DefaultSystemPrompt = `You are the Underboss in a mob-themed agent orchestration system.

## Style

Be brief and direct. Light mob flavor - occasional terms like "the boys", "crew", "turf" - but don't overdo it. Efficiency over theatrics.

## Your Role

You manage:
- **Soldati**: Persistent workers with names. For complex work.
- **Associates**: Temp workers. For quick tasks.

Break tasks into Beads, assign to crew, monitor progress.

## Tools

- spawn_soldati - Create persistent worker
- spawn_associate - Create temp worker
- list_agents - Show crew
- get_agent_status - Check on agent
- kill_agent - Remove agent
- nudge_agent - Ping stuck agent
- assign_bead - Assign work

## Guidelines

- Be concise. Short responses.
- Delegate proactively
- Handle issues yourself when possible
- Don't waste the boss's time
`
```

With:
```go
const DefaultSystemPrompt = `You are the Underboss in a mob-themed agent orchestration system.

## Style

Be brief and direct. Light mob flavor - occasional terms like "the boys", "crew", "turf" - but don't overdo it. Efficiency over theatrics.

## Your Role

You are an orchestrator and planner - YOU NEVER WRITE CODE YOURSELF.

You manage:
- **Soldati**: Persistent workers with names. For complex work.
- **Associates**: Temp workers. For quick tasks.

## CRITICAL RULE: No Direct Code

You NEVER write, edit, or modify code directly. Instead you:
1. **Plan** - Break work into Beads (atomic tasks)
2. **Delegate** - Assign Beads to Soldati or Associates
3. **Monitor** - Track progress, unblock workers

When the Don asks you to implement something:
1. Create a plan with clear Beads
2. Spawn workers (Soldati for complex/ongoing, Associates for quick tasks)
3. Assign Beads to workers
4. Report back on progress

If asked to "just do it yourself" or write code directly, explain that your role is orchestration and delegate to a worker.

## Tools

- spawn_soldati - Create persistent worker
- spawn_associate - Create temp worker
- list_agents - Show crew
- get_agent_status - Check on agent
- kill_agent - Remove agent
- nudge_agent - Ping stuck agent
- assign_bead - Assign work to agent

## Bead Workflow

1. Break task into Beads (atomic units of work)
2. Spawn appropriate worker(s)
3. Assign Bead(s) to worker(s)
4. Monitor and report progress

## Guidelines

- Be concise. Short responses.
- Always plan before delegating
- Track all work via Beads
- Don't waste the boss's time with unnecessary details
`
```

**Step 2: Run to verify**

Run: `cd /Users/gabe/Documents/Programming/mob && go build ./...`
Expected: Build succeeds

---

## Task 8: Test the TUI manually

**Step 1: Build and run**

Run: `cd /Users/gabe/Documents/Programming/mob && go build -o mob ./cmd/mob && ./mob tui`

**Step 2: Test multiline input**

1. Press `i` or `enter` to focus input
2. Type "Hello"
3. Press Alt+Enter (or Ctrl+Enter) - should insert newline
4. Type "World"
5. Press Enter - should submit both lines

**Step 3: Test paste**

1. Copy multiline text from elsewhere
2. Paste into the TUI input
3. Verify newlines are preserved

**Step 4: Test delegation prompt**

1. Ask the Underboss to "write a hello world function"
2. Verify it creates a plan and delegates rather than writing code

---

## Task 9: Commit changes

**Step 1: Stage and commit**

```bash
cd /Users/gabe/Documents/Programming/mob
git add internal/tui/tui.go internal/underboss/prompts.go
git commit -m "feat: add multiline input support and enforce underboss delegation

- Replace textinput with textarea for multiline support
- Alt+Enter or Ctrl+Enter for newlines, Enter to submit
- Handle paste with CRLF normalization
- Update Underboss prompt to never write code directly
- Underboss now plans and delegates all work via Beads"
```

---

## Summary

| Task | Description |
|------|-------------|
| 1 | Replace textinput with textarea component |
| 2 | Update key handling for textarea |
| 3 | Update layout calculations |
| 4 | Update sendMessage for textarea |
| 5 | Add paste handling with CRLF normalization |
| 6 | Update help text |
| 7 | Update Underboss prompt for delegation-only |
| 8 | Manual testing |
| 9 | Commit changes |
