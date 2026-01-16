package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/storage"
	"github.com/spf13/cobra"
)

var rejectCmd = &cobra.Command{
	Use:     "reject <bead-id> [reason]",
	Short:   "Reject a pending bead",
	Long:    `Reject a bead that is in pending_approval status, closing it with a reason.`,
	Aliases: []string{"rej"},
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		beadID := args[0]
		reason := ""
		if len(args) > 1 {
			reason = strings.Join(args[1:], " ")
		}

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

		// Update status to closed with rejection reason
		bead.Status = models.BeadStatusClosed
		now := time.Now()
		bead.ClosedAt = &now
		if reason == "" {
			reason = "Rejected by user"
		}
		bead.CloseReason = reason

		_, err = store.Update(bead)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error updating bead: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✗ Rejected bead %s: %s\n", bead.ID, bead.Title)
		fmt.Printf("  Status changed from pending_approval → closed\n")
		fmt.Printf("  Reason: %s\n", reason)
	},
}

func init() {
	rootCmd.AddCommand(rejectCmd)
}
