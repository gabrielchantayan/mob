package tui

import "testing"

func TestTextareaHeightClamp(t *testing.T) {
	got := clampHeight(1)
	if got != 3 {
		t.Fatalf("expected min 3")
	}
}
