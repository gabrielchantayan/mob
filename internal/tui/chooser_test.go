package tui

import "testing"

func TestChooserSelectNext(t *testing.T) {
	chooser := NewChooser([]string{"A", "B"})
	chooser.Next()
	if chooser.Index != 1 {
		t.Fatalf("expected index 1")
	}
}
