package tui

import (
	"strings"
	"testing"
)

func TestViewIncludesTabs(t *testing.T) {
	m := NewModel()
	view := m.View()
	required := []string{"[Chat]", "[Daemon]", "[Agent Output]", "[Agents]"}
	for _, label := range required {
		if !strings.Contains(view, label) {
			t.Fatalf("missing tab %s", label)
		}
	}
}
