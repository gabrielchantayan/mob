package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/storage"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:     "logs [bead-id]",
	Short:   "View work logs for a bead or all recent activity",
	Long:    `Display the work history and activity logs for a specific bead, or show recent activity across all beads if no bead ID is provided.`,
	Aliases: []string{"log"},
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

		if len(args) > 0 {
			// Show logs for specific bead
			beadID := args[0]
			showBeadLogs(store, beadID)
		} else {
			// Show recent activity across all beads
			showRecentLogs(store)
		}
	},
}

func showBeadLogs(store *storage.BeadStore, beadID string) {
	bead, err := store.Get(beadID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s: %s\n", headerStyle.Render("Bead"), valueStyle.Render(bead.ID))
	fmt.Printf("%s: %s\n", labelStyle.Render("Title"), bead.Title)
	fmt.Printf("%s: %s\n\n", labelStyle.Render("Status"), formatBeadStatus(bead.Status))

	if len(bead.History) == 0 {
		fmt.Println(mutedStyle.Render("No activity logged for this bead."))
		return
	}

	fmt.Println(sectionStyle.Render("Activity Log"))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	for _, event := range bead.History {
		timestamp := event.Timestamp.Format("Jan 2 15:04:05")
		actor := event.Actor
		if actor == "" {
			actor = "system"
		}

		var description string
		switch event.Type {
		case models.BeadEventTypeCreated:
			description = fmt.Sprintf("Created by %s", actor)
		case models.BeadEventTypeStatusChange:
			description = fmt.Sprintf("Status changed: %s → %s", event.From, event.To)
		case models.BeadEventTypeAssigned:
			if event.To != "" {
				description = fmt.Sprintf("Assigned to %s", event.To)
			} else {
				description = "Unassigned"
			}
		case models.BeadEventTypeComment:
			description = fmt.Sprintf("%s: %s", actor, event.Comment)
		case models.BeadEventTypeWorkStarted:
			description = fmt.Sprintf("%s started work", actor)
		case models.BeadEventTypeWorkCompleted:
			description = fmt.Sprintf("%s completed work", actor)
		case models.BeadEventTypeWorktreeCreate:
			description = fmt.Sprintf("Worktree created: %s", event.Comment)
		default:
			description = fmt.Sprintf("%s: %s", event.Type, event.Comment)
		}

		fmt.Fprintf(w, "  %s\t%s\n",
			mutedStyle.Render(timestamp),
			valueStyle.Render(description))
	}

	w.Flush()
}

func showRecentLogs(store *storage.BeadStore) {
	// Get all beads
	allBeads, err := store.List(storage.BeadFilter{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(allBeads) == 0 {
		fmt.Println(mutedStyle.Render("No beads found."))
		return
	}

	// Collect all events with their bead context
	type eventWithBead struct {
		bead  *models.Bead
		event models.BeadEvent
	}
	var allEvents []eventWithBead

	for _, bead := range allBeads {
		for _, event := range bead.History {
			allEvents = append(allEvents, eventWithBead{
				bead:  bead,
				event: event,
			})
		}
	}

	if len(allEvents) == 0 {
		fmt.Println(mutedStyle.Render("No activity logged yet."))
		return
	}

	// Sort by timestamp (most recent first)
	// Simple bubble sort - good enough for display purposes
	for i := 0; i < len(allEvents)-1; i++ {
		for j := 0; j < len(allEvents)-i-1; j++ {
			if allEvents[j].event.Timestamp.Before(allEvents[j+1].event.Timestamp) {
				allEvents[j], allEvents[j+1] = allEvents[j+1], allEvents[j]
			}
		}
	}

	// Limit to most recent 50 events
	limit := 50
	if len(allEvents) > limit {
		allEvents = allEvents[:limit]
	}

	fmt.Println(sectionStyle.Render("Recent Activity"))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	for _, item := range allEvents {
		timestamp := item.event.Timestamp.Format("Jan 2 15:04")
		beadID := item.bead.ID
		actor := item.event.Actor
		if actor == "" {
			actor = "system"
		}

		var description string
		switch item.event.Type {
		case models.BeadEventTypeCreated:
			description = fmt.Sprintf("%s created", item.bead.Title)
		case models.BeadEventTypeStatusChange:
			description = fmt.Sprintf("%s: %s → %s", truncate(item.bead.Title, 30), item.event.From, item.event.To)
		case models.BeadEventTypeAssigned:
			if item.event.To != "" {
				description = fmt.Sprintf("%s assigned to %s", truncate(item.bead.Title, 25), item.event.To)
			} else {
				description = fmt.Sprintf("%s unassigned", truncate(item.bead.Title, 30))
			}
		case models.BeadEventTypeComment:
			description = fmt.Sprintf("%s: %s", truncate(item.bead.Title, 20), truncate(item.event.Comment, 40))
		case models.BeadEventTypeWorkStarted:
			description = fmt.Sprintf("%s work started by %s", truncate(item.bead.Title, 25), actor)
		case models.BeadEventTypeWorkCompleted:
			description = fmt.Sprintf("%s completed by %s", truncate(item.bead.Title, 25), actor)
		case models.BeadEventTypeWorktreeCreate:
			description = fmt.Sprintf("%s worktree created", truncate(item.bead.Title, 30))
		default:
			description = truncate(item.bead.Title, 40)
		}

		fmt.Fprintf(w, "  %s\t%s\t%s\n",
			mutedStyle.Render(timestamp),
			labelStyle.Render(beadID),
			valueStyle.Render(description))
	}

	w.Flush()
}

func formatBeadStatus(status models.BeadStatus) string {
	switch status {
	case models.BeadStatusInProgress:
		return successStyle.Render(string(status))
	case models.BeadStatusBlocked:
		return errorStyle.Render(string(status))
	case models.BeadStatusPendingApproval:
		return warningStyle.Render(string(status))
	case models.BeadStatusClosed:
		return mutedStyle.Render(string(status))
	default:
		return valueStyle.Render(string(status))
	}
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
