package cmd

import (
	"fmt"
	"os"

	"github.com/gabe/mob/internal/setup"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize mob with interactive setup",
	Long:  `Run the first-time setup wizard to configure mob directories and settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		wizard := setup.NewWizard()
		if err := wizard.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
