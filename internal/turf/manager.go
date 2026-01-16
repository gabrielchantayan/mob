package turf

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/gabe/mob/internal/models"
)

// Manager handles turf registration and lookup
type Manager struct {
	path   string
	config models.TurfsConfig
}

// NewManager creates a new turf manager
func NewManager(path string) (*Manager, error) {
	mgr := &Manager{path: path}

	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read turfs file: %w", err)
		}
		if _, err := toml.Decode(string(data), &mgr.config); err != nil {
			return nil, fmt.Errorf("failed to parse turfs file: %w", err)
		}
	}

	return mgr, nil
}

// Add registers a new turf
func (m *Manager) Add(path, name, mainBranch string) error {
	// Validate path exists
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check for duplicate
	for _, t := range m.config.Turfs {
		if t.Name == name {
			return fmt.Errorf("turf already exists: %s", name)
		}
		if t.Path == absPath {
			return fmt.Errorf("path already registered as turf: %s", t.Name)
		}
	}

	m.config.Turfs = append(m.config.Turfs, models.Turf{
		Name:       name,
		Path:       absPath,
		MainBranch: mainBranch,
	})

	return m.save()
}

// Remove unregisters a turf
func (m *Manager) Remove(name string) error {
	for i, t := range m.config.Turfs {
		if t.Name == name {
			m.config.Turfs = append(m.config.Turfs[:i], m.config.Turfs[i+1:]...)
			return m.save()
		}
	}
	return fmt.Errorf("turf not found: %s", name)
}

// List returns all registered turfs
func (m *Manager) List() []models.Turf {
	result := make([]models.Turf, len(m.config.Turfs))
	copy(result, m.config.Turfs)
	return result
}

// Get retrieves a turf by name
func (m *Manager) Get(name string) (*models.Turf, error) {
	for i := range m.config.Turfs {
		if m.config.Turfs[i].Name == name {
			return &m.config.Turfs[i], nil
		}
	}
	return nil, fmt.Errorf("turf not found: %s", name)
}

func (m *Manager) save() error {
	f, err := os.Create(m.path)
	if err != nil {
		return fmt.Errorf("failed to create turfs file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(m.config)
}
