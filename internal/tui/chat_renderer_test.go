package tui

import "testing"

func TestTextareaHeightClamp(t *testing.T) {
	got := clampHeight(1)
	if got != 3 {
		t.Fatalf("expected min 3")
	}
	got = clampHeight(30)
	if got != 24 {
		t.Fatalf("expected max 24")
	}
	got = clampHeight(10)
	if got != 10 {
		t.Fatalf("expected in-range 10")
	}
}
