package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/storage"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:     "status [bead-id]",
	Short:   "Show status of beads",
	Long:    `Show status of all beads or a specific bead by ID.`,
	Aliases: []string{"s"},
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

		if len(args) > 0 {
			// Show specific bead
			bead, err := store.Get(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			printBeadDetail(bead)
			return
		}

		// Show all beads
		filterStatus, _ := cmd.Flags().GetString("status")
		filterTurf, _ := cmd.Flags().GetString("turf")

		filter := storage.BeadFilter{
			Status: models.BeadStatus(filterStatus),
			Turf:   filterTurf,
		}

		beads, err := store.List(filter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(beads) == 0 {
			fmt.Println("No beads found. Use 'mob add <description>' to create one.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tPRI\tTYPE\tTITLE\tTURF")
		for _, b := range beads {
			title := b.Title
			if len(title) > 40 {
				title = title[:37] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n",
				b.ID, b.Status, b.Priority, b.Type, title, b.Turf)
		}
		w.Flush()
	},
}

func printBeadDetail(b *models.Bead) {
	fmt.Printf("Bead: %s\n", b.ID)
	fmt.Printf("  Title:       %s\n", b.Title)
	fmt.Printf("  Status:      %s\n", b.Status)
	fmt.Printf("  Priority:    %d\n", b.Priority)
	fmt.Printf("  Type:        %s\n", b.Type)
	if b.Turf != "" {
		fmt.Printf("  Turf:        %s\n", b.Turf)
	}
	if b.Assignee != "" {
		fmt.Printf("  Assignee:    %s\n", b.Assignee)
	}
	if b.Labels != "" {
		fmt.Printf("  Labels:      %s\n", b.Labels)
	}
	if b.Branch != "" {
		fmt.Printf("  Branch:      %s\n", b.Branch)
	}
	fmt.Printf("  Created:     %s\n", b.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Updated:     %s\n", b.UpdatedAt.Format(time.RFC3339))
	if b.Description != b.Title {
		fmt.Printf("\nDescription:\n%s\n", b.Description)
	}
}

func init() {
	statusCmd.Flags().String("status", "", "Filter by status")
	statusCmd.Flags().String("turf", "", "Filter by turf")

	rootCmd.AddCommand(statusCmd)
}
