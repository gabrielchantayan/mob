package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gabe/mob/internal/hook"
	"github.com/gabe/mob/internal/registry"
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

		// Get runtime status from registry
		reg := registry.New(getRegistryPath())
		activeAgents, _ := reg.ListByType("soldati")
		agentStatus := make(map[string]*registry.AgentRecord)
		for _, a := range activeAgents {
			agentStatus[a.Name] = a
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATUS\tTASK\tTASKS\tSUCCESS\tLAST ACTIVE")
		for _, s := range list {
			tasks := s.Stats.TasksCompleted + s.Stats.TasksFailed
			successStr := "-"
			if tasks > 0 {
				successStr = fmt.Sprintf("%.0f%%", s.Stats.SuccessRate*100)
			}
			lastActive := time.Since(s.LastActive).Round(time.Minute).String() + " ago"

			// Check runtime status
			status := "idle"
			task := "-"
			if agent, ok := agentStatus[s.Name]; ok {
				status = agent.Status
				if agent.Task != "" {
					task = truncateStr(agent.Task, 30)
				}
				// Use registry's last ping if more recent
				if agent.LastPing.After(s.LastActive) {
					lastActive = time.Since(agent.LastPing).Round(time.Minute).String() + " ago"
				}
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n", s.Name, status, task, tasks, successStr, lastActive)
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

		// Also remove from registry if present
		reg := registry.New(getRegistryPath())
		if agent, err := reg.GetByName(name); err == nil {
			reg.Unregister(agent.ID) // Ignore errors
		}

		fmt.Printf("Killed soldati '%s'\n", name)
	},
}

var soldatiAssignBeadID string

var soldatiAssignCmd = &cobra.Command{
	Use:   "assign <name> <task>",
	Short: "Assign work to a soldati",
	Long: `Assign a task to a soldati via their hook file.
The daemon must be running for the soldati to receive and process the work.

Example:
  mob soldati assign vinnie "Fix the login bug in auth.go"
  mob soldati assign vinnie --bead bd-a1b2 "Implement the feature described in the bead"`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		task := strings.Join(args[1:], " ")

		// Verify soldati exists
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

		if _, err := mgr.Get(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: soldati '%s' not found\n", name)
			os.Exit(1)
		}

		// Create hook manager and write assignment
		hookDir := getHookDir()
		hookMgr, err := hook.NewManager(hookDir, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating hook manager: %v\n", err)
			os.Exit(1)
		}

		h := &hook.Hook{
			Type:      hook.HookTypeAssign,
			BeadID:    soldatiAssignBeadID,
			Message:   task,
			Timestamp: time.Now(),
		}

		if err := hookMgr.Write(h); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing hook: %v\n", err)
			os.Exit(1)
		}

		if soldatiAssignBeadID != "" {
			fmt.Printf("Assigned bead '%s' to soldati '%s'\n", soldatiAssignBeadID, name)
		} else {
			fmt.Printf("Assigned task to soldati '%s': %s\n", name, truncateStr(task, 50))
		}
		fmt.Println("(Daemon must be running for soldati to process the work)")
	},
}

func getSoldatiDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, "mob", "soldati"), nil
}

func getHookDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "mob", ".mob", "soldati")
}

func getRegistryPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "mob", ".mob", "agents.json")
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func init() {
	soldatiAssignCmd.Flags().StringVar(&soldatiAssignBeadID, "bead", "", "Bead ID to associate with the task")

	soldatiCmd.AddCommand(soldatiListCmd)
	soldatiCmd.AddCommand(soldatiNewCmd)
	soldatiCmd.AddCommand(soldatiKillCmd)
	soldatiCmd.AddCommand(soldatiAssignCmd)
	rootCmd.AddCommand(soldatiCmd)
}
