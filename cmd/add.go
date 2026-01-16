package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/storage"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <description>",
	Short: "Create a new bead (task)",
	Long:  `Create a new bead with the given description. The bead will be added to the open queue.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		description := strings.Join(args, " ")

		priority, _ := cmd.Flags().GetInt("priority")
		beadType, _ := cmd.Flags().GetString("type")
		turfName, _ := cmd.Flags().GetString("turf")
		labels, _ := cmd.Flags().GetString("labels")

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

		bead := &models.Bead{
			Title:       description,
			Description: description,
			Status:      models.BeadStatusOpen,
			Priority:    priority,
			Type:        models.BeadType(beadType),
			Turf:        turfName,
			Labels:      labels,
		}

		created, err := store.Create(bead)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created bead %s: %s\n", created.ID, created.Title)
	},
}

func getBeadsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, "mob", ".mob", "beads"), nil
}

func init() {
	addCmd.Flags().IntP("priority", "p", 2, "Priority (0=highest, 4=lowest)")
	addCmd.Flags().StringP("type", "t", "task", "Type (bug, feature, task, chore)")
	addCmd.Flags().String("turf", "", "Target turf")
	addCmd.Flags().StringP("labels", "l", "", "Comma-separated labels")

	rootCmd.AddCommand(addCmd)
}
