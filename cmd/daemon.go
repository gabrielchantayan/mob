package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/gabe/mob/internal/daemon"
	"github.com/spf13/cobra"
)

var debug bool

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the mob daemon",
	Long:  `Start, stop, and check the status of the mob daemon process.`,
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the mob daemon",
	Run: func(cmd *cobra.Command, args []string) {
		mobDir, err := getMobDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Always log to daemon.log file for TUI viewing
		logDir := filepath.Join(mobDir, ".mob")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating log directory: %v\n", err)
			os.Exit(1)
		}
		logFile, err := os.OpenFile(filepath.Join(logDir, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
			os.Exit(1)
		}
		defer logFile.Close()

		var out io.Writer = logFile
		if debug {
			// In debug mode, write to both stdout and log file
			out = io.MultiWriter(os.Stdout, logFile)
		}
		logger := log.New(out, "", log.LstdFlags)

		d := daemon.New(mobDir, logger)

		if err := d.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the mob daemon",
	Run: func(cmd *cobra.Command, args []string) {
		mobDir, err := getMobDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		pidFile := filepath.Join(mobDir, ".mob", "daemon.pid")

		pid, err := daemon.ReadPID(pidFile)
		if os.IsNotExist(err) {
			if debug {
				fmt.Println("Daemon not running")
			}
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding process: %v\n", err)
			os.Exit(1)
		}

		if err := process.Signal(syscall.SIGTERM); err != nil {
			fmt.Fprintf(os.Stderr, "Error stopping daemon: %v\n", err)
			os.Exit(1)
		}

		if debug {
			fmt.Println("Daemon stop signal sent")
		}
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Run: func(cmd *cobra.Command, args []string) {
		mobDir, err := getMobDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		var out io.Writer = io.Discard
		if debug {
			out = os.Stdout
		}
		logger := log.New(out, "", log.LstdFlags)

		d := daemon.New(mobDir, logger)

		state, pid, err := d.Status()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if state == daemon.StateIdle {
			fmt.Println("Daemon: not running")
		} else {
			fmt.Printf("Daemon: %s (PID %d)\n", state, pid)
		}
	},
}

func getMobDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, "mob"), nil
}

func init() {
	daemonCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug output")
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	rootCmd.AddCommand(daemonCmd)
}
