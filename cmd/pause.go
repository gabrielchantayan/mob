package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/gabe/mob/internal/daemon"
	"github.com/spf13/cobra"
)

var (
	pauseHard bool
)

var pauseCmd = &cobra.Command{
	Use:     "pause",
	Aliases: []string{"p"},
	Short:   "Pause the daemon and all agents",
	Long: `Pause the mob daemon, stopping all active work.

By default, performs a graceful pause: completes current tasks before pausing.
Use --hard for immediate pause without waiting for task completion.`,
	Run: func(cmd *cobra.Command, args []string) {
		mobDir, err := getMobDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Check if daemon is running
		d := daemon.New(mobDir, log.New(io.Discard, "", 0))
		state, _, err := d.Status()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking daemon status: %v\n", err)
			os.Exit(1)
		}

		if state == daemon.StateIdle {
			fmt.Println(warningStyle.Render("Daemon is not running"))
			return
		}

		if state == daemon.StatePaused {
			fmt.Println(mutedStyle.Render("Daemon is already paused"))
			return
		}

		// Write pause state file
		stateFile := filepath.Join(mobDir, ".mob", "daemon.state")
		pauseType := "graceful"
		if pauseHard {
			pauseType = "hard"
		}

		err = os.WriteFile(stateFile, []byte(fmt.Sprintf("paused:%s", pauseType)), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to write pause state: %v\n", err)
			os.Exit(1)
		}

		if pauseHard {
			fmt.Println(successStyle.Render("✓") + " Daemon pause requested (hard)")
			fmt.Println(mutedStyle.Render("All agents will be stopped immediately"))
		} else {
			fmt.Println(successStyle.Render("✓") + " Daemon pause requested (graceful)")
			fmt.Println(mutedStyle.Render("Agents will finish current tasks before pausing"))
		}
	},
}

func init() {
	pauseCmd.Flags().BoolVar(&pauseHard, "hard", false, "Immediate pause without waiting for task completion")
	rootCmd.AddCommand(pauseCmd)
}
