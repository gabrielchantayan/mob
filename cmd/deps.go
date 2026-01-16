package cmd

import (
	"fmt"
	"os"

	"github.com/gabe/mob/internal/display"
	"github.com/gabe/mob/internal/storage"
	"github.com/spf13/cobra"
)

var (
	depsShowTree bool
	depsShowAll  bool
)

var depsCmd = &cobra.Command{
	Use:     "deps [bead-id]",
	Short:   "Show bead dependencies",
	Long:    `Show blocking/blocked-by relationships for a bead or all beads.`,
	Aliases: []string{"dep", "dependencies"},
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		beadsPath, err := getBeadsPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		store, err := storage.NewBeadStore(beadsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "Error: bead ID required")
			fmt.Fprintln(os.Stderr, "Usage: mob deps <bead-id>")
			os.Exit(1)
		}

		beadID := args[0]

		if depsShowTree {
			showDependencyTree(store, beadID)
		} else {
			showSimpleDeps(store, beadID)
		}
	},
}

func showSimpleDeps(store *storage.BeadStore, beadID string) {
	bead, err := store.Get(beadID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	blockedBy, err := store.GetBlockedBy(beadID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting blockers: %v\n", err)
		os.Exit(1)
	}

	blocking, err := store.GetBlocking(beadID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting blocked beads: %v\n", err)
		os.Exit(1)
	}

	opts := display.DefaultTreeOpts()
	output := display.RenderSimpleDeps(bead, blockedBy, blocking, opts)
	fmt.Print(output)
}

func showDependencyTree(store *storage.BeadStore, beadID string) {
	tree, err := store.GetDependencyTree(beadID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	opts := display.DefaultTreeOpts()
	output := display.RenderDependencyTree(tree, opts)
	fmt.Print(output)
}

func init() {
	depsCmd.Flags().BoolVarP(&depsShowTree, "tree", "t", false, "Show full dependency tree")
	depsCmd.Flags().BoolVarP(&depsShowAll, "all", "a", false, "Show all dependencies in the turf")

	rootCmd.AddCommand(depsCmd)
}
