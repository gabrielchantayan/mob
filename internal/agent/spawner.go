package agent

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os/exec"
	"sync"
	"time"
)

// AgentOutput represents a line of output from an agent
type AgentOutput struct {
	AgentID   string
	AgentName string
	Line      string
	Timestamp time.Time
	Stream    string // "stdout" or "stderr"
}

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
	outputChan     chan AgentOutput    // broadcast channel for agent output
	outputSubs     []chan AgentOutput  // subscribers to agent output
	outputSubsMu   sync.RWMutex        // protects outputSubs
}

// NewSpawner creates a new spawner
func NewSpawner() *Spawner {
	s := &Spawner{
		claudePath:     "claude",
		agents:         make(map[string]*Agent),
		commandCreator: defaultCommandCreator,
		outputChan:     make(chan AgentOutput, 1000),
		outputSubs:     make([]chan AgentOutput, 0),
	}
	// Start output broadcaster
	go s.broadcastOutput()
	return s
}

// NewSpawnerWithPath creates a new spawner with a custom claude binary path
func NewSpawnerWithPath(claudePath string) *Spawner {
	s := &Spawner{
		claudePath:     claudePath,
		agents:         make(map[string]*Agent),
		commandCreator: defaultCommandCreator,
		outputChan:     make(chan AgentOutput, 1000),
		outputSubs:     make([]chan AgentOutput, 0),
	}
	// Start output broadcaster
	go s.broadcastOutput()
	return s
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

// SpawnOptions configures agent creation
type SpawnOptions struct {
	Type         AgentType
	Name         string
	Turf         string
	WorkDir      string
	SystemPrompt string // Injected on first call via --system-prompt
	MCPConfig    string // Path to MCP config JSON file
	Model        string // Model to use (e.g., "sonnet", "opus") - passed as --model flag
}

// Spawn creates a new Claude Code agent that can send messages
// Uses Claude's stream-json protocol with -p mode and --resume for session continuity
func (s *Spawner) Spawn(agentType AgentType, name string, turf string, workDir string) (*Agent, error) {
	return s.SpawnWithOptions(SpawnOptions{
		Type:    agentType,
		Name:    name,
		Turf:    turf,
		WorkDir: workDir,
	})
}

// SpawnWithOptions creates a new agent with full configuration
func (s *Spawner) SpawnWithOptions(opts SpawnOptions) (*Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create agent (no process yet - spawns per-call)
	id := generateID()
	agent := &Agent{
		ID:           id,
		Type:         opts.Type,
		Name:         opts.Name,
		Turf:         opts.Turf,
		WorkDir:      opts.WorkDir,
		SystemPrompt: opts.SystemPrompt,
		MCPConfig:    opts.MCPConfig,
		Model:        opts.Model,
		StartedAt:    time.Now(),
		spawner:      s,
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

// SubscribeOutput creates a new subscription channel for agent output
func (s *Spawner) SubscribeOutput() <-chan AgentOutput {
	s.outputSubsMu.Lock()
	defer s.outputSubsMu.Unlock()

	ch := make(chan AgentOutput, 100)
	s.outputSubs = append(s.outputSubs, ch)
	return ch
}

// UnsubscribeOutput removes a subscription channel
func (s *Spawner) UnsubscribeOutput(ch <-chan AgentOutput) {
	s.outputSubsMu.Lock()
	defer s.outputSubsMu.Unlock()

	for i, sub := range s.outputSubs {
		if sub == ch {
			close(sub)
			s.outputSubs = append(s.outputSubs[:i], s.outputSubs[i+1:]...)
			return
		}
	}
}

// broadcastOutput distributes output to all subscribers
func (s *Spawner) broadcastOutput() {
	for output := range s.outputChan {
		s.outputSubsMu.RLock()
		subs := make([]chan AgentOutput, len(s.outputSubs))
		copy(subs, s.outputSubs)
		s.outputSubsMu.RUnlock()

		for _, sub := range subs {
			select {
			case sub <- output:
			default:
				// Skip if subscriber is slow
			}
		}
	}
}

// emitOutput sends output to the broadcast channel
func (s *Spawner) emitOutput(agentID, agentName, line, stream string) {
	select {
	case s.outputChan <- AgentOutput{
		AgentID:   agentID,
		AgentName: agentName,
		Line:      line,
		Timestamp: time.Now(),
		Stream:    stream,
	}:
	default:
		// Drop if channel is full
	}
}

// teeOutput reads from a reader and writes to both a writer and the output channel
func (s *Spawner) teeOutput(r io.Reader, agentID, agentName, stream string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		s.emitOutput(agentID, agentName, line, stream)
	}
}
