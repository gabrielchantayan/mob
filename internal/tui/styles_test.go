package tui

import "testing"

func TestStylesPalette(t *testing.T) {
	styles := NewStyles()
	if styles.Primary == "" {
		t.Fatal("primary color missing")
	}
}
