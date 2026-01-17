package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestOpencodePalette(t *testing.T) {
	if bgColor != lipgloss.Color("#0a0a0a") {
		t.Fatalf("unexpected bgColor: %v", bgColor)
	}
	if backgroundMenuColor != lipgloss.Color("#1e1e1e") {
		t.Fatalf("unexpected backgroundMenuColor: %v", backgroundMenuColor)
	}
}
