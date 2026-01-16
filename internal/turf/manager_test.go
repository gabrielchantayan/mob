package turf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTurfManager_Add(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-turf-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a fake project directory
	projectDir := filepath.Join(tmpDir, "my-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	turfsFile := filepath.Join(tmpDir, "turfs.toml")
	mgr, err := NewManager(turfsFile)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	err = mgr.Add(projectDir, "my-project", "main")
	if err != nil {
		t.Fatalf("failed to add turf: %v", err)
	}

	turfs := mgr.List()
	if len(turfs) != 1 {
		t.Errorf("expected 1 turf, got %d", len(turfs))
	}
	if turfs[0].Name != "my-project" {
		t.Errorf("expected name 'my-project', got '%s'", turfs[0].Name)
	}
}

func TestTurfManager_Remove(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-turf-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := filepath.Join(tmpDir, "my-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	turfsFile := filepath.Join(tmpDir, "turfs.toml")
	mgr, err := NewManager(turfsFile)
	if err != nil {
		t.Fatal(err)
	}

	mgr.Add(projectDir, "my-project", "main")
	err = mgr.Remove("my-project")
	if err != nil {
		t.Fatalf("failed to remove turf: %v", err)
	}

	turfs := mgr.List()
	if len(turfs) != 0 {
		t.Errorf("expected 0 turfs, got %d", len(turfs))
	}
}
