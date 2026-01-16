package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// m is a short alias wrapper for the mob command
// Usage: m <command> <args>
// Examples:
//   m a "Fix bug"          -> mob add "Fix bug"
//   m s                     -> mob status
//   m soldati ls           -> mob soldati ls
func main() {
	if len(os.Args) < 2 {
		// No args, just run mob
		runMob([]string{})
		return
	}

	// Expand short aliases
	alias := os.Args[1]
	expandedCmd := expandAlias(alias)

	// Build the full mob command
	args := []string{expandedCmd}
	if len(os.Args) > 2 {
		args = append(args, os.Args[2:]...)
	}

	runMob(args)
}

// expandAlias expands short aliases to full command names
func expandAlias(alias string) string {
	// Map of short aliases to full commands
	aliases := map[string]string{
		"a":  "add",
		"s":  "status",
		"st": "status",
		"l":  "logs",
		"ls": "status", // Common git muscle memory
		"t":  "tui",
		"d":  "daemon",
		"p":  "pause",
		"r":  "resume",
		"so": "soldati",
		"sw": "sweep",
		"h":  "heresy",
		"ap": "approve",
		"re": "reject",
		"rp": "reports",
		"ch": "chat",
		"tk": "ask",
		"tl": "tell",
		"tu": "turf",
		"dp": "deps",
		"nu": "nudge",
	}

	if expanded, ok := aliases[alias]; ok {
		return expanded
	}

	// If no alias match, return as-is (might be a full command name)
	return alias
}

// runMob executes the mob command with the given arguments
func runMob(args []string) {
	cmd := exec.Command("mob", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error running mob: %v\n", err)
		os.Exit(1)
	}
}
