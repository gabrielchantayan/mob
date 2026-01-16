package storage

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/gabe/mob/internal/models"
)

// BeadStore manages JSONL-based bead storage
type BeadStore struct {
	dir      string
	openFile string
	mu       sync.RWMutex
}

// BeadFilter defines filtering options for listing beads
type BeadFilter struct {
	Status   models.BeadStatus
	Turf     string
	Assignee string
	Type     models.BeadType
}

// NewBeadStore creates a new bead store at the given directory
func NewBeadStore(dir string) (*BeadStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create bead directory: %w", err)
	}

	return &BeadStore{
		dir:      dir,
		openFile: filepath.Join(dir, "open.jsonl"),
	}, nil
}

// generateID creates a short random ID for beads
func generateID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}
	return "bd-" + hex.EncodeToString(b)[:4], nil
}

// Create adds a new bead to the store
func (s *BeadStore) Create(bead *models.Bead) (*models.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, err := generateID()
	if err != nil {
		return nil, err
	}
	bead.ID = id
	bead.CreatedAt = time.Now()
	bead.UpdatedAt = time.Now()
	bead.Branch = "mob/" + bead.ID

	return bead, s.appendBead(bead)
}

// List returns all beads matching the filter
func (s *BeadStore) List(filter BeadFilter) ([]*models.Bead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	beads, err := s.readAllBeads()
	if err != nil {
		return nil, err
	}

	// Apply filters
	var filtered []*models.Bead
	for _, bead := range beads {
		if filter.Status != "" && bead.Status != filter.Status {
			continue
		}
		if filter.Turf != "" && bead.Turf != filter.Turf {
			continue
		}
		if filter.Assignee != "" && bead.Assignee != filter.Assignee {
			continue
		}
		if filter.Type != "" && bead.Type != filter.Type {
			continue
		}
		filtered = append(filtered, bead)
	}

	return filtered, nil
}

// ListReady returns beads that are ready for assignment:
// - Status is "open"
// - All blocking beads (in Blocks array) are closed
// - Sorted by priority (0 = highest first)
func (s *BeadStore) ListReady(turf string) ([]*models.Bead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allBeads, err := s.readAllBeads()
	if err != nil {
		return nil, err
	}

	// Build map of closed bead IDs for blocker checking
	closedIDs := make(map[string]bool)
	for _, b := range allBeads {
		if b.Status == models.BeadStatusClosed {
			closedIDs[b.ID] = true
		}
	}

	var ready []*models.Bead
	for _, b := range allBeads {
		// Must be open
		if b.Status != models.BeadStatusOpen {
			continue
		}

		// Turf filter
		if turf != "" && b.Turf != turf {
			continue
		}

		// Check blockers - all must be closed
		allBlockersClosed := true
		for _, blockerID := range b.Blocks {
			if !closedIDs[blockerID] {
				allBlockersClosed = false
				break
			}
		}
		if !allBlockersClosed {
			continue
		}

		ready = append(ready, b)
	}

	// Sort by priority (0 = highest priority, should be first)
	sort.Slice(ready, func(i, j int) bool {
		return ready[i].Priority < ready[j].Priority
	})

	return ready, nil
}

// Get retrieves a bead by ID
func (s *BeadStore) Get(id string) (*models.Bead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	beads, err := s.readAllBeads()
	if err != nil {
		return nil, err
	}

	for _, bead := range beads {
		if bead.ID == id {
			return bead, nil
		}
	}

	return nil, fmt.Errorf("bead not found: %s", id)
}

// Update modifies an existing bead
func (s *BeadStore) Update(bead *models.Bead) (*models.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	beads, err := s.readAllBeads()
	if err != nil {
		return nil, err
	}

	found := false
	for i, b := range beads {
		if b.ID == bead.ID {
			bead.UpdatedAt = time.Now()
			beads[i] = bead
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("bead not found: %s", bead.ID)
	}

	return bead, s.writeAllBeads(beads)
}

func (s *BeadStore) appendBead(bead *models.Bead) error {
	f, err := os.OpenFile(s.openFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(bead)
	if err != nil {
		return err
	}

	_, err = f.Write(append(data, '\n'))
	return err
}

func (s *BeadStore) readAllBeads() ([]*models.Bead, error) {
	f, err := os.Open(s.openFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var beads []*models.Bead
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var bead models.Bead
		if err := json.Unmarshal(scanner.Bytes(), &bead); err != nil {
			continue // Skip malformed lines
		}
		beads = append(beads, &bead)
	}

	return beads, scanner.Err()
}

func (s *BeadStore) writeAllBeads(beads []*models.Bead) error {
	// Write to temp file first
	tmpFile := s.openFile + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		return err
	}

	for _, bead := range beads {
		data, err := json.Marshal(bead)
		if err != nil {
			f.Close()
			os.Remove(tmpFile)
			return err
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			f.Close()
			os.Remove(tmpFile)
			return err
		}
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpFile)
		return err
	}

	return os.Rename(tmpFile, s.openFile)
}
