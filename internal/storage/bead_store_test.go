package storage

import (
	"os"
	"testing"
	"time"

	"github.com/gabe/mob/internal/models"
)

func TestBeadStore_Create(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-bead-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewBeadStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	bead := &models.Bead{
		Title:       "Test task",
		Description: "A test task",
		Status:      models.BeadStatusOpen,
		Priority:    1,
		Type:        models.BeadTypeTask,
		Turf:        "test-project",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	created, err := store.Create(bead)
	if err != nil {
		t.Fatalf("failed to create bead: %v", err)
	}

	if created.ID == "" {
		t.Error("expected bead to have ID")
	}
	if created.Title != "Test task" {
		t.Errorf("expected title 'Test task', got '%s'", created.Title)
	}
}

func TestBeadStore_Get(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-bead-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewBeadStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a bead
	bead := &models.Bead{
		Title:     "Test task",
		Status:    models.BeadStatusOpen,
		Type:      models.BeadTypeTask,
		Turf:      "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	created, err := store.Create(bead)
	if err != nil {
		t.Fatal(err)
	}

	// Test Get with valid ID
	retrieved, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("failed to get bead: %v", err)
	}
	if retrieved.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, retrieved.ID)
	}
	if retrieved.Title != "Test task" {
		t.Errorf("expected title 'Test task', got '%s'", retrieved.Title)
	}

	// Test Get with non-existent ID
	_, err = store.Get("nonexistent-id")
	if err == nil {
		t.Error("expected error for non-existent ID, got nil")
	}
}

