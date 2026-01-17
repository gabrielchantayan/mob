# TUI Task 2 Scope Trim Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan.

**Goal:** Remove the extra `TabLabel` style field/tests so only Task 2 scaffold requirements remain.

**Architecture:** Delete the `TabLabel` field from `Styles`, remove its initializer, and delete tests asserting it. Add a focused test to assert no `TabLabel` field exists to prevent regression. Keep existing style palette test.

**Tech Stack:** Go, testing package.

### Task 1: Add failing test for absent TabLabel

**Files:**
- Modify: `internal/tui/styles_test.go`
- Test: `internal/tui/styles_test.go`

**Step 1: Write the failing test**

```go
func TestStylesHasNoTabLabel(t *testing.T) {
	styles := NewStyles()
	typeOf := reflect.TypeOf(styles)
	if _, ok := typeOf.FieldByName("TabLabel"); ok {
		t.Fatal("tab label style should not exist")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestStylesHasNoTabLabel`
Expected: FAIL with "tab label style should not exist"

**Step 3: Write minimal implementation**

Remove the `TabLabel` field and initializer from `internal/tui/styles.go`.

**Step 4: Update tests to match**

Remove `TestStylesTabLabel` and add `reflect` import.

**Step 5: Run test to verify it passes**

Run: `go test ./internal/tui -run TestStylesHasNoTabLabel`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/tui/styles.go internal/tui/styles_test.go
git commit -m "fix: trim TUI scaffold"
```
