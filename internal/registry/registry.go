package registry

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var (
	// ErrAgentNotFound is returned when an agent is not in the registry
	ErrAgentNotFound = errors.New("agent not found in registry")
)

// AgentRecord represents a tracked agent in the registry
type AgentRecord struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // underboss, soldati, associate
	Name      string    `json:"name"`
	Turf      string    `json:"turf"`
	SessionID string    `json:"session_id,omitempty"`
	Status    string    `json:"status"` // active, idle, stuck, dead
	Task      string    `json:"task,omitempty"`
	StartedAt time.Time `json:"started_at"`
	LastPing  time.Time `json:"last_ping"`
}

// Registry manages persistent agent state shared across processes
type Registry struct {
	filepath string
	mu       sync.RWMutex
}

// registryData is the on-disk format
type registryData struct {
	Agents map[string]*AgentRecord `json:"agents"`
}

// New creates a new registry at the specified file path
func New(path string) *Registry {
	return &Registry{
		filepath: path,
	}
}

// DefaultPath returns the default registry path for a mob directory
func DefaultPath(mobDir string) string {
	return filepath.Join(mobDir, ".mob", "agents.json")
}

// load reads the registry from disk (must hold lock)
func (r *Registry) load() (*registryData, error) {
	data := &registryData{
		Agents: make(map[string]*AgentRecord),
	}

	content, err := os.ReadFile(r.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil // Empty registry
		}
		return nil, err
	}

	if len(content) == 0 {
		return data, nil
	}

	if err := json.Unmarshal(content, data); err != nil {
		return nil, err
	}

	if data.Agents == nil {
		data.Agents = make(map[string]*AgentRecord)
	}

	return data, nil
}

// save writes the registry to disk (must hold lock)
func (r *Registry) save(data *registryData) error {
	// Ensure directory exists
	dir := filepath.Dir(r.filepath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// Write atomically via temp file
	tmpFile := r.filepath + ".tmp"
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, r.filepath)
}

// withFileLock executes a function with an exclusive file lock
func (r *Registry) withFileLock(fn func() error) error {
	// Ensure directory exists for lock file
	dir := filepath.Dir(r.filepath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	lockFile := r.filepath + ".lock"
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Acquire exclusive lock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	return fn()
}

// Register adds or updates an agent in the registry
func (r *Registry) Register(agent *AgentRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.withFileLock(func() error {
		data, err := r.load()
		if err != nil {
			return err
		}

		agent.LastPing = time.Now()
		data.Agents[agent.ID] = agent

		return r.save(data)
	})
}

// Unregister removes an agent from the registry
func (r *Registry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.withFileLock(func() error {
		data, err := r.load()
		if err != nil {
			return err
		}

		if _, ok := data.Agents[id]; !ok {
			return ErrAgentNotFound
		}

		delete(data.Agents, id)
		return r.save(data)
	})
}

// Get retrieves an agent by ID
func (r *Registry) Get(id string) (*AgentRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result *AgentRecord
	err := r.withFileLock(func() error {
		data, err := r.load()
		if err != nil {
			return err
		}

		agent, ok := data.Agents[id]
		if !ok {
			return ErrAgentNotFound
		}

		// Make a copy
		copy := *agent
		result = &copy
		return nil
	})

	return result, err
}

// GetByName retrieves an agent by name
func (r *Registry) GetByName(name string) (*AgentRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result *AgentRecord
	err := r.withFileLock(func() error {
		data, err := r.load()
		if err != nil {
			return err
		}

		for _, agent := range data.Agents {
			if agent.Name == name {
				copy := *agent
				result = &copy
				return nil
			}
		}

		return ErrAgentNotFound
	})

	return result, err
}

// List returns all agents in the registry
func (r *Registry) List() ([]*AgentRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*AgentRecord
	err := r.withFileLock(func() error {
		data, err := r.load()
		if err != nil {
			return err
		}

		result = make([]*AgentRecord, 0, len(data.Agents))
		for _, agent := range data.Agents {
			copy := *agent
			result = append(result, &copy)
		}
		return nil
	})

	return result, err
}

// ListByType returns agents of a specific type
func (r *Registry) ListByType(agentType string) ([]*AgentRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*AgentRecord
	err := r.withFileLock(func() error {
		data, err := r.load()
		if err != nil {
			return err
		}

		result = make([]*AgentRecord, 0)
		for _, agent := range data.Agents {
			if agent.Type == agentType {
				copy := *agent
				result = append(result, &copy)
			}
		}
		return nil
	})

	return result, err
}

// UpdateStatus updates an agent's status
func (r *Registry) UpdateStatus(id, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.withFileLock(func() error {
		data, err := r.load()
		if err != nil {
			return err
		}

		agent, ok := data.Agents[id]
		if !ok {
			return ErrAgentNotFound
		}

		agent.Status = status
		agent.LastPing = time.Now()
		return r.save(data)
	})
}

// UpdateTask updates an agent's current task
func (r *Registry) UpdateTask(id, task string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.withFileLock(func() error {
		data, err := r.load()
		if err != nil {
			return err
		}

		agent, ok := data.Agents[id]
		if !ok {
			return ErrAgentNotFound
		}

		agent.Task = task
		agent.LastPing = time.Now()
		return r.save(data)
	})
}

// Ping updates an agent's last ping time
func (r *Registry) Ping(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.withFileLock(func() error {
		data, err := r.load()
		if err != nil {
			return err
		}

		agent, ok := data.Agents[id]
		if !ok {
			return ErrAgentNotFound
		}

		agent.LastPing = time.Now()
		return r.save(data)
	})
}

// Clear removes all agents from the registry
func (r *Registry) Clear() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.withFileLock(func() error {
		data := &registryData{
			Agents: make(map[string]*AgentRecord),
		}
		return r.save(data)
	})
}
