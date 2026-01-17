# Mob TUI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement a Bubble Tea TUI that matches `TUI.md` + toasts + inline ask-user-question chooser.

**Architecture:** Add a new `internal/tui` package with a root model, tab viewports, chat renderer, styles, and data state. Wire `cmd/tui.go` to launch it and keep data sources stubbed behind interfaces so real integrations can be added later without UI changes.

**Tech Stack:** Go, Bubble Tea, Bubbles (viewport, textarea), Lip Gloss.

### Task 1: Add TUI package scaffold and styles

**Files:**
- Create: `internal/tui/tui.go`
- Create: `internal/tui/styles.go`
- Test: `internal/tui/styles_test.go`

**Step 1: Write the failing test**

```go
func TestStylesPalette(t *testing.T) {
	styles := NewStyles()
	if styles.Primary == "" {
		t.Fatal("primary color missing")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestStylesPalette`
Expected: FAIL with "no required module provides package github.com/gabe/mob/internal/tui" or missing symbols.

**Step 3: Write minimal implementation**

```go
type Styles struct {
	Primary string
}

func NewStyles() Styles {
	return Styles{Primary: "#fab283"}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -run TestStylesPalette`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/styles.go internal/tui/styles_test.go
git commit -m "feat: add initial TUI styles"
```

### Task 2: Implement root model layout and tabs

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/styles.go`
- Test: `internal/tui/tui_test.go`

**Step 1: Write the failing test**

```go
func TestModelInitialTab(t *testing.T) {
	m := NewModel()
	if m.ActiveTab != TabChat {
		t.Fatalf("expected chat tab")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestModelInitialTab`
Expected: FAIL with "NewModel undefined".

**Step 3: Write minimal implementation**

```go
const (
	TabChat = iota
	TabDaemon
	TabAgentOutput
	TabAgents
)

type Model struct {
	ActiveTab int
}

func NewModel() Model {
	return Model{ActiveTab: TabChat}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -run TestModelInitialTab`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/styles.go internal/tui/tui_test.go
git commit -m "feat: add TUI model scaffold"
```

### Task 3: Chat rendering + textarea behavior

**Files:**
- Modify: `internal/tui/tui.go`
- Create: `internal/tui/chat_renderer.go`
- Test: `internal/tui/chat_renderer_test.go`

**Step 1: Write the failing test**

```go
func TestTextareaHeightClamp(t *testing.T) {
	got := clampHeight(1)
	if got != 3 {
		t.Fatalf("expected min 3")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestTextareaHeightClamp`
Expected: FAIL with "clampHeight undefined".

**Step 3: Write minimal implementation**

```go
func clampHeight(height int) int {
	if height < 3 {
		return 3
	}
	if height > 24 {
		return 24
	}
	return height
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -run TestTextareaHeightClamp`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/chat_renderer.go internal/tui/chat_renderer_test.go
git commit -m "feat: add chat renderer helpers"
```

### Task 4: Inline chooser for ask-user-question

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/chat_renderer.go`
- Test: `internal/tui/chooser_test.go`

**Step 1: Write the failing test**

```go
func TestChooserSelectNext(t *testing.T) {
	chooser := NewChooser([]string{"A", "B"})
	chooser.Next()
	if chooser.Index != 1 {
		t.Fatalf("expected index 1")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestChooserSelectNext`
Expected: FAIL with "NewChooser undefined".

**Step 3: Write minimal implementation**

```go
type Chooser struct {
	Options []string
	Index   int
}

func NewChooser(options []string) *Chooser {
	return &Chooser{Options: options}
}

func (c *Chooser) Next() {
	if len(c.Options) == 0 {
		return
	}
	c.Index = (c.Index + 1) % len(c.Options)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -run TestChooserSelectNext`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/chat_renderer.go internal/tui/chooser_test.go
git commit -m "feat: add inline chooser"
```

### Task 5: Sidebar + toasts + other tabs

**Files:**
- Modify: `internal/tui/tui.go`
- Create: `internal/tui/toast.go`
- Create: `internal/tui/sidebar.go`
- Create: `internal/tui/tab_daemon.go`
- Create: `internal/tui/tab_agent_output.go`
- Create: `internal/tui/tab_agents.go`
- Test: `internal/tui/toast_test.go`

**Step 1: Write the failing test**

```go
func TestToastQueue(t *testing.T) {
	queue := NewToastQueue()
	queue.Push(Toast{Message: "hi"})
	if queue.Len() != 1 {
		t.Fatal("expected toast")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestToastQueue`
Expected: FAIL with "NewToastQueue undefined".

**Step 3: Write minimal implementation**

```go
type ToastQueue struct {
	items []Toast
}

func NewToastQueue() *ToastQueue {
	return &ToastQueue{}
}

func (q *ToastQueue) Push(toast Toast) {
	q.items = append(q.items, toast)
}

func (q *ToastQueue) Len() int {
	return len(q.items)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -run TestToastQueue`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/toast.go internal/tui/sidebar.go internal/tui/tab_daemon.go internal/tui/tab_agent_output.go internal/tui/tab_agents.go internal/tui/toast_test.go
git commit -m "feat: add sidebar, toasts, and tabs"
```

### Task 6: Wire `mob tui` entrypoint

**Files:**
- Modify: `cmd/tui.go`
- Test: `cmd/tui_test.go`

**Step 1: Write the failing test**

```go
func TestTuiCommand(t *testing.T) {
	cmd := tuiCmd
	if cmd.Use != "tui" {
		t.Fatal("expected tui command")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd -run TestTuiCommand`
Expected: FAIL with "undefined: tuiCmd" if not exposed.

**Step 3: Write minimal implementation**

```go
// Ensure tuiCmd remains in package scope and uses tui.Run()
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd -run TestTuiCommand`
Expected: PASS.

**Step 5: Commit**

```bash
git add cmd/tui.go cmd/tui_test.go
git commit -m "test: cover tui command"
```

### Task 7: Full TUI snapshot test (optional)

**Files:**
- Test: `internal/tui/tui_view_test.go`

**Step 1: Write the failing test**

```go
func TestViewIncludesTabs(t *testing.T) {
	m := NewModel()
	view := m.View()
	if !strings.Contains(view, "[Chat]") {
		t.Fatal("missing tabs")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestViewIncludesTabs`
Expected: FAIL with "missing tabs".

**Step 3: Write minimal implementation**

```go
// Ensure tab bar renders on View().
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -run TestViewIncludesTabs`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/tui/tui_view_test.go
git commit -m "test: add basic view snapshot"
```
