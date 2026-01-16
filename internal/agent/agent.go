package agent

import (
	"os/exec"
	"sync"
	"time"

	"github.com/gabe/mob/internal/ipc"
)

// AgentType represents the type of agent
type AgentType string

const (
	AgentTypeUnderboss AgentType = "underboss"
	AgentTypeSoldati   AgentType = "soldati"
	AgentTypeAssociate AgentType = "associate"
)

// Agent represents a running Claude Code instance
type Agent struct {
	ID        string
	Type      AgentType
	Name      string      // e.g., "vinnie" for soldati
	Turf      string      // project this agent works on
	Cmd       *exec.Cmd
	Client    *ipc.Client // JSON-RPC client
	StartedAt time.Time
	mu        sync.Mutex
}

// Send sends a message to the agent (notification, no response expected)
func (a *Agent) Send(method string, params interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.Client == nil {
		return ErrAgentNotConnected
	}
	return a.Client.Send(method, params)
}

// Call sends a request and waits for response
func (a *Agent) Call(method string, params interface{}) (*ipc.Response, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.Client == nil {
		return nil, ErrAgentNotConnected
	}
	return a.Client.Call(method, params)
}

// Wait waits for the agent process to complete
func (a *Agent) Wait() error {
	if a.Cmd == nil {
		return ErrAgentNotStarted
	}
	return a.Cmd.Wait()
}

// Kill terminates the agent
func (a *Agent) Kill() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.Cmd == nil || a.Cmd.Process == nil {
		return ErrAgentNotStarted
	}
	return a.Cmd.Process.Kill()
}

// IsRunning returns true if the agent is still running
func (a *Agent) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.Cmd == nil || a.Cmd.Process == nil {
		return false
	}

	// Check if process has exited by checking ProcessState
	// If ProcessState is nil, process hasn't exited yet
	if a.Cmd.ProcessState != nil {
		return false
	}

	// Double-check by trying to get exit status without blocking
	// This is a non-blocking check on Unix systems
	return a.Cmd.ProcessState == nil
}