func TestBeadStore_List(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-bead-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewBeadStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a few beads
	for i := 0; i < 3; i++ {
		bead := &models.Bead{
			Title:     "Task",
			Status:    models.BeadStatusOpen,
			Type:      models.BeadTypeTask,
			Turf:      "test",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if _, err := store.Create(bead); err != nil {
			t.Fatal(err)
		}
	}

	beads, err := store.List(BeadFilter{})
	if err != nil {
		t.Fatalf("failed to list beads: %v", err)
	}

	if len(beads) != 3 {
		t.Errorf("expected 3 beads, got %d", len(beads))
	}
}

func TestBeadStore_List_Filters(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-bead-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewBeadStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create beads with different attributes
	beadsData := []struct {
		title    string
		status   models.BeadStatus
		turf     string
		beadType models.BeadType
		assignee string
	}{
		{"Task 1", models.BeadStatusOpen, "frontend", models.BeadTypeTask, "alice"},
		{"Task 2", models.BeadStatusInProgress, "backend", models.BeadTypeTask, "bob"},
		{"Bug 1", models.BeadStatusOpen, "frontend", models.BeadTypeBug, "alice"},
		{"Task 3", models.BeadStatusClosed, "backend", models.BeadTypeTask, "charlie"},
		{"Bug 2", models.BeadStatusInProgress, "frontend", models.BeadTypeBug, "bob"},
	}

	for _, bd := range beadsData {
		bead := &models.Bead{
			Title:     bd.title,
			Status:    bd.status,
			Turf:      bd.turf,
			Type:      bd.beadType,
			Assignee:  bd.assignee,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if _, err := store.Create(bead); err != nil {
			t.Fatal(err)
		}
	}

	// Test filter by Status
	t.Run("filter by status", func(t *testing.T) {
		beads, err := store.List(BeadFilter{Status: models.BeadStatusOpen})
		if err != nil {
			t.Fatalf("failed to list beads: %v", err)
		}
		if len(beads) != 2 {
			t.Errorf("expected 2 open beads, got %d", len(beads))
		}
		for _, b := range beads {
			if b.Status != models.BeadStatusOpen {
				t.Errorf("expected status 'open', got '%s'", b.Status)
			}
		}
	})

	// Test filter by Turf
	t.Run("filter by turf", func(t *testing.T) {
		beads, err := store.List(BeadFilter{Turf: "frontend"})
		if err != nil {
			t.Fatalf("failed to list beads: %v", err)
		}
		if len(beads) != 3 {
			t.Errorf("expected 3 frontend beads, got %d", len(beads))
		}
		for _, b := range beads {
			if b.Turf != "frontend" {
				t.Errorf("expected turf 'frontend', got '%s'", b.Turf)
			}
		}
	})

	// Test filter by Type
	t.Run("filter by type", func(t *testing.T) {
		beads, err := store.List(BeadFilter{Type: models.BeadTypeBug})
		if err != nil {
			t.Fatalf("failed to list beads: %v", err)
		}
		if len(beads) != 2 {
			t.Errorf("expected 2 bug beads, got %d", len(beads))
		}
		for _, b := range beads {
			if b.Type != models.BeadTypeBug {
				t.Errorf("expected type 'bug', got '%s'", b.Type)
			}
		}
	})

	// Test filter by Assignee
	t.Run("filter by assignee", func(t *testing.T) {
		beads, err := store.List(BeadFilter{Assignee: "alice"})
		if err != nil {
			t.Fatalf("failed to list beads: %v", err)
		}
		if len(beads) != 2 {
			t.Errorf("expected 2 beads assigned to alice, got %d", len(beads))
		}
		for _, b := range beads {
			if b.Assignee != "alice" {
				t.Errorf("expected assignee 'alice', got '%s'", b.Assignee)
			}
		}
	})

	// Test combined filters
	t.Run("combined filters", func(t *testing.T) {
		beads, err := store.List(BeadFilter{
			Status: models.BeadStatusOpen,
			Turf:   "frontend",
		})
		if err != nil {
			t.Fatalf("failed to list beads: %v", err)
		}
		if len(beads) != 2 {
			t.Errorf("expected 2 open frontend beads, got %d", len(beads))
		}
		for _, b := range beads {
			if b.Status != models.BeadStatusOpen || b.Turf != "frontend" {
				t.Errorf("bead does not match filter: status=%s, turf=%s", b.Status, b.Turf)
			}
		}
	})

	// Test filter with no matches
	t.Run("filter with no matches", func(t *testing.T) {
		beads, err := store.List(BeadFilter{
			Status: models.BeadStatusClosed,
			Turf:   "frontend",
		})
		if err != nil {
			t.Fatalf("failed to list beads: %v", err)
		}
		if len(beads) != 0 {
			t.Errorf("expected 0 beads, got %d", len(beads))
		}
	})
}

func TestBeadStore_Update(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-bead-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewBeadStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	bead := &models.Bead{
		Title:     "Original",
		Status:    models.BeadStatusOpen,
		Type:      models.BeadTypeTask,
		Turf:      "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	created, err := store.Create(bead)
	if err != nil {
		t.Fatal(err)
	}

	created.Title = "Updated"
	created.Status = models.BeadStatusInProgress
	updated, err := store.Update(created)
	if err != nil {
		t.Fatalf("failed to update bead: %v", err)
	}

	if updated.Title != "Updated" {
		t.Errorf("expected title 'Updated', got '%s'", updated.Title)
	}
	if updated.Status != models.BeadStatusInProgress {
		t.Errorf("expected status 'in_progress', got '%s'", updated.Status)
	}
}

func TestBeadStore_ListReady(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-bead-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewBeadStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create test beads
	// Bead 1: Closed (blocker for bead 2)
	bead1 := &models.Bead{
		Title:     "Completed task",
		Status:    models.BeadStatusClosed,
		Priority:  2,
		Type:      models.BeadTypeTask,
		Turf:      "frontend",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	bead1, err = store.Create(bead1)
	if err != nil {
		t.Fatal(err)
	}

	// Bead 2: Open with closed blocker (should be ready)
	bead2 := &models.Bead{
		Title:     "Task blocked by completed",
		Status:    models.BeadStatusOpen,
		Priority:  1,
		Type:      models.BeadTypeTask,
		Turf:      "frontend",
		Blocks:    []string{bead1.ID},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	bead2, err = store.Create(bead2)
	if err != nil {
		t.Fatal(err)
	}

	// Bead 3: Open with no blockers (should be ready, highest priority)
	bead3 := &models.Bead{
		Title:     "Urgent task",
		Status:    models.BeadStatusOpen,
		Priority:  0,
		Type:      models.BeadTypeTask,
		Turf:      "frontend",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	bead3, err = store.Create(bead3)
	if err != nil {
		t.Fatal(err)
	}

	// Bead 4: Open (blocker for bead 5)
	bead4 := &models.Bead{
		Title:     "Open blocker",
		Status:    models.BeadStatusOpen,
		Priority:  2,
		Type:      models.BeadTypeTask,
		Turf:      "backend",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	bead4, err = store.Create(bead4)
	if err != nil {
		t.Fatal(err)
	}

	// Bead 5: Open with open blocker (should NOT be ready)
	bead5 := &models.Bead{
		Title:     "Task blocked by open",
		Status:    models.BeadStatusOpen,
		Priority:  3,
		Type:      models.BeadTypeTask,
		Turf:      "backend",
		Blocks:    []string{bead4.ID},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err = store.Create(bead5)
	if err != nil {
		t.Fatal(err)
	}

	// Bead 6: In progress (should NOT be ready)
	bead6 := &models.Bead{
		Title:     "In progress task",
		Status:    models.BeadStatusInProgress,
		Priority:  1,
		Type:      models.BeadTypeTask,
		Turf:      "frontend",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err = store.Create(bead6)
	if err != nil {
		t.Fatal(err)
	}

	// Test 1: List all ready beads (no turf filter)
	t.Run("all ready beads", func(t *testing.T) {
		ready, err := store.ListReady("")
		if err != nil {
			t.Fatalf("failed to list ready beads: %v", err)
		}

		// Should return: bead3 (priority 0), bead2 (priority 1), bead4 (priority 2)
		if len(ready) != 3 {
			t.Errorf("expected 3 ready beads, got %d", len(ready))
			for _, b := range ready {
				t.Logf("  - %s (priority %d, status %s)", b.Title, b.Priority, b.Status)
			}
		}

		// Check priority ordering
		if len(ready) >= 2 {
			if ready[0].Priority > ready[1].Priority {
				t.Errorf("beads not sorted by priority: %d > %d", ready[0].Priority, ready[1].Priority)
			}
		}

		// Check first bead is highest priority
		if len(ready) > 0 && ready[0].ID != bead3.ID {
			t.Errorf("expected first bead to be %s (priority 0), got %s (priority %d)",
				bead3.ID, ready[0].ID, ready[0].Priority)
		}
	})

	// Test 2: Filter by turf
	t.Run("filter by turf", func(t *testing.T) {
		ready, err := store.ListReady("frontend")
		if err != nil {
			t.Fatalf("failed to list ready beads: %v", err)
		}

		// Should return: bead3 (priority 0), bead2 (priority 1)
		if len(ready) != 2 {
			t.Errorf("expected 2 ready frontend beads, got %d", len(ready))
		}

		for _, b := range ready {
			if b.Turf != "frontend" {
				t.Errorf("expected turf 'frontend', got '%s'", b.Turf)
			}
		}
	})

	// Test 3: Verify blocked bead is not returned
	t.Run("blocked bead not ready", func(t *testing.T) {
		ready, err := store.ListReady("")
		if err != nil {
			t.Fatalf("failed to list ready beads: %v", err)
		}

		// bead5 should not be in the list (has open blocker bead4)
		for _, b := range ready {
			if b.Title == "Task blocked by open" {
				t.Error("bead with open blocker should not be ready")
			}
		}
	})

	// Test 4: Verify closed and in-progress beads not returned
	t.Run("non-open beads not ready", func(t *testing.T) {
		ready, err := store.ListReady("")
		if err != nil {
			t.Fatalf("failed to list ready beads: %v", err)
		}

		for _, b := range ready {
			if b.Status != models.BeadStatusOpen {
				t.Errorf("expected only open beads, got status '%s'", b.Status)
			}
		}
	})

	// Test 5: Empty turf returns no beads
	t.Run("nonexistent turf", func(t *testing.T) {
		ready, err := store.ListReady("nonexistent")
		if err != nil {
			t.Fatalf("failed to list ready beads: %v", err)
		}

		if len(ready) != 0 {
			t.Errorf("expected 0 beads for nonexistent turf, got %d", len(ready))
		}
	})
}
