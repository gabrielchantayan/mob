package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gabe/mob/internal/daemon"
	"github.com/gabe/mob/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mob",
	Short: "Mob - Claude Code Agent Orchestrator",
	Long:  `A mafia-themed agent orchestrator for managing multiple Claude Code instances.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get mob directory
		mobDir, err := getMobDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		d := daemon.New(mobDir, log.New(io.Discard, "", 0))

		// Check if daemon is already running
		state, _, err := d.Status()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking daemon status: %v\n", err)
			os.Exit(1)
		}

		var daemonStartedByUs bool
		if state == daemon.StateIdle {
			// Daemon not running, start it in a goroutine
			daemonStartedByUs = true
			errChan := make(chan error, 1)
			go func() {
				if err := d.Start(); err != nil {
					errChan <- err
				}
			}()

			// Give the daemon a moment to start up
			select {
			case err := <-errChan:
				fmt.Fprintf(os.Stderr, "Error starting daemon: %v\n", err)
				os.Exit(1)
			case <-time.After(500 * time.Millisecond):
				// Daemon started successfully (no immediate error)
			}
		}

		// Run the TUI - this blocks until user exits
		tuiErr := tui.Run()

		// Clean up daemon if we started it
		if daemonStartedByUs {
			stopDaemon(mobDir)
		}

		if tuiErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", tuiErr)
			os.Exit(1)
		}
	},
}

// stopDaemon sends SIGTERM to the daemon process
func stopDaemon(mobDir string) {
	pidFile := filepath.Join(mobDir, ".mob", "daemon.pid")

	pid, err := daemon.ReadPID(pidFile)
	if os.IsNotExist(err) {
		return // Daemon not running
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read daemon PID: %v\n", err)
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not find daemon process: %v\n", err)
		return
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not stop daemon: %v\n", err)
	}
}

func Execute() error {
	return rootCmd.Execute()
}
