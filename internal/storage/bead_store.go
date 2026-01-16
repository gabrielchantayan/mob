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

	// Add creation event to history
	createdEvent := models.BeadEvent{
		Type:      models.BeadEventTypeCreated,
		Actor:     bead.CreatedBy,
		Timestamp: bead.CreatedAt,
	}
	eventID, err := generateID()
	if err == nil {
		createdEvent.ID = eventID
	}

	if createdEvent.Actor == "" {
		createdEvent.Actor = "user"
	}

	bead.History = []models.BeadEvent{createdEvent}

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
// - Not blocked by any unclosed beads (no unclosed beads list this bead in their Blocks array)
// - Sorted by priority (0 = highest first)
func (s *BeadStore) ListReady(turf string) ([]*models.Bead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allBeads, err := s.readAllBeads()
	if err != nil {
		return nil, err
	}

	// Build map of beads that are blocked by unclosed beads
	// If bead A has Blocks: ["bd-xyz"], then bd-xyz cannot start until A is closed
	blockedBeads := make(map[string]bool)
	for _, b := range allBeads {
		if b.Status != models.BeadStatusClosed {
			for _, blockedID := range b.Blocks {
				blockedBeads[blockedID] = true
			}
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

		// Skip if this bead is blocked by any unclosed beads
		if blockedBeads[b.ID] {
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

// AddEvent adds a historical event to a bead
func (s *BeadStore) AddEvent(beadID string, event models.BeadEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	beads, err := s.readAllBeads()
	if err != nil {
		return err
	}

	found := false
	for i, b := range beads {
		if b.ID == beadID {
			// Generate event ID if not provided
			if event.ID == "" {
				eventID, err := generateID()
				if err != nil {
					return fmt.Errorf("failed to generate event ID: %w", err)
				}
				event.ID = eventID
			}

			// Set timestamp if not provided
			if event.Timestamp.IsZero() {
				event.Timestamp = time.Now()
			}

			// Initialize history slice if nil
			if b.History == nil {
				b.History = []models.BeadEvent{}
			}

			// Add event to history
			b.History = append(b.History, event)
			b.UpdatedAt = time.Now()
			beads[i] = b
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("bead not found: %s", beadID)
	}

	return s.writeAllBeads(beads)
}

// AddComment adds a comment event to a bead's history
func (s *BeadStore) AddComment(beadID, actor, comment string) error {
	event := models.BeadEvent{
		Type:    models.BeadEventTypeComment,
		Actor:   actor,
		Comment: comment,
	}
	return s.AddEvent(beadID, event)
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
	var oldBead *models.Bead
	for i, b := range beads {
		if b.ID == bead.ID {
			oldBead = b
			bead.UpdatedAt = time.Now()

			// Auto-record status changes
			if oldBead.Status != bead.Status {
				event := models.BeadEvent{
					Type:      models.BeadEventTypeStatusChange,
					Actor:     "system",
					From:      string(oldBead.Status),
					To:        string(bead.Status),
					Timestamp: time.Now(),
				}

				// Generate event ID
				eventID, err := generateID()
				if err == nil {
					event.ID = eventID
				}

				// Initialize history if needed
				if bead.History == nil {
					bead.History = oldBead.History
				}
				if bead.History == nil {
					bead.History = []models.BeadEvent{}
				}

				// Add the status change event
				bead.History = append(bead.History, event)
			} else {
				// Preserve existing history if no status change
				if bead.History == nil {
					bead.History = oldBead.History
				}
			}

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

// DependencyTree represents a bead and its dependencies
type DependencyTree struct {
	Bead      *models.Bead
	BlockedBy []*DependencyTree
	Blocking  []*DependencyTree
}

// GetBlockedBy returns all beads that block the given bead
func (s *BeadStore) GetBlockedBy(beadID string) ([]*models.Bead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	targetBead, err := s.get(beadID)
	if err != nil {
		return nil, err
	}

	// If this bead has no blockers, return empty list
	if len(targetBead.Blocks) == 0 {
		return []*models.Bead{}, nil
	}

	allBeads, err := s.readAllBeads()
	if err != nil {
		return nil, err
	}

	// Find all beads that list this bead in their Blocks field
	blockers := []*models.Bead{}
	for _, bead := range allBeads {
		for _, blockedID := range bead.Blocks {
			if blockedID == beadID {
				blockers = append(blockers, bead)
				break
			}
		}
	}

	return blockers, nil
}

// GetBlocking returns all beads that this bead blocks
func (s *BeadStore) GetBlocking(beadID string) ([]*models.Bead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bead, err := s.get(beadID)
	if err != nil {
		return nil, err
	}

	// If this bead doesn't block anything, return empty list
	if len(bead.Blocks) == 0 {
		return []*models.Bead{}, nil
	}

	// Get all the beads this one blocks
	blocking := []*models.Bead{}
	for _, blockedID := range bead.Blocks {
		blocked, err := s.get(blockedID)
		if err != nil {
			// Skip if bead not found (could be deleted)
			continue
		}
		blocking = append(blocking, blocked)
	}

	return blocking, nil
}

// GetDependencyTree returns the full dependency tree for a bead
func (s *BeadStore) GetDependencyTree(beadID string) (*DependencyTree, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	visited := make(map[string]bool)
	return s.buildDependencyTree(beadID, visited)
}

// buildDependencyTree recursively builds the dependency tree
func (s *BeadStore) buildDependencyTree(beadID string, visited map[string]bool) (*DependencyTree, error) {
	// Prevent infinite loops
	if visited[beadID] {
		return nil, nil
	}
	visited[beadID] = true

	bead, err := s.get(beadID)
	if err != nil {
		return nil, err
	}

	tree := &DependencyTree{
		Bead:      bead,
		BlockedBy: []*DependencyTree{},
		Blocking:  []*DependencyTree{},
	}

	// Build blocked-by tree
	allBeads, err := s.readAllBeads()
	if err != nil {
		return nil, err
	}

	for _, b := range allBeads {
		for _, blockedID := range b.Blocks {
			if blockedID == beadID {
				subtree, err := s.buildDependencyTree(b.ID, visited)
				if err == nil && subtree != nil {
					tree.BlockedBy = append(tree.BlockedBy, subtree)
				}
			}
		}
	}

	// Build blocking tree
	for _, blockedID := range bead.Blocks {
		subtree, err := s.buildDependencyTree(blockedID, visited)
		if err == nil && subtree != nil {
			tree.Blocking = append(tree.Blocking, subtree)
		}
	}

	return tree, nil
}

// get is an internal method that doesn't acquire locks (caller must hold lock)
func (s *BeadStore) get(id string) (*models.Bead, error) {
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
