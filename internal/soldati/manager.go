package soldati

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gabe/mob/internal/models"
)

// Manager handles soldati storage operations
type Manager struct {
	dir string
}

// NewManager creates a new soldati manager, creating the storage directory if needed
func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create soldati directory: %w", err)
	}
	return &Manager{dir: dir}, nil
}

// Create creates a new soldati with the given name.
// If name is empty, a unique name is auto-generated.
func (m *Manager) Create(name string) (*models.Soldati, error) {
	if name == "" {
		used, err := m.listNames()
		if err != nil {
			return nil, err
		}
		name = GenerateUniqueName(used)
	}

	// Check if name already exists
	if _, err := m.Get(name); err == nil {
		return nil, fmt.Errorf("soldati %q already exists", name)
	}

	now := time.Now()
	soldati := &models.Soldati{
		Name:       name,
		CreatedAt:  now,
		LastActive: now,
		Stats:      models.SoldatiStats{},
	}

	if err := m.save(soldati); err != nil {
		return nil, err
	}

	return soldati, nil
}

// Get retrieves a soldati by name
func (m *Manager) Get(name string) (*models.Soldati, error) {
	filePath := filepath.Join(m.dir, name+".toml")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("soldati %q not found", name)
		}
		return nil, fmt.Errorf("failed to read soldati file: %w", err)
	}

	var soldati models.Soldati
	if _, err := toml.Decode(string(data), &soldati); err != nil {
		return nil, fmt.Errorf("failed to decode soldati file: %w", err)
	}

	return &soldati, nil
}

// List returns all soldati
func (m *Manager) List() ([]*models.Soldati, error) {
	names, err := m.listNames()
	if err != nil {
		return nil, err
	}

	soldati := make([]*models.Soldati, 0, len(names))
	for _, name := range names {
		s, err := m.Get(name)
		if err != nil {
			return nil, err
		}
		soldati = append(soldati, s)
	}

	return soldati, nil
}

// Update saves changes to an existing soldati
func (m *Manager) Update(soldati *models.Soldati) error {
	// Verify it exists first
	if _, err := m.Get(soldati.Name); err != nil {
		return err
	}
	return m.save(soldati)
}

// Delete removes a soldati by name
func (m *Manager) Delete(name string) error {
	filePath := filepath.Join(m.dir, name+".toml")

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("soldati %q not found", name)
		}
		return fmt.Errorf("failed to delete soldati: %w", err)
	}

	return nil
}

// save writes a soldati to its TOML file
func (m *Manager) save(soldati *models.Soldati) error {
	filePath := filepath.Join(m.dir, soldati.Name+".toml")

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create soldati file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(soldati); err != nil {
		return fmt.Errorf("failed to encode soldati: %w", err)
	}

	return nil
}

// listNames returns the names of all stored soldati
func (m *Manager) listNames() ([]string, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read soldati directory: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".toml") {
			names = append(names, strings.TrimSuffix(name, ".toml"))
		}
	}

	return names, nil
}
