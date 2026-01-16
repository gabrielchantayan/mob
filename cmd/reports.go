package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/storage"
	"github.com/spf13/cobra"
)

var reportsCmd = &cobra.Command{
	Use:     "reports",
	Short:   "Manage agent reports",
	Long:    `List and manage reports from agents (blocked, questions, escalations, progress).`,
	Aliases: []string{"r"},
	Run: func(cmd *cobra.Command, args []string) {
		mobDir, err := getMobDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		reportStore, err := storage.NewReportStore(filepath.Join(mobDir, ".mob", "reports"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Parse filters
		filterType, _ := cmd.Flags().GetString("type")
		filterAgent, _ := cmd.Flags().GetString("agent")
		filterBead, _ := cmd.Flags().GetString("bead")
		showHandled, _ := cmd.Flags().GetBool("handled")
		showUnhandled, _ := cmd.Flags().GetBool("unhandled")

		filter := storage.ReportFilter{
			AgentName: filterAgent,
			BeadID:    filterBead,
		}

		if filterType != "" {
			filter.Type = models.ReportType(filterType)
		}

		if showHandled {
			handled := true
			filter.Handled = &handled
		} else if showUnhandled {
			handled := false
			filter.Handled = &handled
		}

		reports, err := reportStore.List(filter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(reports) == 0 {
			fmt.Println("No reports found.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTYPE\tAGENT\tBEAD\tSTATUS\tTIME")
		for _, r := range reports {
			status := "unhandled"
			if r.Handled {
				status = "handled"
			}
			agent := r.AgentName
			if agent == "" {
				agent = r.AgentID
			}
			if agent == "" {
				agent = "-"
			}
			bead := r.BeadID
			if bead == "" {
				bead = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				r.ID, r.Type, agent, bead, status, r.Timestamp.Format("2006-01-02 15:04"))
		}
		w.Flush()

		// Show unhandled count if not filtering
		if filter.Handled == nil {
			unhandledCount := 0
			for _, r := range reports {
				if !r.Handled {
					unhandledCount++
				}
			}
			if unhandledCount > 0 {
				fmt.Printf("\n%d unhandled report(s)\n", unhandledCount)
			}
		}
	},
}

var reportHandleCmd = &cobra.Command{
	Use:   "handle <report-id>",
	Short: "Mark a report as handled",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		mobDir, err := getMobDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		reportStore, err := storage.NewReportStore(filepath.Join(mobDir, ".mob", "reports"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		reportID := args[0]
		report, err := reportStore.MarkHandled(reportID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Report %s marked as handled.\n", report.ID)
	},
}

var reportShowCmd = &cobra.Command{
	Use:   "show <report-id>",
	Short: "Show details of a specific report",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		mobDir, err := getMobDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		reportStore, err := storage.NewReportStore(filepath.Join(mobDir, ".mob", "reports"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		reportID := args[0]
		report, err := reportStore.Get(reportID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		printReportDetail(report)
	},
}

func printReportDetail(r *models.AgentReport) {
	fmt.Printf("Report: %s\n", r.ID)
	fmt.Printf("  Type:      %s\n", r.Type)
	if r.AgentName != "" {
		fmt.Printf("  Agent:     %s\n", r.AgentName)
	} else if r.AgentID != "" {
		fmt.Printf("  Agent ID:  %s\n", r.AgentID)
	}
	if r.BeadID != "" {
		fmt.Printf("  Bead:      %s\n", r.BeadID)
	}
	status := "unhandled"
	if r.Handled {
		status = "handled"
	}
	fmt.Printf("  Status:    %s\n", status)
	fmt.Printf("  Timestamp: %s\n", r.Timestamp.Format(time.RFC3339))
	fmt.Printf("\nMessage:\n%s\n", r.Message)
}

func init() {
	reportsCmd.Flags().String("type", "", "Filter by type (blocked, question, escalation, progress)")
	reportsCmd.Flags().String("agent", "", "Filter by agent name")
	reportsCmd.Flags().String("bead", "", "Filter by bead ID")
	reportsCmd.Flags().Bool("handled", false, "Show only handled reports")
	reportsCmd.Flags().Bool("unhandled", false, "Show only unhandled reports")

	reportsCmd.AddCommand(reportHandleCmd)
	reportsCmd.AddCommand(reportShowCmd)

	rootCmd.AddCommand(reportsCmd)
}
