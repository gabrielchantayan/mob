package cmd

import (
	"fmt"
	"os"

	"github.com/gabe/mob/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the TUI dashboard",
	Long:  `Launch the interactive TUI dashboard for monitoring and managing mob agents.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := tui.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
