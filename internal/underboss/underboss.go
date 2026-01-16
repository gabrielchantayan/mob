package underboss

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gabe/mob/internal/agent"
	"github.com/gabe/mob/internal/mcp"
	"github.com/gabe/mob/internal/registry"
	"github.com/gabe/mob/internal/soldati"
)

var (
	// ErrUnderbossNotRunning is returned when operations require a running Underboss
	ErrUnderbossNotRunning = errors.New("underboss is not running")

	// ErrUnderbossAlreadyRunning is returned when trying to start an already running Underboss
	ErrUnderbossAlreadyRunning = errors.New("underboss is already running")
)

// Underboss manages the persistent chief-of-staff Claude instance
type Underboss struct {
	agent         *agent.Agent
	spawner       *agent.Spawner
	registry      *registry.Registry
	mobDir        string
	mcpConfigPath string
	mcpEnabled    bool
	mu            sync.RWMutex
}

// New creates a new Underboss manager
func New(mobDir string, spawner *agent.Spawner) *Underboss {
	reg := registry.New(registry.DefaultPath(mobDir))
	return &Underboss{
		mobDir:     mobDir,
		spawner:    spawner,
		registry:   reg,
		mcpEnabled: true, // Enable MCP by default
	}
}

// NewWithRegistry creates a new Underboss manager with a custom registry
func NewWithRegistry(mobDir string, spawner *agent.Spawner, reg *registry.Registry) *Underboss {
	return &Underboss{
		mobDir:     mobDir,
		spawner:    spawner,
		registry:   reg,
		mcpEnabled: true,
	}
}

// SetMCPEnabled enables or disables MCP tools for the Underboss
func (u *Underboss) SetMCPEnabled(enabled bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.mcpEnabled = enabled
}

// Start spawns or reconnects to the Underboss agent
func (u *Underboss) Start(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.agent != nil && u.agent.IsRunning() {
		return ErrUnderbossAlreadyRunning
	}

	// Use the mob directory as the working directory for the underboss
	workDir := u.mobDir
	if workDir == "" {
		// Fall back to current directory
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	// Ensure the work directory exists
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return err
	}

	// Generate MCP config if enabled
	var mcpConfigPath string
	if u.mcpEnabled {
		var err error
		mcpConfigPath, err = mcp.GenerateMCPConfig(workDir)
		if err != nil {
			// Log warning but continue without MCP
			fmt.Fprintf(os.Stderr, "Warning: failed to generate MCP config: %v\n", err)
		} else {
			u.mcpConfigPath = mcpConfigPath
		}
	}

	// Spawn the underboss agent with personality and MCP tools
	a, err := u.spawner.SpawnWithOptions(agent.SpawnOptions{
		Type:         agent.AgentTypeUnderboss,
		Name:         "underboss",
		Turf:         "",
		WorkDir:      workDir,
		SystemPrompt: DefaultSystemPrompt,
		MCPConfig:    mcpConfigPath,
	})
	if err != nil {
		return err
	}

	u.agent = a
	return nil
}

// Stop terminates the Underboss agent
func (u *Underboss) Stop() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.agent == nil {
		return ErrUnderbossNotRunning
	}

	err := u.agent.Kill()
	u.agent = nil
	return err
}

// IsRunning returns true if Underboss is active
func (u *Underboss) IsRunning() bool {
	u.mu.RLock()
	defer u.mu.RUnlock()

	return u.agent != nil && u.agent.IsRunning()
}

// Agent returns the underlying agent
func (u *Underboss) Agent() *agent.Agent {
	u.mu.RLock()
	defer u.mu.RUnlock()

	return u.agent
}

// MobDir returns the mob directory path
func (u *Underboss) MobDir() string {
	return u.mobDir
}

// GetUnderbossDir returns the path to the underboss-specific directory
func (u *Underboss) GetUnderbossDir() string {
	return filepath.Join(u.mobDir, "underboss")
}

// Ask sends a question to the Underboss and returns the response.
// It will start the Underboss if not already running.
func (u *Underboss) Ask(ctx context.Context, question string) (string, error) {
	resp, err := u.AskFull(ctx, question)
	if err != nil {
		return "", err
	}
	return resp.GetText(), nil
}

// AskFull sends a question and returns the full response with all content blocks.
func (u *Underboss) AskFull(ctx context.Context, question string) (*agent.ChatResponse, error) {
	// Start Underboss if not running
	if !u.IsRunning() {
		if err := u.Start(ctx); err != nil {
			return nil, err
		}
	}

	u.mu.RLock()
	a := u.agent
	u.mu.RUnlock()

	if a == nil {
		return nil, ErrUnderbossNotRunning
	}

	return a.Chat(question)
}

// AskStream sends a question with streaming callback for real-time updates.
func (u *Underboss) AskStream(ctx context.Context, question string, callback agent.StreamCallback) (*agent.ChatResponse, error) {
	// Start Underboss if not running
	if !u.IsRunning() {
		if err := u.Start(ctx); err != nil {
			return nil, err
		}
	}

	u.mu.RLock()
	a := u.agent
	u.mu.RUnlock()

	if a == nil {
		return nil, ErrUnderbossNotRunning
	}

	return a.ChatStream(question, callback)
}

// Tell sends an instruction to the Underboss and returns the acknowledgment.
// It will start the Underboss if not already running.
func (u *Underboss) Tell(ctx context.Context, instruction string) (string, error) {
	resp, err := u.AskFull(ctx, instruction)
	if err != nil {
		return "", err
	}
	return resp.GetText(), nil
}

