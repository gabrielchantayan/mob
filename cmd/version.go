package cmd

import (
	"fmt"

	"github.com/gabe/mob/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("mob version %s\n", version.Version)
		fmt.Printf("  commit: %s\n", version.GitCommit)
		fmt.Printf("  built:  %s\n", version.BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
