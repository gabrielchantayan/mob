package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mob",
	Short: "Mob - Claude Code Agent Orchestrator",
	Long:  `A mafia-themed agent orchestrator for managing multiple Claude Code instances.`,
}

func Execute() error {
	return rootCmd.Execute()
}
