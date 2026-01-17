package tui

import (
	"strings"
	"testing"
)

func TestViewIncludesTabs(t *testing.T) {
	m := NewModel()
	view := m.View()
	if !strings.Contains(view, "[Chat]") {
		t.Fatal("missing tabs")
	}
}
