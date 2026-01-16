package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// State represents the daemon's operational state
type State string

const (
	StateIdle    State = "idle"
	StateRunning State = "running"
	StatePaused  State = "paused"
)

// Daemon manages the mob orchestration
type Daemon struct {
	pidFile   string
	stateFile string
	state     State
	ctx       context.Context
	cancel    context.CancelFunc
}

// New creates a new daemon instance
func New(mobDir string) *Daemon {
	return &Daemon{
		pidFile:   filepath.Join(mobDir, ".mob", "daemon.pid"),
		stateFile: filepath.Join(mobDir, ".mob", "daemon.state"),
		state:     StateIdle,
	}
}

// Start begins daemon operation
func (d *Daemon) Start() error {
	// Create .mob directory if it doesn't exist
	mobDir := filepath.Dir(d.pidFile)
	if err := os.MkdirAll(mobDir, 0755); err != nil {
		return fmt.Errorf("failed to create .mob directory: %w", err)
	}

	// Check for existing daemon
	running, pid, err := CheckExistingDaemon(d.pidFile)
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("daemon already running (PID %d)", pid)
	}

	// Write our PID
	if err := WritePID(d.pidFile, os.Getpid()); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Set up context for graceful shutdown
	d.ctx, d.cancel = context.WithCancel(context.Background())
	d.state = StateRunning

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Mob daemon started")

	// Main loop
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return d.shutdown()
		case sig := <-sigChan:
			fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
			return d.shutdown()
		case <-ticker.C:
			d.patrol()
		}
	}
}

// Stop gracefully stops the daemon
func (d *Daemon) Stop() error {
	if d.cancel != nil {
		d.cancel()
	}
	return nil
}

// Status returns the current daemon status
func (d *Daemon) Status() (State, int, error) {
	running, pid, err := CheckExistingDaemon(d.pidFile)
	if err != nil {
		return "", 0, err
	}
	if !running {
		return StateIdle, 0, nil
	}
	return StateRunning, pid, nil
}

func (d *Daemon) shutdown() error {
	d.state = StateIdle
	RemovePID(d.pidFile)
	fmt.Println("Mob daemon stopped")
	return nil
}

func (d *Daemon) patrol() {
	// Placeholder for patrol loop logic
	// Will check agent health, handle stuck workers, etc.
}
