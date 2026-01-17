package cmd

import (
	"errors"
	"testing"

	"github.com/gabe/mob/internal/tui"
)

func TestTuiCommand(t *testing.T) {
	cmd := tuiCmd
	if cmd.Use != "tui" {
		t.Fatal("expected tui command")
	}
	if cmd.RunE == nil {
		t.Fatal("expected RunE")
	}
	err := cmd.RunE(cmd, []string{})
	if !errors.Is(err, tui.ErrNotImplemented) {
		t.Fatal("expected not implemented error")
	}
}
