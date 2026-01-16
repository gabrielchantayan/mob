package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/gabe/mob/internal/daemon"
	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/registry"
	"github.com/gabe/mob/internal/storage"
	"github.com/gabe/mob/internal/turf"
	"github.com/spf13/cobra"
)

// Styles for terminal output
var (
	headerStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00D4FF"))
	labelStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	valueStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#EEEEEE"))
	successStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E22E"))
	warningStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FD971F"))
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#F92672"))
	mutedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	sectionStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EEEEEE"))
)

var (
	flagJSON   bool
	flagBeads  bool
	flagAgents bool
	flagWatch  bool
)

type statusOutput struct {
	Daemon   daemonInfo   `json:"daemon"`
	Agents   []agentInfo  `json:"agents"`
	Beads    beadSummary  `json:"beads"`
	Turfs    []turfInfo   `json:"turfs"`
	Activity []activityEntry `json:"recent_activity,omitempty"`
}

type daemonInfo struct {
	Running bool   `json:"running"`
	PID     int    `json:"pid,omitempty"`
	Uptime  string `json:"uptime,omitempty"`
}

type agentInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Task     string `json:"task"`
	LastPing string `json:"last_ping"`
}

type beadSummary struct {
	InProgress      int `json:"in_progress"`
	Open            int `json:"open"`
	PendingApproval int `json:"pending_approval"`
	Blocked         int `json:"blocked"`
	Closed          int `json:"closed"`
}

type turfInfo struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Agents int    `json:"agents"`
}

type activityEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
}

var statusCmd = &cobra.Command{
	Use:     "status [bead-id]",
	Short:   "Show comprehensive system status",
	Long:    `Show status of daemon, agents, beads, and turfs. If a bead ID is provided, show detailed bead information.`,
	Aliases: []string{"s"},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			// Show specific bead detail (legacy behavior)
			showBeadDetail(args[0])
			return
		}

		if flagWatch {
			// Watch mode - refresh every 2 seconds
			for {
				clearScreen()
				showStatus()
				time.Sleep(2 * time.Second)
			}
		} else {
			showStatus()
		}
	},
}

