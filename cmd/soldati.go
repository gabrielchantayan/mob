package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/gabe/mob/internal/soldati"
	"github.com/spf13/cobra"
)

var soldatiCmd = &cobra.Command{
	Use:   "soldati",
	Short: "Manage worker agents (soldati)",
	Long:  `Create, list, and remove worker agents (soldati) for task execution.`,
}

var soldatiListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all soldati",
	Aliases: []string{"ls"},
	Run: func(cmd *cobra.Command, args []string) {
		dir, err := getSoldatiDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		mgr, err := soldati.NewManager(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		list, err := mgr.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(list) == 0 {
			fmt.Println("No soldati. Use 'mob soldati new' to create one.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTASKS\tSUCCESS\tLAST ACTIVE")
		for _, s := range list {
			tasks := s.Stats.TasksCompleted + s.Stats.TasksFailed
			successStr := "-"
			if tasks > 0 {
				successStr = fmt.Sprintf("%.0f%%", s.Stats.SuccessRate*100)
			}
			lastActive := time.Since(s.LastActive).Round(time.Minute).String() + " ago"
			fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", s.Name, tasks, successStr, lastActive)
		}
		w.Flush()
	},
}

var soldatiNewCmd = &cobra.Command{
	Use:   "new [name]",
	Short: "Create a new soldati",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dir, err := getSoldatiDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		mgr, err := soldati.NewManager(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		s, err := mgr.Create(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created soldati '%s'\n", s.Name)
	},
}

var soldatiKillCmd = &cobra.Command{
	Use:   "kill <name>",
	Short: "Delete a soldati",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		dir, err := getSoldatiDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		mgr, err := soldati.NewManager(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := mgr.Delete(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Killed soldati '%s'\n", name)
	},
}

func getSoldatiDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, "mob", "soldati"), nil
}

func init() {
	soldatiCmd.AddCommand(soldatiListCmd)
	soldatiCmd.AddCommand(soldatiNewCmd)
	soldatiCmd.AddCommand(soldatiKillCmd)
	rootCmd.AddCommand(soldatiCmd)
}
