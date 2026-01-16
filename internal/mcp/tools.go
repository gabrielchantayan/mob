package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabe/mob/internal/agent"
	"github.com/gabe/mob/internal/registry"
	"github.com/gabe/mob/internal/soldati"
)

// ToolContext provides access to mob systems for tool handlers
type ToolContext struct {
	Registry *registry.Registry
	Spawner  *agent.Spawner
	MobDir   string
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

	// Spawn the agent with the Soldati system prompt
	spawnedAgent, err := ctx.Spawner.SpawnWithOptions(agent.SpawnOptions{
		Type:         agent.AgentTypeSoldati,
		Name:         name,
		Turf:         turf,
		WorkDir:      workDir,
		SystemPrompt: agent.SoldatiSystemPrompt,
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

	// Spawn the agent with the Associate system prompt
	spawnedAgent, err := ctx.Spawner.SpawnWithOptions(agent.SpawnOptions{
		Type:         agent.AgentTypeAssociate,
		Name:         "", // Associates don't get names
		Turf:         turf,
		WorkDir:      workDir,
		SystemPrompt: agent.AssociateSystemPrompt,
	})
	if err != nil {
		return "", fmt.Errorf("failed to spawn associate: %w", err)
	}

	// Register in registry
	record := &registry.AgentRecord{
		ID:        spawnedAgent.ID,
		Type:      "associate",
		Turf:      turf,
		Task:      task,
		Status:    "active",
		StartedAt: spawnedAgent.StartedAt,
	}
	if err := ctx.Registry.Register(record); err != nil {
		return "", fmt.Errorf("failed to register associate: %w", err)
	}

	// Execute the task in a background goroutine
	go func(a *agent.Agent, agentID string, taskDesc string, reg *registry.Registry) {
		// Update status to working
		reg.UpdateStatus(agentID, "working")

		// Execute the task
		_, err := a.Chat(taskDesc)

		// Update status based on result
		if err != nil {
			reg.UpdateStatus(agentID, "failed")
		} else {
			reg.UpdateStatus(agentID, "completed")
		}
	}(spawnedAgent, spawnedAgent.ID, task, ctx.Registry)

	return fmt.Sprintf("Associate spawned and working. ID: %s, Task: %s", spawnedAgent.ID, truncate(task, 50)), nil
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
	var agent *registry.AgentRecord
	var err error

	if agentID != "" {
		agent, err = ctx.Registry.Get(agentID)
	} else {
		agent, err = ctx.Registry.GetByName(agentName)
	}

	if err != nil {
		return "", fmt.Errorf("agent not found: %w", err)
	}

	// Determine task description
	taskDesc := description
	if beadID != "" {
		taskDesc = fmt.Sprintf("bead:%s", beadID)
	}

	// Update agent's task
	if err := ctx.Registry.UpdateTask(agent.ID, taskDesc); err != nil {
		return "", fmt.Errorf("failed to assign task: %w", err)
	}

	displayName := agent.Name
	if displayName == "" {
		displayName = agent.ID
	}
	return fmt.Sprintf("Assigned work to '%s': %s", displayName, truncate(taskDesc, 50)), nil
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
