package agent

import "errors"

var (
	// ErrAgentNotConnected is returned when the agent's IPC client is nil
	ErrAgentNotConnected = errors.New("agent not connected")

	// ErrAgentNotStarted is returned when the agent's process hasn't been started
	ErrAgentNotStarted = errors.New("agent not started")

	// ErrAgentNotFound is returned when an agent cannot be found by ID
	ErrAgentNotFound = errors.New("agent not found")
)
