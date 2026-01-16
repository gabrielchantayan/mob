// Package nudge provides functionality to wake up stuck agents through
// escalating nudge levels: stdin, hook file updates, and process restart.
package nudge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gabe/mob/internal/agent"
	"github.com/gabe/mob/internal/hook"
)

// ErrAgentNotFound is returned when the specified agent cannot be found
var ErrAgentNotFound = errors.New("agent not found")

// NudgeLevel represents the escalation level for nudging stuck agents
type NudgeLevel int

const (
	// LevelStdin sends a newline to stdin to wake up the agent
	LevelStdin NudgeLevel = iota
	// LevelHook updates the hook file with a nudge signal
	LevelHook
	// LevelRestart kills and restarts the agent process
	LevelRestart
)

// String returns a human-readable name for the nudge level
func (l NudgeLevel) String() string {
	switch l {
	case LevelStdin:
		return "stdin"
	case LevelHook:
		return "hook"
	case LevelRestart:
		return "restart"
	default:
		return fmt.Sprintf("unknown(%d)", l)
	}
}

// NudgeEvent records a nudge attempt
type NudgeEvent struct {
	Level   NudgeLevel
	Time    time.Time
	Success bool
	Error   string
}

// agentEntry holds an agent and its stdin writer
type agentEntry struct {
	agent *agent.Agent
	stdin io.Writer
}

// Nudger handles nudging stuck agents through escalating intervention levels
type Nudger struct {
	spawner         *agent.Spawner
	hookBase        string // Base directory for hook files
	mu              sync.Mutex
	history         map[string][]NudgeEvent // Track nudge history per agent ID
	agents          map[string]*agentEntry  // Track agents by ID
	nameToID        map[string]string       // Map agent name to ID
	escalationDelay time.Duration           // Delay between escalation levels
}

// New creates a new Nudger
func New(spawner *agent.Spawner, hookBase string) *Nudger {
	return &Nudger{
		spawner:         spawner,
		hookBase:        hookBase,
		history:         make(map[string][]NudgeEvent),
		agents:          make(map[string]*agentEntry),
		nameToID:        make(map[string]string),
		escalationDelay: 30 * time.Second, // Default delay between escalation levels
	}
}

// SetEscalationDelay sets the delay between escalation levels (useful for testing)
func (n *Nudger) SetEscalationDelay(d time.Duration) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.escalationDelay = d
}

// RegisterAgent registers an agent with the nudger for tracking
// This allows the nudger to send stdin nudges to the agent
func (n *Nudger) RegisterAgent(a *agent.Agent, stdin io.Writer) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.agents[a.ID] = &agentEntry{
		agent: a,
		stdin: stdin,
	}
	if a.Name != "" {
		n.nameToID[a.Name] = a.ID
	}
}

// UnregisterAgent removes an agent from tracking
func (n *Nudger) UnregisterAgent(agentID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if entry, ok := n.agents[agentID]; ok {
		if entry.agent.Name != "" {
			delete(n.nameToID, entry.agent.Name)
		}
		delete(n.agents, agentID)
	}
}

// GetByName returns an agent by name
func (n *Nudger) GetByName(name string) (*agent.Agent, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	id, ok := n.nameToID[name]
	if !ok {
		return nil, false
	}
	entry, ok := n.agents[id]
	if !ok {
		return nil, false
	}
	return entry.agent, true
}

// ListAgents returns all registered agents
func (n *Nudger) ListAgents() []*agent.Agent {
	n.mu.Lock()
	defer n.mu.Unlock()

	agents := make([]*agent.Agent, 0, len(n.agents))
	for _, entry := range n.agents {
		agents = append(agents, entry.agent)
	}
	return agents
}

// Nudge attempts to wake up a stuck agent at the specified level
// - Level 0 (LevelStdin): Send newline to stdin
// - Level 1 (LevelHook): Update hook file with nudge message
// - Level 2 (LevelRestart): Kill agent, restart with resume flag
func (n *Nudger) Nudge(agentID string, level NudgeLevel) error {
	n.mu.Lock()
	entry, ok := n.agents[agentID]
	n.mu.Unlock()

	if !ok {
		return ErrAgentNotFound
	}

	event := NudgeEvent{
		Level: level,
		Time:  time.Now(),
	}

	var err error
	switch level {
	case LevelStdin:
		err = n.nudgeStdin(entry)
	case LevelHook:
		err = n.nudgeHook(entry.agent)
	case LevelRestart:
		err = n.nudgeRestart(entry.agent)
	default:
		err = fmt.Errorf("unknown nudge level: %d", level)
	}

	if err != nil {
		event.Success = false
		event.Error = err.Error()
	} else {
		event.Success = true
	}

	n.recordEvent(agentID, event)
	return err
}

