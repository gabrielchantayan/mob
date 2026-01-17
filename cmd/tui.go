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
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := tui.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
