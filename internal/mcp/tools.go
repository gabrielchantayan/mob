package mcp

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gabe/mob/internal/agent"
	"github.com/gabe/mob/internal/git"
	"github.com/gabe/mob/internal/hook"
	"github.com/gabe/mob/internal/merge"
	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/registry"
	"github.com/gabe/mob/internal/soldati"
	"github.com/gabe/mob/internal/storage"
	"github.com/gabe/mob/internal/turf"
)

// ToolContext provides access to mob systems for tool handlers
type ToolContext struct {
	Registry       *registry.Registry
	Spawner        *agent.Spawner
	BeadStore      *storage.BeadStore
	TurfManager    *turf.Manager
	MobDir         string
	TaskWg         *sync.WaitGroup // Track background tasks for graceful shutdown
	NotifyManager  interface {
		NotifyTaskComplete(beadID, title, assignee string) error
		NotifyApprovalNeeded(beadID, title string) error
		NotifyAgentStuck(agentName, agentID, task string) error
		NotifyAgentError(agentName, agentID, errorMsg string) error
	} // Optional notification manager
}

// ToolHandler is a function that executes a tool
type ToolHandler func(ctx *ToolContext, args map[string]interface{}) (string, error)

// Tool defines an MCP tool
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	Handler     ToolHandler
}

