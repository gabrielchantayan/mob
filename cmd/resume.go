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

var resumeCmd = &cobra.Command{
	Use:     "resume",
	Aliases: []string{"res"},
	Short:   "Resume the daemon from paused state",
	Long:  `Resume the mob daemon, allowing agents to continue working.`,
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
			fmt.Println(mutedStyle.Render("Start it with: mob daemon start"))
			return
		}

		if state == daemon.StateRunning {
			fmt.Println(mutedStyle.Render("Daemon is already running"))
			return
		}

		// Remove pause state file to resume
		stateFile := filepath.Join(mobDir, ".mob", "daemon.state")
		err = os.Remove(stateFile)
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: failed to clear pause state: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(successStyle.Render("✓") + " Daemon resumed")
		fmt.Println(mutedStyle.Render("Agents will resume work"))
	},
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}