// NudgeByName nudges an agent by name instead of ID
func (n *Nudger) NudgeByName(name string, level NudgeLevel) error {
	n.mu.Lock()
	id, ok := n.nameToID[name]
	n.mu.Unlock()

	if !ok {
		return ErrAgentNotFound
	}
	return n.Nudge(id, level)
}

// NudgeEscalating attempts levels 0, 1, 2 with delays between each
// Returns the first success or the final error
func (n *Nudger) NudgeEscalating(ctx context.Context, agentID string) error {
	n.mu.Lock()
	delay := n.escalationDelay
	n.mu.Unlock()

	levels := []NudgeLevel{LevelStdin, LevelHook, LevelRestart}

	var lastErr error
	for i, level := range levels {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := n.Nudge(agentID, level)
		if err == nil {
			// Success at this level
			return nil
		}
		lastErr = err

		// Don't wait after the last level
		if i < len(levels)-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Continue to next level
			}
		}
	}

	return lastErr
}

// History returns the nudge history for an agent
func (n *Nudger) History(agentID string) []NudgeEvent {
	n.mu.Lock()
	defer n.mu.Unlock()

	events := n.history[agentID]
	if events == nil {
		return nil
	}
	// Return a copy to avoid race conditions
	result := make([]NudgeEvent, len(events))
	copy(result, events)
	return result
}

// nudgeStdin sends a newline to the agent's stdin to wake it up
func (n *Nudger) nudgeStdin(entry *agentEntry) error {
	if entry.stdin == nil {
		return fmt.Errorf("no stdin available for agent %s", entry.agent.ID)
	}

	_, err := entry.stdin.Write([]byte("\n"))
	if err != nil {
		return fmt.Errorf("failed to write to stdin: %w", err)
	}
	return nil
}

// nudgeHook updates the hook file with a nudge signal
func (n *Nudger) nudgeHook(a *agent.Agent) error {
	if a.Name == "" {
		return fmt.Errorf("agent has no name, cannot create hook")
	}

	hookMgr, err := hook.NewManager(n.hookBase, a.Name)
	if err != nil {
		return fmt.Errorf("failed to create hook manager: %w", err)
	}

	nudgeHook := &hook.Hook{
		Type:      hook.HookTypeNudge,
		Message:   "Wake up - nudge signal",
		Timestamp: time.Now(),
	}

	if err := hookMgr.Write(nudgeHook); err != nil {
		return fmt.Errorf("failed to write nudge hook: %w", err)
	}

	return nil
}

// nudgeRestart kills and restarts the agent
// Note: Full restart with session resume (Seance) would require additional implementation
func (n *Nudger) nudgeRestart(a *agent.Agent) error {
	// Kill the agent
	if err := a.Kill(); err != nil {
		return fmt.Errorf("failed to kill agent: %w", err)
	}

	// Note: In a full implementation, this would use Seance to restart
	// the agent and resume from the previous session. For now, we just
	// return success after killing - the caller should handle respawn
	// using the spawner with appropriate resume flags.

	return nil
}

// recordEvent adds a nudge event to the history
func (n *Nudger) recordEvent(agentID string, event NudgeEvent) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.history[agentID] = append(n.history[agentID], event)
}

// ClearHistory clears the nudge history for an agent
func (n *Nudger) ClearHistory(agentID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	delete(n.history, agentID)
}

// AllHistory returns the complete nudge history for all agents
func (n *Nudger) AllHistory() map[string][]NudgeEvent {
	n.mu.Lock()
	defer n.mu.Unlock()

	result := make(map[string][]NudgeEvent)
	for id, events := range n.history {
		eventsCopy := make([]NudgeEvent, len(events))
		copy(eventsCopy, events)
		result[id] = eventsCopy
	}
	return result
}