// GetTools returns all available MCP tools
func GetTools() []*Tool {
	return []*Tool{
		{
			Name:        "spawn_soldati",
			Description: "Create a new persistent worker (Soldati) for long-running tasks. Give 'em a name and a turf (project) to work.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Name for the soldati (auto-generated mob name if empty)",
					},
					"turf": map[string]interface{}{
						"type":        "string",
						"description": "Project/turf this soldati will work on",
					},
					"work_dir": map[string]interface{}{
						"type":        "string",
						"description": "Working directory for the soldati (defaults to turf path or current dir)",
					},
				},
				"required": []string{"turf"},
			},
			Handler: handleSpawnSoldati,
		},
		{
			Name:        "spawn_associate",
			Description: "Get a temp worker for a quick job. No names, no history - just work.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"turf": map[string]interface{}{
						"type":        "string",
						"description": "Project/turf for this job",
					},
					"task": map[string]interface{}{
						"type":        "string",
						"description": "The task to assign immediately",
					},
					"work_dir": map[string]interface{}{
						"type":        "string",
						"description": "Working directory (defaults to turf path or current dir)",
					},
					"bead_id": map[string]interface{}{
						"type":        "string",
						"description": "Optional bead ID to link - auto-completes when associate finishes successfully, marks blocked on failure",
					},
				},
				"required": []string{"turf", "task"},
			},
			Handler: handleSpawnAssociate,
		},
		{
			Name:        "list_agents",
			Description: "See who's on the payroll right now. Returns all active Soldati and Associates.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by agent type: 'soldati', 'associate', or empty for all",
						"enum":        []string{"soldati", "associate", ""},
					},
				},
			},
			Handler: handleListAgents,
		},
		{
			Name:        "get_agent_status",
			Description: "Check in on a specific member of the crew.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Agent ID to check",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Agent name to check (alternative to ID)",
					},
				},
			},
			Handler: handleGetAgentStatus,
		},
		{
			Name:        "kill_agent",
			Description: "Send someone home. Permanently removes them from the crew.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Agent ID to kill",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Agent name to kill (alternative to ID)",
					},
				},
			},
			Handler: handleKillAgent,
		},
		{
			Name:        "nudge_agent",
			Description: "Wake up someone who's slacking. Sends a ping to check if they're still working.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Agent ID to nudge",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Agent name to nudge (alternative to ID)",
					},
				},
			},
			Handler: handleNudgeAgent,
		},
		{
			Name:        "assign_bead",
			Description: "Hand out work to one of the boys. Assigns a bead (task) to an agent.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agent_id": map[string]interface{}{
						"type":        "string",
						"description": "Agent ID to assign to",
					},
					"agent_name": map[string]interface{}{
						"type":        "string",
						"description": "Agent name to assign to (alternative to ID)",
					},
					"bead_id": map[string]interface{}{
						"type":        "string",
						"description": "Bead ID to assign",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Task description if no bead ID",
					},
				},
			},
			Handler: handleAssignBead,
		},
		{
			Name:        "create_bead",
			Description: "Drop a new job on the board. Creates a bead (work item) for the crew to handle.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "What's the job called",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "The full rundown on what needs doing",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Kind of work: bug, feature, task, epic, chore, review, or heresy",
						"enum":        []string{"bug", "feature", "task", "epic", "chore", "review", "heresy"},
					},
					"priority": map[string]interface{}{
						"type":        "integer",
						"description": "How hot is it? 0=highest priority, 4=lowest",
						"minimum":     0,
						"maximum":     4,
					},
					"turf": map[string]interface{}{
						"type":        "string",
						"description": "Which project/territory this belongs to",
					},
					"labels": map[string]interface{}{
						"type":        "string",
						"description": "Comma-separated tags for the job",
					},
					"parent_id": map[string]interface{}{
						"type":        "string",
						"description": "Parent bead ID if this is a sub-task",
					},
					"blocks": map[string]interface{}{
						"type":        "array",
						"description": "Bead IDs that this work blocks",
						"items":       map[string]interface{}{"type": "string"},
					},
					"related": map[string]interface{}{
						"type":        "array",
						"description": "Related bead IDs",
						"items":       map[string]interface{}{"type": "string"},
					},
					"pending_approval": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, creates bead with pending_approval status requiring approval via 'mob approve <bead-id>' before work can start",
					},
				},
				"required": []string{"title"},
			},
			Handler: handleCreateBead,
		},
		{
			Name:        "list_beads",
			Description: "Check the job board. See what work is pending for the crew.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status: open, in_progress, blocked, closed, pending_approval",
						"enum":        []string{"open", "in_progress", "blocked", "closed", "pending_approval"},
					},
					"turf": map[string]interface{}{
						"type":        "string",
						"description": "Filter by project/territory",
					},
					"assignee": map[string]interface{}{
						"type":        "string",
						"description": "Filter by who's working it",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by work type: bug, feature, task, epic, chore, review, heresy",
						"enum":        []string{"bug", "feature", "task", "epic", "chore", "review", "heresy"},
					},
				},
			},
			Handler: handleListBeads,
		},
		{
			Name:        "list_ready_beads",
			Description: "Get beads that are ready to work - open status with no unmet blockers. Returns sorted by priority.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"turf": map[string]interface{}{
						"type":        "string",
						"description": "Filter by project/territory (optional)",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of beads to return (default 10)",
					},
				},
			},
			Handler: handleListReadyBeads,
		},
		{
			Name:        "get_bead",
			Description: "Check on a piece of work. Returns full details about a bead.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Bead ID to look up",
					},
				},
				"required": []string{"id"},
			},
			Handler: handleGetBead,
		},
		{
			Name:        "update_bead",
			Description: "Make changes to a piece of work. Update any details on a bead.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Bead ID to update",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "New title for the bead",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "New description for the bead",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "New status: open, in_progress, blocked, closed, pending_approval",
						"enum":        []string{"open", "in_progress", "blocked", "closed", "pending_approval"},
					},
					"priority": map[string]interface{}{
						"type":        "integer",
						"description": "Priority level 0-4 (0 = highest)",
					},
					"assignee": map[string]interface{}{
						"type":        "string",
						"description": "Who's working this job",
					},
					"labels": map[string]interface{}{
						"type":        "string",
						"description": "Labels/tags for the bead",
					},
					"blocks": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Bead IDs this work blocks",
					},
					"related": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Related bead IDs",
					},
				},
				"required": []string{"id"},
			},
			Handler: handleUpdateBead,
		},
		{
			Name:        "complete_bead",
			Description: "Mark the job as done. Closes out a bead.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Bead ID to complete",
					},
					"close_reason": map[string]interface{}{
						"type":        "string",
						"description": "Why the job's done (completed, won't fix, duplicate, etc.)",
					},
				},
				"required": []string{"id"},
			},
			Handler: handleCompleteBead,
		},
		{
			Name:        "comment_on_bead",
			Description: "Leave a comment on a bead. Agents can report what they did, blockers found, questions, or progress updates.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"bead_id": map[string]interface{}{
						"type":        "string",
						"description": "Bead ID to comment on",
					},
					"comment": map[string]interface{}{
						"type":        "string",
						"description": "The comment text",
					},
					"actor": map[string]interface{}{
						"type":        "string",
						"description": "Who is making the comment (agent name, user, etc.)",
					},
				},
				"required": []string{"bead_id", "comment"},
			},
			Handler: handleCommentOnBead,
		},
		{
			Name:        "list_turfs",
			Description: "Get the turf mappings. Returns all registered turfs with their paths so you know where projects are located.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
			Handler: handleListTurfs,
		},
	}
}

