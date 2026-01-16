package underboss

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/gabe/mob/internal/agent"
)

var (
	// ErrUnderbossNotRunning is returned when operations require a running Underboss
	ErrUnderbossNotRunning = errors.New("underboss is not running")

	// ErrUnderbossAlreadyRunning is returned when trying to start an already running Underboss
	ErrUnderbossAlreadyRunning = errors.New("underboss is already running")
)

// Underboss manages the persistent chief-of-staff Claude instance
type Underboss struct {
	agent   *agent.Agent
	spawner *agent.Spawner
	mobDir  string
	mu      sync.RWMutex
}

// New creates a new Underboss manager
func New(mobDir string, spawner *agent.Spawner) *Underboss {
	return &Underboss{
		mobDir:  mobDir,
		spawner: spawner,
	}
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

	// Spawn the underboss agent
	a, err := u.spawner.Spawn(agent.AgentTypeUnderboss, "underboss", "", workDir)
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
