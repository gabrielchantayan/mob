package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gabe/mob/internal/agent"
	"github.com/gabe/mob/internal/nudge"
	"github.com/gabe/mob/internal/soldati"
	"github.com/spf13/cobra"
)

var nudgeCmd = &cobra.Command{
	Use:     "nudge [soldati|all]",
	Aliases: []string{"n"},
	Short:   "Nudge stuck agents",
	Long: `Send a wake-up signal to stuck agents.

Use 'all' to nudge all agents, or specify a soldati name to nudge a specific agent.

Nudge levels:
  0 - Send newline to stdin (gentlest)
  1 - Update hook file with nudge signal
  2 - Kill and restart the agent (most aggressive)

Examples:
  mob nudge vinnie          # Nudge soldati 'vinnie' at level 0
  mob nudge vinnie -l 1     # Nudge soldati 'vinnie' at level 1 (hook)
  mob nudge all             # Nudge all soldati at level 0
  mob nudge all -l 2        # Kill and restart all soldati`,
	Run: runNudge,
}

func init() {
	nudgeCmd.Flags().IntP("level", "l", 0, "Nudge level (0=stdin, 1=hook, 2=restart)")
	rootCmd.AddCommand(nudgeCmd)
}

func runNudge(cmd *cobra.Command, args []string) {
	level, _ := cmd.Flags().GetInt("level")

	if level < 0 || level > 2 {
		fmt.Fprintf(os.Stderr, "Error: invalid nudge level %d (must be 0, 1, or 2)\n", level)
		os.Exit(1)
	}

	nudgeLevel := nudge.NudgeLevel(level)

	// Get the soldati directory
	soldatiDir, err := getSoldatiDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Get the hook base directory
	hookBase, err := getHookBaseDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create soldati manager to get list of soldati
	soldatiMgr, err := soldati.NewManager(soldatiDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create spawner and nudger
	spawner := agent.NewSpawner()
	nudger := nudge.New(spawner, hookBase)

	// Determine which soldati to nudge
	var targets []string

	if len(args) == 0 || args[0] == "all" {
		// Nudge all soldati
		list, err := soldatiMgr.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing soldati: %v\n", err)
			os.Exit(1)
		}

		if len(list) == 0 {
			fmt.Println("No soldati to nudge.")
			return
		}

		for _, s := range list {
			targets = append(targets, s.Name)
		}
	} else {
		// Nudge specific soldati
		targets = append(targets, args[0])

		// Verify the soldati exists
		if _, err := soldatiMgr.Get(args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: soldati %q not found\n", args[0])
			os.Exit(1)
		}
	}

	// For hook-based nudges (level 1), we don't need a running agent
	// We can write directly to the hook file
	if nudgeLevel == nudge.LevelHook {
		for _, name := range targets {
			// Create a virtual agent entry just to enable hook writing
			virtualAgent := &agent.Agent{
				ID:   fmt.Sprintf("virtual-%s", name),
				Type: agent.AgentTypeSoldati,
				Name: name,
			}
			nudger.RegisterAgent(virtualAgent, nil)

			err := nudger.NudgeByName(name, nudgeLevel)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to nudge %s: %v\n", name, err)
			} else {
				fmt.Printf("Nudged %s (level %d: %s)\n", name, level, nudgeLevel.String())
			}
		}
		return
	}

	// For stdin and restart nudges, we need the actual running agents
	// These would typically come from the daemon or a running session
	// For now, we'll just attempt the hook-based nudge as a fallback
	// since we may not have access to the actual process

	// In a full implementation, we would:
	// 1. Connect to the daemon via IPC
	// 2. Request the daemon to nudge the specified agents
	// 3. The daemon would use its spawner which tracks running processes

	fmt.Println("Note: stdin/restart nudges require running agents.")
	fmt.Println("Falling back to hook-based nudge for persistence...")

	for _, name := range targets {
		virtualAgent := &agent.Agent{
			ID:   fmt.Sprintf("virtual-%s", name),
			Type: agent.AgentTypeSoldati,
			Name: name,
		}
		nudger.RegisterAgent(virtualAgent, nil)

		// Use hook nudge as fallback
		err := nudger.NudgeByName(name, nudge.LevelHook)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to nudge %s: %v\n", name, err)
		} else {
			fmt.Printf("Nudged %s via hook file\n", name)
		}
	}
}

// getHookBaseDir returns the base directory for hook files
func getHookBaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, "mob", ".mob", "soldati"), nil
}
