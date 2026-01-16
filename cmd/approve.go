package cmd

import (
	"fmt"
	"os"

	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/storage"
	"github.com/spf13/cobra"
)

var approveCmd = &cobra.Command{
	Use:     "approve <bead-id>",
	Short:   "Approve a pending bead",
	Long:    `Approve a bead that is in pending_approval status, allowing work to proceed.`,
	Aliases: []string{"app"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		beadID := args[0]

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

		// Get the bead
		bead, err := store.Get(beadID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Check if it's in pending_approval status
		if bead.Status != models.BeadStatusPendingApproval {
			fmt.Fprintf(os.Stderr, "Error: Bead %s is not pending approval (current status: %s)\n", beadID, bead.Status)
			os.Exit(1)
		}

		// Update status to open so it can be picked up
		bead.Status = models.BeadStatusOpen

		_, err = store.Update(bead)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error updating bead: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Approved bead %s: %s\n", bead.ID, bead.Title)
		fmt.Printf("  Status changed from pending_approval → open\n")
	},
}

func init() {
	rootCmd.AddCommand(approveCmd)
}
