package tui

import "testing"

func TestModelInitialTab(t *testing.T) {
	m := NewModel()
	if m.ActiveTab != TabChat {
		t.Fatalf("expected chat tab")
	}
}
