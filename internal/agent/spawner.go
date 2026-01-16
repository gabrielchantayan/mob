package agent

import (
	"crypto/rand"
	"encoding/hex"
	"os/exec"
	"sync"
	"time"

	"github.com/gabe/mob/internal/ipc"
)

// CommandCreator is a function type that creates exec.Cmd instances
// This allows for dependency injection in tests
type CommandCreator func(name string, args ...string) *exec.Cmd

// defaultCommandCreator uses exec.Command
func defaultCommandCreator(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// Spawner manages spawning and tracking Claude Code instances
type Spawner struct {
	claudePath     string              // path to claude binary (default: "claude")
	agents         map[string]*Agent
	mu             sync.RWMutex
	commandCreator CommandCreator      // for dependency injection in tests
}

// NewSpawner creates a new spawner
func NewSpawner() *Spawner {
	return &Spawner{
		claudePath:     "claude",
		agents:         make(map[string]*Agent),
		commandCreator: defaultCommandCreator,
	}
}

// NewSpawnerWithPath creates a new spawner with a custom claude binary path
func NewSpawnerWithPath(claudePath string) *Spawner {
	return &Spawner{
		claudePath:     claudePath,
		agents:         make(map[string]*Agent),
		commandCreator: defaultCommandCreator,
	}
}

// SetCommandCreator sets a custom command creator (useful for testing)
func (s *Spawner) SetCommandCreator(cc CommandCreator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commandCreator = cc
}

// generateID creates a unique identifier for an agent
func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if random fails
		return hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000")))
	}
	return hex.EncodeToString(b)
}

// Spawn starts a new Claude Code instance
// - Runs: claude --dangerously-skip-permissions --print jsonrpc
// - Sets working directory
// - Connects stdin/stdout to IPC client
func (s *Spawner) Spawn(agentType AgentType, name string, turf string, workDir string) (*Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create the command
	cmd := s.commandCreator(s.claudePath, "--dangerously-skip-permissions", "--print", "jsonrpc")
	cmd.Dir = workDir

	// Set up pipes for IPC
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Create IPC client
	client := ipc.NewClient(stdin, stdout)

	// Create agent
	id := generateID()
	agent := &Agent{
		ID:        id,
		Type:      agentType,
		Name:      name,
		Turf:      turf,
		Cmd:       cmd,
		Client:    client,
		StartedAt: time.Now(),
	}

	// Track the agent
	s.agents[id] = agent

	return agent, nil
}

// Get returns an agent by ID
func (s *Spawner) Get(id string) (*Agent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent, ok := s.agents[id]
	return agent, ok
}

// List returns all agents
func (s *Spawner) List() []*Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]*Agent, 0, len(s.agents))
	for _, agent := range s.agents {
		agents = append(agents, agent)
	}
	return agents
}

// Kill terminates an agent by ID
func (s *Spawner) Kill(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[id]
	if !ok {
		return ErrAgentNotFound
	}

	// Kill the process
	if err := agent.Kill(); err != nil {
		return err
	}

	// Remove from tracking
	delete(s.agents, id)
	return nil
}

// KillAll terminates all agents
func (s *Spawner) KillAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, agent := range s.agents {
		// Best effort kill - ignore errors
		_ = agent.Kill()
		delete(s.agents, id)
	}
}

// Count returns the number of tracked agents
func (s *Spawner) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.agents)
}
