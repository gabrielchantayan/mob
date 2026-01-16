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
