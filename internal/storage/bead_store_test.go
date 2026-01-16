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
