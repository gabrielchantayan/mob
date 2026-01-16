// Package patrol provides background health monitoring for agents.
// It runs continuously to detect stuck or dead agents.
package patrol

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gabe/mob/internal/agent"
)

// ErrAgentNotFound is returned when an agent cannot be found
var ErrAgentNotFound = errors.New("agent not found")

// AgentStatus represents the health status of an agent
type AgentStatus struct {
	AgentID        string
	Name           string
	Type           agent.AgentType
	Status         string    // "healthy", "stuck", "dead"
	LastSeen       time.Time
	LastBeadUpdate time.Time
	Message        string
}

// AgentLister is an interface for listing and getting agents
type AgentLister interface {
	List() []*agent.Agent
	Get(id string) (*agent.Agent, bool)
}

// RunningChecker is an interface for checking if an agent is running
// This allows for dependency injection in tests
type RunningChecker interface {
	IsRunning(agentID string) bool
}

// defaultRunningChecker uses the agent's built-in IsRunning method
type defaultRunningChecker struct {
	spawner AgentLister
}

func (d *defaultRunningChecker) IsRunning(agentID string) bool {
	a, ok := d.spawner.Get(agentID)
	if !ok {
		return false
	}
	return a.IsRunning()
}

// Patrol manages background health monitoring
type Patrol struct {
	spawner        AgentLister
	runningChecker RunningChecker
	interval       time.Duration // Default 2 minutes
	stuckTimeout   time.Duration // Default 10 minutes
	onStuck        func(status AgentStatus)
	onDead         func(status AgentStatus)
	mu             sync.RWMutex
	agentStatus    map[string]*AgentStatus
}

// Option functions for configuration
type Option func(*Patrol)

// WithInterval sets the patrol check interval
func WithInterval(d time.Duration) Option {
	return func(p *Patrol) {
		p.interval = d
	}
}

// WithStuckTimeout sets the duration after which an agent is considered stuck
func WithStuckTimeout(d time.Duration) Option {
	return func(p *Patrol) {
		p.stuckTimeout = d
	}
}

// WithOnStuck sets the callback for when an agent is detected as stuck
func WithOnStuck(fn func(AgentStatus)) Option {
	return func(p *Patrol) {
		p.onStuck = fn
	}
}

// WithOnDead sets the callback for when an agent is detected as dead
func WithOnDead(fn func(AgentStatus)) Option {
	return func(p *Patrol) {
		p.onDead = fn
	}
}

// WithRunningChecker sets a custom running checker (useful for testing)
func WithRunningChecker(rc RunningChecker) Option {
	return func(p *Patrol) {
		p.runningChecker = rc
	}
}

// New creates a new patrol instance
func New(spawner AgentLister, opts ...Option) *Patrol {
	p := &Patrol{
		spawner:      spawner,
		interval:     2 * time.Minute,
		stuckTimeout: 10 * time.Minute,
		agentStatus:  make(map[string]*AgentStatus),
	}

	for _, opt := range opts {
		opt(p)
	}

	// Set default running checker if not provided
	if p.runningChecker == nil {
		p.runningChecker = &defaultRunningChecker{spawner: spawner}
	}

	return p
}

// Start begins the patrol loop
// - Runs ticker at interval
// - Calls checkAll() each tick
// - Stops when context is cancelled
func (p *Patrol) Start(ctx context.Context) error {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Do an initial check immediately
	p.checkAll()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			p.checkAll()
		}
	}
}

// checkAll checks all agents
// - For each agent in spawner.List()
// - Check if process is running
// - Check last activity time
// - Update status
// - Call onStuck/onDead callbacks if needed
func (p *Patrol) checkAll() {
	agents := p.spawner.List()
	now := time.Now()

	for _, a := range agents {
		p.mu.Lock()

		// Get or create status
		status, exists := p.agentStatus[a.ID]
		if !exists {
			status = &AgentStatus{
				AgentID:        a.ID,
				Name:           a.Name,
				Type:           a.Type,
				Status:         "healthy",
				LastSeen:       now,
				LastBeadUpdate: a.StartedAt,
			}
			p.agentStatus[a.ID] = status
		}

		previousStatus := status.Status

		// Check if agent is running
		isRunning := p.runningChecker.IsRunning(a.ID)

		if !isRunning {
			// Agent process is not running - it's dead
			status.Status = "dead"
			status.Message = "agent process is not running"
			status.LastSeen = now

			p.mu.Unlock()

			// Only call callback if status changed to dead
			if previousStatus != "dead" && p.onDead != nil {
				p.onDead(*status)
			}
			continue
		}

		// Agent is running, check if it's stuck
		// An agent is stuck if it hasn't updated beads in stuckTimeout
		timeSinceBeadUpdate := now.Sub(status.LastBeadUpdate)

		if timeSinceBeadUpdate > p.stuckTimeout {
			status.Status = "stuck"
			status.Message = "no bead updates for " + timeSinceBeadUpdate.Round(time.Second).String()
			status.LastSeen = now

			p.mu.Unlock()

			// Only call callback if status changed to stuck
			if previousStatus != "stuck" && p.onStuck != nil {
				p.onStuck(*status)
			}
			continue
		}

		// Agent is healthy
		status.Status = "healthy"
		status.Message = ""
		status.LastSeen = now
		p.mu.Unlock()
	}
}

// Check checks a single agent
func (p *Patrol) Check(agentID string) (*AgentStatus, error) {
	a, ok := p.spawner.Get(agentID)
	if !ok {
		return nil, ErrAgentNotFound
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()

	// Get or create status
	status, exists := p.agentStatus[agentID]
	if !exists {
		status = &AgentStatus{
			AgentID:        a.ID,
			Name:           a.Name,
			Type:           a.Type,
			Status:         "healthy",
			LastSeen:       now,
			LastBeadUpdate: a.StartedAt,
		}
		p.agentStatus[agentID] = status
	}

	// Check if agent is running
	isRunning := p.runningChecker.IsRunning(agentID)

	if !isRunning {
		status.Status = "dead"
		status.Message = "agent process is not running"
		status.LastSeen = now
		return status, nil
	}

	// Check if stuck
	timeSinceBeadUpdate := now.Sub(status.LastBeadUpdate)
	if timeSinceBeadUpdate > p.stuckTimeout {
		status.Status = "stuck"
		status.Message = "no bead updates for " + timeSinceBeadUpdate.Round(time.Second).String()
		status.LastSeen = now
		return status, nil
	}

	status.Status = "healthy"
	status.Message = ""
	status.LastSeen = now
	return status, nil
}

// Status returns all agent statuses
func (p *Patrol) Status() []*AgentStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	statuses := make([]*AgentStatus, 0, len(p.agentStatus))
	for _, s := range p.agentStatus {
		// Make a copy to avoid race conditions
		statusCopy := *s
		statuses = append(statuses, &statusCopy)
	}
	return statuses
}

// UpdateBeadTime updates the last bead update time for an agent
// This should be called whenever an agent produces a bead
func (p *Patrol) UpdateBeadTime(agentID string, t time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	status, exists := p.agentStatus[agentID]
	if !exists {
		// Create a new status entry
		status = &AgentStatus{
			AgentID:        agentID,
			Status:         "healthy",
			LastSeen:       t,
			LastBeadUpdate: t,
		}
		p.agentStatus[agentID] = status
	} else {
		status.LastBeadUpdate = t
	}
}