func handleSpawnSoldati(ctx *ToolContext, args map[string]interface{}) (string, error) {
	turf, _ := args["turf"].(string)
	name, _ := args["name"].(string)
	workDir, _ := args["work_dir"].(string)

	if turf == "" {
		return "", fmt.Errorf("turf is required")
	}

	// Get soldati manager for persistent storage
	soldatiDir := filepath.Join(ctx.MobDir, "soldati")
	mgr, err := soldati.NewManager(soldatiDir)
	if err != nil {
		return "", fmt.Errorf("failed to create soldati manager: %w", err)
	}

	// Generate name if not provided, checking both registry and TOML files
	if name == "" {
		// Get existing names from registry
		agents, err := ctx.Registry.ListByType("soldati")
		if err != nil {
			return "", fmt.Errorf("failed to list existing soldati: %w", err)
		}
		usedNames := make([]string, 0, len(agents))
		for _, a := range agents {
			usedNames = append(usedNames, a.Name)
		}
		// Also check TOML files
		existingSoldati, _ := mgr.List()
		for _, s := range existingSoldati {
			usedNames = append(usedNames, s.Name)
		}
		name = soldati.GenerateUniqueName(usedNames)
	}

	// Default work directory
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Create persistent soldati record (TOML file)
	_, err = mgr.Create(name)
	if err != nil {
		return "", fmt.Errorf("failed to create soldati: %w", err)
	}

	// Generate MCP config for tool access
	mcpConfigPath, err := GenerateMCPConfig(ctx.MobDir)
	if err != nil {
		log.Printf("Warning: failed to generate MCP config: %v", err)
	}

	// Spawn the agent with the Soldati system prompt
	spawnedAgent, err := ctx.Spawner.SpawnWithOptions(agent.SpawnOptions{
		Type:         agent.AgentTypeSoldati,
		Name:         name,
		Turf:         turf,
		WorkDir:      workDir,
		SystemPrompt: agent.SoldatiSystemPrompt,
		MCPConfig:    mcpConfigPath,
		Model:        "sonnet", // Default to sonnet for cost efficiency
	})
	if err != nil {
		// Clean up TOML file on failure
		mgr.Delete(name)
		return "", fmt.Errorf("failed to spawn soldati: %w", err)
	}

	// Register in registry
	record := &registry.AgentRecord{
		ID:        spawnedAgent.ID,
		Type:      "soldati",
		Name:      name,
		Turf:      turf,
		Status:    "active",
		StartedAt: spawnedAgent.StartedAt,
	}
	if err := ctx.Registry.Register(record); err != nil {
		// Clean up on failure
		mgr.Delete(name)
		return "", fmt.Errorf("failed to register soldati: %w", err)
	}

	return fmt.Sprintf("Soldati '%s' is now on the payroll. ID: %s, Turf: %s", name, spawnedAgent.ID, turf), nil
}

func handleSpawnAssociate(ctx *ToolContext, args map[string]interface{}) (string, error) {
	turf, _ := args["turf"].(string)
	task, _ := args["task"].(string)
	workDir, _ := args["work_dir"].(string)
	beadID, _ := args["bead_id"].(string)

	if turf == "" {
		return "", fmt.Errorf("turf is required")
	}
	if task == "" {
		return "", fmt.Errorf("task is required")
	}

	// Default work directory
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// If bead_id provided, update the bead to in_progress
	if beadID != "" && ctx.BeadStore != nil {
		bead, err := ctx.BeadStore.Get(beadID)
		if err != nil {
			return "", fmt.Errorf("bead not found: %w", err)
		}
		bead.Status = models.BeadStatusInProgress
		if _, err := ctx.BeadStore.Update(bead); err != nil {
			return "", fmt.Errorf("failed to update bead status: %w", err)
		}
	}

	// Generate MCP config for tool access
	mcpConfigPath, err := GenerateMCPConfig(ctx.MobDir)
	if err != nil {
		log.Printf("Warning: failed to generate MCP config: %v", err)
	}

	// Spawn the agent with the Associate system prompt
	spawnedAgent, err := ctx.Spawner.SpawnWithOptions(agent.SpawnOptions{
		Type:         agent.AgentTypeAssociate,
		Name:         "", // Associates don't get names
		Turf:         turf,
		WorkDir:      workDir,
		SystemPrompt: agent.AssociateSystemPrompt,
		MCPConfig:    mcpConfigPath,
		Model:        "sonnet", // Default to sonnet for cost efficiency
	})
	if err != nil {
		return "", fmt.Errorf("failed to spawn associate: %w", err)
	}

	// Register in registry with linked bead
	record := &registry.AgentRecord{
		ID:        spawnedAgent.ID,
		Type:      "associate",
		Turf:      turf,
		Task:      task,
		BeadID:    beadID, // Link the bead for auto-completion
		Status:    "active",
		StartedAt: spawnedAgent.StartedAt,
	}
	if err := ctx.Registry.Register(record); err != nil {
		return "", fmt.Errorf("failed to register associate: %w", err)
	}

	// Execute the task in a background goroutine
	ctx.TaskWg.Add(1)
	go func(a *agent.Agent, agentID string, taskDesc string, linkedBeadID string, reg *registry.Registry, beadStore *storage.BeadStore, notifyMgr interface {
		NotifyTaskComplete(beadID, title, assignee string) error
		NotifyAgentError(agentName, agentID, errorMsg string) error
	}) {
		defer ctx.TaskWg.Done()

		// Update status to working
		reg.UpdateStatus(agentID, "working")

		// Execute the task
		_, err := a.Chat(taskDesc)

		// Update status based on result (CompletedAt is set automatically by UpdateStatus)
		if err != nil {
			log.Printf("Associate %s failed: %v", agentID, err)
			reg.UpdateStatus(agentID, "failed")

			// Send error notification
			if notifyMgr != nil {
				if notifyErr := notifyMgr.NotifyAgentError("Associate", agentID, err.Error()); notifyErr != nil {
					log.Printf("Warning: failed to send error notification: %v", notifyErr)
				}
			}

			// If linked to a bead, mark it as blocked
			if linkedBeadID != "" && beadStore != nil {
				if bead, berr := beadStore.Get(linkedBeadID); berr == nil {
					bead.Status = models.BeadStatusBlocked
					bead.CloseReason = fmt.Sprintf("associate %s failed: %v", agentID, err)
					beadStore.Update(bead)
					log.Printf("Bead %s marked as blocked due to associate failure", linkedBeadID)
				}
			}
		} else {
			reg.UpdateStatus(agentID, "completed")

			// If linked to a bead, auto-complete it
			if linkedBeadID != "" && beadStore != nil {
				if bead, berr := beadStore.Get(linkedBeadID); berr == nil {
					bead.Status = models.BeadStatusClosed
					now := time.Now()
					bead.ClosedAt = &now
					bead.CloseReason = fmt.Sprintf("completed by associate %s", agentID)
					beadStore.Update(bead)
					log.Printf("Bead %s auto-completed by associate %s", linkedBeadID, agentID)

					// Send completion notification
					if notifyMgr != nil {
						if notifyErr := notifyMgr.NotifyTaskComplete(linkedBeadID, bead.Title, "Associate"); notifyErr != nil {
							log.Printf("Warning: failed to send completion notification: %v", notifyErr)
						}
					}
				}
			}
		}
	}(spawnedAgent, spawnedAgent.ID, task, beadID, ctx.Registry, ctx.BeadStore, ctx.NotifyManager)

	result := fmt.Sprintf("Associate spawned and working. ID: %s, Task: %s", spawnedAgent.ID, truncate(task, 50))
	if beadID != "" {
		result += fmt.Sprintf(", Linked Bead: %s", beadID)
	}
	return result, nil
}