func showBeadDetail(beadID string) {
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

	bead, err := store.Get(beadID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	printBeadDetail(bead)
}

func showStatus() {
	mobDir, _ := getMobDir()
	output := collectStatusData(mobDir)

	if flagJSON {
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
		return
	}

	if flagBeads {
		printBeadsSummary(output.Beads)
		return
	}

	if flagAgents {
		printAgents(output.Agents)
		return
	}

	// Full status display
	printDaemonStatus(output.Daemon)
	fmt.Println()

	if len(output.Agents) > 0 {
		printAgents(output.Agents)
		fmt.Println()
	}

	printBeadsSummary(output.Beads)
	fmt.Println()

	if len(output.Activity) > 0 {
		printRecentActivity(output.Activity)
		fmt.Println()
	}

	if len(output.Turfs) > 0 {
		printTurfs(output.Turfs)
	}
}

func collectStatusData(mobDir string) statusOutput {
	output := statusOutput{}

	// Daemon status
	d := daemon.New(mobDir, log.New(io.Discard, "", 0))
	state, pid, err := d.Status()
	if err == nil {
		output.Daemon.Running = (state == daemon.StateRunning)
		output.Daemon.PID = pid
		if output.Daemon.Running {
			// Try to get uptime from daemon start time (simplified)
			output.Daemon.Uptime = "running"
		}
	}

	// Agent status
	reg := registry.New(registry.DefaultPath(mobDir))
	agents, err := reg.List()
	if err == nil {
		for _, a := range agents {
			name := a.Name
			if name == "" {
				name = a.ID[:8]
			}
			output.Agents = append(output.Agents, agentInfo{
				Name:     name,
				Type:     a.Type,
				Status:   a.Status,
				Task:     truncate(a.Task, 40),
				LastPing: formatRelativeTime(a.LastPing),
			})
		}
	}

	// Bead summary
	beadsPath := filepath.Join(mobDir, "beads")
	store, err := storage.NewBeadStore(beadsPath)
	if err == nil {
		allBeads, err := store.List(storage.BeadFilter{})
		if err == nil {
			for _, b := range allBeads {
				switch b.Status {
				case models.BeadStatusOpen:
					output.Beads.Open++
				case models.BeadStatusInProgress:
					output.Beads.InProgress++
				case models.BeadStatusPendingApproval:
					output.Beads.PendingApproval++
				case models.BeadStatusBlocked:
					output.Beads.Blocked++
				case models.BeadStatusClosed:
					output.Beads.Closed++
				}
			}
		}
	}

	// Turf information
	turfMgr, err := turf.NewManager(filepath.Join(mobDir, "turfs.json"))
	if err == nil {
		turfs := turfMgr.List()
		for _, t := range turfs {
			// Count agents in this turf (simplified - count all agents for now)
			agentCount := len(output.Agents)
			output.Turfs = append(output.Turfs, turfInfo{
				Name:   t.Name,
				Path:   t.Path,
				Agents: agentCount,
			})
		}
	}

	// Recent activity from daemon log
	logPath := filepath.Join(mobDir, ".mob", "daemon.log")
	if entries := parseRecentActivity(logPath, 5); len(entries) > 0 {
		output.Activity = entries
	}

	return output
}

func printDaemonStatus(info daemonInfo) {
	fmt.Println(sectionStyle.Render("Daemon"))
	if info.Running {
		fmt.Printf("  %s %s (PID %d)\n",
			successStyle.Render("●"),
			valueStyle.Render("running"),
			info.PID)
	} else {
		fmt.Printf("  %s %s\n",
			errorStyle.Render("○"),
			mutedStyle.Render("not running"))
	}
}

func printAgents(agents []agentInfo) {
	fmt.Printf("%s (%d)\n", sectionStyle.Render("Agents"), len(agents))
	if len(agents) == 0 {
		fmt.Println(mutedStyle.Render("  No active agents"))
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, a := range agents {
		statusColored := formatAgentStatus(a.Status)
		task := a.Task
		if task == "" {
			task = "-"
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n",
			valueStyle.Render(a.Name),
			statusColored,
			mutedStyle.Render(task),
			mutedStyle.Render(a.LastPing))
	}
	w.Flush()
}

func printBeadsSummary(summary beadSummary) {
	fmt.Println(sectionStyle.Render("Beads"))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if summary.InProgress > 0 {
		fmt.Fprintf(w, "  In Progress:\t%s\n", successStyle.Render(fmt.Sprintf("%d", summary.InProgress)))
	}
	if summary.Open > 0 {
		fmt.Fprintf(w, "  Open:\t%s\n", valueStyle.Render(fmt.Sprintf("%d", summary.Open)))
	}
	if summary.PendingApproval > 0 {
		fmt.Fprintf(w, "  Pending Approval:\t%s\n", warningStyle.Render(fmt.Sprintf("%d", summary.PendingApproval)))
	}
	if summary.Blocked > 0 {
		fmt.Fprintf(w, "  Blocked:\t%s\n", errorStyle.Render(fmt.Sprintf("%d", summary.Blocked)))
	}
	if summary.Closed > 0 {
		fmt.Fprintf(w, "  Closed:\t%s\n", mutedStyle.Render(fmt.Sprintf("%d", summary.Closed)))
	}

	total := summary.InProgress + summary.Open + summary.PendingApproval + summary.Blocked + summary.Closed
	if total == 0 {
		fmt.Fprintln(w, mutedStyle.Render("  No beads"))
	}
	w.Flush()
}

func printRecentActivity(activity []activityEntry) {
	fmt.Println(sectionStyle.Render("Recent Activity"))
	for _, entry := range activity {
		fmt.Printf("  %s  %s\n",
			mutedStyle.Render(entry.Time),
			valueStyle.Render(entry.Message))
	}
}

func printTurfs(turfs []turfInfo) {
	fmt.Println(sectionStyle.Render("Turfs"))
	if len(turfs) == 0 {
		fmt.Println(mutedStyle.Render("  No turfs configured"))
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, t := range turfs {
		fmt.Fprintf(w, "  %s\t%s\n",
			valueStyle.Render(t.Name),
			mutedStyle.Render(t.Path))
	}
	w.Flush()
}

func formatAgentStatus(status string) string {
	switch status {
	case "active":
		return successStyle.Render(status)
	case "idle":
		return mutedStyle.Render(status)
	case "stuck":
		return errorStyle.Render(status)
	default:
		return valueStyle.Render(status)
	}
}

func parseRecentActivity(logPath string, limit int) []activityEntry {
	content, err := os.ReadFile(logPath)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	entries := []activityEntry{}

	// Process from end to get most recent
	for i := len(lines) - 1; i >= 0 && len(entries) < limit; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Filter for important events
		if strings.Contains(line, "started") ||
			strings.Contains(line, "completed") ||
			strings.Contains(line, "spawning") ||
			strings.Contains(line, "error") ||
			strings.Contains(line, "created") {

			// Try to extract timestamp and message
			// Format: "2024/01/16 12:34:56 Message"
			if len(line) >= 20 && line[4] == '/' && line[7] == '/' {
				timestamp := line[:19]
				message := strings.TrimSpace(line[20:])

				// Make message more readable
				message = simplifyLogMessage(message)

				entries = append(entries, activityEntry{
					Time:    formatLogTime(timestamp),
					Message: message,
				})
			}
		}
	}

	// Reverse to get chronological order
	for i := 0; i < len(entries)/2; i++ {
		entries[i], entries[len(entries)-1-i] = entries[len(entries)-1-i], entries[i]
	}

	return entries
}

func simplifyLogMessage(msg string) string {
	// Remove common prefixes
	msg = strings.TrimPrefix(msg, "daemon: ")
	msg = strings.TrimPrefix(msg, "patrol: ")
	msg = strings.TrimPrefix(msg, "hook: ")
	return msg
}

func formatLogTime(timestamp string) string {
	// Parse "2024/01/16 12:34:56"
	t, err := time.Parse("2006/01/02 15:04:05", timestamp)
	if err != nil {
		return timestamp
	}

	// Format as relative time
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return t.Format("Jan 2 15:04")
}

func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return t.Format("Jan 2")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
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
	statusCmd.Flags().BoolVar(&flagJSON, "json", false, "Output in JSON format")
	statusCmd.Flags().BoolVar(&flagBeads, "beads", false, "Show only bead summary")
	statusCmd.Flags().BoolVar(&flagAgents, "agents", false, "Show only agent list")
	statusCmd.Flags().BoolVar(&flagWatch, "watch", false, "Refresh every 2 seconds")

	// Legacy flags for backward compatibility
	statusCmd.Flags().String("status", "", "Filter by status (deprecated, use 'mob list' instead)")
	statusCmd.Flags().String("turf", "", "Filter by turf (deprecated, use 'mob list' instead)")
	statusCmd.Flags().MarkHidden("status")
	statusCmd.Flags().MarkHidden("turf")

	rootCmd.AddCommand(statusCmd)
}