// Registry returns the agent registry
func (u *Underboss) Registry() *registry.Registry {
	return u.registry
}

// SpawnSoldati creates a new persistent worker (Soldati)
// This provides direct access for CLI/TUI, bypassing MCP
func (u *Underboss) SpawnSoldati(name, turf, workDir string) (*agent.Agent, error) {
	// Create soldati manager to persist to .toml files (for CLI visibility)
	soldatiDir := filepath.Join(u.mobDir, "soldati")
	mgr, err := soldati.NewManager(soldatiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create soldati manager: %w", err)
	}

	// Generate name if not provided
	if name == "" {
		// Check both registry and persisted soldati for used names
		agents, _ := u.registry.ListByType("soldati")
		persistedSoldati, _ := mgr.List()

		usedNames := make(map[string]bool)
		for _, a := range agents {
			usedNames[a.Name] = true
		}
		for _, s := range persistedSoldati {
			usedNames[s.Name] = true
		}

		nameSlice := make([]string, 0, len(usedNames))
		for n := range usedNames {
			nameSlice = append(nameSlice, n)
		}
		name = soldati.GenerateUniqueName(nameSlice)
	}

	// Default work directory
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	// Persist soldati to .toml file (so CLI can see it)
	if _, err := mgr.Create(name); err != nil {
		// If it already exists, that's fine - just continue
		if _, getErr := mgr.Get(name); getErr != nil {
			return nil, fmt.Errorf("failed to persist soldati: %w", err)
		}
	}

	// Spawn the agent with system prompt
	a, err := u.spawner.SpawnWithOptions(agent.SpawnOptions{
		Type:         agent.AgentTypeSoldati,
		Name:         name,
		Turf:         turf,
		WorkDir:      workDir,
		SystemPrompt: agent.SoldatiSystemPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to spawn soldati: %w", err)
	}

	// Register in registry
	record := &registry.AgentRecord{
		ID:        a.ID,
		Type:      "soldati",
		Name:      name,
		Turf:      turf,
		Status:    "active",
		StartedAt: a.StartedAt,
	}
	if err := u.registry.Register(record); err != nil {
		return nil, fmt.Errorf("failed to register soldati: %w", err)
	}

	return a, nil
}

// SpawnAssociate creates a new temporary worker (Associate)
// This provides direct access for CLI/TUI, bypassing MCP
func (u *Underboss) SpawnAssociate(turf, task, workDir string) (*agent.Agent, error) {
	// Default work directory
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	// Spawn the agent with system prompt
	a, err := u.spawner.SpawnWithOptions(agent.SpawnOptions{
		Type:         agent.AgentTypeAssociate,
		Name:         "", // Associates don't get names
		Turf:         turf,
		WorkDir:      workDir,
		SystemPrompt: agent.AssociateSystemPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to spawn associate: %w", err)
	}

	// Register in registry
	record := &registry.AgentRecord{
		ID:        a.ID,
		Type:      "associate",
		Turf:      turf,
		Task:      task,
		Status:    "active",
		StartedAt: a.StartedAt,
	}
	if err := u.registry.Register(record); err != nil {
		return nil, fmt.Errorf("failed to register associate: %w", err)
	}

	return a, nil
}

// ListAgents returns all agents from the registry
func (u *Underboss) ListAgents() ([]*registry.AgentRecord, error) {
	return u.registry.List()
}

// ListSoldati returns all soldati from the registry
func (u *Underboss) ListSoldati() ([]*registry.AgentRecord, error) {
	return u.registry.ListByType("soldati")
}

// ListAssociates returns all associates from the registry
func (u *Underboss) ListAssociates() ([]*registry.AgentRecord, error) {
	return u.registry.ListByType("associate")
}

// GetAgent retrieves an agent by ID or name
func (u *Underboss) GetAgent(idOrName string) (*registry.AgentRecord, error) {
	// Try by ID first
	if record, err := u.registry.Get(idOrName); err == nil {
		return record, nil
	}
	// Try by name
	return u.registry.GetByName(idOrName)
}

// KillAgent terminates an agent by ID or name
func (u *Underboss) KillAgent(idOrName string) error {
	// Find the agent
	record, err := u.GetAgent(idOrName)
	if err != nil {
		return err
	}

	// Kill in spawner (ignore error if not found)
	_ = u.spawner.Kill(record.ID)

	// Remove from registry
	return u.registry.Unregister(record.ID)
}

// UpdateAgentStatus updates an agent's status
func (u *Underboss) UpdateAgentStatus(idOrName, status string) error {
	record, err := u.GetAgent(idOrName)
	if err != nil {
		return err
	}
	return u.registry.UpdateStatus(record.ID, status)
}

// AssignTask assigns a task to an agent
func (u *Underboss) AssignTask(idOrName, task string) error {
	record, err := u.GetAgent(idOrName)
	if err != nil {
		return err
	}
	return u.registry.UpdateTask(record.ID, task)
}

// NudgeAgent pings an agent to update its last seen time
func (u *Underboss) NudgeAgent(idOrName string) error {
	record, err := u.GetAgent(idOrName)
	if err != nil {
		return err
	}
	return u.registry.Ping(record.ID)
}

// AgentInfo contains information about a spawned agent
type AgentInfo struct {
	ID        string
	Type      string
	Name      string
	Turf      string
	Status    string
	Task      string
	StartedAt time.Time
	LastPing  time.Time
}