func handleListAgents(ctx *ToolContext, args map[string]interface{}) (string, error) {
	agentType, _ := args["type"].(string)

	var agents []*registry.AgentRecord
	var err error

	if agentType != "" {
		agents, err = ctx.Registry.ListByType(agentType)
	} else {
		agents, err = ctx.Registry.List()
	}

	if err != nil {
		return "", fmt.Errorf("failed to list agents: %w", err)
	}

	if len(agents) == 0 {
		return "No agents on the payroll right now.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("The crew (%d members):\n\n", len(agents)))

	for _, a := range agents {
		name := a.Name
		if name == "" {
			name = "(no name)"
		}
		sb.WriteString(fmt.Sprintf("- %s [%s] (ID: %s)\n", name, a.Type, a.ID))
		sb.WriteString(fmt.Sprintf("  Turf: %s, Status: %s\n", a.Turf, a.Status))
		if a.Task != "" {
			sb.WriteString(fmt.Sprintf("  Current job: %s\n", truncate(a.Task, 60)))
		}
		sb.WriteString(fmt.Sprintf("  Last seen: %s\n", a.LastPing.Format(time.RFC3339)))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func handleGetAgentStatus(ctx *ToolContext, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	name, _ := args["name"].(string)

	if id == "" && name == "" {
		return "", fmt.Errorf("either id or name is required")
	}

	var agent *registry.AgentRecord
	var err error

	if id != "" {
		agent, err = ctx.Registry.Get(id)
	} else {
		agent, err = ctx.Registry.GetByName(name)
	}

	if err != nil {
		return "", fmt.Errorf("agent not found: %w", err)
	}

	data, _ := json.MarshalIndent(agent, "", "  ")
	return string(data), nil
}

func handleKillAgent(ctx *ToolContext, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	name, _ := args["name"].(string)

	if id == "" && name == "" {
		return "", fmt.Errorf("either id or name is required")
	}

	// Find the agent
	var agent *registry.AgentRecord
	var err error

	if id != "" {
		agent, err = ctx.Registry.Get(id)
	} else {
		agent, err = ctx.Registry.GetByName(name)
	}

	if err != nil {
		return "", fmt.Errorf("agent not found: %w", err)
	}

	// Kill in spawner
	if err := ctx.Spawner.Kill(agent.ID); err != nil {
		// Ignore if not found in spawner (might have been killed already)
	}

	// Remove from registry
	if err := ctx.Registry.Unregister(agent.ID); err != nil {
		return "", fmt.Errorf("failed to unregister agent: %w", err)
	}

	// If this is a soldati, also remove the TOML file
	if agent.Type == "soldati" && agent.Name != "" {
		soldatiDir := filepath.Join(ctx.MobDir, "soldati")
		if mgr, err := soldati.NewManager(soldatiDir); err == nil {
			mgr.Delete(agent.Name) // Ignore errors - file might not exist
		}
	}

	displayName := agent.Name
	if displayName == "" {
		displayName = agent.ID
	}
	return fmt.Sprintf("Agent '%s' has been sent home.", displayName), nil
}

func handleNudgeAgent(ctx *ToolContext, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	name, _ := args["name"].(string)

	if id == "" && name == "" {
		return "", fmt.Errorf("either id or name is required")
	}

	// Find the agent
	var agent *registry.AgentRecord
	var err error

	if id != "" {
		agent, err = ctx.Registry.Get(id)
	} else {
		agent, err = ctx.Registry.GetByName(name)
	}

	if err != nil {
		return "", fmt.Errorf("agent not found: %w", err)
	}

	// Update the ping time
	if err := ctx.Registry.Ping(agent.ID); err != nil {
		return "", fmt.Errorf("failed to nudge agent: %w", err)
	}

	displayName := agent.Name
	if displayName == "" {
		displayName = agent.ID
	}
	return fmt.Sprintf("Nudged '%s'. They better be working.", displayName), nil
}

func handleAssignBead(ctx *ToolContext, args map[string]interface{}) (string, error) {
	agentID, _ := args["agent_id"].(string)
	agentName, _ := args["agent_name"].(string)
	beadID, _ := args["bead_id"].(string)
	description, _ := args["description"].(string)

	if agentID == "" && agentName == "" {
		return "", fmt.Errorf("either agent_id or agent_name is required")
	}
	if beadID == "" && description == "" {
		return "", fmt.Errorf("either bead_id or description is required")
	}

	// Find the agent
	var agentRecord *registry.AgentRecord
	var err error

	if agentID != "" {
		agentRecord, err = ctx.Registry.Get(agentID)
	} else {
		agentRecord, err = ctx.Registry.GetByName(agentName)
	}

	if err != nil {
		return "", fmt.Errorf("agent not found: %w", err)
	}

	// Determine task description
	taskDesc := description
	var worktreePath string
	if beadID != "" {
		taskDesc = fmt.Sprintf("bead:%s", beadID)

		// If bead_id is provided, update the bead's assignee and status
		if ctx.BeadStore != nil {
			bead, err := ctx.BeadStore.Get(beadID)
			if err != nil {
				return "", fmt.Errorf("bead not found: %w", err)
			}

			// Check if bead is pending approval
			if bead.Status == models.BeadStatusPendingApproval {
				return "", fmt.Errorf("bead %s is pending approval - use 'mob approve %s' to approve it before assigning", beadID, beadID)
			}

			// Update assignee to the agent's name (or ID if no name)
			assigneeName := agentRecord.Name
			if assigneeName == "" {
				assigneeName = agentRecord.ID
			}
			bead.Assignee = assigneeName
			bead.Status = models.BeadStatusInProgress

			// Create worktree for this bead if turf is set
			if bead.Turf != "" && ctx.TurfManager != nil {
				turfInfo, err := ctx.TurfManager.Get(bead.Turf)
				if err == nil {
					// Create worktree manager for this turf's repo
					wtMgr, err := git.NewWorktreeManager(turfInfo.Path)
					if err == nil {
						// Try to create worktree (may already exist)
						wt, err := wtMgr.Create(beadID)
						if err == nil {
							worktreePath = wt.Path
							bead.WorktreePath = worktreePath
							log.Printf("Created worktree for bead %s at %s", beadID, worktreePath)
						} else if err == git.ErrWorktreeExists {
							// Worktree already exists, get its path
							wt, _ := wtMgr.Get(beadID)
							if wt != nil {
								worktreePath = wt.Path
								bead.WorktreePath = worktreePath
							}
						} else {
							log.Printf("Warning: failed to create worktree for bead %s: %v", beadID, err)
						}
					} else {
						log.Printf("Warning: failed to create worktree manager for turf %s: %v", bead.Turf, err)
					}
				}
			}

			if _, err := ctx.BeadStore.Update(bead); err != nil {
				return "", fmt.Errorf("failed to update bead: %w", err)
			}
		}
	}

	// Update agent's task in registry
	if err := ctx.Registry.UpdateTask(agentRecord.ID, taskDesc); err != nil {
		return "", fmt.Errorf("failed to assign task: %w", err)
	}

	// Update status to active
	if err := ctx.Registry.UpdateStatus(agentRecord.ID, "active"); err != nil {
		return "", fmt.Errorf("failed to update status: %w", err)
	}

	// Write hook file so daemon processes the work
	hookDir := filepath.Join(ctx.MobDir, ".mob", "soldati")
	hookMgr, err := hook.NewManager(hookDir, agentRecord.Name)
	if err != nil {
		return "", fmt.Errorf("failed to create hook manager: %w", err)
	}

	h := &hook.Hook{
		Type:      hook.HookTypeAssign,
		BeadID:    beadID,
		Message:   description,
		Timestamp: time.Now(),
	}
	if err := hookMgr.Write(h); err != nil {
		return "", fmt.Errorf("failed to write hook: %w", err)
	}

	displayName := agentRecord.Name
	if displayName == "" {
		displayName = agentRecord.ID
	}
	result := fmt.Sprintf("Assigned work to '%s': %s", displayName, truncate(taskDesc, 50))
	if worktreePath != "" {
		result += fmt.Sprintf("\nWorktree: %s", worktreePath)
	}
	return result, nil
}

func handleCreateBead(ctx *ToolContext, args map[string]interface{}) (string, error) {
	title, _ := args["title"].(string)
	if title == "" {
		return "", fmt.Errorf("title is required")
	}

	if ctx.BeadStore == nil {
		return "", fmt.Errorf("bead store not available")
	}

	// Build the bead from args
	// Check if pending_approval is requested
	status := models.BeadStatusOpen
	if pendingApproval, ok := args["pending_approval"].(bool); ok && pendingApproval {
		status = models.BeadStatusPendingApproval
	}

	bead := &models.Bead{
		Title:  title,
		Status: status,
	}

	// Optional fields
	if description, ok := args["description"].(string); ok {
		bead.Description = description
	}
	if beadType, ok := args["type"].(string); ok && beadType != "" {
		bead.Type = models.BeadType(beadType)
	} else {
		bead.Type = models.BeadTypeTask // Default to task
	}
	if priority, ok := args["priority"].(float64); ok {
		bead.Priority = int(priority)
	} else {
		bead.Priority = 2 // Default to medium priority
	}
	if turf, ok := args["turf"].(string); ok {
		bead.Turf = turf
	}
	if labels, ok := args["labels"].(string); ok {
		bead.Labels = labels
	}
	if parentID, ok := args["parent_id"].(string); ok {
		bead.ParentID = parentID
	}
	if blocks, ok := args["blocks"].([]interface{}); ok {
		bead.Blocks = make([]string, 0, len(blocks))
		for _, b := range blocks {
			if s, ok := b.(string); ok {
				bead.Blocks = append(bead.Blocks, s)
			}
		}
	}
	if related, ok := args["related"].([]interface{}); ok {
		bead.Related = make([]string, 0, len(related))
		for _, r := range related {
			if s, ok := r.(string); ok {
				bead.Related = append(bead.Related, s)
			}
		}
	}

	// Create the bead
	createdBead, err := ctx.BeadStore.Create(bead)
	if err != nil {
		return "", fmt.Errorf("failed to create bead: %w", err)
	}

	// Send notification if approval is needed
	if createdBead.Status == models.BeadStatusPendingApproval && ctx.NotifyManager != nil {
		if err := ctx.NotifyManager.NotifyApprovalNeeded(createdBead.ID, createdBead.Title); err != nil {
			log.Printf("Warning: failed to send approval notification: %v", err)
		}
	}

	// Format a nice response
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("New job on the board: %s\n\n", createdBead.ID))
	sb.WriteString(fmt.Sprintf("Title: %s\n", createdBead.Title))
	sb.WriteString(fmt.Sprintf("Type: %s\n", createdBead.Type))
	sb.WriteString(fmt.Sprintf("Priority: %d\n", createdBead.Priority))
	sb.WriteString(fmt.Sprintf("Status: %s\n", createdBead.Status))
	if createdBead.Status == models.BeadStatusPendingApproval {
		sb.WriteString("\n⚠ This bead requires approval before work can start.\n")
		sb.WriteString(fmt.Sprintf("Approve with: mob approve %s\n", createdBead.ID))
	}
	if createdBead.Turf != "" {
		sb.WriteString(fmt.Sprintf("Turf: %s\n", createdBead.Turf))
	}
	if createdBead.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", truncate(createdBead.Description, 100)))
	}
	sb.WriteString(fmt.Sprintf("Branch: %s\n", createdBead.Branch))

	return sb.String(), nil
}

func handleListBeads(ctx *ToolContext, args map[string]interface{}) (string, error) {
	if ctx.BeadStore == nil {
		return "", fmt.Errorf("bead store not available")
	}

	// Build filter from args
	filter := storage.BeadFilter{}

	if status, ok := args["status"].(string); ok && status != "" {
		filter.Status = models.BeadStatus(status)
	}
	if turf, ok := args["turf"].(string); ok {
		filter.Turf = turf
	}
	if assignee, ok := args["assignee"].(string); ok {
		filter.Assignee = assignee
	}
	if beadType, ok := args["type"].(string); ok && beadType != "" {
		filter.Type = models.BeadType(beadType)
	}

	beads, err := ctx.BeadStore.List(filter)
	if err != nil {
		return "", fmt.Errorf("failed to list beads: %w", err)
	}

	if len(beads) == 0 {
		return "No jobs on the board matching those filters.", nil
	}

	// Priority labels for display
	priorityLabels := []string{"🔴 Critical", "🟠 High", "🟡 Medium", "🔵 Low", "⚪ Lowest"}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("The job board (%d items):\n\n", len(beads)))

	for _, bead := range beads {
		// Priority indicator
		priority := bead.Priority
		if priority < 0 {
			priority = 0
		}
		if priority > 4 {
			priority = 4
		}
		priorityLabel := priorityLabels[priority]

		sb.WriteString(fmt.Sprintf("• [%s] %s\n", bead.ID, bead.Title))
		sb.WriteString(fmt.Sprintf("  %s | %s | %s\n", priorityLabel, bead.Type, bead.Status))
		if bead.Assignee != "" {
			sb.WriteString(fmt.Sprintf("  Assigned to: %s\n", bead.Assignee))
		}
		if bead.Turf != "" {
			sb.WriteString(fmt.Sprintf("  Turf: %s\n", bead.Turf))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func handleListReadyBeads(ctx *ToolContext, args map[string]interface{}) (string, error) {
	if ctx.BeadStore == nil {
		return "", fmt.Errorf("bead store not available")
	}

	turf, _ := args["turf"].(string)
	limit := 10 // Default limit
	if limitArg, ok := args["limit"].(float64); ok {
		limit = int(limitArg)
	}

	beads, err := ctx.BeadStore.ListReady(turf)
	if err != nil {
		return "", fmt.Errorf("failed to list ready beads: %w", err)
	}

	// Apply limit
	if len(beads) > limit {
		beads = beads[:limit]
	}

	if len(beads) == 0 {
		if turf != "" {
			return fmt.Sprintf("No ready beads for turf '%s'.", turf), nil
		}
		return "No ready beads right now.", nil
	}

	// Priority labels for display
	priorityLabels := []string{"🔴 Critical", "🟠 High", "🟡 Medium", "🔵 Low", "⚪ Lowest"}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Ready beads (%d):\n\n", len(beads)))

	for _, bead := range beads {
		// Priority indicator
		priority := bead.Priority
		if priority < 0 {
			priority = 0
		}
		if priority > 4 {
			priority = 4
		}
		priorityLabel := priorityLabels[priority]

		sb.WriteString(fmt.Sprintf("• [%s] %s\n", bead.ID, bead.Title))
		sb.WriteString(fmt.Sprintf("  %s | %s | %s\n", priorityLabel, bead.Type, bead.Status))
		if bead.Turf != "" {
			sb.WriteString(fmt.Sprintf("  Turf: %s\n", bead.Turf))
		}
		if bead.Description != "" {
			sb.WriteString(fmt.Sprintf("  Description: %s\n", truncate(bead.Description, 80)))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func handleGetBead(ctx *ToolContext, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)

	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	if ctx.BeadStore == nil {
		return "", fmt.Errorf("bead store not available")
	}

	bead, err := ctx.BeadStore.Get(id)
	if err != nil {
		return "", fmt.Errorf("bead not found: %w", err)
	}

	data, err := json.MarshalIndent(bead, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize bead: %w", err)
	}

	return string(data), nil
}

func handleUpdateBead(ctx *ToolContext, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)

	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	if ctx.BeadStore == nil {
		return "", fmt.Errorf("bead store not available")
	}

	// Fetch existing bead
	bead, err := ctx.BeadStore.Get(id)
	if err != nil {
		return "", fmt.Errorf("bead not found: %w", err)
	}

	// Update only fields that are provided
	if title, ok := args["title"].(string); ok && title != "" {
		bead.Title = title
	}
	if description, ok := args["description"].(string); ok && description != "" {
		bead.Description = description
	}
	if status, ok := args["status"].(string); ok && status != "" {
		bead.Status = models.BeadStatus(status)
	}
	if priority, ok := args["priority"].(float64); ok {
		bead.Priority = int(priority)
	}
	if assignee, ok := args["assignee"].(string); ok {
		bead.Assignee = assignee
	}
	if labels, ok := args["labels"].(string); ok {
		bead.Labels = labels
	}
	if blocks, ok := args["blocks"].([]interface{}); ok {
		bead.Blocks = make([]string, 0, len(blocks))
		for _, b := range blocks {
			if s, ok := b.(string); ok {
				bead.Blocks = append(bead.Blocks, s)
			}
		}
	}
	if related, ok := args["related"].([]interface{}); ok {
		bead.Related = make([]string, 0, len(related))
		for _, r := range related {
			if s, ok := r.(string); ok {
				bead.Related = append(bead.Related, s)
			}
		}
	}

	// Save the updated bead
	updatedBead, err := ctx.BeadStore.Update(bead)
	if err != nil {
		return "", fmt.Errorf("failed to update bead: %w", err)
	}

	data, err := json.MarshalIndent(updatedBead, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize bead: %w", err)
	}

	return string(data), nil
}

func handleCompleteBead(ctx *ToolContext, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	closeReason, _ := args["close_reason"].(string)

	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	if ctx.BeadStore == nil {
		return "", fmt.Errorf("bead store not available")
	}

	// Fetch existing bead
	bead, err := ctx.BeadStore.Get(id)
	if err != nil {
		return "", fmt.Errorf("bead not found: %w", err)
	}

	var mergeResult *merge.MergeResult
	var mergeErr error

	// If bead has a worktree and turf, attempt to merge the work
	if bead.WorktreePath != "" && bead.Turf != "" && ctx.TurfManager != nil {
		turfInfo, err := ctx.TurfManager.Get(bead.Turf)
		if err == nil {
			// Create merge queue for this repo
			mq := merge.New(turfInfo.Path)

			// Add the bead to merge queue
			if err := mq.Add(bead.ID, bead.Branch, bead.Turf, bead.Blocks); err != nil && err != merge.ErrItemExists {
				log.Printf("Warning: failed to add bead %s to merge queue: %v", bead.ID, err)
			}

			// Process the merge
			mergeResult, mergeErr = mq.Process()
			if mergeErr != nil {
				log.Printf("Warning: merge processing error for bead %s: %v", bead.ID, mergeErr)
			}

			// If merge succeeded, clean up the worktree
			if mergeResult != nil && mergeResult.Success {
				wtMgr, err := git.NewWorktreeManager(turfInfo.Path)
				if err == nil {
					if err := wtMgr.Remove(bead.ID, true); err != nil {
						log.Printf("Warning: failed to remove worktree for bead %s: %v", bead.ID, err)
					} else {
						log.Printf("Removed worktree and branch for bead %s", bead.ID)
						bead.WorktreePath = "" // Clear the path since worktree is gone
					}
				}
			} else if mergeResult != nil && !mergeResult.Success {
				// Merge failed - mark bead as blocked instead of closed
				bead.Status = models.BeadStatusBlocked
				bead.CloseReason = fmt.Sprintf("merge failed: %s", mergeResult.Message)
				if _, err := ctx.BeadStore.Update(bead); err != nil {
					return "", fmt.Errorf("failed to update bead: %w", err)
				}
				return fmt.Sprintf("Job '%s' merge failed: %s. Bead marked as blocked.", bead.Title, mergeResult.Message), nil
			}
		}
	}

	// Mark as completed
	bead.Status = models.BeadStatusClosed
	now := time.Now()
	bead.ClosedAt = &now
	if closeReason != "" {
		bead.CloseReason = closeReason
	} else {
		bead.CloseReason = "completed"
	}

	// Save the updated bead
	_, err = ctx.BeadStore.Update(bead)
	if err != nil {
		return "", fmt.Errorf("failed to complete bead: %w", err)
	}

	// Send notification about task completion
	if ctx.NotifyManager != nil {
		assignee := bead.Assignee
		if assignee == "" {
			assignee = "Unknown"
		}
		if err := ctx.NotifyManager.NotifyTaskComplete(bead.ID, bead.Title, assignee); err != nil {
			log.Printf("Warning: failed to send completion notification: %v", err)
		}
	}

	result := fmt.Sprintf("Job '%s' is done. Closed at %s.", bead.Title, now.Format(time.RFC3339))
	if mergeResult != nil && mergeResult.Success {
		result += fmt.Sprintf(" Branch merged: %s", mergeResult.Message)
	}
	return result, nil
}

// GenerateMCPConfig creates an MCP config file for Claude
func GenerateMCPConfig(mobDir string) (string, error) {
	// Find the mob binary path
	mobPath, err := os.Executable()
	if err != nil {
		mobPath = "mob" // Fall back to PATH lookup
	}

	registryPath := filepath.Join(mobDir, ".mob", "agents.json")

	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"mob-tools": map[string]interface{}{
				"command": mobPath,
				"args":    []string{"mcp-server", "--registry", registryPath, "--mob-dir", mobDir},
			},
		},
	}

	// Ensure config directory exists
	configDir := filepath.Join(mobDir, ".mob")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}

	configPath := filepath.Join(configDir, "mcp-config.json")
	data, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", err
	}

	return configPath, nil
}

