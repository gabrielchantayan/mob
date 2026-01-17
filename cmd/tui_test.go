package cmd

import "testing"

func TestTuiCommand(t *testing.T) {
	cmd := tuiCmd
	if cmd.Use != "tui" {
		t.Fatal("expected tui command")
	}
}
