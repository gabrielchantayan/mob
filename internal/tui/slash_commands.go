package tui

import "strings"

type SlashCommand struct {
	Name        string
	Description string
	Shortcut    string
}

func FilterSlashCommands(commands []SlashCommand, input string) []SlashCommand {
	query := strings.TrimPrefix(strings.TrimSpace(input), "/")
	if query == "" {
		return commands
	}
	var out []SlashCommand
	for _, cmd := range commands {
		if strings.HasPrefix(cmd.Name, query) {
			out = append(out, cmd)
		}
	}
	return out
}

func NextSlashIndex(current int, total int, delta int) int {
	if total <= 0 {
		return 0
	}
	updated := current + delta
	if updated < 0 {
		return total - 1
	}
	if updated >= total {
		return 0
	}
	return updated
}
