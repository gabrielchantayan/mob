package soldati

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gabe/mob/internal/models"
)

// ErrInvalidName is returned when a soldati name contains invalid characters
var ErrInvalidName = errors.New("invalid soldati name")

// validNameRegex matches names that contain only alphanumeric characters, hyphens, and underscores
var validNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// maxNameLength is the maximum allowed length for a soldati name
const maxNameLength = 64

// validateName checks if the given name is valid for a soldati.
// Names must:
// - Not be empty
// - Not exceed 64 characters
// - Start with alphanumeric and contain only alphanumeric, hyphen, underscore
// - Not contain path traversal sequences
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name cannot be empty", ErrInvalidName)
	}

	if len(name) > maxNameLength {
		return fmt.Errorf("%w: name exceeds maximum length of %d characters", ErrInvalidName, maxNameLength)
	}

	// Check for path traversal attempts
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("%w: name contains invalid path characters", ErrInvalidName)
	}

	// Check for names starting with dot (hidden files)
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("%w: name cannot start with a dot", ErrInvalidName)
	}

	// Validate against regex (alphanumeric, hyphen, underscore, starting with alphanumeric)
	if !validNameRegex.MatchString(name) {
		return fmt.Errorf("%w: name must start with alphanumeric and contain only alphanumeric characters, hyphens, and underscores", ErrInvalidName)
	}

	return nil
}

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

	// Validate the name (auto-generated names should always be valid, but validate anyway)
	if err := validateName(name); err != nil {
		return nil, err
	}

	now := time.Now()
	soldati := &models.Soldati{
		Name:       name,
		CreatedAt:  now,
		LastActive: now,
		Stats:      models.SoldatiStats{},
	}

	// Use atomic create to avoid race conditions (O_CREATE|O_EXCL fails if file exists)
	if err := m.createNew(soldati); err != nil {
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

// createNew atomically creates a new soldati file, failing if it already exists.
// This avoids TOCTOU race conditions by using O_CREATE|O_EXCL flags.
func (m *Manager) createNew(soldati *models.Soldati) error {
	filePath := filepath.Join(m.dir, soldati.Name+".toml")

	// O_CREATE|O_EXCL ensures atomic create-if-not-exists
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("soldati %q already exists", soldati.Name)
		}
		return fmt.Errorf("failed to create soldati file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(soldati); err != nil {
		// Clean up the file if encoding fails
		os.Remove(filePath)
		return fmt.Errorf("failed to encode soldati: %w", err)
	}

	return nil
}

// save writes a soldati to its TOML file (overwrites existing)
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

// AssignTurf assigns a soldati to a specific turf
func (m *Manager) AssignTurf(name, turf string) error {
	soldati, err := m.Get(name)
	if err != nil {
		return err
	}

	// Check if already assigned
	for _, t := range soldati.Turfs {
		if t == turf {
			return nil // Already assigned, no-op
		}
	}

	// Add turf to list
	soldati.Turfs = append(soldati.Turfs, turf)

	// If this is the first turf, make it primary
	if soldati.PrimaryTurf == "" {
		soldati.PrimaryTurf = turf
	}

	return m.Update(soldati)
}

// UnassignTurf removes a turf assignment from a soldati
func (m *Manager) UnassignTurf(name, turf string) error {
	soldati, err := m.Get(name)
	if err != nil {
		return err
	}

	// Remove turf from list
	newTurfs := make([]string, 0, len(soldati.Turfs))
	for _, t := range soldati.Turfs {
		if t != turf {
			newTurfs = append(newTurfs, t)
		}
	}
	soldati.Turfs = newTurfs

	// Clear primary if it was the removed turf
	if soldati.PrimaryTurf == turf {
		if len(soldati.Turfs) > 0 {
			soldati.PrimaryTurf = soldati.Turfs[0]
		} else {
			soldati.PrimaryTurf = ""
		}
	}

	return m.Update(soldati)
}

// SetPrimaryTurf sets the primary turf for a soldati
func (m *Manager) SetPrimaryTurf(name, turf string) error {
	soldati, err := m.Get(name)
	if err != nil {
		return err
	}

	// Verify turf is assigned
	found := false
	for _, t := range soldati.Turfs {
		if t == turf {
			found = true
			break
		}
	}

	if !found && turf != "" {
		return fmt.Errorf("turf %q is not assigned to soldati %q", turf, name)
	}

	soldati.PrimaryTurf = turf
	return m.Update(soldati)
}

// ListByTurf returns all soldati assigned to a specific turf (or all turfs if empty)
func (m *Manager) ListByTurf(turf string) ([]*models.Soldati, error) {
	all, err := m.List()
	if err != nil {
		return nil, err
	}

	if turf == "" {
		return all, nil
	}

	result := make([]*models.Soldati, 0)
	for _, s := range all {
		// Empty turfs list means assigned to all turfs
		if len(s.Turfs) == 0 {
			result = append(result, s)
			continue
		}

		// Check if turf is in the list
		for _, t := range s.Turfs {
			if t == turf {
				result = append(result, s)
				break
			}
		}
	}

	return result, nil
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