func handleCommentOnBead(ctx *ToolContext, args map[string]interface{}) (string, error) {
	beadID, _ := args["bead_id"].(string)
	comment, _ := args["comment"].(string)
	actor, _ := args["actor"].(string)

	if beadID == "" {
		return "", fmt.Errorf("bead_id is required")
	}
	if comment == "" {
		return "", fmt.Errorf("comment is required")
	}

	if ctx.BeadStore == nil {
		return "", fmt.Errorf("bead store not available")
	}

	// Default actor to "user" if not specified
	if actor == "" {
		actor = "user"
	}

	// Add the comment to the bead's history
	if err := ctx.BeadStore.AddComment(beadID, actor, comment); err != nil {
		return "", fmt.Errorf("failed to add comment: %w", err)
	}

	return fmt.Sprintf("Comment added to bead %s by %s", beadID, actor), nil
}

func handleListTurfs(ctx *ToolContext, args map[string]interface{}) (string, error) {
	if ctx.TurfManager == nil {
		return "", fmt.Errorf("turf manager not available")
	}

	turfs := ctx.TurfManager.List()

	if len(turfs) == 0 {
		return "No turfs registered. Use 'mob turf add' to register a project.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Registered turfs (%d):\n\n", len(turfs)))

	for _, t := range turfs {
		sb.WriteString(fmt.Sprintf("• %s\n", t.Name))
		sb.WriteString(fmt.Sprintf("  Path: %s\n", t.Path))
		if t.MainBranch != "" {
			sb.WriteString(fmt.Sprintf("  Main branch: %s\n", t.MainBranch))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
