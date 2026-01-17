package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRunUsesStartProgram(t *testing.T) {
	called := false
	original := startProgram
	startProgram = func(model tea.Model) error {
		called = true
		return nil
	}
	defer func() {
		startProgram = original
	}()

	if err := Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected startProgram to be called")
	}
}
