package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/gabe/mob/internal/turf"
	"github.com/spf13/cobra"
)

var turfCmd = &cobra.Command{
	Use:   "turf",
	Short: "Manage registered projects (turfs)",
	Long:  `Register, list, and remove projects under mob's management.`,
}

var turfAddCmd = &cobra.Command{
	Use:   "add <path> [name]",
	Short: "Register a project as a turf",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		name := ""
		if len(args) > 1 {
			name = args[1]
		} else {
			// Use directory name as default
			name = filepath.Base(path)
		}

		mainBranch, _ := cmd.Flags().GetString("branch")

		turfsPath, err := getTurfsPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		mgr, err := turf.NewManager(turfsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := mgr.Add(path, name, mainBranch); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Registered turf '%s' at %s\n", name, path)
	},
}

var turfListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all registered turfs",
	Aliases: []string{"ls"},
	Run: func(cmd *cobra.Command, args []string) {
		turfsPath, err := getTurfsPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		mgr, err := turf.NewManager(turfsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		turfs := mgr.List()
		if len(turfs) == 0 {
			fmt.Println("No turfs registered. Use 'mob turf add <path>' to register a project.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tPATH\tBRANCH")
		for _, t := range turfs {
			fmt.Fprintf(w, "%s\t%s\t%s\n", t.Name, t.Path, t.MainBranch)
		}
		w.Flush()
	},
}

var turfRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Unregister a turf",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		turfsPath, err := getTurfsPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		mgr, err := turf.NewManager(turfsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := mgr.Remove(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Removed turf '%s'\n", name)
	},
}

func getTurfsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, "mob", "turfs.toml"), nil
}

func init() {
	turfAddCmd.Flags().StringP("branch", "b", "main", "Main branch name")

	turfCmd.AddCommand(turfAddCmd)
	turfCmd.AddCommand(turfListCmd)
	turfCmd.AddCommand(turfRemoveCmd)
	rootCmd.AddCommand(turfCmd)
}
