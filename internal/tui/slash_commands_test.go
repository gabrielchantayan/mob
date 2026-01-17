package tui

import "testing"

func TestFilterSlashCommands(t *testing.T) {
	cmds := []SlashCommand{{Name: "new", Description: "New chat"}, {Name: "status", Description: "Status"}}
	matches := FilterSlashCommands(cmds, "/n")
	if len(matches) != 1 || matches[0].Name != "new" {
		t.Fatalf("expected /new match")
	}
}

func TestFilterSlashCommandsSelection(t *testing.T) {
	cmds := []SlashCommand{{Name: "new"}, {Name: "status"}}

	if next := NextSlashIndex(0, len(cmds), 1); next != 1 {
		t.Fatalf("expected index 1, got %d", next)
	}
	if next := NextSlashIndex(1, len(cmds), 1); next != 0 {
		t.Fatalf("expected wrap to 0, got %d", next)
	}
	if next := NextSlashIndex(0, len(cmds), -1); next != 1 {
		t.Fatalf("expected wrap to 1, got %d", next)
	}
	if next := NextSlashIndex(0, 0, 1); next != 0 {
		t.Fatalf("expected index 0 for empty list, got %d", next)
	}
}
